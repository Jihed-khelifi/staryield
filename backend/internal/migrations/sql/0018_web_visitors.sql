-- ---------------------------------------------------------------------------
-- 0018_web_visitors: calculator usage and conversion analytics (revisions 3.0).
--
-- Roll-up tables fed asynchronously by the analytics ingestion path. A visitor
-- is a browser's random anonymous id (never an IP, never a fingerprint); only
-- the host of a referrer is kept. Rows older than 25 months are deleted by the
-- web-visitors-retention job and an account's rows are erased with the account.
-- ---------------------------------------------------------------------------
CREATE TABLE web_visitors (
    anonymous_id   text        PRIMARY KEY CHECK (anonymous_id ~ '^[A-Za-z0-9_-]{8,64}$'),
    first_seen_at  timestamptz NOT NULL,
    last_seen_at   timestamptz NOT NULL,
    first_locale   text        NOT NULL DEFAULT 'en' CHECK (first_locale IN ('en', 'fr', 'es')),
    utm_source     text        NOT NULL DEFAULT '' CHECK (char_length(utm_source) <= 100),
    utm_medium     text        NOT NULL DEFAULT '' CHECK (char_length(utm_medium) <= 100),
    utm_campaign   text        NOT NULL DEFAULT '' CHECK (char_length(utm_campaign) <= 100),
    utm_content    text        NOT NULL DEFAULT '' CHECK (char_length(utm_content) <= 100),
    utm_term       text        NOT NULL DEFAULT '' CHECK (char_length(utm_term) <= 100),
    click_id       uuid,
    link_id        uuid REFERENCES short_links (id) ON DELETE SET NULL,
    campaign_id    uuid REFERENCES campaigns (id) ON DELETE SET NULL,
    referrer_host  text        NOT NULL DEFAULT '' CHECK (char_length(referrer_host) <= 255),
    user_id        uuid REFERENCES users (id) ON DELETE CASCADE,
    linked_at      timestamptz,
    signed_up_at   timestamptz,
    CONSTRAINT web_visitors_seen_order CHECK (last_seen_at >= first_seen_at),
    CONSTRAINT web_visitors_link_pair CHECK ((user_id IS NULL) = (linked_at IS NULL)),
    CONSTRAINT web_visitors_signup_needs_user CHECK (signed_up_at IS NULL OR user_id IS NOT NULL)
);
CREATE INDEX web_visitors_user_idx ON web_visitors (user_id) WHERE user_id IS NOT NULL;
-- Conversions page: newest sign-ups first, keyset (signed_up_at, user_id).
CREATE INDEX web_visitors_signup_idx ON web_visitors (signed_up_at DESC, user_id DESC) WHERE signed_up_at IS NOT NULL;
CREATE INDEX web_visitors_source_idx ON web_visitors (utm_source);
CREATE INDEX web_visitors_campaign_idx ON web_visitors (campaign_id) WHERE campaign_id IS NOT NULL;
CREATE INDEX web_visitors_link_idx ON web_visitors (link_id) WHERE link_id IS NOT NULL;
CREATE INDEX web_visitors_last_seen_idx ON web_visitors (last_seen_at);

CREATE TABLE calculator_visits (
    day           date    NOT NULL,
    anonymous_id  text    NOT NULL CHECK (anonymous_id ~ '^[A-Za-z0-9_-]{8,64}$'),
    kind          text    NOT NULL CHECK (kind IN ('natal-chart', 'path-of-life', 'astrology')),
    locale        text    NOT NULL CHECK (locale IN ('en', 'fr', 'es')),
    views         integer NOT NULL DEFAULT 0 CHECK (views >= 0),
    started       integer NOT NULL DEFAULT 0 CHECK (started >= 0),
    completed     integer NOT NULL DEFAULT 0 CHECK (completed >= 0),
    signed_in     boolean NOT NULL DEFAULT false,
    PRIMARY KEY (day, anonymous_id, kind, locale)
);
-- Range scans per calculator, and the per-visitor lookups of the conversions
-- page and of account erasure.
CREATE INDEX calculator_visits_kind_day_idx ON calculator_visits (kind, day);
CREATE INDEX calculator_visits_anon_idx ON calculator_visits (anonymous_id, day);
