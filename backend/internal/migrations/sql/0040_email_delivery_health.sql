-- 0040_email_delivery_health: the admin console's e-mail delivery view.
--
-- GET /admin/mail/health counts the outbox rows created in the last N days per
-- template, status and reason. Finished rows are purged after
-- STARYIELD_MAIL_RETENTION (30 days by default), so the table holds up to a
-- month of e-mail: without an index on created_at the counts scan all of it.
-- The template is included so the per-template grouping reads the index only.
CREATE INDEX IF NOT EXISTS email_outbox_created_idx ON email_outbox (created_at, template);
