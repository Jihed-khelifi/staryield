-- Staryield schema v1.
-- Money is integer cents; credits are integer units. Every credit movement is
-- recorded in the append-only credit_ledger (BR-10).

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Append-only guard shared by ledger, audit and earnings tables.
CREATE OR REPLACE FUNCTION forbid_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = 'P0001';
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------------
-- Identity
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id                  uuid PRIMARY KEY,
    email               text        NOT NULL,
    password_hash       text        NOT NULL,
    role                text        NOT NULL CHECK (role IN ('client', 'psychic', 'admin')),
    status              text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deactivated')),
    display_name        text        NOT NULL DEFAULT '' CHECK (char_length(display_name) <= 80),
    locale              text        NOT NULL DEFAULT 'en' CHECK (locale IN ('en', 'fr', 'es')),
    email_verified_at   timestamptz,
    terms_accepted_at   timestamptz NOT NULL,
    token_version       integer     NOT NULL DEFAULT 0 CHECK (token_version >= 0),
    last_login_at       timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_normalized CHECK (email = lower(email) AND char_length(email) <= 254)
);
CREATE UNIQUE INDEX users_email_key ON users (email);
CREATE INDEX users_role_created_idx ON users (role, created_at DESC);

CREATE TABLE user_profiles (
    user_id           uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    nickname          text        NOT NULL DEFAULT '' CHECK (char_length(nickname) <= 40),
    birth_date        date,
    birth_time        text        NOT NULL DEFAULT '' CHECK (birth_time = '' OR birth_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    birth_place_id    text        NOT NULL DEFAULT '',
    birth_place       text        NOT NULL DEFAULT '' CHECK (char_length(birth_place) <= 120),
    birth_latitude    double precision CHECK (birth_latitude BETWEEN -90 AND 90),
    birth_longitude   double precision CHECK (birth_longitude BETWEEN -180 AND 180),
    birth_timezone    text        NOT NULL DEFAULT '',
    location          text        NOT NULL DEFAULT '' CHECK (char_length(location) <= 60),
    mood              text        NOT NULL DEFAULT 'Empowered',
    theme             text        NOT NULL DEFAULT 'Warm Beige',
    avatar_url        text        NOT NULL DEFAULT '',
    avatar_key        text        NOT NULL DEFAULT '',
    notify_horoscope  boolean     NOT NULL DEFAULT true,
    notify_events     boolean     NOT NULL DEFAULT false,
    notify_chart      boolean     NOT NULL DEFAULT false,
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id            uuid PRIMARY KEY,
    user_id       uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    family_id     uuid        NOT NULL,
    token_hash    text        NOT NULL,
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz,
    revoke_reason text        NOT NULL DEFAULT '',
    replaced_by   uuid,
    user_agent    text        NOT NULL DEFAULT '',
    ip_hash       text        NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    last_used_at  timestamptz
);
CREATE UNIQUE INDEX refresh_tokens_hash_key ON refresh_tokens (token_hash);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id) WHERE revoked_at IS NULL;
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);

CREATE TABLE password_reset_tokens (
    id          uuid PRIMARY KEY,
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  text        NOT NULL,
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX password_reset_tokens_hash_key ON password_reset_tokens (token_hash);
CREATE INDEX password_reset_tokens_user_idx ON password_reset_tokens (user_id);

-- ---------------------------------------------------------------------------
-- Psychic advisors and their listed services (custom requests, FR-06)
-- ---------------------------------------------------------------------------
CREATE TABLE psychics (
    user_id           uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    slug              text        NOT NULL CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND char_length(slug) <= 64),
    name              text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    headline          text        NOT NULL DEFAULT '' CHECK (char_length(headline) <= 200),
    about             jsonb       NOT NULL DEFAULT '[]'::jsonb,
    philosophy        text        NOT NULL DEFAULT '' CHECK (char_length(philosophy) <= 600),
    image_url         text        NOT NULL DEFAULT '',
    zodiac            text        NOT NULL DEFAULT '',
    skills            jsonb       NOT NULL DEFAULT '[]'::jsonb,
    rating            numeric(2,1) NOT NULL DEFAULT 5.0 CHECK (rating BETWEEN 0 AND 5),
    reviews_count     integer     NOT NULL DEFAULT 0 CHECK (reviews_count >= 0),
    readings_count    integer     NOT NULL DEFAULT 0 CHECK (readings_count >= 0),
    years_experience  integer     NOT NULL DEFAULT 0 CHECK (years_experience BETWEEN 0 AND 80),
    rate_per_minute   bigint      NOT NULL CHECK (rate_per_minute BETWEEN 1 AND 10000),
    availability      text        NOT NULL DEFAULT 'offline' CHECK (availability IN ('online', 'away', 'offline')),
    listed            boolean     NOT NULL DEFAULT true,
    sort_order        integer     NOT NULL DEFAULT 0,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX psychics_slug_key ON psychics (slug);

CREATE TABLE psychic_services (
    id             uuid PRIMARY KEY,
    psychic_id     uuid        NOT NULL REFERENCES psychics (user_id) ON DELETE CASCADE,
    name           text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    subtitle       text        NOT NULL DEFAULT '' CHECK (char_length(subtitle) <= 80),
    description    text        NOT NULL DEFAULT '' CHECK (char_length(description) <= 1000),
    price_credits  bigint      NOT NULL CHECK (price_credits BETWEEN 1 AND 1000000),
    active         boolean     NOT NULL DEFAULT true,
    sort_order     integer     NOT NULL DEFAULT 0,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX psychic_services_psychic_idx ON psychic_services (psychic_id, sort_order);

-- ---------------------------------------------------------------------------
-- Credits: wallet, immutable ledger, packs, purchases
-- ---------------------------------------------------------------------------
CREATE TABLE wallets (
    user_id       uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    balance       bigint      NOT NULL DEFAULT 0 CHECK (balance >= 0),
    held          bigint      NOT NULL DEFAULT 0 CHECK (held >= 0),
    free_minutes  integer     NOT NULL DEFAULT 0 CHECK (free_minutes >= 0),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE credit_ledger (
    id               uuid PRIMARY KEY,
    user_id          uuid        NOT NULL REFERENCES users (id),
    entry_type       text        NOT NULL CHECK (entry_type IN (
                        'purchase', 'signup_bonus', 'chat_charge', 'hold', 'hold_release',
                        'hold_refund', 'admin_adjustment', 'refund')),
    amount           bigint      NOT NULL,
    held_amount      bigint      NOT NULL DEFAULT 0,
    balance_after    bigint      NOT NULL CHECK (balance_after >= 0),
    held_after       bigint      NOT NULL CHECK (held_after >= 0),
    reference_type   text        NOT NULL DEFAULT '',
    reference_id     uuid,
    idempotency_key  text        NOT NULL,
    description      text        NOT NULL DEFAULT '',
    actor_id         uuid,
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX credit_ledger_idempotency_key ON credit_ledger (idempotency_key);
CREATE INDEX credit_ledger_user_idx ON credit_ledger (user_id, created_at DESC, id DESC);
CREATE INDEX credit_ledger_type_created_idx ON credit_ledger (entry_type, created_at);
CREATE TRIGGER credit_ledger_append_only BEFORE UPDATE OR DELETE ON credit_ledger
    FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
CREATE TRIGGER credit_ledger_no_truncate BEFORE TRUNCATE ON credit_ledger
    FOR EACH STATEMENT EXECUTE FUNCTION forbid_mutation();

CREATE TABLE credit_packs (
    id            uuid PRIMARY KEY,
    name          text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    credits       bigint      NOT NULL CHECK (credits BETWEEN 1 AND 1000000),
    price_cents   bigint      NOT NULL CHECK (price_cents BETWEEN 50 AND 10000000),
    currency      text        NOT NULL DEFAULT 'USD' CHECK (currency ~ '^[A-Z]{3}$'),
    active        boolean     NOT NULL DEFAULT true,
    featured      boolean     NOT NULL DEFAULT false,
    sort_order    integer     NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE credit_purchases (
    id               uuid PRIMARY KEY,
    user_id          uuid        NOT NULL REFERENCES users (id),
    pack_id          uuid        NOT NULL REFERENCES credit_packs (id),
    credits          bigint      NOT NULL CHECK (credits > 0),
    price_cents      bigint      NOT NULL CHECK (price_cents > 0),
    currency         text        NOT NULL,
    status           text        NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed', 'refunded')),
    provider         text        NOT NULL,
    provider_ref     text,
    idempotency_key  text        NOT NULL,
    receipt_email    text        NOT NULL DEFAULT '',
    failure_reason   text        NOT NULL DEFAULT '',
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    completed_at     timestamptz
);
CREATE UNIQUE INDEX credit_purchases_idempotency_key ON credit_purchases (user_id, idempotency_key);
CREATE UNIQUE INDEX credit_purchases_provider_ref_key ON credit_purchases (provider, provider_ref) WHERE provider_ref IS NOT NULL;
CREATE INDEX credit_purchases_user_idx ON credit_purchases (user_id, created_at DESC);
CREATE INDEX credit_purchases_status_created_idx ON credit_purchases (status, created_at);

-- ---------------------------------------------------------------------------
-- Live chat: conversations, sessions (state machine §6), messages
-- ---------------------------------------------------------------------------
CREATE TABLE conversations (
    id                    uuid PRIMARY KEY,
    client_id             uuid        NOT NULL REFERENCES users (id),
    psychic_id            uuid        NOT NULL REFERENCES psychics (user_id),
    last_message_id       uuid,
    last_message_at       timestamptz,
    client_last_read_at   timestamptz,
    psychic_last_read_at  timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX conversations_pair_key ON conversations (client_id, psychic_id);
CREATE INDEX conversations_psychic_idx ON conversations (psychic_id, last_message_at DESC NULLS LAST);
CREATE INDEX conversations_client_idx ON conversations (client_id, last_message_at DESC NULLS LAST);

CREATE TABLE chat_sessions (
    id                 uuid PRIMARY KEY,
    conversation_id    uuid        NOT NULL REFERENCES conversations (id),
    client_id          uuid        NOT NULL REFERENCES users (id),
    psychic_id         uuid        NOT NULL REFERENCES psychics (user_id),
    status             text        NOT NULL CHECK (status IN (
                          'requested', 'accepted', 'active', 'disconnected', 'ended',
                          'settled', 'declined', 'expired', 'cancelled')),
    end_reason         text        NOT NULL DEFAULT '',
    rate_per_minute    bigint      NOT NULL CHECK (rate_per_minute > 0),
    request_key        text        NOT NULL,
    requested_at       timestamptz NOT NULL,
    expires_at         timestamptz NOT NULL,
    accepted_at        timestamptz,
    started_at         timestamptz,
    last_resumed_at    timestamptz,
    disconnected_at    timestamptz,
    disconnected_party text        NOT NULL DEFAULT '',
    grace_expires_at   timestamptz,
    ended_at           timestamptz,
    settled_at         timestamptz,
    next_billing_at    timestamptz,
    active_seconds     bigint      NOT NULL DEFAULT 0 CHECK (active_seconds >= 0),
    billed_minutes     integer     NOT NULL DEFAULT 0 CHECK (billed_minutes >= 0),
    free_minutes_used  integer     NOT NULL DEFAULT 0 CHECK (free_minutes_used >= 0),
    credits_charged    bigint      NOT NULL DEFAULT 0 CHECK (credits_charged >= 0),
    psychic_earnings   bigint      NOT NULL DEFAULT 0 CHECK (psychic_earnings >= 0),
    reconnects         integer     NOT NULL DEFAULT 0,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX chat_sessions_request_key ON chat_sessions (client_id, request_key);
-- A client has at most one open session at a time.
CREATE UNIQUE INDEX chat_sessions_one_open_per_client ON chat_sessions (client_id)
    WHERE status IN ('requested', 'accepted', 'active', 'disconnected');
-- A psychic handles one session (or pending request) at a time: no waiting queue.
CREATE UNIQUE INDEX chat_sessions_one_open_per_psychic ON chat_sessions (psychic_id)
    WHERE status IN ('requested', 'accepted', 'active', 'disconnected');
CREATE INDEX chat_sessions_billing_idx ON chat_sessions (next_billing_at) WHERE status = 'active';
CREATE INDEX chat_sessions_requested_idx ON chat_sessions (expires_at) WHERE status IN ('requested', 'accepted');
CREATE INDEX chat_sessions_grace_idx ON chat_sessions (grace_expires_at) WHERE status = 'disconnected';
CREATE INDEX chat_sessions_ended_idx ON chat_sessions (ended_at) WHERE status = 'ended';
CREATE INDEX chat_sessions_conversation_idx ON chat_sessions (conversation_id, requested_at DESC);
CREATE INDEX chat_sessions_client_idx ON chat_sessions (client_id, requested_at DESC);
CREATE INDEX chat_sessions_psychic_idx ON chat_sessions (psychic_id, requested_at DESC);
CREATE INDEX chat_sessions_status_requested_idx ON chat_sessions (status, requested_at);

CREATE TABLE chat_messages (
    id               uuid PRIMARY KEY,
    conversation_id  uuid        NOT NULL REFERENCES conversations (id),
    session_id       uuid        NOT NULL REFERENCES chat_sessions (id),
    sender_id        uuid        REFERENCES users (id),
    kind             text        NOT NULL DEFAULT 'text' CHECK (kind IN ('text', 'system')),
    body             text        NOT NULL CHECK (char_length(body) BETWEEN 1 AND 10000),
    client_msg_id    text,
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX chat_messages_client_msg_key ON chat_messages (sender_id, client_msg_id) WHERE client_msg_id IS NOT NULL;
CREATE INDEX chat_messages_conversation_idx ON chat_messages (conversation_id, created_at DESC, id DESC);
CREATE INDEX chat_messages_session_idx ON chat_messages (session_id, created_at);

ALTER TABLE conversations ADD CONSTRAINT conversations_last_message_fk
    FOREIGN KEY (last_message_id) REFERENCES chat_messages (id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE saved_messages (
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    message_id  uuid        NOT NULL REFERENCES chat_messages (id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, message_id)
);

-- ---------------------------------------------------------------------------
-- Custom requests (§5.1): credits held on order, released on delivery (BR-06)
-- ---------------------------------------------------------------------------
CREATE TABLE custom_requests (
    id               uuid PRIMARY KEY,
    client_id        uuid        NOT NULL REFERENCES users (id),
    psychic_id       uuid        NOT NULL REFERENCES psychics (user_id),
    service_id       uuid        NOT NULL REFERENCES psychic_services (id),
    service_name     text        NOT NULL,
    price_credits    bigint      NOT NULL CHECK (price_credits > 0),
    status           text        NOT NULL CHECK (status IN (
                        'pending', 'accepted', 'delivered', 'declined', 'cancelled',
                        'expired', 'disputed', 'refunded')),
    client_note      text        NOT NULL DEFAULT '',
    preferred_at     timestamptz,
    delivery         text        NOT NULL DEFAULT '',
    decline_reason   text        NOT NULL DEFAULT '',
    dispute_reason   text        NOT NULL DEFAULT '',
    resolution_note  text        NOT NULL DEFAULT '',
    idempotency_key  text        NOT NULL,
    accept_by        timestamptz NOT NULL,
    deliver_by       timestamptz,
    accepted_at      timestamptz,
    delivered_at     timestamptz,
    closed_at        timestamptz,
    resolved_by      uuid REFERENCES users (id),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX custom_requests_idempotency_key ON custom_requests (client_id, idempotency_key);
CREATE INDEX custom_requests_client_idx ON custom_requests (client_id, created_at DESC);
CREATE INDEX custom_requests_psychic_idx ON custom_requests (psychic_id, created_at DESC);
CREATE INDEX custom_requests_status_idx ON custom_requests (status, created_at);
CREATE INDEX custom_requests_accept_by_idx ON custom_requests (accept_by) WHERE status = 'pending';
CREATE INDEX custom_requests_deliver_by_idx ON custom_requests (deliver_by) WHERE status = 'accepted';

-- ---------------------------------------------------------------------------
-- Psychic earnings (BR-08): one row per settled session / delivered request
-- ---------------------------------------------------------------------------
CREATE TABLE psychic_earnings (
    id                  uuid PRIMARY KEY,
    psychic_id          uuid        NOT NULL REFERENCES psychics (user_id),
    source_type         text        NOT NULL CHECK (source_type IN ('chat_session', 'custom_request')),
    source_id           uuid        NOT NULL,
    entry_kind          text        NOT NULL DEFAULT 'earning' CHECK (entry_kind IN ('earning', 'reversal')),
    gross_credits       bigint      NOT NULL,
    commission_credits  bigint      NOT NULL,
    net_credits         bigint      NOT NULL,
    share_percent       integer     NOT NULL CHECK (share_percent BETWEEN 0 AND 100),
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX psychic_earnings_source_key ON psychic_earnings (source_type, source_id, entry_kind);
CREATE INDEX psychic_earnings_psychic_idx ON psychic_earnings (psychic_id, created_at DESC);
CREATE TRIGGER psychic_earnings_append_only BEFORE UPDATE OR DELETE ON psychic_earnings
    FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

-- ---------------------------------------------------------------------------
-- Business analytics (append-only event store for the admin dashboard)
-- ---------------------------------------------------------------------------
CREATE TABLE analytics_events (
    id            bigserial PRIMARY KEY,
    event_id      uuid        NOT NULL,
    name          text        NOT NULL CHECK (name ~ '^[a-z][a-z0-9_.]{1,63}$'),
    source        text        NOT NULL CHECK (source IN ('server', 'client')),
    user_id       uuid,
    user_role     text        NOT NULL DEFAULT '',
    anonymous_id  text        NOT NULL DEFAULT '',
    session_key   text        NOT NULL DEFAULT '',
    occurred_at   timestamptz NOT NULL,
    received_at   timestamptz NOT NULL DEFAULT now(),
    properties    jsonb       NOT NULL DEFAULT '{}'::jsonb,
    path          text        NOT NULL DEFAULT '',
    referrer      text        NOT NULL DEFAULT '',
    locale        text        NOT NULL DEFAULT '',
    user_agent    text        NOT NULL DEFAULT '',
    ip_hash       text        NOT NULL DEFAULT '',
    request_id    text        NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX analytics_events_event_id_key ON analytics_events (event_id);
CREATE INDEX analytics_events_name_time_idx ON analytics_events (name, occurred_at DESC);
CREATE INDEX analytics_events_user_time_idx ON analytics_events (user_id, occurred_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX analytics_events_anon_time_idx ON analytics_events (anonymous_id, occurred_at DESC) WHERE anonymous_id <> '';
CREATE INDEX analytics_events_time_brin ON analytics_events USING brin (occurred_at);
CREATE TRIGGER analytics_events_append_only BEFORE UPDATE ON analytics_events
    FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

-- ---------------------------------------------------------------------------
-- Admin audit trail
-- ---------------------------------------------------------------------------
CREATE TABLE audit_logs (
    id           uuid PRIMARY KEY,
    actor_id     uuid        REFERENCES users (id),
    action       text        NOT NULL,
    target_type  text        NOT NULL DEFAULT '',
    target_id    text        NOT NULL DEFAULT '',
    metadata     jsonb       NOT NULL DEFAULT '{}'::jsonb,
    ip_hash      text        NOT NULL DEFAULT '',
    request_id   text        NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_id, created_at DESC);
CREATE INDEX audit_logs_target_idx ON audit_logs (target_type, target_id, created_at DESC);
CREATE INDEX audit_logs_created_idx ON audit_logs (created_at DESC);
CREATE TRIGGER audit_logs_append_only BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
