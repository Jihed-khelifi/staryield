-- Growth: e-mail verification. The admin console filters users by whether
-- their address is verified and pages by (created_at, id): partial indexes
-- keep both filtered pages index-ordered (no sort of the whole table).
CREATE INDEX IF NOT EXISTS users_unverified_created_idx ON users (created_at DESC, id DESC) WHERE email_verified_at IS NULL;
CREATE INDEX IF NOT EXISTS users_verified_created_idx ON users (created_at DESC, id DESC) WHERE email_verified_at IS NOT NULL;

-- Sending a verification link reads the user's links of the last 24 hours,
-- used or not (the resend cooldown and the daily ceiling on e-mails to an
-- address nobody has proven yet). The existing partial index only covers
-- unused tokens.
CREATE INDEX IF NOT EXISTS user_action_tokens_user_issued_idx ON user_action_tokens (user_id, purpose, created_at DESC);
