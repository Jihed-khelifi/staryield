-- Staryield schema v5: transactional e-mail outbox, suppressions and Web Push
-- subscriptions.

-- ---------------------------------------------------------------------------
-- E-mail outbox
-- ---------------------------------------------------------------------------
-- E-mail is written to the outbox (in the transaction of the change that
-- caused it) and delivered by background workers with retries. Recipients and
-- rendered bodies can contain one-time links, so both are sealed (AES-GCM,
-- bound to the row id); bodies and recipients are wiped once the row reaches a
-- final state. to_hash (keyed HMAC of the normalised address) supports
-- suppression checks and audits without storing the address in clear.
CREATE TABLE email_outbox (
    id                   uuid PRIMARY KEY,
    user_id              uuid REFERENCES users (id) ON DELETE SET NULL,
    category             text        NOT NULL CHECK (category IN ('security', 'billing', 'account', 'notification', 'marketing')),
    template             text        NOT NULL CHECK (char_length(template) BETWEEN 1 AND 64),
    locale               text        NOT NULL DEFAULT 'en' CHECK (locale IN ('en', 'fr', 'es')),
    to_hash              text        NOT NULL CHECK (char_length(to_hash) = 64),
    to_sealed            text        NOT NULL DEFAULT '',
    payload_sealed       text        NOT NULL DEFAULT '',
    dedupe_key           text        NOT NULL DEFAULT '' CHECK (char_length(dedupe_key) <= 128),
    status               text        NOT NULL CHECK (status IN ('queued', 'sending', 'sent', 'failed', 'dead', 'suppressed', 'cancelled')),
    status_reason        text        NOT NULL DEFAULT '' CHECK (char_length(status_reason) <= 64),
    attempts             integer     NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at      timestamptz NOT NULL,
    locked_until         timestamptz,
    last_error           text        NOT NULL DEFAULT '' CHECK (char_length(last_error) <= 300),
    provider             text        NOT NULL DEFAULT '' CHECK (char_length(provider) <= 32),
    provider_message_id  text        NOT NULL DEFAULT '' CHECK (char_length(provider_message_id) <= 200),
    sent_at              timestamptz,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_outbox_due_idx ON email_outbox (next_attempt_at) WHERE status = 'queued';
CREATE INDEX email_outbox_lease_idx ON email_outbox (locked_until) WHERE status = 'sending';
-- Per-user daily cap.
CREATE INDEX email_outbox_user_idx ON email_outbox (user_id, created_at DESC) WHERE user_id IS NOT NULL;
-- Idempotent enqueue (e.g. one e-mail per notification).
CREATE UNIQUE INDEX email_outbox_dedupe_key ON email_outbox (dedupe_key) WHERE dedupe_key <> '';
-- Retention sweep of finished rows.
CREATE INDEX email_outbox_finished_idx ON email_outbox (updated_at)
    WHERE status IN ('sent', 'failed', 'dead', 'suppressed', 'cancelled');
CREATE TRIGGER email_outbox_touch_updated_at BEFORE UPDATE ON email_outbox
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- Addresses that must not be e-mailed (hard bounces, complaints, manual
-- blocks), keyed by the same HMAC as email_outbox.to_hash. No plaintext.
CREATE TABLE email_suppressions (
    email_hash  text PRIMARY KEY CHECK (char_length(email_hash) = 64),
    reason      text        NOT NULL CHECK (reason IN ('hard_bounce', 'complaint', 'manual')),
    source      text        NOT NULL DEFAULT '' CHECK (char_length(source) <= 40),
    expires_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER email_suppressions_touch_updated_at BEFORE UPDATE ON email_suppressions
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- ---------------------------------------------------------------------------
-- Web Push subscriptions
-- ---------------------------------------------------------------------------
-- A browser endpoint is a bearer capability: it is sealed at rest with its
-- keys and identified by a keyed hash. The hash is unique across the table so
-- a browser that changes hands (sign out, sign in as someone else) is
-- re-assigned instead of duplicated; unsubscribing soft-deletes the row.
CREATE TABLE push_subscriptions (
    id               uuid PRIMARY KEY,
    user_id          uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    endpoint_hash    text        NOT NULL CHECK (char_length(endpoint_hash) = 64),
    endpoint_sealed  text        NOT NULL,
    p256dh_sealed    text        NOT NULL,
    auth_sealed      text        NOT NULL,
    user_agent       text        NOT NULL DEFAULT '' CHECK (char_length(user_agent) <= 256),
    last_success_at  timestamptz,
    failure_count    integer     NOT NULL DEFAULT 0 CHECK (failure_count >= 0),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz
);
CREATE UNIQUE INDEX push_subscriptions_endpoint_key ON push_subscriptions (endpoint_hash);
CREATE INDEX push_subscriptions_user_idx ON push_subscriptions (user_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE TRIGGER push_subscriptions_touch_updated_at BEFORE UPDATE ON push_subscriptions
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
