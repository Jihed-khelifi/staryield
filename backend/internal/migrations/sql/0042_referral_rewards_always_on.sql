-- 0042_referral_rewards_always_on: the referral rewards no longer wait for a launch.
--
-- Staryield is out of beta: a qualified, unheld referral reward is always
-- granted by the next run of the referral-grants job, so the one-time launch
-- (site_settings.referral_program.launched_at, migration 0036) is gone. The
-- key is removed from the stored value (a stamped or a null one) and the
-- value CHECK no longer mentions it. No data moves: rewards already granted
-- or still waiting keep their rows; the waiting ones are granted by the next run.

UPDATE site_settings SET value = value - 'launched_at' WHERE key = 'referral_program';

ALTER TABLE site_settings DROP CONSTRAINT site_settings_referral_program_valid;
ALTER TABLE site_settings ADD CONSTRAINT site_settings_referral_program_valid
    CHECK (CASE WHEN key = 'referral_program'
        THEN jsonb_typeof(value -> 'enabled') = 'boolean'
         AND jsonb_typeof(value -> 'reward_minutes') = 'number'
         AND (value ->> 'reward_minutes')::numeric BETWEEN 1 AND 60
         AND (value ->> 'reward_minutes')::numeric = trunc((value ->> 'reward_minutes')::numeric)
         AND jsonb_typeof(value -> 'max_rewards_per_referrer') = 'number'
         AND (value ->> 'max_rewards_per_referrer')::numeric BETWEEN 1 AND 500
         AND (value ->> 'max_rewards_per_referrer')::numeric = trunc((value ->> 'max_rewards_per_referrer')::numeric)
        ELSE true END);
