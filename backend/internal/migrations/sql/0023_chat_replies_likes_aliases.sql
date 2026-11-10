-- 0023_chat_replies_likes_aliases: reply-to-message, message likes and the
-- advisors' private client nicknames (revisions 4.0, docs/platform-extensions.md
-- §23.2-23.4).
--
-- * chat_messages.reply_to_id points at the quoted message of the same
--   conversation (checked by the service before the insert); the quote is
--   derived at read time, so nothing is copied. ON DELETE SET NULL keeps a
--   reply readable if its target is ever hard-deleted. The partial index
--   serves the FK (only replies are indexed).
-- * message_likes: at most one row per (message, participant); at most two
--   rows per message. Rows go with the message or the user (cascade); the
--   primary key serves the batched "likes of these messages" lookup.
-- * client_aliases: an advisor's private nickname for a client, sealed at
--   rest (AES-GCM, associated data "alias:<psychic_id>:<client_id>"). The
--   alias never leaves the advisor's own responses.

ALTER TABLE chat_messages
    ADD COLUMN reply_to_id uuid REFERENCES chat_messages (id) ON DELETE SET NULL;

CREATE INDEX chat_messages_reply_to_idx ON chat_messages (reply_to_id) WHERE reply_to_id IS NOT NULL;

CREATE TABLE message_likes (
    message_id uuid        NOT NULL REFERENCES chat_messages (id) ON DELETE CASCADE,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, user_id)
);

-- Erasure and "likes of this user" (the data export).
CREATE INDEX message_likes_user_idx ON message_likes (user_id);

CREATE TABLE client_aliases (
    psychic_id uuid        NOT NULL REFERENCES psychics (user_id) ON DELETE CASCADE,
    client_id  uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    alias_enc  text        NOT NULL CHECK (char_length(alias_enc) BETWEEN 1 AND 1024),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (psychic_id, client_id),
    CONSTRAINT client_aliases_not_self CHECK (psychic_id <> client_id)
);

-- Erasure of a client.
CREATE INDEX client_aliases_client_idx ON client_aliases (client_id);

CREATE TRIGGER client_aliases_touch_updated_at BEFORE UPDATE ON client_aliases
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

COMMENT ON TABLE client_aliases IS
    'An advisor''s private nickname for a client. alias_enc is sealed (AES-GCM, AD alias:<psychic_id>:<client_id>); never exposed to the client or other advisors.';
