-- Staryield schema v10: account settings, signed-in devices, one-time account
-- tokens, consent records and personal-data exports.

-- ---------------------------------------------------------------------------
-- Accounts
-- ---------------------------------------------------------------------------
-- A self-service closure keeps the account paused (deactivated_by =
-- 'closure') until users.closure_scheduled_at, when the eraser anonymises it.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_deactivated_by_check;
ALTER TABLE users ADD CONSTRAINT users_deactivated_by_check
    CHECK (deactivated_by IN ('', 'self', 'admin', 'closure'));

-- settings_version increments with every settings save so concurrent tabs
-- cannot overwrite each other silently (PATCH /me/settings expected_version).
ALTER TABLE user_profiles ADD COLUMN settings_version integer NOT NULL DEFAULT 1 CHECK (settings_version >= 1);

-- ---------------------------------------------------------------------------
-- Signed-in devices: one row per refresh-token family
-- ---------------------------------------------------------------------------
-- The id is the family id carried by access tokens, so a single device can be
-- signed out (its access tokens die at once through a Redis revocation key).
CREATE TABLE user_sessions (
    id            uuid PRIMARY KEY,
    user_id       uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    user_agent    text        NOT NULL DEFAULT '' CHECK (char_length(user_agent) <= 256),
    device_label  text        NOT NULL DEFAULT '' CHECK (char_length(device_label) <= 80),
    device_type   text        NOT NULL DEFAULT 'unknown' CHECK (device_type IN ('desktop', 'mobile', 'tablet', 'bot', 'unknown')),
    browser       text        NOT NULL DEFAULT '' CHECK (char_length(browser) <= 40),
    os            text        NOT NULL DEFAULT '' CHECK (char_length(os) <= 40),
    ip_hash       text        NOT NULL DEFAULT '' CHECK (char_length(ip_hash) <= 64),
    country_code  text        NOT NULL DEFAULT '' CHECK (country_code = '' OR country_code ~ '^[A-Z]{2}$'),
    last_seen_at  timestamptz NOT NULL,
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz,
    revoke_reason text        NOT NULL DEFAULT '' CHECK (char_length(revoke_reason) <= 64),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_sessions_active_idx ON user_sessions (user_id, last_seen_at DESC, id DESC) WHERE revoked_at IS NULL;
CREATE INDEX user_sessions_expiry_idx ON user_sessions (expires_at);
CREATE TRIGGER user_sessions_touch_updated_at BEFORE UPDATE ON user_sessions
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- Backfill one session per existing family (latest token wins for the device
-- details; a family is revoked only when every token in it is).
INSERT INTO user_sessions (id, user_id, user_agent, ip_hash, last_seen_at, expires_at, revoked_at, revoke_reason, created_at, updated_at)
SELECT DISTINCT ON (t.family_id)
       t.family_id, t.user_id, left(t.user_agent, 256), left(t.ip_hash, 64),
       agg.last_seen, agg.expires,
       CASE WHEN agg.live = 0 THEN agg.revoked END,
       CASE WHEN agg.live = 0 THEN left(t.revoke_reason, 64) ELSE '' END,
       agg.created, now()
FROM refresh_tokens t
JOIN (
    SELECT family_id, min(created_at) AS created, max(COALESCE(last_used_at, created_at)) AS last_seen,
           max(expires_at) AS expires, max(revoked_at) AS revoked, count(*) FILTER (WHERE revoked_at IS NULL) AS live
    FROM refresh_tokens
    GROUP BY family_id
) agg ON agg.family_id = t.family_id
ORDER BY t.family_id, t.created_at DESC, t.id DESC;

ALTER TABLE refresh_tokens ADD CONSTRAINT refresh_tokens_family_fkey
    FOREIGN KEY (family_id) REFERENCES user_sessions (id) ON DELETE CASCADE;

-- The application creates the device row (with its parsed user agent) before
-- the first token of a family. This trigger guarantees the row for any other
-- writer (e.g. an instance still running the previous release during a
-- rolling deploy) and keeps expiry and activity current on rotation.
CREATE FUNCTION ensure_user_session() RETURNS trigger AS $$
BEGIN
    INSERT INTO user_sessions (id, user_id, user_agent, ip_hash, last_seen_at, expires_at, created_at, updated_at)
    VALUES (NEW.family_id, NEW.user_id, left(NEW.user_agent, 256), left(NEW.ip_hash, 64), NEW.created_at, NEW.expires_at,
            NEW.created_at, NEW.created_at)
    ON CONFLICT (id) DO UPDATE
        SET expires_at   = GREATEST(user_sessions.expires_at, EXCLUDED.expires_at),
            last_seen_at = GREATEST(user_sessions.last_seen_at, EXCLUDED.last_seen_at),
            updated_at   = GREATEST(user_sessions.updated_at, EXCLUDED.updated_at);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER refresh_tokens_ensure_session BEFORE INSERT ON refresh_tokens
    FOR EACH ROW EXECUTE FUNCTION ensure_user_session();

-- ---------------------------------------------------------------------------
-- One-time account tokens (e-mail change, reactivation, closure cancel,
-- export downloads). Only the SHA-256 of the token is stored; payloads (e.g.
-- the requested new address) are sealed and bound to the row.
-- ---------------------------------------------------------------------------
CREATE TABLE user_action_tokens (
    id          uuid PRIMARY KEY,
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose     text        NOT NULL CHECK (purpose IN ('email_change', 'email_change_cancel', 'reactivate', 'closure_cancel', 'email_verify', 'export_download')),
    token_hash  text        NOT NULL CHECK (char_length(token_hash) = 64),
    payload_enc text        NOT NULL DEFAULT '' CHECK (char_length(payload_enc) <= 4096),
    subject_id  uuid,
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX user_action_tokens_hash_key ON user_action_tokens (token_hash);
CREATE INDEX user_action_tokens_open_idx ON user_action_tokens (user_id, purpose) WHERE used_at IS NULL;
CREATE INDEX user_action_tokens_expiry_idx ON user_action_tokens (expires_at);
CREATE TRIGGER user_action_tokens_touch_updated_at BEFORE UPDATE ON user_action_tokens
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- ---------------------------------------------------------------------------
-- Consent: current state per category plus an append-only history (proof of
-- consent: when, which policy version, from where).
-- ---------------------------------------------------------------------------
CREATE TABLE user_consents (
    user_id        uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    category       text        NOT NULL CHECK (category IN ('analytics', 'marketing_attribution', 'personalization')),
    granted        boolean     NOT NULL,
    policy_version text        NOT NULL CHECK (char_length(policy_version) BETWEEN 1 AND 32),
    source         text        NOT NULL DEFAULT 'settings' CHECK (source IN ('settings', 'signup', 'banner', 'admin', 'import')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, category)
);
CREATE TRIGGER user_consents_touch_updated_at BEFORE UPDATE ON user_consents
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE consent_events (
    id             uuid PRIMARY KEY,
    user_id        uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    category       text        NOT NULL CHECK (category IN ('analytics', 'marketing_attribution', 'personalization')),
    granted        boolean     NOT NULL,
    policy_version text        NOT NULL CHECK (char_length(policy_version) BETWEEN 1 AND 32),
    source         text        NOT NULL CHECK (source IN ('settings', 'signup', 'banner', 'admin', 'import')),
    ip_hash        text        NOT NULL DEFAULT '' CHECK (char_length(ip_hash) <= 64),
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX consent_events_user_idx ON consent_events (user_id, created_at DESC);
-- History is never rewritten (rows only disappear with a hard-deleted user).
CREATE TRIGGER consent_events_append_only BEFORE UPDATE ON consent_events
    FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
COMMENT ON TABLE consent_events IS 'Append-only consent history (no updated_at/deleted_at by design).';

-- ---------------------------------------------------------------------------
-- Personal-data exports (GDPR art. 15/20): built by a worker into a private
-- object, downloadable for 7 days with a single-use token.
-- ---------------------------------------------------------------------------
CREATE TABLE data_export_jobs (
    id             uuid PRIMARY KEY,
    user_id        uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status         text        NOT NULL CHECK (status IN ('queued', 'running', 'ready', 'failed', 'expired')),
    object_key     text        NOT NULL DEFAULT '' CHECK (char_length(object_key) <= 256),
    bytes          bigint      NOT NULL DEFAULT 0 CHECK (bytes >= 0),
    sha256         text        NOT NULL DEFAULT '' CHECK (sha256 = '' OR char_length(sha256) = 64),
    attempts       integer     NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    locked_until   timestamptz,
    error          text        NOT NULL DEFAULT '' CHECK (char_length(error) <= 300),
    requested_at   timestamptz NOT NULL,
    started_at     timestamptz,
    completed_at   timestamptz,
    expires_at     timestamptz,
    download_count integer     NOT NULL DEFAULT 0 CHECK (download_count >= 0),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz
);
CREATE UNIQUE INDEX data_export_jobs_active_key ON data_export_jobs (user_id)
    WHERE status IN ('queued', 'running') AND deleted_at IS NULL;
CREATE INDEX data_export_jobs_user_idx ON data_export_jobs (user_id, requested_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX data_export_jobs_due_idx ON data_export_jobs (requested_at) WHERE status IN ('queued', 'running');
CREATE INDEX data_export_jobs_expiry_idx ON data_export_jobs (expires_at) WHERE status = 'ready';
CREATE TRIGGER data_export_jobs_touch_updated_at BEFORE UPDATE ON data_export_jobs
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
