-- Staryield schema v9: Premium Membership, saved payment methods, provider
-- routing and a single money-movement ledger (payment_transactions).
--
-- Card data never reaches Staryield: payment_methods hold sealed provider
-- references (AES-GCM, bound to the row) plus display metadata only.
-- Subscriptions, invoices, transactions and grants are financial records:
-- they are never deleted (no deleted_at); plans, customers, payment methods
-- and routing rules are soft-deleted.

-- ---------------------------------------------------------------------------
-- Plans and immutable plan versions
-- ---------------------------------------------------------------------------
CREATE TABLE subscription_plans (
    id           uuid PRIMARY KEY,
    code         text        NOT NULL CHECK (code ~ '^[a-z0-9][a-z0-9-]{1,39}$'),
    name         jsonb       NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(name) = 'object'),
    description  jsonb       NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(description) = 'object'),
    active       boolean     NOT NULL DEFAULT true,
    sort_order   integer     NOT NULL DEFAULT 0 CHECK (sort_order BETWEEN -1000 AND 1000),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz
);
CREATE UNIQUE INDEX subscription_plans_code_key ON subscription_plans (code) WHERE deleted_at IS NULL;

CREATE TABLE subscription_plan_versions (
    id                    uuid PRIMARY KEY,
    plan_id               uuid        NOT NULL REFERENCES subscription_plans (id),
    version               integer     NOT NULL CHECK (version >= 1),
    price_cents           bigint      NOT NULL CHECK (price_cents BETWEEN 50 AND 10000000),
    currency              text        NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    billing_interval      text        NOT NULL CHECK (billing_interval IN ('month', 'year')),
    interval_count        smallint    NOT NULL DEFAULT 1 CHECK (interval_count BETWEEN 1 AND 12),
    monthly_credit_grant  bigint      NOT NULL DEFAULT 0 CHECK (monthly_credit_grant BETWEEN 0 AND 1000000),
    chat_discount_bps     integer     NOT NULL DEFAULT 0 CHECK (chat_discount_bps BETWEEN 0 AND 5000),
    monthly_free_minutes  integer     NOT NULL DEFAULT 0 CHECK (monthly_free_minutes BETWEEN 0 AND 600),
    waitlist_priority     boolean     NOT NULL DEFAULT false,
    benefits              jsonb       NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(benefits) = 'object'),
    is_current            boolean     NOT NULL DEFAULT false,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX subscription_plan_versions_key ON subscription_plan_versions (plan_id, version);
CREATE UNIQUE INDEX subscription_plan_versions_current_key ON subscription_plan_versions (plan_id) WHERE is_current;

-- Price, interval and benefits are immutable: a change is a new version, so
-- every subscription keeps the terms it was sold.
CREATE OR REPLACE FUNCTION plan_version_immutable() RETURNS trigger AS $$
BEGIN
    IF NEW.plan_id <> OLD.plan_id OR NEW.version <> OLD.version OR NEW.price_cents <> OLD.price_cents
       OR NEW.currency <> OLD.currency OR NEW.billing_interval <> OLD.billing_interval
       OR NEW.interval_count <> OLD.interval_count OR NEW.monthly_credit_grant <> OLD.monthly_credit_grant
       OR NEW.chat_discount_bps <> OLD.chat_discount_bps OR NEW.monthly_free_minutes <> OLD.monthly_free_minutes
       OR NEW.waitlist_priority <> OLD.waitlist_priority THEN
        RAISE EXCEPTION 'subscription_plan_versions: price and benefits are immutable, create a new version'
            USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER subscription_plan_versions_immutable BEFORE UPDATE ON subscription_plan_versions
    FOR EACH ROW EXECUTE FUNCTION plan_version_immutable();
CREATE TRIGGER subscription_plan_versions_no_delete BEFORE DELETE ON subscription_plan_versions
    FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

-- ---------------------------------------------------------------------------
-- Provider customers and saved payment methods
-- ---------------------------------------------------------------------------
CREATE TABLE payment_customers (
    id                          uuid PRIMARY KEY,
    user_id                     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider                    text        NOT NULL CHECK (provider ~ '^[a-z][a-z0-9_]{1,31}$'),
    provider_customer_ref_enc   text        NOT NULL CHECK (char_length(provider_customer_ref_enc) BETWEEN 1 AND 1024),
    created_at                  timestamptz NOT NULL DEFAULT now(),
    updated_at                  timestamptz NOT NULL DEFAULT now(),
    deleted_at                  timestamptz
);
CREATE UNIQUE INDEX payment_customers_user_provider_key ON payment_customers (user_id, provider) WHERE deleted_at IS NULL;

CREATE TABLE payment_methods (
    id                        uuid PRIMARY KEY,
    user_id                   uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    customer_id               uuid        NOT NULL REFERENCES payment_customers (id),
    provider                  text        NOT NULL CHECK (provider ~ '^[a-z][a-z0-9_]{1,31}$'),
    provider_method_ref_enc   text        NOT NULL CHECK (char_length(provider_method_ref_enc) BETWEEN 1 AND 1024),
    fingerprint_hash          text        NOT NULL DEFAULT '' CHECK (char_length(fingerprint_hash) <= 128),
    brand                     text        NOT NULL CHECK (brand ~ '^[a-z][a-z_]{1,19}$'),
    last4                     text        NOT NULL CHECK (last4 ~ '^[0-9]{4}$'),
    exp_month                 smallint    NOT NULL CHECK (exp_month BETWEEN 1 AND 12),
    exp_year                  smallint    NOT NULL CHECK (exp_year BETWEEN 2000 AND 2100),
    country                   text        NOT NULL DEFAULT '' CHECK (country = '' OR country ~ '^[A-Z]{2}$'),
    is_default                boolean     NOT NULL DEFAULT false,
    status                    text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired', 'failed', 'detached')),
    network_txn_ref_enc       text        NOT NULL DEFAULT '' CHECK (char_length(network_txn_ref_enc) <= 1024),
    idempotency_key           text        CHECK (idempotency_key IS NULL OR char_length(idempotency_key) BETWEEN 8 AND 64),
    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now(),
    deleted_at                timestamptz
);
CREATE UNIQUE INDEX payment_methods_default_key ON payment_methods (user_id) WHERE is_default AND deleted_at IS NULL;
CREATE UNIQUE INDEX payment_methods_idempotency_key ON payment_methods (user_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX payment_methods_user_idx ON payment_methods (user_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- Subscriptions, invoices and grants
-- ---------------------------------------------------------------------------
CREATE TABLE subscriptions (
    id                      uuid PRIMARY KEY,
    user_id                 uuid        NOT NULL REFERENCES users (id),
    plan_id                 uuid        NOT NULL REFERENCES subscription_plans (id),
    plan_version_id         uuid        NOT NULL REFERENCES subscription_plan_versions (id),
    status                  text        NOT NULL CHECK (status IN ('incomplete', 'active', 'past_due', 'canceled', 'expired')),
    cancel_at_period_end    boolean     NOT NULL DEFAULT false,
    payment_method_id       uuid        REFERENCES payment_methods (id),
    provider                text        NOT NULL CHECK (provider ~ '^[a-z][a-z0-9_]{1,31}$'),
    billing_anchor_day      smallint    NOT NULL CHECK (billing_anchor_day BETWEEN 1 AND 31),
    current_period_start    timestamptz,
    current_period_end      timestamptz,
    period_index            integer     NOT NULL DEFAULT 0 CHECK (period_index >= 0),
    next_grant_at           timestamptz,
    grant_index             integer     NOT NULL DEFAULT 0 CHECK (grant_index >= 0),
    next_attempt_at         timestamptz,
    attempt_count           integer     NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    locked_until            timestamptz,
    grace_until             timestamptz,
    canceled_at             timestamptz,
    ended_at                timestamptz,
    cancel_reason           text        NOT NULL DEFAULT '' CHECK (char_length(cancel_reason) <= 300),
    idempotency_key         text        NOT NULL CHECK (char_length(idempotency_key) BETWEEN 8 AND 64),
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CHECK (current_period_end IS NULL OR current_period_end > current_period_start)
);
-- At most one open membership per user.
CREATE UNIQUE INDEX subscriptions_open_user_key ON subscriptions (user_id) WHERE status IN ('incomplete', 'active', 'past_due');
CREATE UNIQUE INDEX subscriptions_idempotency_key ON subscriptions (user_id, idempotency_key);
CREATE INDEX subscriptions_due_idx ON subscriptions (next_attempt_at) WHERE status IN ('active', 'past_due');
CREATE INDEX subscriptions_grant_due_idx ON subscriptions (next_grant_at) WHERE status IN ('active', 'past_due') AND next_grant_at IS NOT NULL;
CREATE INDEX subscriptions_grace_idx ON subscriptions (grace_until) WHERE status = 'past_due';
CREATE INDEX subscriptions_incomplete_idx ON subscriptions (created_at) WHERE status = 'incomplete';
CREATE INDEX subscriptions_user_idx ON subscriptions (user_id, created_at DESC, id DESC);
CREATE INDEX subscriptions_status_idx ON subscriptions (status, created_at DESC, id DESC);
CREATE INDEX subscriptions_method_idx ON subscriptions (payment_method_id) WHERE status IN ('incomplete', 'active', 'past_due');

CREATE TABLE subscription_invoices (
    id                       uuid PRIMARY KEY,
    subscription_id          uuid        NOT NULL REFERENCES subscriptions (id),
    user_id                  uuid        NOT NULL REFERENCES users (id),
    period_index             integer     NOT NULL CHECK (period_index >= 1),
    period_start             timestamptz NOT NULL,
    period_end               timestamptz NOT NULL,
    amount_cents             bigint      NOT NULL CHECK (amount_cents > 0),
    currency                 text        NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    status                   text        NOT NULL CHECK (status IN ('open', 'paid', 'failed', 'void', 'refunded')),
    attempt_count            integer     NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    paid_at                  timestamptz,
    payment_transaction_id   uuid,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    CHECK (period_end > period_start)
);
CREATE UNIQUE INDEX subscription_invoices_period_key ON subscription_invoices (subscription_id, period_index);

CREATE TABLE subscription_grants (
    id                     uuid PRIMARY KEY,
    subscription_id        uuid        NOT NULL REFERENCES subscriptions (id),
    user_id                uuid        NOT NULL REFERENCES users (id),
    grant_index            integer     NOT NULL CHECK (grant_index >= 1),
    period_index           integer     NOT NULL CHECK (period_index >= 1),
    credits                bigint      NOT NULL DEFAULT 0 CHECK (credits >= 0),
    credits_reversed       bigint      NOT NULL DEFAULT 0 CHECK (credits_reversed >= 0),
    free_minutes           integer     NOT NULL DEFAULT 0 CHECK (free_minutes >= 0),
    free_minutes_revoked   integer     NOT NULL DEFAULT 0 CHECK (free_minutes_revoked >= 0),
    free_minutes_after     integer     NOT NULL DEFAULT 0 CHECK (free_minutes_after >= 0),
    granted_at             timestamptz NOT NULL,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX subscription_grants_key ON subscription_grants (subscription_id, grant_index);

-- ---------------------------------------------------------------------------
-- Money-movement ledger (credit purchases, subscription charges, refunds)
-- ---------------------------------------------------------------------------
CREATE TABLE payment_transactions (
    id                       uuid PRIMARY KEY,
    user_id                  uuid        NOT NULL REFERENCES users (id),
    kind                     text        NOT NULL CHECK (kind IN ('credit_purchase', 'subscription_initial', 'subscription_renewal', 'refund')),
    status                   text        NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed', 'refunded', 'partially_refunded', 'disputed')),
    amount_cents             bigint      NOT NULL CHECK (amount_cents > 0),
    refunded_amount_cents    bigint      NOT NULL DEFAULT 0 CHECK (refunded_amount_cents >= 0),
    currency                 text        NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    credits                  bigint      NOT NULL DEFAULT 0 CHECK (credits >= 0),
    provider                 text        NOT NULL CHECK (char_length(provider) BETWEEN 1 AND 32),
    provider_ref             text        CHECK (provider_ref IS NULL OR char_length(provider_ref) <= 200),
    payment_method_id        uuid        REFERENCES payment_methods (id),
    subscription_id          uuid        REFERENCES subscriptions (id),
    invoice_id               uuid        REFERENCES subscription_invoices (id),
    credit_purchase_id       uuid        REFERENCES credit_purchases (id),
    parent_transaction_id    uuid        REFERENCES payment_transactions (id),
    failure_code             text        NOT NULL DEFAULT '' CHECK (char_length(failure_code) <= 64),
    description              text        NOT NULL DEFAULT '' CHECK (char_length(description) <= 200),
    idempotency_key          text        NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 128),
    attempt_no               integer     NOT NULL DEFAULT 1 CHECK (attempt_no >= 1),
    locked_until             timestamptz,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    CHECK (refunded_amount_cents <= amount_cents)
);
CREATE UNIQUE INDEX payment_transactions_idempotency_key ON payment_transactions (provider, idempotency_key);
CREATE UNIQUE INDEX payment_transactions_provider_ref_key ON payment_transactions (provider, provider_ref) WHERE provider_ref IS NOT NULL;
CREATE UNIQUE INDEX payment_transactions_purchase_key ON payment_transactions (credit_purchase_id) WHERE kind = 'credit_purchase';
CREATE INDEX payment_transactions_user_idx ON payment_transactions (user_id, created_at DESC, id DESC);
CREATE INDEX payment_transactions_created_idx ON payment_transactions (created_at DESC, id DESC);
CREATE INDEX payment_transactions_pending_idx ON payment_transactions (locked_until) WHERE status = 'pending';
CREATE INDEX payment_transactions_subscription_idx ON payment_transactions (subscription_id) WHERE subscription_id IS NOT NULL;
CREATE INDEX payment_transactions_parent_idx ON payment_transactions (parent_transaction_id) WHERE parent_transaction_id IS NOT NULL;

ALTER TABLE subscription_invoices ADD CONSTRAINT subscription_invoices_transaction_fkey
    FOREIGN KEY (payment_transaction_id) REFERENCES payment_transactions (id);

-- Existing credit purchases join the ledger (provider references kept).
INSERT INTO payment_transactions (id, user_id, kind, status, amount_cents, currency, credits, provider, provider_ref,
                                  credit_purchase_id, failure_code, description, idempotency_key, created_at, updated_at)
SELECT gen_random_uuid(), p.user_id, 'credit_purchase', p.status, p.price_cents, p.currency, p.credits, p.provider,
       p.provider_ref, p.id, left(p.failure_reason, 64), left(coalesce(k.name, 'Credit pack'), 200),
       'purchase:' || p.id::text, p.created_at, p.updated_at
FROM credit_purchases p
LEFT JOIN credit_packs k ON k.id = p.pack_id;

-- ---------------------------------------------------------------------------
-- Provider routing rules
-- ---------------------------------------------------------------------------
CREATE TABLE payment_routing_rules (
    id                 uuid PRIMARY KEY,
    name               text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    priority           integer     NOT NULL CHECK (priority BETWEEN 0 AND 100000),
    enabled            boolean     NOT NULL DEFAULT true,
    purpose            text        NOT NULL DEFAULT 'any' CHECK (purpose IN ('any', 'credits', 'subscription')),
    providers          jsonb       NOT NULL CHECK (jsonb_typeof(providers) = 'array' AND jsonb_array_length(providers) BETWEEN 1 AND 5),
    match_countries    jsonb       NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(match_countries) = 'array' AND jsonb_array_length(match_countries) <= 250),
    match_currencies   jsonb       NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(match_currencies) = 'array' AND jsonb_array_length(match_currencies) <= 50),
    match_plan_ids     jsonb       NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(match_plan_ids) = 'array' AND jsonb_array_length(match_plan_ids) <= 100),
    match_user_ids     jsonb       NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(match_user_ids) = 'array' AND jsonb_array_length(match_user_ids) <= 1000),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz
);
CREATE INDEX payment_routing_rules_priority_idx ON payment_routing_rules (priority, created_at) WHERE deleted_at IS NULL;

INSERT INTO payment_routing_rules (id, name, priority, enabled, purpose, providers)
VALUES (gen_random_uuid(), 'Default', 1000, true, 'any', '["sandbox_a"]');

-- ---------------------------------------------------------------------------
-- Webhook de-duplication (no raw payloads are stored)
-- ---------------------------------------------------------------------------
CREATE TABLE webhook_events (
    id               uuid PRIMARY KEY,
    provider         text        NOT NULL CHECK (char_length(provider) BETWEEN 1 AND 32),
    event_id         text        NOT NULL CHECK (char_length(event_id) BETWEEN 1 AND 200),
    event_type       text        NOT NULL CHECK (char_length(event_type) BETWEEN 1 AND 64),
    payload_sha256   text        NOT NULL CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    processed_at     timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX webhook_events_key ON webhook_events (provider, event_id);

-- ---------------------------------------------------------------------------
-- Ledger: membership grants
-- ---------------------------------------------------------------------------
ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_entry_type_check;
ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_entry_type_check CHECK (entry_type IN (
    'purchase', 'signup_bonus', 'chat_charge', 'hold', 'hold_release',
    'hold_refund', 'admin_adjustment', 'refund', 'subscription_grant', 'subscription_grant_reversal'));

-- ---------------------------------------------------------------------------
-- Chat sessions: member discount snapshot (wired by the chat flow). The list
-- rate is the advisor's rate; rate_per_minute is what the client pays.
-- ---------------------------------------------------------------------------
ALTER TABLE chat_sessions ADD COLUMN list_rate_per_minute bigint;
UPDATE chat_sessions SET list_rate_per_minute = rate_per_minute;
CREATE OR REPLACE FUNCTION chat_sessions_list_rate_default() RETURNS trigger AS $$
BEGIN
    IF NEW.list_rate_per_minute IS NULL THEN
        NEW.list_rate_per_minute := NEW.rate_per_minute;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER chat_sessions_list_rate BEFORE INSERT ON chat_sessions
    FOR EACH ROW EXECUTE FUNCTION chat_sessions_list_rate_default();
ALTER TABLE chat_sessions ALTER COLUMN list_rate_per_minute SET NOT NULL;
ALTER TABLE chat_sessions ADD CONSTRAINT chat_sessions_list_rate_check CHECK (list_rate_per_minute > 0);
ALTER TABLE chat_sessions ADD COLUMN discount_bps integer NOT NULL DEFAULT 0
    CHECK (discount_bps BETWEEN 0 AND 5000);

-- ---------------------------------------------------------------------------
-- Seed: the Premium Membership (benefits are set by an administrator).
-- ---------------------------------------------------------------------------
WITH plan AS (
    INSERT INTO subscription_plans (id, code, name, description, active, sort_order)
    VALUES (gen_random_uuid(), 'premium',
            '{"en": "Premium Membership", "es": "Membresía Premium", "fr": "Abonnement Premium"}',
            '{"en": "Monthly membership with member benefits.", "es": "Membresía mensual con ventajas para miembros.", "fr": "Abonnement mensuel avec avantages réservés aux membres."}',
            true, 0)
    RETURNING id
)
INSERT INTO subscription_plan_versions (id, plan_id, version, price_cents, currency, billing_interval, interval_count, is_current)
SELECT gen_random_uuid(), plan.id, 1, 999, 'USD', 'month', 1, true FROM plan;
