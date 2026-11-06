-- 0013_support_tickets: requests sent through the contact form.
--
-- The reply address and the message body are sealed with AES-GCM
-- (associated data "support:<id>"), so a database dump alone does not reveal
-- them; email_hash (HMAC) lets the team find all tickets of one address
-- without decrypting. Tickets from signed-in users keep a link to the
-- account; anonymous tickets have user_id NULL.

CREATE TABLE support_tickets (
    id            uuid PRIMARY KEY,
    reference     text        NOT NULL CHECK (reference ~ '^SY-[A-HJ-NP-Z2-9]{6}$'),
    user_id       uuid        REFERENCES users (id) ON DELETE SET NULL,
    name          text        NOT NULL DEFAULT '' CHECK (char_length(name) <= 80),
    email_enc     text        NOT NULL CHECK (char_length(email_enc) BETWEEN 1 AND 2048),
    email_hash    text        NOT NULL CHECK (char_length(email_hash) = 64),
    topic         text        NOT NULL CHECK (topic IN ('general', 'privacy', 'billing', 'advisor', 'technical')),
    subject       text        NOT NULL CHECK (char_length(subject) BETWEEN 3 AND 120),
    message_enc   text        NOT NULL CHECK (char_length(message_enc) BETWEEN 1 AND 40000),
    message_chars integer     NOT NULL CHECK (message_chars BETWEEN 10 AND 5000),
    locale        text        NOT NULL DEFAULT 'en' CHECK (locale IN ('en', 'es', 'fr')),
    status        text        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    ip_hash       text        NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);

CREATE UNIQUE INDEX support_tickets_reference_key ON support_tickets (reference);
CREATE INDEX support_tickets_status_idx ON support_tickets (status, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX support_tickets_user_idx ON support_tickets (user_id, created_at DESC) WHERE user_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX support_tickets_email_idx ON support_tickets (email_hash, created_at DESC) WHERE deleted_at IS NULL;

COMMENT ON TABLE support_tickets IS
    'Contact-form requests. email_enc and message_enc are sealed (AES-GCM, AD support:<id>); email_hash is HMAC-SHA256 of the lower-cased address.';
