-- 0036_referrals: the referral loop. Every client can invite friends with a
-- personal link; for each invited friend who signs up with it, verifies the
-- e-mail address and completes a reading, the inviter earns free minutes,
-- granted at launch (the admin launches the rewards once).
--
--   referral_codes    one 8-character code per inviter, created on first use
--   referrals         one row per invited friend (the friend's account is the
--                     unique key), its status, flags and the reward
--   site_settings     key referral_program:
--                     {"enabled": bool, "reward_minutes": 1..60,
--                      "max_rewards_per_referrer": 1..500, "launched_at": null | RFC 3339}
--
-- Privacy: only the admin console ever reads who invited whom. The inviter's
-- API answers counts.

-- Codes use an alphabet without 0, O, 1, I, L and U (30 symbols), so they
-- survive being read aloud or typed from a screenshot.
CREATE TABLE referral_codes (
    user_id    uuid        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    code       text        NOT NULL CHECK (code ~ '^[ABCDEFGHJKMNPQRSTVWXYZ2-9]{8}$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX referral_codes_code_key ON referral_codes (code);

CREATE TRIGGER referral_codes_touch_updated_at BEFORE UPDATE ON referral_codes
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

COMMENT ON TABLE referral_codes IS
    'Personal referral code of an inviter (created on first use, 8 characters, unambiguous alphabet).';

-- status:
--   joined     the friend signed up with the link
--   qualified  verified e-mail and a settled reading: the inviter has earned reward_minutes
--   granted    the minutes were added to the inviter's wallet (after launch)
--   rejected   never rewarded (cap_reached, self_referral, or an admin's reason)
-- flagged rows (flags lists why: same_network, signup_burst, ...) wait for an
-- admin's decision and are never granted automatically.
-- referee_id is unique (one reward per friend) and becomes NULL when the
-- friend's account is erased; the row then stays only if it holds a decision.
CREATE TABLE referrals (
    id               uuid        PRIMARY KEY,
    referrer_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    referee_id       uuid        REFERENCES users (id) ON DELETE SET NULL,
    code             text        NOT NULL CHECK (code ~ '^[ABCDEFGHJKMNPQRSTVWXYZ2-9]{8}$'),
    status           text        NOT NULL DEFAULT 'joined'
                     CHECK (status IN ('joined', 'qualified', 'granted', 'rejected')),
    reward_minutes   integer     NOT NULL DEFAULT 0 CHECK (reward_minutes BETWEEN 0 AND 60),
    flagged          boolean     NOT NULL DEFAULT false,
    flags            jsonb       NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(flags) = 'array'),
    reject_reason    text        NOT NULL DEFAULT '' CHECK (char_length(reject_reason) <= 200),
    first_session_id uuid        REFERENCES chat_sessions (id) ON DELETE SET NULL,
    signup_ip_hash   text        NOT NULL DEFAULT '' CHECK (char_length(signup_ip_hash) <= 64),
    joined_at        timestamptz NOT NULL,
    qualified_at     timestamptz,
    granted_at       timestamptz,
    decided_by       uuid        REFERENCES users (id) ON DELETE SET NULL,
    decided_at       timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT referrals_not_self CHECK (referee_id IS NULL OR referee_id <> referrer_id),
    CONSTRAINT referrals_reward_state CHECK (
        status NOT IN ('qualified', 'granted') OR (qualified_at IS NOT NULL AND reward_minutes > 0)),
    CONSTRAINT referrals_granted_state CHECK ((status = 'granted') = (granted_at IS NOT NULL)),
    CONSTRAINT referrals_rejected_reason CHECK (status <> 'rejected' OR reject_reason <> '')
);

-- One reward per friend; also the lookup of a friend's row when they verify
-- their address or finish a reading.
CREATE UNIQUE INDEX referrals_referee_key ON referrals (referee_id);
-- The inviter's counters (GET /me/referrals): one aggregate over their rows.
CREATE INDEX referrals_referrer_status_idx ON referrals (referrer_id, status);
-- Burst detection: sign-ups credited to one inviter in the last 24 hours.
CREATE INDEX referrals_referrer_joined_idx ON referrals (referrer_id, joined_at DESC);
-- The grant job's claim: qualified, not flagged, oldest first.
CREATE INDEX referrals_grant_idx ON referrals (qualified_at, id) WHERE status = 'qualified' AND NOT flagged;
-- The reconciliation sweep (friends who verified and read before a hook ran).
CREATE INDEX referrals_joined_idx ON referrals (joined_at) WHERE status = 'joined';
-- The admin list, newest first (keyset on created_at, id), unfiltered, per status and flagged only.
CREATE INDEX referrals_recent_idx ON referrals (created_at DESC, id DESC);
CREATE INDEX referrals_status_recent_idx ON referrals (status, created_at DESC, id DESC);
CREATE INDEX referrals_flagged_recent_idx ON referrals (created_at DESC, id DESC) WHERE flagged;
-- The admin summary: counts and minutes per status without touching the table.
CREATE INDEX referrals_summary_idx ON referrals (status) INCLUDE (flagged, reward_minutes);

CREATE TRIGGER referrals_touch_updated_at BEFORE UPDATE ON referrals
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

COMMENT ON TABLE referrals IS
    'One row per invited friend. Read by the admin console only; inviters see counts. signup_ip_hash is a keyed hash, never an address.';

-- The program's setting (one more key of the same table as the other site settings).
ALTER TABLE site_settings DROP CONSTRAINT site_settings_key_check;
ALTER TABLE site_settings ADD CONSTRAINT site_settings_key_check
    CHECK (key IN ('banners', 'signup_offer', 'free_mode', 'off_session_messages', 'pricing',
                   'trial_hold', 'payment_policies', 'calculator_report', 'referral_program'));

ALTER TABLE site_settings ADD CONSTRAINT site_settings_referral_program_valid
    CHECK (CASE WHEN key = 'referral_program'
        THEN jsonb_typeof(value -> 'enabled') = 'boolean'
         AND jsonb_typeof(value -> 'reward_minutes') = 'number'
         AND (value ->> 'reward_minutes')::numeric BETWEEN 1 AND 60
         AND (value ->> 'reward_minutes')::numeric = trunc((value ->> 'reward_minutes')::numeric)
         AND jsonb_typeof(value -> 'max_rewards_per_referrer') = 'number'
         AND (value ->> 'max_rewards_per_referrer')::numeric BETWEEN 1 AND 500
         AND (value ->> 'max_rewards_per_referrer')::numeric = trunc((value ->> 'max_rewards_per_referrer')::numeric)
         AND COALESCE(jsonb_typeof(value -> 'launched_at'), 'null') IN ('null', 'string')
        ELSE true END);

INSERT INTO site_settings (key, value) VALUES
    ('referral_program', '{"enabled": true, "reward_minutes": 5, "max_rewards_per_referrer": 20, "launched_at": null}')
ON CONFLICT (key) DO NOTHING;
