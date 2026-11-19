-- 0035_calculator_reports: e-mail leads of the calculators' e-mail report
-- (revisions 6.0, docs/platform-extensions.md section 26.4).
--
-- One row per address. A visitor who asks for the report of a calculator by
-- e-mail becomes a lead: the address is kept sealed (AES-GCM, bound to the row
-- id) and identified by its keyed hash (the same hash as the e-mail outbox and
-- the suppression list). Birth details are NEVER stored here: the report is
-- recalculated and rendered into the e-mail outbox, and the outbox payload is
-- wiped as soon as the message is sent.
--
-- email_hash          keyed hash of the normalised address (unique)
-- email_sealed        sealed address (needed for the admin's CSV export)
-- locale / country    language of the page; trusted edge country (CF-IPCountry), '' when unknown
-- source              the first calculator the address asked a report for
-- kinds               every calculator it asked for (jsonb array, at most 3 entries)
-- reports_sent        report e-mails queued for this address
-- marketing_status    none (never opted in) | subscribed | unsubscribed
-- consent_*           the evidence of the latest opt-in: time, whether the box
--                     started ticked, consent text version (YYYY-MM-DD), IP hash
--                     (keyed with the analytics secret; never exported) and the
--                     cleaned user agent
-- unsubscribed_at     when the unsubscribe link was used
-- user_id             the account that uses the address (registration or a
--                     signed-in request); the lead is deleted with the account
-- last_request_at     the latest report request

CREATE TABLE email_leads (
    id                    uuid        PRIMARY KEY,
    email_hash            text        NOT NULL CHECK (char_length(email_hash) BETWEEN 32 AND 128),
    email_sealed          text        NOT NULL CHECK (email_sealed <> ''),
    locale                text        NOT NULL DEFAULT 'en' CHECK (locale IN ('en', 'fr', 'es')),
    country               text        NOT NULL DEFAULT '' CHECK (country = '' OR country ~ '^[A-Z]{2}$'),
    source                text        NOT NULL CHECK (source IN ('natal-chart', 'path-of-life', 'astrology')),
    kinds                 jsonb       NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(kinds) = 'array' AND jsonb_array_length(kinds) <= 3),
    reports_sent          integer     NOT NULL DEFAULT 0 CHECK (reports_sent >= 0),
    marketing_status      text        NOT NULL DEFAULT 'none' CHECK (marketing_status IN ('none', 'subscribed', 'unsubscribed')),
    consent_at            timestamptz,
    consent_prechecked    boolean,
    consent_text_version  text        CHECK (consent_text_version IS NULL OR consent_text_version ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'),
    consent_ip_hash       text,
    consent_user_agent    text        CHECK (consent_user_agent IS NULL OR char_length(consent_user_agent) <= 300),
    unsubscribed_at       timestamptz,
    user_id               uuid        REFERENCES users (id) ON DELETE SET NULL,
    last_request_at       timestamptz NOT NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    -- A subscribed address always carries the evidence of its consent.
    CONSTRAINT email_leads_consent_evidence CHECK (
        marketing_status <> 'subscribed'
        OR (consent_at IS NOT NULL AND consent_prechecked IS NOT NULL AND consent_text_version IS NOT NULL)),
    CONSTRAINT email_leads_unsubscribed_time CHECK ((marketing_status = 'unsubscribed') = (unsubscribed_at IS NOT NULL))
);

-- One lead per address; looked up by the request's address hash and by the
-- admin's exact-address search.
CREATE UNIQUE INDEX email_leads_hash_idx ON email_leads (email_hash);
-- The admin list, newest first (keyset on created_at, id), unfiltered and per status.
CREATE INDEX email_leads_recent_idx ON email_leads (created_at DESC, id DESC);
CREATE INDEX email_leads_status_idx ON email_leads (marketing_status, created_at DESC, id DESC);
-- The admin's calculator filter (kinds @> '["natal-chart"]').
CREATE INDEX email_leads_kinds_idx ON email_leads USING gin (kinds jsonb_path_ops);
-- Account erasure and the registration link.
CREATE INDEX email_leads_user_idx ON email_leads (user_id) WHERE user_id IS NOT NULL;

CREATE TRIGGER email_leads_touch_updated_at BEFORE UPDATE ON email_leads
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

COMMENT ON TABLE email_leads IS
    'Addresses that asked for a calculator report by e-mail (section 26.4). The address is sealed; birth details are never stored.';
