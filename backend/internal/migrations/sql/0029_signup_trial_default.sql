-- revisions 5.0 (docs/platform-extensions.md §24.0): the pricing specification fixes the free
-- trial at 4 minutes, and the published Refund Policy (§2 "Free trial") says new users receive
-- "four free reading minutes". The sign-up offer stays an admin setting (§15.3): only an offer
-- nobody has edited since the 0016 seed (version 1, no editor, still 5 minutes) moves to 4.
-- An offer an admin has already set is left exactly as it is.
UPDATE site_settings
   SET value = jsonb_set(value, '{free_minutes}', '4'),
       version = version + 1,
       updated_at = now()
 WHERE key = 'signup_offer'
   AND version = 1
   AND updated_by IS NULL
   AND (value ->> 'free_minutes') = '5';
