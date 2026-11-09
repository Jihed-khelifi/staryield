-- 0021_off_session_messages: messages outside a live chat (revisions 3.3,
-- docs/platform-extensions.md §22.3).
--
-- * chat_messages.session_id becomes nullable: a message written while no
--   live chat is active or paused belongs to the conversation only
--   ("off-session"). System notices always belong to a session.
-- * Two partial indexes serve the per-sender rolling 24 h quota and the
--   admin totals; both only index off-session rows.
-- * site_settings gains the key off_session_messages
--     {"enabled": bool, "client_daily_limit": 0..500, "advisor_daily_limit": 0..500}
--   (kill switch and per-role daily caps), seeded enabled with 20 / 20.

ALTER TABLE chat_messages ALTER COLUMN session_id DROP NOT NULL;

ALTER TABLE chat_messages ADD CONSTRAINT chat_messages_system_has_session
    CHECK (kind <> 'system' OR session_id IS NOT NULL);

CREATE INDEX chat_messages_off_session_quota_idx
    ON chat_messages (conversation_id, sender_id, created_at) WHERE session_id IS NULL;

CREATE INDEX chat_messages_off_session_recent_idx
    ON chat_messages (created_at) WHERE session_id IS NULL;

ALTER TABLE site_settings DROP CONSTRAINT site_settings_key_check;
ALTER TABLE site_settings ADD CONSTRAINT site_settings_key_check
    CHECK (key IN ('banners', 'signup_offer', 'free_mode', 'off_session_messages'));

ALTER TABLE site_settings ADD CONSTRAINT site_settings_off_session_valid
    CHECK (CASE WHEN key = 'off_session_messages'
        THEN jsonb_typeof(value -> 'enabled') = 'boolean'
         AND jsonb_typeof(value -> 'client_daily_limit') = 'number'
         AND (value ->> 'client_daily_limit')::numeric BETWEEN 0 AND 500
         AND (value ->> 'client_daily_limit')::numeric = trunc((value ->> 'client_daily_limit')::numeric)
         AND jsonb_typeof(value -> 'advisor_daily_limit') = 'number'
         AND (value ->> 'advisor_daily_limit')::numeric BETWEEN 0 AND 500
         AND (value ->> 'advisor_daily_limit')::numeric = trunc((value ->> 'advisor_daily_limit')::numeric)
        ELSE true END);

INSERT INTO site_settings (key, value) VALUES
    ('off_session_messages', '{"enabled": true, "client_daily_limit": 20, "advisor_daily_limit": 20}')
ON CONFLICT (key) DO NOTHING;
