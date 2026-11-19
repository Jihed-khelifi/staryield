-- 0031_revisions_6_settings: admin settings of revisions 6.0
-- (docs/platform-extensions.md section 26.0). Written once for the whole
-- round so later migrations never rewrite the key list again.
--
-- site_settings gains three keys, each validated and seeded:
--
--   trial_hold         {"after_trial": bool, "after_paid": bool,
--                       "hold_seconds": 30..300, "payment_grace_seconds": 0..300}
--                      The one-minute hold when a client's free minutes (or,
--                      when enabled, paid credits) run out mid-reading (§26.1).
--   payment_policies   {"block_purchases_on_dispute": bool, "dispute_clawback": bool,
--                       "efw_auto_refund": bool, "three_d_secure": "automatic"|"any"}
--                      Dispute and fraud-warning handling of card payments (§26.5).
--   calculator_report  {"enabled": bool, "marketing_prechecked_countries": ["US", ...]}
--                      The calculators' e-mail report and where its marketing
--                      box starts ticked (§26.4). Country codes are ISO 3166-1
--                      alpha-2, upper case, at most 60 of them.

ALTER TABLE site_settings DROP CONSTRAINT site_settings_key_check;
ALTER TABLE site_settings ADD CONSTRAINT site_settings_key_check
    CHECK (key IN ('banners', 'signup_offer', 'free_mode', 'off_session_messages', 'pricing',
                   'trial_hold', 'payment_policies', 'calculator_report'));

ALTER TABLE site_settings ADD CONSTRAINT site_settings_trial_hold_valid
    CHECK (CASE WHEN key = 'trial_hold'
        THEN jsonb_typeof(value -> 'after_trial') = 'boolean'
         AND jsonb_typeof(value -> 'after_paid') = 'boolean'
         AND jsonb_typeof(value -> 'hold_seconds') = 'number'
         AND (value ->> 'hold_seconds')::numeric BETWEEN 30 AND 300
         AND (value ->> 'hold_seconds')::numeric = trunc((value ->> 'hold_seconds')::numeric)
         AND jsonb_typeof(value -> 'payment_grace_seconds') = 'number'
         AND (value ->> 'payment_grace_seconds')::numeric BETWEEN 0 AND 300
         AND (value ->> 'payment_grace_seconds')::numeric = trunc((value ->> 'payment_grace_seconds')::numeric)
        ELSE true END);

ALTER TABLE site_settings ADD CONSTRAINT site_settings_payment_policies_valid
    CHECK (CASE WHEN key = 'payment_policies'
        THEN jsonb_typeof(value -> 'block_purchases_on_dispute') = 'boolean'
         AND jsonb_typeof(value -> 'dispute_clawback') = 'boolean'
         AND jsonb_typeof(value -> 'efw_auto_refund') = 'boolean'
         AND value ->> 'three_d_secure' IN ('automatic', 'any')
        ELSE true END);

ALTER TABLE site_settings ADD CONSTRAINT site_settings_calculator_report_valid
    CHECK (CASE WHEN key = 'calculator_report'
        THEN jsonb_typeof(value -> 'enabled') = 'boolean'
         AND jsonb_typeof(value -> 'marketing_prechecked_countries') = 'array'
         AND jsonb_array_length(value -> 'marketing_prechecked_countries') <= 60
        ELSE true END);

INSERT INTO site_settings (key, value) VALUES
    ('trial_hold', '{"after_trial": true, "after_paid": false, "hold_seconds": 60, "payment_grace_seconds": 120}'),
    ('payment_policies', '{"block_purchases_on_dispute": true, "dispute_clawback": true, "efw_auto_refund": false, "three_d_secure": "automatic"}'),
    ('calculator_report', '{"enabled": true, "marketing_prechecked_countries": ["US"]}')
ON CONFLICT (key) DO NOTHING;

-- Credit ledger: three new movement types (one list for the whole round).
--   chat_reveal       the client paid one minute to read the replies hidden by
--                     an expired hold (§26.1)
--   dispute_reversal  unused credits of a disputed purchase taken back (§26.5)
--   dispute_restore   those credits given back when the dispute is won (§26.5)
ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_entry_type_check;
ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_entry_type_check CHECK (entry_type IN (
    'purchase', 'signup_bonus', 'chat_charge', 'hold', 'hold_release',
    'hold_refund', 'admin_adjustment', 'refund', 'subscription_grant', 'subscription_grant_reversal',
    'chat_reveal', 'dispute_reversal', 'dispute_restore'));

-- Advisor earnings: a reveal pays the advisor's share of that minute (§26.1).
ALTER TABLE psychic_earnings DROP CONSTRAINT psychic_earnings_source_type_check;
ALTER TABLE psychic_earnings ADD CONSTRAINT psychic_earnings_source_type_check
    CHECK (source_type IN ('chat_session', 'custom_request', 'chat_hold_reveal'));
