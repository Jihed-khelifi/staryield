-- 0012_waitlist: clients waiting for an advisor to come online.
--
-- A client joins an advisor's waiting list while the advisor cannot take a
-- chat. When the advisor comes online, their open entries are stamped
-- (available_at) and the waitlist job notifies each one once: members with
-- the waitlist-priority benefit straight away, everyone else after a short
-- lead if the advisor is still online. A notified entry is finished
-- (notified_at); leaving the list soft-deletes it (deleted_at). Open entries
-- expire after 7 days, finished ones are purged after 90 days, and an
-- erased account's rows are hard-deleted.

CREATE TABLE advisor_waitlist (
    id           uuid PRIMARY KEY,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    psychic_id   uuid        NOT NULL REFERENCES psychics (user_id) ON DELETE CASCADE,
    available_at timestamptz,
    notified_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz,
    CONSTRAINT advisor_waitlist_not_self CHECK (user_id <> psychic_id)
);

-- One open entry per client and advisor.
CREATE UNIQUE INDEX advisor_waitlist_open_key ON advisor_waitlist (user_id, psychic_id)
    WHERE deleted_at IS NULL AND notified_at IS NULL;
-- A client's open entries, newest first (GET /me/waitlist and the cap).
CREATE INDEX advisor_waitlist_user_idx ON advisor_waitlist (user_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL AND notified_at IS NULL;
-- An advisor's open queue (stamped when the advisor comes online).
CREATE INDEX advisor_waitlist_queue_idx ON advisor_waitlist (psychic_id)
    WHERE deleted_at IS NULL AND notified_at IS NULL AND available_at IS NULL;
-- Entries due for a notification, oldest stamp first.
CREATE INDEX advisor_waitlist_due_idx ON advisor_waitlist (available_at, created_at, id)
    WHERE deleted_at IS NULL AND notified_at IS NULL AND available_at IS NOT NULL;
-- Expiry of stale open entries and retention of finished ones.
CREATE INDEX advisor_waitlist_open_age_idx ON advisor_waitlist (created_at)
    WHERE deleted_at IS NULL AND notified_at IS NULL;
CREATE INDEX advisor_waitlist_finished_idx ON advisor_waitlist (updated_at)
    WHERE deleted_at IS NOT NULL OR notified_at IS NOT NULL;
-- Erasure of an advisor's queue.
CREATE INDEX advisor_waitlist_psychic_idx ON advisor_waitlist (psychic_id);

CREATE TRIGGER advisor_waitlist_touch_updated_at BEFORE UPDATE ON advisor_waitlist
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

COMMENT ON TABLE advisor_waitlist IS
    'Clients waiting for an advisor to come online. Open: deleted_at and notified_at NULL; available_at is set while the advisor is online and a notification is due.';
