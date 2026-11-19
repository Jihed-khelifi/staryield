-- 0032_trial_hold: the free-trial hold of revisions 6.0
-- (docs/platform-extensions.md section 26.1).
--
-- When a reading's next minute cannot be paid the reading is HELD for a short
-- time instead of ending: the advisor can keep writing, the client sees those
-- replies locked, and a top-up in time continues the reading and reveals them.
--
--   chat_sessions        status gains 'held'; hold_expires_at (when the open hold
--                        runs out); last_hold_id (the reading's newest hold, so a
--                        view never searches for it). end_reason is free text
--                        (no CHECK), so the new reason 'hold_expired' needs no
--                        constraint change.
--   chat_session_holds   one row per hold (a reading can be held more than once
--                        when after_paid is on).
--   chat_messages        held / hold_id: the advisor's replies written during a
--                        hold, hidden from the client until the hold resolves.
--
-- Cost on a large chat_messages: both new columns are metadata-only (constant
-- default / NULL default), the foreign key and the check are added NOT VALID
-- (every existing row satisfies them, so nothing scans the table) and no index is
-- built on it: a hold's messages are found through chat_messages_session_idx
-- (session_id, created_at), bounded by the hold's start.

-- A held reading still counts as open: the client and the advisor are busy, and
-- the conversation shows an open session. The partial indexes (and the two
-- uniqueness guarantees) are rebuilt with the new status.
ALTER TABLE chat_sessions DROP CONSTRAINT chat_sessions_status_check;
ALTER TABLE chat_sessions ADD CONSTRAINT chat_sessions_status_check CHECK (status IN (
    'requested', 'accepted', 'active', 'disconnected', 'held', 'ended',
    'settled', 'declined', 'expired', 'cancelled'));

DROP INDEX chat_sessions_one_open_per_client;
CREATE UNIQUE INDEX chat_sessions_one_open_per_client ON chat_sessions (client_id)
    WHERE status IN ('requested', 'accepted', 'active', 'disconnected', 'held');
DROP INDEX chat_sessions_one_open_per_psychic;
CREATE UNIQUE INDEX chat_sessions_one_open_per_psychic ON chat_sessions (psychic_id)
    WHERE status IN ('requested', 'accepted', 'active', 'disconnected', 'held');
DROP INDEX chat_sessions_open_conversation_idx;
CREATE INDEX chat_sessions_open_conversation_idx ON chat_sessions (conversation_id)
    WHERE status IN ('requested', 'accepted', 'active', 'disconnected', 'held');

ALTER TABLE chat_sessions ADD COLUMN hold_expires_at timestamptz;
ALTER TABLE chat_sessions ADD COLUMN last_hold_id uuid;
ALTER TABLE chat_sessions ADD CONSTRAINT chat_sessions_held_has_deadline
    CHECK (status <> 'held' OR hold_expires_at IS NOT NULL) NOT VALID;
-- The expiry job claims held readings whose deadline passed.
CREATE INDEX chat_sessions_held_idx ON chat_sessions (hold_expires_at) WHERE status = 'held';

CREATE TABLE chat_session_holds (
    id                         uuid PRIMARY KEY,
    session_id                 uuid        NOT NULL REFERENCES chat_sessions (id),
    conversation_id            uuid        NOT NULL REFERENCES conversations (id),
    client_id                  uuid        NOT NULL REFERENCES users (id),
    psychic_id                 uuid        NOT NULL REFERENCES psychics (user_id),
    reason                     text        NOT NULL CHECK (reason IN ('trial_ended', 'credits_exhausted')),
    status                     text        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'converted', 'expired', 'revealed')),
    started_at                 timestamptz NOT NULL,
    -- expires_at moves forward (at most to max_expires_at) while a payment the
    -- client started during the hold is still processing.
    expires_at                 timestamptz NOT NULL,
    max_expires_at             timestamptz NOT NULL,
    resolved_at                timestamptz,
    revealed_at                timestamptz,
    -- price_credits is what continuing or revealing costs the client (the
    -- reading's per-minute rate after a member discount); list_rate_credits is
    -- the advisor's rate, on which the reveal earning is computed.
    price_credits              bigint      NOT NULL CHECK (price_credits > 0),
    list_rate_credits          bigint      NOT NULL CHECK (list_rate_credits > 0),
    held_messages              integer     NOT NULL DEFAULT 0 CHECK (held_messages >= 0),
    -- converted_credits is what the top-up minute cost (0 when a free minute paid it).
    converted_credits          bigint      NOT NULL DEFAULT 0 CHECK (converted_credits >= 0),
    reveal_credits             bigint      NOT NULL DEFAULT 0 CHECK (reveal_credits >= 0),
    reveal_revenue_millicents  bigint      NOT NULL DEFAULT 0 CHECK (reveal_revenue_millicents >= 0),
    created_at                 timestamptz NOT NULL DEFAULT now(),
    updated_at                 timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chat_session_holds_expiry_order CHECK (expires_at >= started_at AND max_expires_at >= expires_at),
    CONSTRAINT chat_session_holds_resolved_when_closed CHECK ((status = 'open') = (resolved_at IS NULL)),
    CONSTRAINT chat_session_holds_revealed_consistent CHECK ((status = 'revealed') = (revealed_at IS NOT NULL))
);
CREATE INDEX chat_session_holds_session_idx ON chat_session_holds (session_id);
CREATE INDEX chat_session_holds_client_idx ON chat_session_holds (client_id, status);
CREATE INDEX chat_session_holds_open_expiry_idx ON chat_session_holds (expires_at) WHERE status = 'open';
-- A reading has at most one open hold at a time.
CREATE UNIQUE INDEX chat_session_holds_one_open_per_session ON chat_session_holds (session_id) WHERE status = 'open';
-- The 30-day statistics of the admin card.
CREATE INDEX chat_session_holds_started_idx ON chat_session_holds (started_at);

ALTER TABLE chat_sessions ADD CONSTRAINT chat_sessions_last_hold_fk
    FOREIGN KEY (last_hold_id) REFERENCES chat_session_holds (id) NOT VALID;

ALTER TABLE chat_messages ADD COLUMN held boolean NOT NULL DEFAULT false;
ALTER TABLE chat_messages ADD COLUMN hold_id uuid;
ALTER TABLE chat_messages ADD CONSTRAINT chat_messages_hold_fk
    FOREIGN KEY (hold_id) REFERENCES chat_session_holds (id) NOT VALID;
ALTER TABLE chat_messages ADD CONSTRAINT chat_messages_held_has_hold
    CHECK (NOT held OR hold_id IS NOT NULL) NOT VALID;
