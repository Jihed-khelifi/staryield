-- 0011_presence_intervals: history of when advisors were available, used for
-- the "Active hours" KPI. One open interval (ended_at IS NULL) per advisor at
-- most; the presence tracker closes it on every status change. History starts
-- when this migration is deployed.

CREATE TABLE psychic_presence_intervals (
    id          uuid PRIMARY KEY,
    psychic_id  uuid        NOT NULL REFERENCES psychics (user_id) ON DELETE CASCADE,
    status      text        NOT NULL CHECK (status IN ('online', 'busy', 'away')),
    started_at  timestamptz NOT NULL,
    ended_at    timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT psychic_presence_intervals_order CHECK (ended_at IS NULL OR ended_at >= started_at)
);

CREATE UNIQUE INDEX psychic_presence_intervals_open_key ON psychic_presence_intervals (psychic_id) WHERE ended_at IS NULL;
CREATE INDEX psychic_presence_intervals_psychic_idx ON psychic_presence_intervals (psychic_id, started_at DESC);

CREATE TRIGGER psychic_presence_intervals_touch_updated_at BEFORE UPDATE ON psychic_presence_intervals
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

COMMENT ON TABLE psychic_presence_intervals IS
    'Append-mostly availability history (online/busy/away). Rows are closed by setting ended_at; never deleted except with the advisor account.';
