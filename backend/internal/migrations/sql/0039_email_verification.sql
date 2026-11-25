-- Growth: e-mail verification. The admin console filters users by whether
-- their address is verified and pages by (created_at, id): partial indexes
-- keep both filtered pages index-ordered (no sort of the whole table).
CREATE INDEX IF NOT EXISTS users_unverified_created_idx ON users (created_at DESC, id DESC) WHERE email_verified_at IS NULL;
CREATE INDEX IF NOT EXISTS users_verified_created_idx ON users (created_at DESC, id DESC) WHERE email_verified_at IS NOT NULL;
