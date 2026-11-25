-- 0037_lifecycle: lifecycle messaging, the client onboarding checklist and the
-- activation reports of the admin console.
--
-- lifecycle_messages   one row per (user, journey, cycle): the claim that makes a
--                      nudge exactly-once across API instances (the instance whose
--                      INSERT ... ON CONFLICT DO NOTHING RETURNING returns the row
--                      is the only one that sends), the record of what was attempted
--                      and the conversion (a reading within the attribution window).
-- lifecycle_readers    per client: first and last completed reading and their count.
--                      It lets the due queries and the activation funnel start from
--                      an indexed time range instead of scanning chat_sessions.
--                      Written by the settlement hook (idempotent: recomputed from
--                      the client's sessions) and reconciled by the job.
-- user_profiles.onboarding_dismissed_at  the client hid the "Get started" checklist.
-- site_settings key 'lifecycle'  {"journeys": {key: {"enabled": bool, "delay_hours": 1..2160}},
--                                 "attribution_days": 1..30}

CREATE TABLE lifecycle_messages (
    id            uuid PRIMARY KEY,
    user_id       uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    journey       text        NOT NULL CHECK (journey IN ('verify_reminder', 'first_reading', 'after_first_reading', 'win_back')),
    cycle         integer     NOT NULL DEFAULT 1 CHECK (cycle >= 1),
    status        text        NOT NULL CHECK (status IN ('sent', 'skipped')),
    skip_reason   text        NOT NULL DEFAULT '' CHECK (char_length(skip_reason) <= 60),
    channels      jsonb       NOT NULL DEFAULT '{}'::jsonb,
    converted_at  timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT lifecycle_messages_skip_reason_valid CHECK ((status = 'skipped') = (skip_reason <> '')),
    CONSTRAINT lifecycle_messages_converted_sent CHECK (converted_at IS NULL OR status = 'sent')
);
-- Idempotency and the "already handled" probes of the due queries.
CREATE UNIQUE INDEX lifecycle_messages_once_key ON lifecycle_messages (user_id, journey, cycle);
-- The frequency cap: the latest sent message of a user.
CREATE INDEX lifecycle_messages_sent_idx ON lifecycle_messages (user_id, created_at DESC) WHERE status = 'sent';
-- Conversion: a user's messages still waiting for a reading.
CREATE INDEX lifecycle_messages_open_idx ON lifecycle_messages (user_id, created_at) WHERE status = 'sent' AND converted_at IS NULL;
-- Statistics per journey and period.
CREATE INDEX lifecycle_messages_stats_idx ON lifecycle_messages (journey, created_at);

CREATE TRIGGER lifecycle_messages_touch BEFORE UPDATE ON lifecycle_messages
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE lifecycle_readers (
    user_id          uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    first_reading_at timestamptz NOT NULL,
    last_reading_at  timestamptz NOT NULL,
    readings         integer     NOT NULL CHECK (readings >= 1),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT lifecycle_readers_order CHECK (last_reading_at >= first_reading_at)
);
-- The due queries walk a time band of these in keyset order (time, user).
CREATE INDEX lifecycle_readers_first_idx ON lifecycle_readers (first_reading_at, user_id);
CREATE INDEX lifecycle_readers_last_idx ON lifecycle_readers (last_reading_at, user_id);

CREATE TRIGGER lifecycle_readers_touch BEFORE UPDATE ON lifecycle_readers
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- A completed reading is a settled session with at least one billed minute (the
-- same rule as psychics.readings_count). The partial index serves the job's
-- reconciliation of the last hours.
CREATE INDEX chat_sessions_reading_settled_idx ON chat_sessions (settled_at)
    WHERE status = 'settled' AND billed_minutes > 0;

-- One set-based pass over the readings already on file.
INSERT INTO lifecycle_readers (user_id, first_reading_at, last_reading_at, readings)
SELECT client_id, min(settled_at), max(settled_at), count(*)
FROM chat_sessions
WHERE status = 'settled' AND billed_minutes > 0 AND settled_at IS NOT NULL
GROUP BY client_id
ON CONFLICT (user_id) DO NOTHING;

ALTER TABLE user_profiles ADD COLUMN onboarding_dismissed_at timestamptz;

-- site_settings: the key list is rewritten once more (it already holds the keys
-- of 0036 and earlier, the referral program included).
ALTER TABLE site_settings DROP CONSTRAINT site_settings_key_check;
ALTER TABLE site_settings ADD CONSTRAINT site_settings_key_check
    CHECK (key IN ('banners', 'signup_offer', 'free_mode', 'off_session_messages', 'pricing',
                   'trial_hold', 'payment_policies', 'calculator_report', 'referral_program', 'lifecycle'));

ALTER TABLE site_settings ADD CONSTRAINT site_settings_lifecycle_valid
    CHECK (CASE WHEN key = 'lifecycle'
        THEN jsonb_typeof(value -> 'journeys') = 'object'
         AND jsonb_typeof(value -> 'attribution_days') = 'number'
         AND (value ->> 'attribution_days')::numeric BETWEEN 1 AND 30
         AND (value ->> 'attribution_days')::numeric = trunc((value ->> 'attribution_days')::numeric)
        ELSE true END);

INSERT INTO site_settings (key, value) VALUES
    ('lifecycle', '{"journeys": {
        "verify_reminder":     {"enabled": true,  "delay_hours": 24},
        "first_reading":       {"enabled": true,  "delay_hours": 24},
        "after_first_reading": {"enabled": true,  "delay_hours": 24},
        "win_back":            {"enabled": false, "delay_hours": 336}},
      "attribution_days": 7}')
ON CONFLICT (key) DO NOTHING;
