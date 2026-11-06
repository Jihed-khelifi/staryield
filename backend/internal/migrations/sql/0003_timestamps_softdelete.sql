-- Staryield schema v3: timestamp hygiene and soft deletes.
--
-- Every mutable table carries created_at and updated_at; rows users delete
-- themselves carry deleted_at (soft delete, filtered by GORM). Append-only
-- tables are immutable by design and keep created_at only: credit_ledger,
-- psychic_earnings, audit_logs, analytics_events, link_clicks,
-- user_attributions, chat_messages and broadcasts.

-- touch_updated_at bumps updated_at on UPDATE when the statement left it
-- unchanged. Values written by the application (including the fake clock in
-- tests) always win, and the timestamp never moves backwards.
CREATE OR REPLACE FUNCTION touch_updated_at() RETURNS trigger AS $$
BEGIN
    IF NEW.updated_at IS NOT DISTINCT FROM OLD.updated_at THEN
        NEW.updated_at := GREATEST(now(), OLD.updated_at);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------------
-- Missing created_at (backfilled from the owning row's history)
-- ---------------------------------------------------------------------------
ALTER TABLE user_profiles ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();
UPDATE user_profiles p SET created_at = u.created_at FROM users u WHERE u.id = p.user_id;

ALTER TABLE wallets ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();
UPDATE wallets w SET created_at = u.created_at FROM users u WHERE u.id = w.user_id;

ALTER TABLE admin_mfa ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();
UPDATE admin_mfa SET created_at = enabled_at;

-- ---------------------------------------------------------------------------
-- Missing updated_at (backfilled from the latest known change)
-- ---------------------------------------------------------------------------
ALTER TABLE refresh_tokens ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
UPDATE refresh_tokens SET updated_at = GREATEST(created_at, COALESCE(last_used_at, created_at), COALESCE(revoked_at, created_at));

ALTER TABLE password_reset_tokens ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
UPDATE password_reset_tokens SET updated_at = COALESCE(used_at, created_at);

ALTER TABLE notifications ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
UPDATE notifications SET updated_at = COALESCE(read_at, created_at);

ALTER TABLE chat_attachments ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
UPDATE chat_attachments SET updated_at = created_at;

ALTER TABLE admin_sessions ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
UPDATE admin_sessions SET updated_at = GREATEST(created_at, last_seen_at, COALESCE(revoked_at, created_at));

-- ---------------------------------------------------------------------------
-- Saved messages: soft delete. The (user_id, message_id) key stays, so saving
-- a removed bookmark again revives the row (upsert) instead of duplicating it.
-- ---------------------------------------------------------------------------
ALTER TABLE saved_messages ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE saved_messages ADD COLUMN deleted_at timestamptz;
UPDATE saved_messages SET updated_at = created_at;
CREATE INDEX saved_messages_user_idx ON saved_messages (user_id, created_at DESC) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- Advisor services: kind (admin-curated "standard" or the advisor's own
-- "custom" offering), delivery time and soft delete. Orders keep referencing
-- a removed service, so deleting no longer conflicts with order history.
-- ---------------------------------------------------------------------------
ALTER TABLE psychic_services ADD COLUMN kind text NOT NULL DEFAULT 'standard'
    CHECK (kind IN ('standard', 'custom'));
ALTER TABLE psychic_services ADD COLUMN delivery_days smallint NOT NULL DEFAULT 7
    CHECK (delivery_days BETWEEN 1 AND 60);
ALTER TABLE psychic_services ADD COLUMN deleted_at timestamptz;
-- At most one live custom offering per advisor.
CREATE UNIQUE INDEX psychic_services_one_custom ON psychic_services (psychic_id)
    WHERE kind = 'custom' AND deleted_at IS NULL;
CREATE INDEX psychic_services_live_idx ON psychic_services (psychic_id, sort_order) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- updated_at maintenance for every mutable table
-- ---------------------------------------------------------------------------
CREATE TRIGGER users_touch_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER user_profiles_touch_updated_at BEFORE UPDATE ON user_profiles
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER refresh_tokens_touch_updated_at BEFORE UPDATE ON refresh_tokens
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER password_reset_tokens_touch_updated_at BEFORE UPDATE ON password_reset_tokens
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER psychics_touch_updated_at BEFORE UPDATE ON psychics
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER psychic_services_touch_updated_at BEFORE UPDATE ON psychic_services
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER wallets_touch_updated_at BEFORE UPDATE ON wallets
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER credit_packs_touch_updated_at BEFORE UPDATE ON credit_packs
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER credit_purchases_touch_updated_at BEFORE UPDATE ON credit_purchases
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER conversations_touch_updated_at BEFORE UPDATE ON conversations
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER chat_sessions_touch_updated_at BEFORE UPDATE ON chat_sessions
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER saved_messages_touch_updated_at BEFORE UPDATE ON saved_messages
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER custom_requests_touch_updated_at BEFORE UPDATE ON custom_requests
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER chat_attachments_touch_updated_at BEFORE UPDATE ON chat_attachments
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER psychic_reviews_touch_updated_at BEFORE UPDATE ON psychic_reviews
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER withdrawals_touch_updated_at BEFORE UPDATE ON withdrawals
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER notifications_touch_updated_at BEFORE UPDATE ON notifications
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER campaigns_touch_updated_at BEFORE UPDATE ON campaigns
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER short_links_touch_updated_at BEFORE UPDATE ON short_links
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER admin_mfa_touch_updated_at BEFORE UPDATE ON admin_mfa
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER admin_sessions_touch_updated_at BEFORE UPDATE ON admin_sessions
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
