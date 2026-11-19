-- 0034_invoices: Staryield invoices and credit notes (revisions 6.0,
-- docs/platform-extensions.md section 26.6).
--
-- Every paid credit purchase gets an invoice and every refund a credit note,
-- whichever processor took the payment. A document is a snapshot: once issued
-- it never changes (buyer, seller, payment details and lines are copied at
-- issue time). The buyer's name, e-mail and country are sealed (AES-GCM, the
-- application sealer, associated data "invoice:<id>"); everything else is
-- plain accounting data that is kept when the account is erased.
--
-- Numbers are gapless per kind and calendar year (UTC of issued_at):
-- "SY-2026-000001" for invoices, "SYC-2026-000001" for credit notes. The
-- counter lives in invoice_sequences(scope, next); the application locks and
-- increments it in the same transaction that inserts the document, so a
-- rolled-back insert gives its number back.
--
--   scope  "invoice:2026" | "credit_note:2026"
--   next   the number the next document of the scope receives

CREATE TABLE invoice_sequences (
    scope      text        PRIMARY KEY,
    next       bigint      NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT invoice_sequences_scope_valid CHECK (scope ~ '^(invoice|credit_note):[0-9]{4}$'),
    CONSTRAINT invoice_sequences_next_valid CHECK (next >= 1)
);

COMMENT ON TABLE invoice_sequences IS 'Gapless invoice and credit note counters, one row per kind and year; locked in the issuing transaction';

CREATE TABLE invoices (
    id                      uuid        PRIMARY KEY,
    number                  text        NOT NULL,
    kind                    text        NOT NULL,
    user_id                 uuid        NOT NULL REFERENCES users (id),
    -- The purchase (invoice) or refund (credit note) payment transaction the
    -- document is about: one invoice and one credit note per transaction.
    payment_transaction_id  uuid        NOT NULL REFERENCES payment_transactions (id),
    credit_purchase_id      uuid        REFERENCES credit_purchases (id),
    -- Credit notes only: the invoice they correct (number copied: immutable).
    original_invoice_id     uuid        REFERENCES invoices (id),
    original_invoice_number text        NOT NULL DEFAULT '',
    currency                text        NOT NULL,
    subtotal_cents          bigint      NOT NULL,
    tax_cents               bigint      NOT NULL DEFAULT 0,
    total_cents             bigint      NOT NULL,
    -- [{description, quantity, unit_amount_cents, amount_cents, credits?}]
    lines                   jsonb       NOT NULL,
    -- {name, email, country?} sealed with the application sealer.
    buyer_sealed            text        NOT NULL,
    -- {legal_name, address[], email, website} copied from the company details.
    seller                  jsonb       NOT NULL,
    -- {provider, card?{brand,last4}, reference?, paid_at?, refunded_at?, statement_descriptor?}
    payment                 jsonb       NOT NULL,
    locale                  text        NOT NULL,
    issued_at               timestamptz NOT NULL,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT invoices_kind_valid CHECK (kind IN ('invoice', 'credit_note')),
    CONSTRAINT invoices_number_valid CHECK (number ~ '^(SY|SYC)-[0-9]{4}-[0-9]{6,}$'),
    CONSTRAINT invoices_number_matches_kind CHECK ((kind = 'invoice') = (number LIKE 'SY-%')),
    CONSTRAINT invoices_currency_valid CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT invoices_locale_valid CHECK (locale IN ('en', 'fr', 'es')),
    CONSTRAINT invoices_totals_valid CHECK (subtotal_cents > 0 AND tax_cents >= 0 AND total_cents = subtotal_cents + tax_cents),
    CONSTRAINT invoices_original_valid CHECK ((kind = 'credit_note') = (original_invoice_id IS NOT NULL))
);

-- The numbering contract and the "issued exactly once" guarantee.
CREATE UNIQUE INDEX invoices_number_key ON invoices (number);
CREATE UNIQUE INDEX invoices_transaction_kind_key ON invoices (payment_transaction_id, kind);

-- A client's own documents, newest first.
CREATE INDEX invoices_user_idx ON invoices (user_id, issued_at DESC, id DESC);
-- The admin list and the accounting export (by issue time, optionally by kind).
CREATE INDEX invoices_issued_idx ON invoices (issued_at DESC, id DESC);
CREATE INDEX invoices_kind_issued_idx ON invoices (kind, issued_at DESC, id DESC);
-- Admin search by number prefix ("SY-2026-0001"): LIKE 'prefix%' needs the pattern operator class.
CREATE INDEX invoices_number_prefix_idx ON invoices (number text_pattern_ops);
-- Purchase views: the invoice of a purchase.
CREATE INDEX invoices_purchase_idx ON invoices (credit_purchase_id, kind) WHERE credit_purchase_id IS NOT NULL;
-- Credit notes of an invoice.
CREATE INDEX invoices_original_idx ON invoices (original_invoice_id) WHERE original_invoice_id IS NOT NULL;

-- The backfill job (invoices-backfill) looks for settled credit-purchase and
-- refund payments of the last weeks that have no document yet; this keeps that
-- lookup proportional to recent activity, not to the whole ledger.
CREATE INDEX payment_transactions_invoicing_idx ON payment_transactions (updated_at)
    WHERE kind IN ('credit_purchase', 'refund') AND status IN ('succeeded', 'partially_refunded', 'refunded', 'disputed');

COMMENT ON TABLE invoices IS 'Immutable invoices and credit notes of credit purchases and refunds (snapshots)';
COMMENT ON COLUMN invoices.buyer_sealed IS 'Sealed JSON {name, email, country}; AD "invoice:<id>"';
COMMENT ON COLUMN invoices.original_invoice_number IS 'Credit notes: number of the corrected invoice';
