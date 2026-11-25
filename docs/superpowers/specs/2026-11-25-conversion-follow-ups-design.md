# Conversion follow-ups (growth PRs) — design

Date: 2026-11-25. Scope: the nine "conversion follow-ups from the funnel audit"
listed as work left in staryield #6 and staryield-admin #4 (branch
`claude/upbeat-davinci-wyd07q`). Approved by the owner on 2026-11-25 ("go, 3 is
fine"), built in these PRs.

## Intent

Close the leaks between a visitor's first page and their first paid reading:
unanswered questions, missing calls to action, dead ends after a declined or
missed request, and a funnel report that loses visitors at sign-up.

## Goals

1. Real answers on `/faq`, in en/fr/es, including the growth topics.
2. Advisors with no reviews read "New advisor", never "5.0 (0 reviews)".
3. Signed-out visitors on phones always see Sign up and can reach Pricing.
4. Public advisor profiles look like the rest of the public site for visitors.
5. The calculator report success screen offers a next step; the report e-mail's
   links carry UTM tags.
6. The report form's footnote no longer contradicts the marketing checkbox.
7. An advisor who misses 3 requests in a row is set to Away and told why.
8. A client whose request is declined or expires sees other advisors online.
9. The analytics funnel counts a visitor and the account they created as one
   actor.

## Non-goals and anti-goals

- No change to the marketing checkbox text (it is consent evidence,
  `CalculatorReportConsentVersion` stays `2026-11-19`).
- No UTM tags on internal site links: they would overwrite the visitor's real
  first-touch source in `lib/attribution.ts`.
- No rewriting of `analytics_events` rows: the table is append-only (trigger
  `analytics_events_append_only`).
- Auto-away never sets an advisor Offline, never counts a decline as a miss and
  never triggers while the advisor is in a reading.
- No new public API for "online advisors": reuse `GET /psychics/featured`.

## Constraints

- Every user-facing string in en, fr and es (`frontend/src/i18n/dictionaries/copy-*.json`,
  backend `internal/i18n`, mail catalog). English source text is the key.
- Tests in every codebase touched: Go unit + `backend/tests/e2e`, vitest +
  Playwright in `frontend/`, vitest + Playwright in `staryield-admin/`.
- Queries through GORM (the raw-SQL guard `repository/rawsql_test.go` must pass);
  every new table/column change in a numbered migration from `0041`; every new
  table has `created_at`/`updated_at`.
- New or changed hot queries get an EXPLAIN check in `backend/tests/load`.
- Contract changes are documented in `docs/platform-extensions.md` (new section
  "27. Growth conversion follow-ups").

## Approaches considered (and picked)

- **Auto-away counting.** (a) Query the advisor's last 3 sessions at each
  expiry; (b) a counter on `psychics` (`missed_requests_in_row`), incremented
  atomically on a `request_timeout` expiry and reset to 0 on accept, decline,
  any availability change made by the advisor, and the auto-away itself.
  **Picked (b):** "in a row" needs a reset point; with (a) an advisor who comes
  back Online after an auto-away would be sent Away again by the next single
  miss, because the last 3 sessions are still misses.
- **Visit stitching.** (a) Back-fill `user_id` onto old events (blocked by the
  append-only trigger); (b) stamp `anonymous_id` on server events such as
  `user.registered` (fixes only new events, and only that one event);
  (c) resolve the actor at query time: `COALESCE(e.user_id, wv.user_id)` via a
  `LEFT JOIN web_visitors wv ON wv.anonymous_id = e.anonymous_id`, falling back
  to `anonymous_id`. **Picked (c)**, verified with EXPLAIN on the 200k-user load
  database; works for history too.
- **UTM placement.** E-mail links (traffic coming back to the site) vs internal
  CTAs. **Picked e-mail links only** (see anti-goals); the success-screen CTA is
  measured with the existing client analytics `placement` instead.
- **Alternative advisors.** A new component vs `AdvisorStrip` with an
  `excludeSlug` prop. **Picked the prop** (one strip, one data source).
- **Profile frame.** `PublicFrame` for everyone vs only for signed-out
  visitors. **Picked signed-out only**; signed-in users keep the app frame.

## Decisions per item

1. **FAQ answers** (`components/content/faq.tsx`, `content/faq/*`): replace the
   "contact our support team" placeholder of all 22 questions with real answers
   taken from the product's actual rules (credits, per-minute billing, refunds
   per the Refund Policy, password reset, settings, chat). Add a "Free minutes,
   invites and your account" group: free trial minutes, invite rewards (+5
   minutes per qualifying friend, granted at launch), confirming your e-mail
   address, the free calculators. Every answer goes into the FAQPage JSON-LD
   (`lib/seo/screen-json-ld.ts`). Answers must stay consistent with the Terms /
   Refund Policy and pass `faq.test.tsx`'s banned-wording rules (reword
   "verification" as "confirm your e-mail address" where needed).
2. **New advisor label** (`components/psychics/psychics.tsx` directory card and
   profile stat tile): when `reviews_count === 0` show the existing "New
   advisor" key and no star or count; otherwise unchanged.
3. **Mobile header** (`components/layout/site-header.tsx`): add Pricing
   (`/pricing`) to the public nav items (desktop nav and drawer). For
   signed-out visitors below `lg`, show a compact Sign up button next to the
   menu trigger (keeps `?next=` behaviour of the existing Sign up link).
4. **Profile frame** (`PsychicProfileView` incl. its loading and error
   branches): signed-out visitors get `PublicFrame` (public header with drawer,
   footer); signed-in users unchanged.
5. **Report success screen** (`components/calculators/report-email-dialog.tsx`):
   below the confirmation, an `AdvisorStrip` (up to 3, online first, heading
   "Ask an advisor about your results", placement `report_success`) and, for
   signed-out visitors, a "Create a free account" link to sign-up with
   `?next=` back to the calculator. Report e-mail links
   (`backend/internal/mail/calculator_report.go`: view report, advisor and
   sign-up CTAs; not unsubscribe) get
   `utm_source=staryield&utm_medium=email&utm_campaign=calculator_report&utm_content=<kind>`,
   inserted before any `#fragment`.
6. **Footnote** (same dialog): en "We use your e-mail address to send this
   report and, only if you tick the box above, to send you news and offers. We
   keep your birth details only until the report is sent." plus fr/es, and the
   Privacy Policy link as today. Must agree with the Privacy Policy section on
   report requests.
7. **Auto-away**: migration adds `psychics.missed_requests_in_row` (int, not
   null, default 0). On a `request_timeout` expiry (`ChatService.ExpireDue`,
   after commit) increment atomically; when it reaches the threshold and the
   advisor's stored availability is Online, set Away through the same path as
   `PsychicService.SetAvailability` (realtime presence + directory cache stay
   correct), reset the counter, and notify the advisor with a new kind
   `availability.auto_away` (in-app, push, e-mail; en/fr/es copy: "You were set
   to Away after 3 missed requests. Set yourself Online when you are ready.").
   Reset to 0 on accept, decline and any availability change by the advisor.
   Site setting `advisor_auto_away` = `{"enabled": true, "missed_requests": 3}`
   (2–10), admin API GET/PATCH with audit, admin console card (settings page
   for advisors/chat), Playwright + vitest. Advisor dashboard shows the
   notification like other kinds (icon + copy).
8. **Alternative advisors** (`components/chat/session-panel.tsx`, client side):
   when the latest session ended `declined` or `request_timeout`, show
   `AdvisorStrip` with `excludeSlug` = that advisor, heading "Other advisors
   online now", placement `chat_alternatives`. `AdvisorStrip` fetches 4 and
   filters before slicing to 3; hides itself when nothing remains.
9. **Visit stitching** (`repository/analytics.go` `Funnel`): actor =
   `COALESCE(e.user_id, wv.user_id)::text`, else `e.anonymous_id`, with the
   `web_visitors` left join on every step. EXPLAIN check at size; e2e: a visitor
   with page views before sign-up and `user.registered` after counts once and
   converts.

## Left to the builders

Exact component structure, test names, dictionary key wording in fr/es (native,
not literal), icon choice for the auto-away notification, where the admin card
sits in the console's navigation.
