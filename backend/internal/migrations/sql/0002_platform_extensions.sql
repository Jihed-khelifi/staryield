-- Staryield schema v2: admin console security, moderation, growth tooling,
-- notifications, chat attachments, ratings and advisor payouts.
-- See docs/platform-extensions.md.

-- ---------------------------------------------------------------------------
-- Accounts: ban / soft delete
-- ---------------------------------------------------------------------------
ALTER TABLE users DROP CONSTRAINT users_status_check;
ALTER TABLE users ADD CONSTRAINT users_status_check
    CHECK (status IN ('active', 'deactivated', 'banned', 'deleted'));
ALTER TABLE users ADD COLUMN status_reason text NOT NULL DEFAULT ''
    CHECK (char_length(status_reason) <= 300);
ALTER TABLE users ADD COLUMN status_changed_at timestamptz;
ALTER TABLE users ADD COLUMN deleted_at timestamptz;
CREATE INDEX users_status_idx ON users (status, created_at DESC);

-- Country of the birth place (ISO 3166-1 alpha-2), for audience insights and
-- the advisor's view of a client.
ALTER TABLE user_profiles ADD COLUMN country_code text NOT NULL DEFAULT ''
    CHECK (country_code = '' OR country_code ~ '^[A-Z]{2}$');

-- ---------------------------------------------------------------------------
-- Advisors: featured placement, uploaded image, exact rating aggregate
-- ---------------------------------------------------------------------------
ALTER TABLE psychics ADD COLUMN featured boolean NOT NULL DEFAULT false;
ALTER TABLE psychics ADD COLUMN featured_rank integer NOT NULL DEFAULT 0
    CHECK (featured_rank BETWEEN 0 AND 1000);
ALTER TABLE psychics ADD COLUMN image_key text NOT NULL DEFAULT '';
-- rating = rating_sum / reviews_count; seeded ratings become the starting sum.
ALTER TABLE psychics ADD COLUMN rating_sum numeric(14,1) NOT NULL DEFAULT 0 CHECK (rating_sum >= 0);
UPDATE psychics SET rating_sum = rating * reviews_count;
CREATE INDEX psychics_directory_idx ON psychics (listed, featured DESC, featured_rank, sort_order, name);

-- ---------------------------------------------------------------------------
-- Chat: image messages and private attachments
-- ---------------------------------------------------------------------------
ALTER TABLE chat_messages DROP CONSTRAINT chat_messages_kind_check;
ALTER TABLE chat_messages ADD CONSTRAINT chat_messages_kind_check CHECK (kind IN ('text', 'system', 'image'));
ALTER TABLE chat_messages DROP CONSTRAINT chat_messages_body_check;
ALTER TABLE chat_messages ADD CONSTRAINT chat_messages_body_check
    CHECK (char_length(body) <= 10000 AND (kind = 'image' OR char_length(body) >= 1));

CREATE TABLE chat_attachments (
    id               uuid PRIMARY KEY,
    message_id       uuid        NOT NULL REFERENCES chat_messages (id),
    conversation_id  uuid        NOT NULL REFERENCES conversations (id),
    uploader_id      uuid        NOT NULL REFERENCES users (id),
    object_key       text        NOT NULL,
    content_type     text        NOT NULL CHECK (content_type IN ('image/jpeg', 'image/png')),
    width            integer     NOT NULL CHECK (width BETWEEN 1 AND 10000),
    height           integer     NOT NULL CHECK (height BETWEEN 1 AND 10000),
    bytes            integer     NOT NULL CHECK (bytes > 0),
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_attachments_message_idx ON chat_attachments (message_id);
CREATE UNIQUE INDEX chat_attachments_key ON chat_attachments (object_key);

-- ---------------------------------------------------------------------------
-- Ratings (one per session, client → advisor)
-- ---------------------------------------------------------------------------
CREATE TABLE psychic_reviews (
    id               uuid PRIMARY KEY,
    session_id       uuid        NOT NULL REFERENCES chat_sessions (id),
    client_id        uuid        NOT NULL REFERENCES users (id),
    psychic_id       uuid        NOT NULL REFERENCES psychics (user_id),
    rating           smallint    NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment          text        NOT NULL DEFAULT '' CHECK (char_length(comment) <= 1000),
    status           text        NOT NULL DEFAULT 'visible' CHECK (status IN ('visible', 'hidden')),
    moderation_note  text        NOT NULL DEFAULT '' CHECK (char_length(moderation_note) <= 300),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX psychic_reviews_session_key ON psychic_reviews (session_id);
CREATE INDEX psychic_reviews_psychic_idx ON psychic_reviews (psychic_id, created_at DESC, id DESC);
CREATE INDEX psychic_reviews_status_idx ON psychic_reviews (status, created_at DESC);

-- ---------------------------------------------------------------------------
-- Advisor withdrawals (payout of net earnings)
-- ---------------------------------------------------------------------------
CREATE TABLE withdrawals (
    id                uuid PRIMARY KEY,
    psychic_id        uuid        NOT NULL REFERENCES psychics (user_id),
    amount_credits    bigint      NOT NULL CHECK (amount_credits > 0),
    amount_cents      bigint      NOT NULL CHECK (amount_cents >= 0),
    currency          text        NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    method            text        NOT NULL CHECK (method IN ('bank_transfer', 'paypal')),
    destination_enc   text        NOT NULL,
    destination_hint  text        NOT NULL DEFAULT '',
    status            text        NOT NULL CHECK (status IN ('pending', 'approved', 'paid', 'rejected', 'cancelled')),
    note              text        NOT NULL DEFAULT '' CHECK (char_length(note) <= 500),
    reference         text        NOT NULL DEFAULT '' CHECK (char_length(reference) <= 120),
    idempotency_key   text        NOT NULL,
    requested_at      timestamptz NOT NULL,
    processed_at      timestamptz,
    processed_by      uuid REFERENCES users (id),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX withdrawals_idempotency_key ON withdrawals (psychic_id, idempotency_key);
-- At most one open withdrawal per advisor.
CREATE UNIQUE INDEX withdrawals_one_open ON withdrawals (psychic_id) WHERE status IN ('pending', 'approved');
CREATE INDEX withdrawals_status_idx ON withdrawals (status, requested_at DESC);
CREATE INDEX withdrawals_psychic_idx ON withdrawals (psychic_id, requested_at DESC);

-- ---------------------------------------------------------------------------
-- Notifications and admin broadcasts
-- ---------------------------------------------------------------------------
CREATE TABLE broadcasts (
    id            uuid PRIMARY KEY,
    created_by    uuid REFERENCES users (id),
    audience      text        NOT NULL CHECK (audience IN ('all', 'clients', 'psychics', 'users')),
    online_only   boolean     NOT NULL DEFAULT false,
    target_ids    jsonb       NOT NULL DEFAULT '[]'::jsonb,
    campaign_id   uuid,
    title         jsonb       NOT NULL,
    body          jsonb       NOT NULL,
    link          text        NOT NULL DEFAULT '' CHECK (char_length(link) <= 300),
    status        text        NOT NULL CHECK (status IN ('queued', 'sending', 'sent', 'failed')),
    recipients    integer     NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    sent_at       timestamptz
);
CREATE INDEX broadcasts_created_idx ON broadcasts (created_at DESC, id DESC);

CREATE TABLE notifications (
    id            uuid PRIMARY KEY,
    user_id       uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind          text        NOT NULL CHECK (kind ~ '^[a-z][a-z0-9_.]{1,63}$'),
    data          jsonb       NOT NULL DEFAULT '{}'::jsonb,
    link          text        NOT NULL DEFAULT '' CHECK (char_length(link) <= 300),
    broadcast_id  uuid REFERENCES broadcasts (id),
    read_at       timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC, id DESC);
CREATE INDEX notifications_unread_idx ON notifications (user_id) WHERE read_at IS NULL;

-- ---------------------------------------------------------------------------
-- Growth: campaigns, short links, clicks, first-touch attribution
-- ---------------------------------------------------------------------------
CREATE TABLE campaigns (
    id            uuid PRIMARY KEY,
    name          text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    slug          text        NOT NULL CHECK (slug ~ '^[a-z0-9]+([-_][a-z0-9]+)*$' AND char_length(slug) <= 64),
    channel       text        NOT NULL DEFAULT '' CHECK (char_length(channel) <= 40),
    status        text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'paused', 'ended', 'archived')),
    budget_cents  bigint      NOT NULL DEFAULT 0 CHECK (budget_cents BETWEEN 0 AND 100000000000),
    currency      text        NOT NULL DEFAULT 'USD' CHECK (currency ~ '^[A-Z]{3}$'),
    starts_at     timestamptz,
    ends_at       timestamptz,
    notes         text        NOT NULL DEFAULT '' CHECK (char_length(notes) <= 2000),
    created_by    uuid REFERENCES users (id),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT campaigns_dates CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at >= starts_at)
);
CREATE UNIQUE INDEX campaigns_slug_key ON campaigns (slug);
ALTER TABLE broadcasts ADD CONSTRAINT broadcasts_campaign_fk FOREIGN KEY (campaign_id) REFERENCES campaigns (id);

CREATE TABLE short_links (
    id            uuid PRIMARY KEY,
    code          text        NOT NULL CHECK (code ~ '^[A-Za-z0-9_-]{3,32}$'),
    campaign_id   uuid REFERENCES campaigns (id),
    destination   text        NOT NULL CHECK (char_length(destination) BETWEEN 1 AND 2048),
    utm_source    text        NOT NULL DEFAULT '' CHECK (char_length(utm_source) <= 100),
    utm_medium    text        NOT NULL DEFAULT '' CHECK (char_length(utm_medium) <= 100),
    utm_campaign  text        NOT NULL DEFAULT '' CHECK (char_length(utm_campaign) <= 100),
    utm_content   text        NOT NULL DEFAULT '' CHECK (char_length(utm_content) <= 100),
    utm_term      text        NOT NULL DEFAULT '' CHECK (char_length(utm_term) <= 100),
    label         text        NOT NULL DEFAULT '' CHECK (char_length(label) <= 120),
    active        boolean     NOT NULL DEFAULT true,
    expires_at    timestamptz,
    clicks        bigint      NOT NULL DEFAULT 0 CHECK (clicks >= 0),
    created_by    uuid REFERENCES users (id),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX short_links_code_key ON short_links (lower(code));
CREATE INDEX short_links_campaign_idx ON short_links (campaign_id, created_at DESC);
CREATE INDEX short_links_created_idx ON short_links (created_at DESC, id DESC);

CREATE TABLE link_clicks (
    id             uuid PRIMARY KEY,
    link_id        uuid        NOT NULL REFERENCES short_links (id),
    campaign_id    uuid REFERENCES campaigns (id),
    visitor_hash   text        NOT NULL,
    referrer_host  text        NOT NULL DEFAULT '' CHECK (char_length(referrer_host) <= 255),
    device         text        NOT NULL DEFAULT '' CHECK (device IN ('', 'mobile', 'tablet', 'desktop', 'bot')),
    is_bot         boolean     NOT NULL DEFAULT false,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX link_clicks_link_idx ON link_clicks (link_id, created_at);
CREATE INDEX link_clicks_campaign_idx ON link_clicks (campaign_id, created_at);
CREATE INDEX link_clicks_time_brin ON link_clicks USING brin (created_at);

CREATE TABLE user_attributions (
    user_id        uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    click_id       uuid REFERENCES link_clicks (id),
    link_id        uuid REFERENCES short_links (id),
    campaign_id    uuid REFERENCES campaigns (id),
    utm_source     text        NOT NULL DEFAULT '',
    utm_medium     text        NOT NULL DEFAULT '',
    utm_campaign   text        NOT NULL DEFAULT '',
    utm_content    text        NOT NULL DEFAULT '',
    utm_term       text        NOT NULL DEFAULT '',
    referrer_host  text        NOT NULL DEFAULT '',
    landing_path   text        NOT NULL DEFAULT '',
    first_seen_at  timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_attributions_campaign_idx ON user_attributions (campaign_id);
CREATE INDEX user_attributions_link_idx ON user_attributions (link_id);
CREATE INDEX user_attributions_click_idx ON user_attributions (click_id);
CREATE INDEX user_attributions_source_idx ON user_attributions (utm_source);

-- ---------------------------------------------------------------------------
-- Admin console: TOTP second factor and server-side sessions
-- ---------------------------------------------------------------------------
CREATE TABLE admin_mfa (
    user_id          uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    secret_enc       text        NOT NULL,
    last_used_step   bigint      NOT NULL DEFAULT 0,
    recovery_hashes  jsonb       NOT NULL DEFAULT '[]'::jsonb,
    enabled_at       timestamptz NOT NULL,
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE admin_sessions (
    id             uuid PRIMARY KEY,
    user_id        uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash     text        NOT NULL,
    ip_hash        text        NOT NULL DEFAULT '',
    user_agent     text        NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL,
    last_seen_at   timestamptz NOT NULL,
    expires_at     timestamptz NOT NULL,
    mfa_at         timestamptz NOT NULL,
    revoked_at     timestamptz,
    revoke_reason  text        NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX admin_sessions_token_key ON admin_sessions (token_hash);
CREATE INDEX admin_sessions_user_idx ON admin_sessions (user_id) WHERE revoked_at IS NULL;

-- Audit filters by action.
CREATE INDEX audit_logs_action_idx ON audit_logs (action, created_at DESC);
