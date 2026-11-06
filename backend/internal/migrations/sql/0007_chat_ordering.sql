-- 0007 chat ordering.
--
-- Every chat message gets a per-conversation sequence number (seq) assigned
-- under the conversation row lock, so seq order equals commit order and
-- clients can detect gaps. Read positions and unread counters move from
-- timestamps to sequence numbers, and the inbox gets keyset indexes on the
-- conversation's last activity.

-- conversations.last_message_id is a deferred foreign key: the backfill
-- UPDATEs below would leave deferred trigger events pending, and ALTER TABLE
-- refuses to run while any are. Check constraints per statement in this
-- migration's transaction instead.
SET CONSTRAINTS ALL IMMEDIATE;

-- Conversations: sequence head, read positions, unread counters, activity.
ALTER TABLE conversations
    ADD COLUMN last_seq          bigint      NOT NULL DEFAULT 0,
    ADD COLUMN client_read_seq   bigint      NOT NULL DEFAULT 0,
    ADD COLUMN psychic_read_seq  bigint      NOT NULL DEFAULT 0,
    ADD COLUMN client_unread     integer     NOT NULL DEFAULT 0,
    ADD COLUMN psychic_unread    integer     NOT NULL DEFAULT 0,
    ADD COLUMN last_activity_at  timestamptz;

-- Messages: sequence number and updated_at (messages are immutable, so
-- updated_at equals created_at; the column exists for the table convention).
ALTER TABLE chat_messages
    ADD COLUMN seq        bigint,
    ADD COLUMN updated_at timestamptz;

-- Single-pass backfill: number each conversation's messages in commit order.
UPDATE chat_messages m
SET seq = o.rn, updated_at = m.created_at
FROM (
    SELECT id, row_number() OVER (PARTITION BY conversation_id ORDER BY created_at, id) AS rn
    FROM chat_messages
) o
WHERE o.id = m.id;

ALTER TABLE chat_messages
    ALTER COLUMN seq SET NOT NULL,
    ALTER COLUMN updated_at SET NOT NULL,
    ALTER COLUMN updated_at SET DEFAULT now(),
    ADD CONSTRAINT chat_messages_seq_check CHECK (seq > 0);

CREATE UNIQUE INDEX chat_messages_conversation_seq_key ON chat_messages (conversation_id, seq);

-- Sequence heads.
UPDATE conversations c
SET last_seq = x.max_seq
FROM (SELECT conversation_id, max(seq) AS max_seq FROM chat_messages GROUP BY conversation_id) x
WHERE x.conversation_id = c.id;

-- Read positions: the last message at or before each read timestamp.
UPDATE conversations c
SET client_read_seq = COALESCE((
    SELECT max(m.seq) FROM chat_messages m
    WHERE m.conversation_id = c.id AND m.created_at <= c.client_last_read_at), 0)
WHERE c.client_last_read_at IS NOT NULL;

UPDATE conversations c
SET psychic_read_seq = COALESCE((
    SELECT max(m.seq) FROM chat_messages m
    WHERE m.conversation_id = c.id AND m.created_at <= c.psychic_last_read_at), 0)
WHERE c.psychic_last_read_at IS NOT NULL;

-- Unread counters: messages of every kind after the read position that the
-- participant did not send (system messages count for both sides).
UPDATE conversations c
SET client_unread = (
        SELECT count(*) FROM chat_messages m
        WHERE m.conversation_id = c.id AND m.seq > c.client_read_seq
          AND m.sender_id IS DISTINCT FROM c.client_id),
    psychic_unread = (
        SELECT count(*) FROM chat_messages m
        WHERE m.conversation_id = c.id AND m.seq > c.psychic_read_seq
          AND m.sender_id IS DISTINCT FROM c.psychic_id);

UPDATE conversations SET last_activity_at = COALESCE(last_message_at, created_at);

ALTER TABLE conversations
    ALTER COLUMN last_activity_at SET NOT NULL,
    ALTER COLUMN last_activity_at SET DEFAULT now(),
    ADD CONSTRAINT conversations_seq_check CHECK (
        last_seq >= 0 AND client_read_seq >= 0 AND psychic_read_seq >= 0
        AND client_read_seq <= last_seq AND psychic_read_seq <= last_seq),
    ADD CONSTRAINT conversations_unread_check CHECK (client_unread >= 0 AND psychic_unread >= 0);

-- Inbox keyset indexes replace the last_message_at ones (only the inbox
-- ordered by last_message_at).
DROP INDEX IF EXISTS conversations_psychic_idx;
DROP INDEX IF EXISTS conversations_client_idx;
CREATE INDEX conversations_client_activity_idx ON conversations (client_id, last_activity_at DESC, id DESC);
CREATE INDEX conversations_psychic_activity_idx ON conversations (psychic_id, last_activity_at DESC, id DESC);

-- Sessions: open session per conversation (inbox), and the advisor/client
-- pair history used by advisor statistics.
CREATE INDEX chat_sessions_open_conversation_idx ON chat_sessions (conversation_id)
    WHERE status IN ('requested', 'accepted', 'active', 'disconnected');
CREATE INDEX chat_sessions_pair_started_idx ON chat_sessions (psychic_id, client_id, started_at)
    INCLUDE (credits_charged, billed_minutes, free_minutes_used)
    WHERE started_at IS NOT NULL;
