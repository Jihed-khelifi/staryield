-- 0027_age_gate: 18+ age gate (docs/platform-extensions.md section 24.6).
--
-- adult_confirmed_at: a date of birth that meets the minimum age was saved.
-- age_restricted_at: an under-age date of birth was saved (the account may
-- not use chat, purchases or other client actions until an admin review).
-- A user with neither is "birth_date_required".

ALTER TABLE users ADD COLUMN IF NOT EXISTS adult_confirmed_at timestamptz;
ALTER TABLE users ADD COLUMN IF NOT EXISTS age_restricted_at timestamptz;

-- Backfill from the birth dates already on file, in one set-based pass over
-- the profiles that have one (a join on the primary key, no per-row work).
-- Rows with a date >= 18 years ago are confirmed; under 18 are restricted
-- (clients only: advisors and admins are vetted by Staryield and are always
-- "adult"). Accounts without a birth date stay "birth_date_required".
-- The cutoff is the UTC date, the same day the API's age check uses.
UPDATE users u
SET adult_confirmed_at = CASE WHEN p.birth_date <= c.cutoff THEN now() END,
    age_restricted_at  = CASE WHEN p.birth_date >  c.cutoff AND u.role = 'client' THEN now() END
FROM user_profiles p,
     (SELECT ((now() AT TIME ZONE 'UTC')::date - interval '18 years')::date AS cutoff) c
WHERE p.user_id = u.id
  AND p.birth_date IS NOT NULL
  AND (p.birth_date <= c.cutoff OR u.role = 'client')
  AND u.adult_confirmed_at IS NULL
  AND u.age_restricted_at IS NULL;
