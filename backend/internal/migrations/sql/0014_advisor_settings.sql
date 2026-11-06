-- 0014_advisor_settings: the advisor's own settings page.
--
-- * psychics.settings_version: optimistic concurrency for
--   GET/PATCH /psychic/settings (send it back as expected_version; any save
--   of the advisor-editable profile bumps it).
-- * psychic_photo_submissions: profile photos advisors upload themselves.
--   A submission stays private (private bucket, advisor-photos/ prefix)
--   until an administrator approves it; approval copies the image to the
--   public advisor bucket and makes it the profile photo. One pending
--   submission per advisor; a new upload supersedes it. Withdrawn and
--   superseded submissions are soft-deleted at once; rejected ones stay
--   visible to the advisor for a few days, then the clean-up removes their
--   image and soft-deletes them; soft-deleted rows are purged later.
--   object_key is cleared once the private object is gone, so the clean-up
--   can retry deletions that failed.
--
-- The advisor's custom offering needs no schema change: 0003 added
-- psychic_services.kind ('standard' | 'custom', at most one live custom
-- service per advisor), delivery_days (1–60) and deleted_at.

ALTER TABLE psychics ADD COLUMN settings_version integer NOT NULL DEFAULT 1
    CHECK (settings_version > 0);

CREATE TABLE psychic_photo_submissions (
    id           uuid PRIMARY KEY,
    psychic_id   uuid        NOT NULL REFERENCES psychics (user_id) ON DELETE CASCADE,
    status       text        NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'withdrawn')),
    object_key   text        NOT NULL DEFAULT ''
        CHECK (char_length(object_key) <= 300 AND (object_key = '' OR object_key LIKE 'advisor-photos/%')),
    content_type text        NOT NULL CHECK (content_type IN ('image/jpeg', 'image/png')),
    bytes        integer     NOT NULL CHECK (bytes BETWEEN 1 AND 20971520),
    width        integer     NOT NULL CHECK (width BETWEEN 1 AND 10000),
    height       integer     NOT NULL CHECK (height BETWEEN 1 AND 10000),
    reason       text        NOT NULL DEFAULT '' CHECK (char_length(reason) <= 500),
    reviewed_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    reviewed_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz,
    -- A pending submission always has its private image.
    CONSTRAINT psychic_photo_pending_has_object CHECK (status <> 'pending' OR object_key <> ''),
    -- A rejection always carries the reason shown to the advisor.
    CONSTRAINT psychic_photo_rejected_has_reason CHECK (status <> 'rejected' OR char_length(reason) >= 1),
    -- Exactly the reviewed states carry a review time.
    CONSTRAINT psychic_photo_reviewed CHECK ((status IN ('approved', 'rejected')) = (reviewed_at IS NOT NULL))
);

-- One live pending submission per advisor.
CREATE UNIQUE INDEX psychic_photo_one_pending ON psychic_photo_submissions (psychic_id)
    WHERE status = 'pending' AND deleted_at IS NULL;
-- The advisor's current submission (newest live row).
CREATE INDEX psychic_photo_current_idx ON psychic_photo_submissions (psychic_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
-- The daily upload cap counts every upload, withdrawn ones included; also
-- serves account erasure.
CREATE INDEX psychic_photo_psychic_idx ON psychic_photo_submissions (psychic_id, created_at);
-- Review queue per status (admin list).
CREATE INDEX psychic_photo_status_idx ON psychic_photo_submissions (status, created_at, id)
    WHERE deleted_at IS NULL;
-- Clean-up: rejected submissions past their retention.
CREATE INDEX psychic_photo_rejected_idx ON psychic_photo_submissions (reviewed_at)
    WHERE status = 'rejected' AND deleted_at IS NULL;
-- Clean-up: private images still stored for finished submissions (retries).
CREATE INDEX psychic_photo_leftover_idx ON psychic_photo_submissions (updated_at)
    WHERE object_key <> '' AND (deleted_at IS NOT NULL OR status IN ('approved', 'withdrawn'));
-- Clean-up: purge of soft-deleted rows.
CREATE INDEX psychic_photo_deleted_idx ON psychic_photo_submissions (deleted_at)
    WHERE deleted_at IS NOT NULL;

CREATE TRIGGER psychic_photo_submissions_touch_updated_at BEFORE UPDATE ON psychic_photo_submissions
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

COMMENT ON TABLE psychic_photo_submissions IS
    'Advisor-uploaded profile photos awaiting or past review. object_key points into the PRIVATE bucket (advisor-photos/); it is never exposed publicly and is cleared once the object is deleted.';
