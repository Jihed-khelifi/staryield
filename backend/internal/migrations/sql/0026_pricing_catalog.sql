-- 0026_pricing_catalog: pricing catalog and the welcome (intro) offer
-- (revisions 5.0, docs/platform-extensions.md §24.1 and §24.2).
--
-- * credit_packs gains an immutable code, a kind (standard | intro), localized
--   name / tagline / label, the reference flag (the standard-rate pack savings
--   are measured against) and new_clients_only. Existing packs keep their rows
--   (purchases reference them), get legacy_<8 hex> codes and are deactivated;
--   the six canonical packs of the pricing spec are inserted in their place.
-- * site_settings gains the key pricing (advisor rate range, "most popular"
--   substantiation threshold, unit-economics assumptions for admin analytics).
-- * offer_redemptions is the state machine of the one-time welcome offer
--   (reserved -> redeemed | released): at most one active redemption per
--   account and per payment method (HMAC of the card fingerprint).
-- * Two partial indexes on credit_purchases serve the catalog's eligibility
--   check and the "most popular" aggregation.

-- ---------------------------------------------------------------------------
-- credit_packs
-- ---------------------------------------------------------------------------
ALTER TABLE credit_packs
    ADD COLUMN code             text,
    ADD COLUMN kind             text    NOT NULL DEFAULT 'standard',
    ADD COLUMN name_l10n        jsonb   NOT NULL DEFAULT '{}',
    ADD COLUMN tagline_l10n     jsonb   NOT NULL DEFAULT '{}',
    ADD COLUMN label_l10n       jsonb   NOT NULL DEFAULT '{}',
    ADD COLUMN reference        boolean NOT NULL DEFAULT false,
    ADD COLUMN new_clients_only boolean NOT NULL DEFAULT false;

-- Codes of the retired packs: legacy_<first 8 hex of the id>. Time-ordered ids
-- created in the same instant share their prefix, so a colliding prefix falls
-- back to the last 8 hex (the random part).
UPDATE credit_packs p
   SET code      = 'legacy_' || CASE WHEN c.n = 1 THEN c.prefix ELSE c.suffix END,
       name_l10n = jsonb_build_object('en', p.name),
       active    = false,
       featured  = false,
       -- after the canonical catalog (sort_order 1..6) in the admin list
       sort_order = 1000 + p.sort_order
  FROM (SELECT id,
               substr(replace(id::text, '-', ''), 1, 8)  AS prefix,
               substr(replace(id::text, '-', ''), 25, 8) AS suffix,
               count(*) OVER (PARTITION BY substr(replace(id::text, '-', ''), 1, 8)) AS n
          FROM credit_packs) c
 WHERE c.id = p.id;

ALTER TABLE credit_packs
    ALTER COLUMN code SET NOT NULL,
    ADD CONSTRAINT credit_packs_code_key UNIQUE (code),
    ADD CONSTRAINT credit_packs_code_format CHECK (code ~ '^[a-z0-9_]{2,40}$'),
    ADD CONSTRAINT credit_packs_kind_valid CHECK (kind IN ('standard', 'intro')),
    ADD CONSTRAINT credit_packs_name_l10n_valid CHECK (jsonb_typeof(name_l10n) = 'object'
        AND jsonb_typeof(name_l10n -> 'en') = 'string' AND char_length(name_l10n ->> 'en') BETWEEN 1 AND 60),
    ADD CONSTRAINT credit_packs_tagline_l10n_valid CHECK (jsonb_typeof(tagline_l10n) = 'object'),
    ADD CONSTRAINT credit_packs_label_l10n_valid CHECK (jsonb_typeof(label_l10n) = 'object'),
    -- The reference pack is a standard, active pack.
    ADD CONSTRAINT credit_packs_reference_valid CHECK (NOT reference OR (kind = 'standard' AND active));

-- At most one active intro pack and at most one reference pack.
CREATE UNIQUE INDEX credit_packs_one_intro_idx ON credit_packs (kind) WHERE kind = 'intro' AND active;
CREATE UNIQUE INDEX credit_packs_one_reference_idx ON credit_packs (reference) WHERE reference;
-- The public catalog reads active packs in canonical order.
CREATE INDEX credit_packs_active_order_idx ON credit_packs (sort_order, credits, id) WHERE active;

-- The canonical catalog (spec 1352:3210): name, credits, price and flags.
INSERT INTO credit_packs (id, code, kind, name, name_l10n, tagline_l10n, label_l10n, credits, price_cents, currency,
                          active, featured, reference, new_clients_only, sort_order)
VALUES
    (gen_random_uuid(), 'welcome_600', 'intro', 'Welcome Pack',
     '{"en": "Welcome Pack", "fr": "Pack de bienvenue", "es": "Pack de bienvenida"}',
     '{"en": "Extra time to explore your first paid reading.", "fr": "Du temps en plus pour découvrir votre première lecture payante.", "es": "Más tiempo para explorar tu primera lectura de pago."}',
     '{}', 600, 1000, 'USD', true, false, false, true, 1),
    (gen_random_uuid(), 'quick_150', 'standard', 'Quick Insight',
     '{"en": "Quick Insight", "fr": "Éclairage express", "es": "Consulta rápida"}',
     '{"en": "For one focused question.", "fr": "Pour une question précise.", "es": "Para una pregunta concreta."}',
     '{"en": "One focused question", "fr": "Une question précise", "es": "Una pregunta concreta"}',
     150, 2000, 'USD', true, false, true, false, 2),
    (gen_random_uuid(), 'starter_315', 'standard', 'Starter',
     '{"en": "Starter", "fr": "Démarrage", "es": "Inicial"}',
     '{"en": "A little more time for a fuller conversation.", "fr": "Un peu plus de temps pour une conversation plus complète.", "es": "Un poco más de tiempo para una conversación más completa."}',
     '{}', 315, 4000, 'USD', true, false, false, false, 3),
    (gen_random_uuid(), 'popular_675', 'standard', 'Most Popular',
     '{"en": "Most Popular", "fr": "Le plus populaire", "es": "Más popular"}',
     '{"en": "The best balance of price and reading time.", "fr": "Le meilleur équilibre entre prix et durée de lecture.", "es": "El mejor equilibrio entre precio y tiempo de lectura."}',
     '{}', 675, 8000, 'USD', true, true, false, false, 4),
    (gen_random_uuid(), 'deep_1350', 'standard', 'Deep Dive',
     '{"en": "Deep Dive", "fr": "Exploration approfondie", "es": "Inmersión profunda"}',
     '{"en": "For multiple questions or a longer reading.", "fr": "Pour plusieurs questions ou une lecture plus longue.", "es": "Para varias preguntas o una lectura más larga."}',
     '{}', 1350, 15000, 'USD', true, false, false, false, 5),
    (gen_random_uuid(), 'premium_2400', 'standard', 'Premium',
     '{"en": "Premium", "fr": "Premium", "es": "Premium"}',
     '{"en": "Our best ongoing rate for regular readings.", "fr": "Notre meilleur tarif durable pour les lectures régulières.", "es": "Nuestra mejor tarifa continuada para lecturas regulares."}',
     '{}', 2400, 25000, 'USD', true, false, false, false, 6);

-- ---------------------------------------------------------------------------
-- Pricing settings (site_settings key "pricing", versioned like the others)
--   {"rate_min": 1..10000, "rate_max": rate_min..10000, "popular_min_purchases": 1..100000,
--    "popular_window_days": 7..365, "processing_fee_bps": 0..3000, "processing_fixed_cents": 0..500}
-- ---------------------------------------------------------------------------
ALTER TABLE site_settings DROP CONSTRAINT site_settings_key_check;
ALTER TABLE site_settings ADD CONSTRAINT site_settings_key_check
    CHECK (key IN ('banners', 'signup_offer', 'free_mode', 'off_session_messages', 'pricing'));

ALTER TABLE site_settings ADD CONSTRAINT site_settings_pricing_valid
    CHECK (CASE WHEN key = 'pricing'
        THEN jsonb_typeof(value -> 'rate_min') = 'number'
         AND (value ->> 'rate_min')::numeric BETWEEN 1 AND 10000
         AND jsonb_typeof(value -> 'rate_max') = 'number'
         AND (value ->> 'rate_max')::numeric BETWEEN 1 AND 10000
         AND (value ->> 'rate_min')::numeric <= (value ->> 'rate_max')::numeric
         AND jsonb_typeof(value -> 'popular_min_purchases') = 'number'
         AND (value ->> 'popular_min_purchases')::numeric BETWEEN 1 AND 100000
         AND jsonb_typeof(value -> 'popular_window_days') = 'number'
         AND (value ->> 'popular_window_days')::numeric BETWEEN 7 AND 365
         AND jsonb_typeof(value -> 'processing_fee_bps') = 'number'
         AND (value ->> 'processing_fee_bps')::numeric BETWEEN 0 AND 3000
         AND jsonb_typeof(value -> 'processing_fixed_cents') = 'number'
         AND (value ->> 'processing_fixed_cents')::numeric BETWEEN 0 AND 500
        ELSE true END);

INSERT INTO site_settings (key, value) VALUES
    ('pricing', '{"rate_min": 30, "rate_max": 60, "popular_min_purchases": 25, "popular_window_days": 90, "processing_fee_bps": 800, "processing_fixed_cents": 30}')
ON CONFLICT (key) DO NOTHING;

-- ---------------------------------------------------------------------------
-- offer_redemptions: the welcome offer's state machine (§24.2)
-- ---------------------------------------------------------------------------
CREATE TABLE offer_redemptions (
    id          uuid        PRIMARY KEY,
    user_id     uuid        NOT NULL REFERENCES users (id),
    pack_id     uuid        NOT NULL REFERENCES credit_packs (id),
    purchase_id uuid        NOT NULL UNIQUE REFERENCES credit_purchases (id),
    provider    text        NOT NULL,
    -- HMAC-SHA256(provider ":" fingerprint); the raw fingerprint is never
    -- stored. Empty when no fingerprint was available (card check disabled).
    card_key    text        NOT NULL DEFAULT '',
    status      text        NOT NULL CHECK (status IN ('reserved', 'redeemed', 'released')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    redeemed_at timestamptz,
    released_at timestamptz,
    CONSTRAINT offer_redemptions_timestamps_valid CHECK (
        (status = 'redeemed') = (redeemed_at IS NOT NULL) AND (status = 'released') = (released_at IS NOT NULL))
);

-- One per customer and one per payment method, while reserved or redeemed.
CREATE UNIQUE INDEX offer_redemptions_user_active_idx
    ON offer_redemptions (user_id) WHERE status IN ('reserved', 'redeemed');
CREATE UNIQUE INDEX offer_redemptions_card_active_idx
    ON offer_redemptions (provider, card_key) WHERE card_key <> '' AND status IN ('reserved', 'redeemed');
-- Admin analytics count redemptions per period.
CREATE INDEX offer_redemptions_status_created_idx ON offer_redemptions (status, created_at);

CREATE TRIGGER offer_redemptions_touch_updated_at BEFORE UPDATE ON offer_redemptions
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

COMMENT ON TABLE offer_redemptions IS
    'Welcome-offer redemptions: reserved before the charge, redeemed on success, released on failure. A refund keeps it redeemed (Refund Policy §5).';

-- ---------------------------------------------------------------------------
-- credit_purchases: indexes of the catalog
-- ---------------------------------------------------------------------------
-- "Has this account ever paid?" (welcome-offer eligibility). A purchase that
-- was refunded afterwards still counts: it succeeded first.
CREATE INDEX credit_purchases_user_paid_idx ON credit_purchases (user_id) WHERE status IN ('succeeded', 'refunded');
-- "Most popular": succeeded purchases per pack inside a rolling window.
CREATE INDEX credit_purchases_succeeded_completed_idx
    ON credit_purchases (completed_at) INCLUDE (pack_id) WHERE status = 'succeeded';
