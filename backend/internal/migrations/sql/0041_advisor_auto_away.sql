-- 0041_advisor_auto_away: auto-away after missed live-chat requests.
--
-- psychics.missed_requests_in_row  live-chat requests that expired unanswered
--                      since the advisor last answered one (accepted or
--                      declined), changed their own availability or was set
--                      Away automatically. Incremented atomically by one
--                      UPDATE ... RETURNING on every request_timeout expiry
--                      and reset to 0 by the events above.
-- site_settings key 'advisor_auto_away'
--                      {"enabled": bool, "missed_requests": 2..10}: the
--                      threshold at which an Online advisor is set Away.

ALTER TABLE psychics ADD COLUMN missed_requests_in_row integer NOT NULL DEFAULT 0
    CONSTRAINT psychics_missed_requests_in_row_nonneg CHECK (missed_requests_in_row >= 0);

-- site_settings: the key list is rewritten once more (it already holds the keys
-- of 0037 and earlier).
ALTER TABLE site_settings DROP CONSTRAINT site_settings_key_check;
ALTER TABLE site_settings ADD CONSTRAINT site_settings_key_check
    CHECK (key IN ('banners', 'signup_offer', 'free_mode', 'off_session_messages', 'pricing',
                   'trial_hold', 'payment_policies', 'calculator_report', 'referral_program', 'lifecycle',
                   'advisor_auto_away'));

ALTER TABLE site_settings ADD CONSTRAINT site_settings_advisor_auto_away_valid
    CHECK (CASE WHEN key = 'advisor_auto_away'
        THEN jsonb_typeof(value -> 'enabled') = 'boolean'
         AND jsonb_typeof(value -> 'missed_requests') = 'number'
         AND (value ->> 'missed_requests')::numeric BETWEEN 2 AND 10
         AND (value ->> 'missed_requests')::numeric = trunc((value ->> 'missed_requests')::numeric)
        ELSE true END);

INSERT INTO site_settings (key, value) VALUES
    ('advisor_auto_away', '{"enabled": true, "missed_requests": 3}')
ON CONFLICT (key) DO NOTHING;
