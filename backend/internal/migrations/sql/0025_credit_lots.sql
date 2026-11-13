-- 0025_credit_lots: purchased vs bonus credits (docs/platform-extensions.md
-- section 24.3).
--
-- Every credit in a wallet belongs to a lot. A lot remembers where the
-- credits came from (paid pack, membership grant, bonus) and the money they
-- were bought with, so refunds claw back exactly the purchase being refunded
-- and revenue can be recognised when paid credits are consumed.
--
-- Invariant (checked daily by the reconciliation job, never repaired
-- automatically): per user, SUM(credits_remaining) = wallets.balance. Held
-- credits are not in a lot: they left their lots when they were held.

CREATE TABLE credit_lots (
    id                          uuid PRIMARY KEY,
    user_id                     uuid        NOT NULL REFERENCES users (id),
    source                      text        NOT NULL CHECK (source IN ('purchase', 'membership', 'bonus')),
    -- Consumption order: purchased credits, then membership, then bonus.
    priority                    smallint    NOT NULL CHECK (priority BETWEEN 0 AND 2),
    -- The ledger entry type that created the lot, or legacy_backfill /
    -- legacy_return.
    origin                      text        NOT NULL,
    -- The movement that created the lot. NULL only for the one
    -- legacy_backfill lot of a pre-lots wallet, which has no movement of its
    -- own (the append-only ledger is not given marker rows for it).
    ledger_entry_id             uuid        REFERENCES credit_ledger (id),
    reference_type              text        NOT NULL DEFAULT '',
    reference_id                uuid,
    credits_total               bigint      NOT NULL CHECK (credits_total > 0),
    credits_remaining           bigint      NOT NULL CHECK (credits_remaining >= 0),
    paid_cents                  bigint      NOT NULL DEFAULT 0 CHECK (paid_cents >= 0),
    currency                    text        NOT NULL DEFAULT '',
    -- Revenue basis still in the lot: paid_cents * 1000 at creation.
    value_remaining_millicents  bigint      NOT NULL DEFAULT 0 CHECK (value_remaining_millicents >= 0),
    created_at                  timestamptz NOT NULL DEFAULT now(),
    updated_at                  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT credit_lots_remaining_le_total CHECK (credits_remaining <= credits_total),
    CONSTRAINT credit_lots_priority_matches_source CHECK (
        priority = CASE source WHEN 'purchase' THEN 0 WHEN 'membership' THEN 1 ELSE 2 END),
    CONSTRAINT credit_lots_value_le_paid CHECK (value_remaining_millicents <= paid_cents * 1000),
    CONSTRAINT credit_lots_paid_only_purchase CHECK (source = 'purchase' OR paid_cents = 0),
    CONSTRAINT credit_lots_movement CHECK (ledger_entry_id IS NOT NULL OR origin = 'legacy_backfill')
);
-- One lot per movement (NULLs, the legacy lots, are distinct).
CREATE UNIQUE INDEX credit_lots_ledger_entry_key ON credit_lots (ledger_entry_id);
-- One lot per credit purchase; also the lookup for refunds and statements.
CREATE UNIQUE INDEX credit_lots_purchase_key ON credit_lots (reference_id) WHERE reference_type = 'credit_purchase';
-- One lot per membership grant, found again when a refund reverses the grant.
CREATE UNIQUE INDEX credit_lots_grant_key ON credit_lots (reference_id) WHERE reference_type = 'subscription_grant';
-- Consumption: the next lots to spend, in order, for one user.
CREATE INDEX credit_lots_consume_idx ON credit_lots (user_id, priority, created_at, id) WHERE credits_remaining > 0;
-- Per-user statements (all lots, newest first).
CREATE INDEX credit_lots_user_idx ON credit_lots (user_id, created_at DESC, id DESC);
CREATE TRIGGER credit_lots_touch_updated_at BEFORE UPDATE ON credit_lots
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- Which lots a movement consumed (credits > 0) or restored (credits < 0).
-- Append-only like the ledger it hangs from (created_at only, section 0003).
CREATE TABLE credit_lot_allocations (
    id                uuid PRIMARY KEY,
    ledger_entry_id   uuid        NOT NULL REFERENCES credit_ledger (id),
    lot_id            uuid        NOT NULL REFERENCES credit_lots (id),
    user_id           uuid        NOT NULL REFERENCES users (id),
    credits           bigint      NOT NULL CHECK (credits <> 0),
    value_millicents  bigint      NOT NULL,
    -- For a restore: the movement whose allocations it reverses.
    restores_entry_id uuid        REFERENCES credit_ledger (id),
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT credit_lot_allocations_same_sign CHECK (
        (credits > 0 AND value_millicents >= 0) OR (credits < 0 AND value_millicents <= 0)),
    CONSTRAINT credit_lot_allocations_restore_sign CHECK (restores_entry_id IS NULL OR credits < 0)
);
CREATE INDEX credit_lot_allocations_entry_idx ON credit_lot_allocations (ledger_entry_id);
CREATE INDEX credit_lot_allocations_lot_idx ON credit_lot_allocations (lot_id);
CREATE INDEX credit_lot_allocations_restores_idx ON credit_lot_allocations (restores_entry_id) WHERE restores_entry_id IS NOT NULL;
CREATE TRIGGER credit_lot_allocations_append_only BEFORE UPDATE OR DELETE ON credit_lot_allocations
    FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
CREATE TRIGGER credit_lot_allocations_no_truncate BEFORE TRUNCATE ON credit_lot_allocations
    FOR EACH STATEMENT EXECUTE FUNCTION forbid_mutation();

-- Revenue basis of the paid credits a chat or a custom request consumed.
ALTER TABLE chat_sessions ADD COLUMN revenue_millicents bigint NOT NULL DEFAULT 0 CHECK (revenue_millicents >= 0);
ALTER TABLE custom_requests ADD COLUMN revenue_millicents bigint NOT NULL DEFAULT 0 CHECK (revenue_millicents >= 0);

-- ---------------------------------------------------------------------------
-- Backfill (set-based, one statement per step; runs inside the migration
-- transaction, so a failure leaves nothing half-assigned).
--
-- 1. Every succeeded credit purchase that has its ledger entry gets a purchase
--    lot, assigned newest first against what is left of the wallet balance
--    (remaining = the part of the balance not already assigned to newer
--    purchases, capped at the purchase's credits). Fully spent purchases get a
--    lot with 0 remaining, so refund quotes can tell how much was used.
--    value_remaining = floor(remaining * paid * 1000 / credits).
-- 2. The part of the balance no purchase explains becomes one bonus lot with
--    origin legacy_backfill (no ledger entry, see ledger_entry_id above).
-- ---------------------------------------------------------------------------
INSERT INTO credit_lots (id, user_id, source, priority, origin, ledger_entry_id, reference_type, reference_id,
                         credits_total, credits_remaining, paid_cents, currency, value_remaining_millicents,
                         created_at, updated_at)
SELECT gen_random_uuid(), r.user_id, 'purchase', 0, 'purchase', r.ledger_id, 'credit_purchase', r.id,
       r.credits, r.remaining, r.price_cents, r.currency,
       (r.remaining * r.price_cents * 1000) / r.credits,
       r.bought_at, now()
FROM (
    SELECT p.*,
           GREATEST(LEAST(p.balance - COALESCE((SUM(p.credits) OVER (
                    PARTITION BY p.user_id ORDER BY p.bought_at DESC, p.id DESC
                    ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING))::bigint, 0), p.credits), 0)::bigint AS remaining
    FROM (
        SELECT cp.id, cp.user_id, cp.credits, cp.price_cents, cp.currency,
               COALESCE(cp.completed_at, cp.created_at) AS bought_at, l.id AS ledger_id, w.balance
        FROM credit_purchases cp
        JOIN credit_ledger l ON l.idempotency_key = 'purchase:' || cp.id::text
        JOIN wallets w ON w.user_id = cp.user_id
        WHERE cp.status = 'succeeded'
    ) p
) r;

INSERT INTO credit_lots (id, user_id, source, priority, origin, ledger_entry_id, reference_type, reference_id,
                         credits_total, credits_remaining, paid_cents, currency, value_remaining_millicents,
                         created_at, updated_at)
SELECT gen_random_uuid(), w.user_id, 'bonus', 2, 'legacy_backfill', NULL, '', NULL,
       w.balance - COALESCE(s.assigned, 0), w.balance - COALESCE(s.assigned, 0), 0, '', 0,
       w.created_at, now()
FROM wallets w
LEFT JOIN (SELECT user_id, SUM(credits_remaining)::bigint AS assigned FROM credit_lots GROUP BY user_id) s ON s.user_id = w.user_id
WHERE w.balance > COALESCE(s.assigned, 0);
