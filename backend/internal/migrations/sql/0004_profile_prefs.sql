-- Staryield schema v4: account preferences, notification channels and the
-- columns later account-settings and advisor features build on.

-- ---------------------------------------------------------------------------
-- Profile preferences
-- ---------------------------------------------------------------------------
-- timezone: the account's IANA zone ('' = follow the browser); used for
--   "today" in horoscopes and for formatting dates in e-mails.
-- house_system: preferred natal-chart house system.
-- notify_*: delivery channels. In-app notifications are always recorded;
--   e-mail and push follow these switches (security and billing e-mails are
--   always sent). Marketing is opt-in.
-- birth_place_label: the birthplace as shown to the user ("Memphis, Tennessee,
--   United States").
-- share_birth_details_with_advisors: consent for advisors to see the
--   client's date of birth and birthplace (off by default).
-- An unknown birth time is already represented by birth_time = ''.
ALTER TABLE user_profiles ADD COLUMN timezone text NOT NULL DEFAULT ''
    CHECK (timezone = '' OR (char_length(timezone) <= 64 AND timezone ~ '^[A-Za-z][A-Za-z0-9_+/-]*$'));
ALTER TABLE user_profiles ADD COLUMN house_system text NOT NULL DEFAULT 'placidus'
    CHECK (house_system IN ('placidus', 'whole_sign', 'equal'));
ALTER TABLE user_profiles ADD COLUMN notify_email boolean NOT NULL DEFAULT true;
ALTER TABLE user_profiles ADD COLUMN notify_push boolean NOT NULL DEFAULT false;
ALTER TABLE user_profiles ADD COLUMN notify_marketing boolean NOT NULL DEFAULT false;
ALTER TABLE user_profiles ADD COLUMN birth_place_label text NOT NULL DEFAULT ''
    CHECK (char_length(birth_place_label) <= 160);
ALTER TABLE user_profiles ADD COLUMN share_birth_details_with_advisors boolean NOT NULL DEFAULT false;

-- Accounts that had muted every notification type keep e-mail off.
UPDATE user_profiles SET notify_email = (notify_horoscope OR notify_events OR notify_chart);
UPDATE user_profiles SET birth_place_label = birth_place WHERE birth_place_label = '' AND birth_place <> '';

-- ---------------------------------------------------------------------------
-- Accounts: self-service deactivation and scheduled closure
-- ---------------------------------------------------------------------------
-- deactivated_by distinguishes a user pausing their own account ('self',
-- reactivated by signing in again) from staff action ('admin').
ALTER TABLE users ADD COLUMN deactivated_by text NOT NULL DEFAULT ''
    CHECK (deactivated_by IN ('', 'self', 'admin'));
ALTER TABLE users ADD COLUMN closure_scheduled_at timestamptz;
-- Every deactivation before this release was made by staff.
UPDATE users SET deactivated_by = 'admin' WHERE status = 'deactivated';
CREATE INDEX users_closure_due_idx ON users (closure_scheduled_at) WHERE closure_scheduled_at IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Advisors: sealed phone number, presence history anchor, photo review
-- ---------------------------------------------------------------------------
ALTER TABLE psychics ADD COLUMN phone_sealed text NOT NULL DEFAULT ''
    CHECK (char_length(phone_sealed) <= 512);
ALTER TABLE psychics ADD COLUMN last_online_at timestamptz;
ALTER TABLE psychics ADD COLUMN photo_review_status text NOT NULL DEFAULT 'approved'
    CHECK (photo_review_status IN ('approved', 'pending', 'rejected'));
