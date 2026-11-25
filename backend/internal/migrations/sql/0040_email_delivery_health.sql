-- 0040_email_delivery_health: the admin console's e-mail delivery view.
--
-- GET /admin/mail/health reads the outbox rows created in the last N days.
-- Finished rows are purged after STARYIELD_MAIL_RETENTION (30 days by
-- default), so a week is a large share of the table and the planner would
-- scan all of it.
--
-- Counts per template, status and reason: a covering index, read with an
-- index-only scan (~23 ms for a week of 300k rows in the load dataset).
CREATE INDEX IF NOT EXISTS email_outbox_created_idx ON email_outbox (created_at, template, status, status_reason);

-- Latest problems (did not go out, or being retried after an error), newest
-- first: a partial index holding only those rows, read in order and stopped
-- after the page. The query repeats this predicate as literals so the index
-- also matches under generic (cached) plans.
CREATE INDEX IF NOT EXISTS email_outbox_problems_idx ON email_outbox (updated_at DESC, id DESC)
    WHERE status IN ('failed', 'dead', 'suppressed', 'cancelled') OR (status = 'queued' AND last_error <> '');
