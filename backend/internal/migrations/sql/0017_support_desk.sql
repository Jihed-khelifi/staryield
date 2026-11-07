-- 0017_support_desk: ticketing on top of the contact form (revisions 3.0, section 16).
--
-- Tickets gain a workflow (open / pending / resolved / closed), a priority, an
-- assignee, counters and timestamps for the queue, and a private-link token
-- for anonymous requesters. Replies and internal notes live in
-- support_messages (append-only: bodies are sealed and immutable; only the
-- e-mail delivery status can change).

-- ---------------------------------------------------------------------------
-- support_tickets
-- ---------------------------------------------------------------------------
ALTER TABLE support_tickets DROP CONSTRAINT support_tickets_status_check;
ALTER TABLE support_tickets ADD CONSTRAINT support_tickets_status_check
    CHECK (status IN ('open', 'pending', 'resolved', 'closed'));

ALTER TABLE support_tickets
    ADD COLUMN priority                 text        NOT NULL DEFAULT 'normal',
    ADD COLUMN assignee_id              uuid        REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN version                  integer     NOT NULL DEFAULT 1,
    ADD COLUMN message_count            integer     NOT NULL DEFAULT 1,
    ADD COLUMN last_message_at          timestamptz,
    ADD COLUMN last_customer_message_at timestamptz,
    ADD COLUMN last_staff_message_at    timestamptz,
    ADD COLUMN first_response_at        timestamptz,
    ADD COLUMN resolved_at              timestamptz,
    ADD COLUMN closed_at                timestamptz,
    ADD COLUMN staff_seen_at            timestamptz,
    -- keyed hash of the private-link token (never the token itself) and a
    -- sealed copy, so the same link can be e-mailed again (extended, not rotated).
    ADD COLUMN access_token_hash        text,
    ADD COLUMN access_token_enc         text,
    ADD COLUMN access_token_expires_at  timestamptz,
    -- "j•••@g•••.com": lets the queue show who wrote without decrypting.
    ADD COLUMN email_masked             text        NOT NULL DEFAULT '';

-- Existing requests: the original message is the whole thread so far.
UPDATE support_tickets SET last_message_at = created_at, last_customer_message_at = created_at;
ALTER TABLE support_tickets
    ALTER COLUMN last_message_at SET NOT NULL,
    ALTER COLUMN last_message_at SET DEFAULT now(),
    ALTER COLUMN last_customer_message_at SET NOT NULL,
    ALTER COLUMN last_customer_message_at SET DEFAULT now();
-- Tickets closed before this migration have no close time; retention counts from now.
UPDATE support_tickets SET closed_at = updated_at WHERE status = 'closed';

ALTER TABLE support_tickets
    ADD CONSTRAINT support_tickets_priority_check CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    ADD CONSTRAINT support_tickets_version_check CHECK (version >= 1),
    ADD CONSTRAINT support_tickets_message_count_check CHECK (message_count BETWEEN 1 AND 200),
    ADD CONSTRAINT support_tickets_token_hash_check CHECK (access_token_hash IS NULL OR char_length(access_token_hash) = 64),
    ADD CONSTRAINT support_tickets_token_enc_check CHECK (access_token_enc IS NULL OR char_length(access_token_enc) BETWEEN 1 AND 2048),
    ADD CONSTRAINT support_tickets_token_pair_check CHECK ((access_token_hash IS NULL) = (access_token_expires_at IS NULL)),
    ADD CONSTRAINT support_tickets_email_masked_check CHECK (char_length(email_masked) <= 120),
    ADD CONSTRAINT support_tickets_closed_at_check CHECK (status <> 'closed' OR closed_at IS NOT NULL);

-- Queue and filters. Every list is keyset-paginated, so each index ends in the
-- sort key and the id tie-breaker.
DROP INDEX support_tickets_status_idx;
CREATE INDEX support_tickets_created_idx ON support_tickets (created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX support_tickets_status_idx ON support_tickets (status, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX support_tickets_priority_idx ON support_tickets (priority, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX support_tickets_assignee_idx ON support_tickets (assignee_id, status, created_at DESC, id DESC)
    WHERE assignee_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX support_tickets_unassigned_idx ON support_tickets (status, created_at DESC, id DESC)
    WHERE assignee_id IS NULL AND deleted_at IS NULL;
-- sort=waiting: the oldest unanswered customer message first.
CREATE INDEX support_tickets_waiting_idx ON support_tickets (last_customer_message_at, id)
    WHERE status = 'open' AND deleted_at IS NULL;
CREATE INDEX support_tickets_waiting_rest_idx ON support_tickets (last_customer_message_at, id)
    WHERE status <> 'open' AND deleted_at IS NULL;
-- A signed-in requester's list: latest activity first.
DROP INDEX support_tickets_user_idx;
CREATE INDEX support_tickets_user_idx ON support_tickets (user_id, last_message_at DESC, id DESC)
    WHERE user_id IS NOT NULL AND deleted_at IS NULL;
-- Subject words (q): to_tsvector('simple', subject) @@ plainto_tsquery('simple', ...).
CREATE INDEX support_tickets_subject_search_idx ON support_tickets USING gin (to_tsvector('simple', subject))
    WHERE deleted_at IS NULL;
-- Maintenance: auto-close and retention.
CREATE INDEX support_tickets_resolved_idx ON support_tickets (resolved_at) WHERE status = 'resolved';
CREATE INDEX support_tickets_closed_idx ON support_tickets (closed_at) WHERE status = 'closed';
-- Masked addresses of tickets created before this migration are filled in by the maintenance job.
CREATE INDEX support_tickets_unmasked_idx ON support_tickets (created_at) WHERE email_masked = '';

COMMENT ON COLUMN support_tickets.access_token_hash IS
    'HMAC-SHA256 of the private-link token; the raw token is never stored (a sealed copy lives in access_token_enc).';

-- ---------------------------------------------------------------------------
-- support_messages
-- ---------------------------------------------------------------------------
CREATE TABLE support_messages (
    id          uuid PRIMARY KEY,
    ticket_id   uuid        NOT NULL REFERENCES support_tickets (id) ON DELETE CASCADE,
    author_type text        NOT NULL CHECK (author_type IN ('customer', 'staff', 'system')),
    author_id   uuid        REFERENCES users (id) ON DELETE SET NULL,
    visibility  text        NOT NULL CHECK (visibility IN ('public', 'internal')),
    body_enc    text        NOT NULL CHECK (char_length(body_enc) BETWEEN 1 AND 40000),
    body_chars  integer     NOT NULL CHECK (body_chars BETWEEN 1 AND 5000),
    email_status text       NOT NULL DEFAULT 'none' CHECK (email_status IN ('none', 'queued', 'sent', 'failed')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    -- Customers only write public messages; only a public staff reply is e-mailed.
    CONSTRAINT support_messages_customer_check CHECK (author_type <> 'customer' OR (visibility = 'public' AND author_id IS NULL)),
    CONSTRAINT support_messages_system_check CHECK (author_type <> 'system' OR (visibility = 'internal' AND author_id IS NULL)),
    CONSTRAINT support_messages_email_check CHECK (email_status = 'none' OR (author_type = 'staff' AND visibility = 'public'))
);

CREATE INDEX support_messages_ticket_idx ON support_messages (ticket_id, created_at, id);

-- Bodies are immutable. Only email_status may change (and author_id may be
-- cleared when the staff account is erased).
CREATE OR REPLACE FUNCTION support_messages_guard() RETURNS trigger AS $$
BEGIN
    IF NEW.id IS DISTINCT FROM OLD.id
       OR NEW.ticket_id IS DISTINCT FROM OLD.ticket_id
       OR NEW.author_type IS DISTINCT FROM OLD.author_type
       OR NEW.visibility IS DISTINCT FROM OLD.visibility
       OR NEW.body_enc IS DISTINCT FROM OLD.body_enc
       OR NEW.body_chars IS DISTINCT FROM OLD.body_chars
       OR NEW.created_at IS DISTINCT FROM OLD.created_at
       OR (NEW.author_id IS DISTINCT FROM OLD.author_id AND NEW.author_id IS NOT NULL) THEN
        RAISE EXCEPTION 'support_messages are immutable (only email_status may change)' USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER support_messages_immutable BEFORE UPDATE ON support_messages
    FOR EACH ROW EXECUTE FUNCTION support_messages_guard();

COMMENT ON TABLE support_messages IS
    'Replies, internal notes and system entries of a ticket. body_enc is sealed (AES-GCM, AD support-message:<id>); rows are immutable apart from email_status.';
