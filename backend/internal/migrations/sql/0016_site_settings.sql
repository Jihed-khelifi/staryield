-- 0016_site_settings: runtime site configuration controlled from the admin
-- console (revisions 3.0, §15).
--
-- * site_settings: one row per setting document (jsonb value, optimistic
--   concurrency through version). Only the keys listed in the CHECK exist.
--     banners      {"enabled": bool, "rotation_seconds": 4..30}
--     signup_offer {"enabled": bool, "free_minutes": 1..60}
--     free_mode    {"enabled": bool, "reason", "ends_at", "enabled_at",
--                   "enabled_by", "disabled_at", "disabled_by"}
-- * site_banners: the promo bar items (localized copy as jsonb, at most 10
--   rows, enforced by the service under the banners row lock).
-- * chat_sessions.sponsored*: a chat requested while free mode is active is
--   sponsored for its whole life; billing workers read the column, never
--   the setting.

CREATE TABLE site_settings (
    key        text        PRIMARY KEY
        CHECK (key IN ('banners', 'signup_offer', 'free_mode')),
    value      jsonb       NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(value) = 'object'),
    version    integer     NOT NULL DEFAULT 1 CHECK (version > 0),
    updated_by uuid        REFERENCES users (id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT site_settings_banners_valid CHECK (CASE WHEN key = 'banners'
        THEN jsonb_typeof(value -> 'enabled') = 'boolean'
         AND jsonb_typeof(value -> 'rotation_seconds') = 'number'
         AND (value ->> 'rotation_seconds')::numeric BETWEEN 4 AND 30
        ELSE true END),
    CONSTRAINT site_settings_signup_offer_valid CHECK (CASE WHEN key = 'signup_offer'
        THEN jsonb_typeof(value -> 'enabled') = 'boolean'
         AND jsonb_typeof(value -> 'free_minutes') = 'number'
         AND (value ->> 'free_minutes')::numeric BETWEEN 1 AND 60
        ELSE true END),
    CONSTRAINT site_settings_free_mode_valid CHECK (CASE WHEN key = 'free_mode'
        THEN jsonb_typeof(value -> 'enabled') = 'boolean'
        ELSE true END)
);

CREATE TRIGGER site_settings_touch_updated_at BEFORE UPDATE ON site_settings
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

INSERT INTO site_settings (key, value) VALUES
    ('banners',      '{"enabled": true, "rotation_seconds": 6}'),
    ('signup_offer', '{"enabled": true, "free_minutes": 5}'),
    ('free_mode',    '{"enabled": false, "reason": ""}');

CREATE TABLE site_banners (
    id          uuid        PRIMARY KEY,
    text        jsonb       NOT NULL
        CHECK (jsonb_typeof(text) = 'object' AND jsonb_typeof(text -> 'en') = 'string'
               AND char_length(text ->> 'en') BETWEEN 1 AND 140),
    cta_label   jsonb       NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(cta_label) = 'object'),
    action_type text        NOT NULL DEFAULT 'none'
        CHECK (action_type IN ('link', 'none')),
    action_href text        NOT NULL DEFAULT ''
        CHECK (char_length(action_href) <= 300),
    audience    text        NOT NULL DEFAULT 'all'
        CHECK (audience IN ('all', 'anonymous', 'signed_in')),
    active      boolean     NOT NULL DEFAULT true,
    starts_at   timestamptz,
    ends_at     timestamptz,
    sort_order  integer     NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
    version     integer     NOT NULL DEFAULT 1 CHECK (version > 0),
    created_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    updated_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    -- A link always has a destination; "none" never does.
    CONSTRAINT site_banners_action_valid CHECK (
        (action_type = 'none' AND action_href = '') OR (action_type = 'link' AND action_href <> '')),
    CONSTRAINT site_banners_schedule_valid CHECK (starts_at IS NULL OR ends_at IS NULL OR ends_at > starts_at)
);

-- The public list and the admin list both read in display order.
CREATE INDEX site_banners_order_idx ON site_banners (sort_order, id);

CREATE TRIGGER site_banners_touch_updated_at BEFORE UPDATE ON site_banners
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

COMMENT ON TABLE site_banners IS
    'Promo bar items shown on the public site. Copy is plain text (clients render it as text, never HTML); the only placeholder is {free_minutes}.';

INSERT INTO site_banners (id, text, cta_label, action_type, action_href, audience, active, sort_order) VALUES (
    '5a1f0c3e-6b2d-4f7a-9c1e-3d8b7a4e2f10',
    jsonb_build_object(
        'en', '{free_minutes} MINUTES FREE + 80% OFF FOR NEW CUSTOMERS',
        'fr', '{free_minutes} MINUTES OFFERTES + 80 % DE RÉDUCTION POUR LES NOUVEAUX CLIENTS',
        'es', '{free_minutes} MINUTOS GRATIS + 80 % DE DESCUENTO PARA NUEVOS CLIENTES'),
    jsonb_build_object(
        'en', 'CLAIM INTRO OFFER →',
        'fr', 'PROFITEZ DE L''OFFRE DE BIENVENUE →',
        'es', 'APROVECHA LA OFERTA DE BIENVENIDA →'),
    'link', '/signup', 'anonymous', true, 0);

-- Sponsored (free mode) chats.
ALTER TABLE chat_sessions
    ADD COLUMN sponsored         boolean NOT NULL DEFAULT false,
    ADD COLUMN sponsored_minutes integer NOT NULL DEFAULT 0 CHECK (sponsored_minutes >= 0),
    ADD COLUMN sponsored_credits bigint  NOT NULL DEFAULT 0 CHECK (sponsored_credits >= 0),
    ADD CONSTRAINT chat_sessions_sponsored_consistent CHECK (sponsored OR (sponsored_minutes = 0 AND sponsored_credits = 0));

-- Free-mode statistics and the admin overview only look at sponsored chats.
CREATE INDEX chat_sessions_sponsored_idx ON chat_sessions (requested_at) WHERE sponsored;
