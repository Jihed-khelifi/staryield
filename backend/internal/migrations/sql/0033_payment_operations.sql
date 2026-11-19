-- 0033_payment_operations: card-payment operations of revisions 6.0
-- (docs/platform-extensions.md section 26.5).
--
--   webhook_events            becomes the webhook log: status, error, attempts,
--                             livemode, the provider's own type and the object
--                             the event is about.
--   payment_disputes          one row per dispute or inquiry (credits frozen
--                             and restored, evidence deadlines).
--   payment_fraud_warnings    one row per early fraud warning.
--   payment_transactions      card country, card brand and last four, risk
--                             assessment, 3-D Secure result and the review flag.
--   payment_methods / payment_customers
--                             a hash of the provider reference, so a
--                             payment_method.detached or customer.deleted
--                             webhook can find the (sealed) local row.

-- ---------------------------------------------------------------------------
-- Webhook log
-- ---------------------------------------------------------------------------
ALTER TABLE webhook_events
    ADD COLUMN status        text    NOT NULL DEFAULT 'processed' CHECK (status IN ('received', 'processed', 'failed', 'ignored')),
    ADD COLUMN error         text    NOT NULL DEFAULT '' CHECK (char_length(error) <= 500),
    ADD COLUMN attempts      integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    ADD COLUMN livemode      boolean NOT NULL DEFAULT false,
    ADD COLUMN provider_type text    NOT NULL DEFAULT '' CHECK (char_length(provider_type) <= 100),
    ADD COLUMN object_ref    text    NOT NULL DEFAULT '' CHECK (char_length(object_ref) <= 200);

-- Rows written before the log was extended: processed ones are done, the
-- rest are re-applied when the provider redelivers them (as before).
UPDATE webhook_events SET attempts = 1 WHERE processed_at IS NOT NULL;
UPDATE webhook_events SET status = 'received' WHERE processed_at IS NULL;

-- The console pages the log newest first (all, per provider, per status); the
-- status page counts the last 24 hours of a provider from the index alone.
CREATE INDEX webhook_events_created_idx ON webhook_events (created_at DESC, id DESC);
CREATE INDEX webhook_events_provider_idx ON webhook_events (provider, created_at DESC, id DESC) INCLUDE (status);
CREATE INDEX webhook_events_status_idx ON webhook_events (status, created_at DESC, id DESC);

-- ---------------------------------------------------------------------------
-- Disputes
-- ---------------------------------------------------------------------------
CREATE TABLE payment_disputes (
    id                     uuid PRIMARY KEY,
    provider               text        NOT NULL CHECK (provider ~ '^[a-z][a-z0-9_]{1,31}$'),
    dispute_ref            text        NOT NULL CHECK (char_length(dispute_ref) BETWEEN 1 AND 200),
    transaction_id         uuid        NOT NULL REFERENCES payment_transactions (id),
    user_id                uuid        NOT NULL REFERENCES users (id),
    amount_cents           bigint      NOT NULL DEFAULT 0 CHECK (amount_cents >= 0),
    currency               text        NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    reason                 text        NOT NULL DEFAULT '' CHECK (char_length(reason) <= 100),
    status                 text        NOT NULL CHECK (status IN ('warning_needs_response', 'warning_under_review', 'warning_closed',
                                                                    'needs_response', 'under_review', 'won', 'lost', 'prevented')),
    -- The payment's status before the dispute froze it (restored when the dispute is won).
    prior_status           text        NOT NULL DEFAULT '' CHECK (prior_status IN ('', 'pending', 'succeeded', 'failed', 'refunded',
                                                                                  'partially_refunded', 'disputed')),
    evidence_due_by        timestamptz,
    evidence_submitted_at  timestamptz,
    credits_frozen         bigint      NOT NULL DEFAULT 0 CHECK (credits_frozen >= 0),
    credits_restored       bigint      NOT NULL DEFAULT 0 CHECK (credits_restored >= 0 AND credits_restored <= credits_frozen),
    funds_withdrawn        boolean     NOT NULL DEFAULT false,
    -- When the dispute's effects (payment status, credit freeze) were applied;
    -- an inquiry has none until it becomes a dispute.
    activated_at           timestamptz,
    closed_at              timestamptz,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX payment_disputes_ref_key ON payment_disputes (provider, dispute_ref);
CREATE INDEX payment_disputes_transaction_idx ON payment_disputes (transaction_id);
-- "Does this user have an open dispute?" (the purchase gate, one indexed probe).
CREATE INDEX payment_disputes_open_user_idx ON payment_disputes (user_id) WHERE status IN ('needs_response', 'under_review');
CREATE INDEX payment_disputes_status_idx ON payment_disputes (status, created_at DESC, id DESC);
CREATE INDEX payment_disputes_user_idx ON payment_disputes (user_id, created_at DESC, id DESC);
CREATE INDEX payment_disputes_created_idx ON payment_disputes (created_at DESC, id DESC);

-- ---------------------------------------------------------------------------
-- Early fraud warnings
-- ---------------------------------------------------------------------------
CREATE TABLE payment_fraud_warnings (
    id              uuid PRIMARY KEY,
    provider        text        NOT NULL CHECK (provider ~ '^[a-z][a-z0-9_]{1,31}$'),
    warning_ref     text        NOT NULL CHECK (char_length(warning_ref) BETWEEN 1 AND 200),
    transaction_id  uuid        NOT NULL REFERENCES payment_transactions (id),
    user_id         uuid        NOT NULL REFERENCES users (id),
    fraud_type      text        NOT NULL DEFAULT '' CHECK (char_length(fraud_type) <= 100),
    actionable      boolean     NOT NULL DEFAULT false,
    action          text        NOT NULL DEFAULT 'none' CHECK (action IN ('none', 'refunded', 'dismissed')),
    actioned_at     timestamptz,
    note            text        NOT NULL DEFAULT '' CHECK (char_length(note) <= 1000),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX payment_fraud_warnings_ref_key ON payment_fraud_warnings (provider, warning_ref);
CREATE INDEX payment_fraud_warnings_transaction_idx ON payment_fraud_warnings (transaction_id);
CREATE INDEX payment_fraud_warnings_action_idx ON payment_fraud_warnings (action, created_at DESC, id DESC);
CREATE INDEX payment_fraud_warnings_created_idx ON payment_fraud_warnings (created_at DESC, id DESC);

-- ---------------------------------------------------------------------------
-- Payments: card, risk, 3-D Secure, review
-- ---------------------------------------------------------------------------
ALTER TABLE payment_transactions
    ADD COLUMN card_country text     NOT NULL DEFAULT '' CHECK (card_country = '' OR card_country ~ '^[A-Z]{2}$'),
    ADD COLUMN card_brand   text     NOT NULL DEFAULT '' CHECK (card_brand = '' OR card_brand ~ '^[a-z][a-z_]{1,19}$'),
    ADD COLUMN card_last4   text     NOT NULL DEFAULT '' CHECK (card_last4 = '' OR card_last4 ~ '^[0-9]{4}$'),
    ADD COLUMN risk_level   text     NOT NULL DEFAULT '' CHECK (risk_level IN ('', 'normal', 'elevated', 'highest', 'not_assessed', 'unknown')),
    ADD COLUMN risk_score   smallint CHECK (risk_score IS NULL OR risk_score BETWEEN 0 AND 100),
    ADD COLUMN three_ds     text     NOT NULL DEFAULT '' CHECK (char_length(three_ds) <= 32),
    ADD COLUMN in_review    boolean  NOT NULL DEFAULT false;
CREATE INDEX payment_transactions_review_idx ON payment_transactions (updated_at) WHERE in_review;

-- ---------------------------------------------------------------------------
-- Saved cards and customers: find the sealed row of a provider reference
-- ---------------------------------------------------------------------------
ALTER TABLE payment_methods
    ADD COLUMN provider_method_ref_hash text NOT NULL DEFAULT '' CHECK (char_length(provider_method_ref_hash) <= 64);
ALTER TABLE payment_customers
    ADD COLUMN provider_customer_ref_hash text NOT NULL DEFAULT '' CHECK (char_length(provider_customer_ref_hash) <= 64);
CREATE INDEX payment_methods_ref_hash_idx ON payment_methods (provider, provider_method_ref_hash) WHERE provider_method_ref_hash <> '';
CREATE INDEX payment_customers_ref_hash_idx ON payment_customers (provider, provider_customer_ref_hash) WHERE provider_customer_ref_hash <> '';
