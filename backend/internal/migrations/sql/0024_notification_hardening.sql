-- 0024_notification_hardening: idempotent, resumable broadcasts and batched
-- notification retention (docs/platform-extensions.md section 23.6).
--
-- A broadcast delivers one notification per recipient. The unique partial
-- index makes a re-run (resume after a crash, two runners) insert nothing
-- twice; broadcasts carry their fan-out position and a heartbeat so the
-- broadcast-recovery job can resume an interrupted one.

-- Any duplicate left by an older, non-idempotent run keeps its earliest row.
DELETE FROM notifications n
USING (
    SELECT id, row_number() OVER (PARTITION BY broadcast_id, user_id ORDER BY created_at, id) AS rn
    FROM notifications
    WHERE broadcast_id IS NOT NULL
) d
WHERE n.id = d.id AND d.rn > 1;

CREATE UNIQUE INDEX IF NOT EXISTS notifications_broadcast_user_key
    ON notifications (broadcast_id, user_id) WHERE broadcast_id IS NOT NULL;

ALTER TABLE broadcasts ADD COLUMN IF NOT EXISTS cursor_user_id uuid;
ALTER TABLE broadcasts ADD COLUMN IF NOT EXISTS heartbeat_at timestamptz;

-- Retention deletes old notifications in batches ordered by age.
CREATE INDEX IF NOT EXISTS notifications_created_idx ON notifications (created_at);

-- The recovery job looks for unfinished broadcasts.
CREATE INDEX IF NOT EXISTS broadcasts_unfinished_idx ON broadcasts (heartbeat_at) WHERE status IN ('queued', 'sending');
