-- 0022_auth_identities: sign in / sign up with Google (revisions 4.0,
-- docs/platform-extensions.md §23.1).
--
-- * user_identities links an account to an external identity provider. The
--   identity key is the provider's stable subject ("sub"), never the e-mail;
--   the e-mail is kept only to show "connected as …" and to explain a link.
-- * users.password_hash stays NOT NULL: the empty string means "no password"
--   (an account created with Google). No hash ever equals '', so such an
--   account cannot sign in with a password.

CREATE TABLE user_identities (
    id            uuid        PRIMARY KEY,
    user_id       uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider      text        NOT NULL CHECK (provider IN ('google')),
    subject       text        NOT NULL CHECK (char_length(subject) BETWEEN 1 AND 255),
    email         text        NOT NULL CHECK (char_length(email) <= 254 AND email = lower(email)),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    last_login_at timestamptz,
    CONSTRAINT user_identities_provider_subject_key UNIQUE (provider, subject),
    CONSTRAINT user_identities_user_provider_key UNIQUE (user_id, provider)
);

CREATE TRIGGER user_identities_touch_updated_at BEFORE UPDATE ON user_identities
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
