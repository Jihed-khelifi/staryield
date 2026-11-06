-- 0008_client_notes: notes an advisor keeps about a client.
--
-- Notes are private to their author by default. The author may share a note
-- with the other advisors who have had a started session with the same
-- client; those advisors can read it but never edit it. Bodies are sealed
-- with AES-GCM (associated data "note:<id>:<author_id>") so a database dump
-- alone does not reveal them, and a ciphertext copied to another row will not
-- open. body_chars mirrors the service-side length rule for the CHECK.

CREATE TABLE client_notes (
    id              uuid PRIMARY KEY,
    client_id       uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    author_id       uuid        NOT NULL REFERENCES psychics (user_id) ON DELETE CASCADE,
    conversation_id uuid        REFERENCES conversations (id) ON DELETE SET NULL,
    body_enc        text        NOT NULL CHECK (char_length(body_enc) BETWEEN 1 AND 16384),
    body_chars      integer     NOT NULL CHECK (body_chars BETWEEN 1 AND 2000),
    shared          boolean     NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,
    CONSTRAINT client_notes_not_self CHECK (client_id <> author_id)
);

-- An advisor's own notes about one client, newest first.
CREATE INDEX client_notes_author_idx ON client_notes (author_id, client_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
-- Shared notes about one client, newest first ("Notes from other psychics").
CREATE INDEX client_notes_shared_idx ON client_notes (client_id, created_at DESC, id DESC)
    WHERE shared AND deleted_at IS NULL;
-- Erasure and conversation clean-up.
CREATE INDEX client_notes_client_idx ON client_notes (client_id);
CREATE INDEX client_notes_conversation_idx ON client_notes (conversation_id) WHERE conversation_id IS NOT NULL;

CREATE TRIGGER client_notes_touch_updated_at BEFORE UPDATE ON client_notes
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

COMMENT ON TABLE client_notes IS
    'Advisor notes about a client. body_enc is sealed (AES-GCM, AD note:<id>:<author_id>); soft-deleted via deleted_at and hard-deleted when either account is erased.';
