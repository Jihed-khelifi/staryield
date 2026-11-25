# Conversion Follow-ups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the nine conversion follow-ups of the growth PRs (FAQ answers, New advisor label, mobile header, public profile frame, report next step + UTM, footnote, auto-away, alternative advisors, visit stitching).

**Architecture:** Five independent slices (A auto-away, B visit stitching, C public frame and labels, D report and chat next steps, E FAQ), each built by one implementer in its own detached worktree of branch `claude/upbeat-davinci-wyd07q`, merged back by patch. Slices only share the i18n dictionaries and `docs/platform-extensions.md` §27, where each writes its own keys / its own subsection.

**Tech Stack:** Go 1.x + GORM + PostgreSQL + Redis (backend), Next.js 16 app router + vitest + Playwright (frontend, admin console).

**Spec:** `docs/superpowers/specs/2026-11-25-conversion-follow-ups-design.md`

## Global Constraints

- Every user-facing string in en, fr and es (`frontend/src/i18n/dictionaries/copy-{en,fr,es}.json`, English source text is the key; backend `internal/i18n`; mail catalog `internal/mail`). fr/es written natively, not word for word.
- Tests in every codebase touched: Go unit + `backend/tests/e2e`; vitest + Playwright in `frontend/`; vitest + Playwright in `staryield-admin/`.
- Queries through GORM; `internal/repository/rawsql_test.go` must pass. New migrations are numbered from `0041`. New tables have `created_at`/`updated_at`.
- New or changed hot queries get an EXPLAIN case in `backend/tests/load`.
- Document contract changes in `docs/platform-extensions.md` §27 "Growth conversion follow-ups", in the slice's own subsection: 27.1 FAQ (E), 27.2 labels, header and profile frame (C), 27.3 report next step, footnote, UTM and alternatives (D), 27.4 auto-away (A), 27.5 visit stitching (B).
- The marketing checkbox text and `CalculatorReportConsentVersion = "2026-11-19"` do not change.
- No UTM tags on internal site links. Never update `analytics_events` rows (append-only trigger).
- Auto-away threshold default 3, allowed 2–10, enabled by default. Only Online → Away. Declines never count.
- Run only the targeted Go e2e tests (`go test ./tests/e2e -count=1 -run '<regex>'`) and targeted Playwright specs, with your own database `staryield_test_<slice>` and Redis DB index given in your brief. Never stage, commit or stash.

## Review Focus

1. The advisor is not Online when the 3rd miss lands (already Away/Offline, or busy in another reading): no availability change, no notification, counter reset to 0. → Task A1 test `TestAutoAwayOnlyFromOnline`.
2. Two expiries of the same advisor in one scheduler tick, or on two instances: the counter increments atomically and the advisor gets exactly one auto-away notification. → Task A1 test `TestAutoAwayOnceUnderConcurrentExpiry`.
3. The declining advisor is the only one online: the alternatives strip renders nothing (no orphan heading). → Task D1 test "hides when every advisor is excluded".
4. A report e-mail link that already has a query string and/or a `#v=` fragment: UTM parameters are appended with `&`, placed before the fragment, and the fragment is unchanged. → Task D3 test `TestReportLinksCarryUTM`.
5. A user with events from two browsers, one of them linked: counted once in the funnel; an unlinked browser stays its own anonymous actor. → Task B1 test `TestFunnelStitchesVisitorsToAccounts`.

---

## Slice A — Auto-away after missed requests (backend, admin console, web)

### Task A1: Counter, auto-away and notification (backend)

**Files:**
- Create: `backend/internal/migrations/sql/0041_advisor_auto_away.sql`
- Create: `backend/internal/service/auto_away.go`, `backend/internal/service/auto_away_test.go`
- Modify: `backend/internal/service/chat.go` (`ExpireDue` post-commit loop ~927-940; `AcceptSession`; `closeRequest`), `backend/internal/service/psychics.go` (`SetAvailability`), `backend/internal/repository/psychics.go`, `backend/internal/domain/domain.go` (Psychic field), `backend/internal/service/notifications.go` + `notification_channels.go`, `backend/internal/mail/catalog.go`, `backend/internal/i18n/*.go`
- Test: `backend/tests/e2e/auto_away_test.go`

**Interfaces:**
- Produces: column `psychics.missed_requests_in_row integer NOT NULL DEFAULT 0 CHECK (>= 0)`; `domain.Psychic.MissedRequestsInRow int`; `const NotifyAutoAway = "availability.auto_away"` (channels: in-app, push, e-mail, category `domain.EmailNotification`); repository `IncrementMissed(ctx, psychicID uuid.UUID) (int, error)` (single `UPDATE … SET n = n + 1 … RETURNING n`), `ResetMissed(ctx, psychicID uuid.UUID) error`; service `(*AutoAwayService) OnRequestMissed(ctx context.Context, psychicID uuid.UUID)`; site setting `domain.SiteSettingAdvisorAutoAway = "advisor_auto_away"` read via `AutoAwaySettings{Enabled bool; MissedRequests int}` (defaults true / 3).

- [ ] **Step 1: Write the failing e2e tests** in `auto_away_test.go`:
  - `TestAutoAwayAfterThreeMissedRequests`: advisor Online; 3 requests each expire (`e.tick(91*time.Second)`) → `GET /psychics/<slug>` status `away`; advisor has exactly one notification of kind `availability.auto_away`; `e.mail` has one message to the advisor; counter is 0.
  - `TestAutoAwayCountsOnlyMissesInARow`: miss, miss, decline, miss, miss → still `online`; accept in between also resets.
  - `TestAutoAwayResetsWhenAdvisorSetsAvailability`: miss ×2, advisor PATCHes availability online → counter 0; 1 more miss → still online.
  - `TestAutoAwayOnlyFromOnline`: counter at 2, advisor sets away, then an already-pending request expires → availability stays `away`, no notification, counter 0.
  - `TestAutoAwayOnceUnderConcurrentExpiry`: counter at 1, two requests to the same advisor expire in the same tick → exactly one auto-away notification.
  - `TestAutoAwayDisabledBySetting`: setting `{"enabled": false}` → 3 misses keep the advisor online.
- [ ] **Step 2: Run** `go test ./tests/e2e -count=1 -run '^TestAutoAway'` → FAIL (column/kind missing).
- [ ] **Step 3: Implement** the migration (column, plus `site_settings` default row `advisor_auto_away` = `{"enabled": true, "missed_requests": 3}` and a CHECK constraint on that key's shape, as `0037_lifecycle.sql` does for `lifecycle`), the repository methods, `AutoAwayService.OnRequestMissed` (increment; if `n >= MissedRequests` and setting enabled: reset; if stored availability is Online and the advisor is not busy, set Away through `PsychicService.SetAvailability`'s internal path without resetting twice, then `Notify(NotifyAutoAway, Data{"missed": n})`; otherwise just reset), the hook in `ExpireDue` for `EndRequestTimeout` only, and `ResetMissed` calls on accept, decline and advisor-initiated `SetAvailability`. E-mail + push copy en/fr/es: "You were set to Away after {n} missed requests. Set yourself Online when you are ready." Unit tests in `auto_away_test.go` for the threshold and setting parsing (bounds 2–10, invalid → defaults).
- [ ] **Step 4: Run** the e2e tests above plus `go test ./internal/... -count=1` and `TestRequestExpiresAndDecline` → PASS.
- [ ] **Step 5: EXPLAIN** — add the increment/reset statements to `backend/tests/load/explain_test.go` style checks (primary-key update, no seq scan).

### Task A2: Admin API for the setting

**Files:** Modify `backend/internal/api/router.go` (admin group, next to `/lifecycle`), `backend/internal/api/handler/*` (new `auto_away.go`), `backend/internal/service/auto_away.go`; Test `backend/tests/e2e/auto_away_test.go`; Doc `docs/platform-extensions.md` §27.4.

**Interfaces:**
- Produces: `GET /api/v1/admin/advisors/auto-away` → `{"enabled": bool, "missed_requests": int, "version": int}`; `PUT /api/v1/admin/advisors/auto-away` body `{"enabled"?: bool, "missed_requests"?: int, "version": int}` with `stepUp` + `adminRL` middleware, optimistic version (409 `version_conflict` like lifecycle), validation error field `missed_requests` "Choose between 2 and 10.", audit action `advisors.auto_away.updated`.

- [ ] **Step 1: Write failing e2e** `TestAutoAwayAdminSettings`: GET defaults (true, 3); PUT `{missed_requests: 2}` → next 2 misses set Away; PUT 11 → 422 field `missed_requests`; stale version → 409; audit row exists; non-admin → 403.
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement.** **Step 4: Run** → PASS.

### Task A3: Admin console page (staryield-admin)

**Files:** Create `src/app/(dashboard)/psychics/settings/page.tsx`, `src/features/psychics/auto-away-card.tsx` (+ `.test.tsx`), `src/lib/api/auto-away.ts`; Modify `src/components/layout/nav.ts` (+ `nav.test.ts`: item "Advisor settings" `/psychics/settings` under the Psychics item); Test `e2e/auto-away.spec.ts`.

**Interfaces:** Consumes A2's endpoints.

- [ ] **Step 1: Failing vitest**: card renders current values; Save sends `{enabled, missed_requests, version}`; 2–10 validation message before sending; 409 shows "Someone else changed these settings. Reload to see them."; step-up prompt path is the console's existing one.
- [ ] **Step 2: Failing Playwright** `e2e/auto-away.spec.ts`: open Advisor settings, set 4, save, reload shows 4, restore 3.
- [ ] **Step 3: Implement** following `src/features/lifecycle/lifecycle-settings-card.tsx`. **Step 4: Run** vitest + spec → PASS.

### Task A4: Advisor-side notification copy (web)

**Files:** Modify the notification kind registry in `frontend/src` (where `chat.missed` and `referral.*` get their icon and copy), dictionaries; Test the registry's vitest.

- [ ] **Step 1: Failing vitest**: kind `availability.auto_away` renders "You were set to Away after 3 missed requests." with link to the advisor's availability control and a non-default icon. **Step 2–4:** implement, PASS. Playwright: extend the advisor availability spec (`e2e/advisor-settings.spec.ts` or `chat-flow.spec.ts`) only if a live request can be expired within the spec's time budget; otherwise the Go e2e covers behaviour.

---

## Slice B — Funnel visit stitching (backend)

### Task B1: Actor resolution in `Funnel`

**Files:** Modify `backend/internal/repository/analytics.go:115-165`; Test `backend/tests/e2e/analytics_funnel_test.go` (create) or the existing funnel e2e; `backend/tests/load/` EXPLAIN case; Doc §27.5.

**Interfaces:** `Funnel(ctx, steps []string, from, to time.Time) ([]FunnelStep, error)` unchanged.

- [ ] **Step 1: Failing e2e** `TestFunnelStitchesVisitorsToAccounts`: browser A (anonymous id `a`) sends `web.page_view` before sign-up, registers with attribution `anonymous_id: a` (server emits `user.registered` with user id only); browser B (anonymous `b`) never signs up; the same user later sends a `web.page_view` from browser C (linked by an authed event). Funnel `[web.page_view, user.registered]` → step 1 = 2 actors (the user once, `b` once), step 2 = 1.
- [ ] **Step 2: Run** → FAIL (step 2 = 0 today).
- [ ] **Step 3: Implement**: in every step CTE, `LEFT JOIN web_visitors wv ON wv.anonymous_id = e.anonymous_id AND e.user_id IS NULL AND e.anonymous_id <> ''` and actor `COALESCE(e.user_id, wv.user_id)::text`, else `e.anonymous_id`; join condition between steps on the same expression. Keep it in the GORM builder style the file already uses.
- [ ] **Step 4: Run** the new test + existing funnel/analytics e2e (`-run 'Funnel|Analytics'`) → PASS.
- [ ] **Step 5: EXPLAIN** on the load database (`STARYIELD_LOAD_DSN` per `backend/tests/load/README.md`): a 30-day 3-step funnel; `web_visitors` read through its primary key, budget ≤ the existing funnel budget. Record the timing in §27.5.

---

## Slice C — New advisor label, mobile header, profile frame (web)

### Task C1: New advisor label

**Files:** Modify `frontend/src/components/psychics/psychics.tsx` (directory card ~148-150, profile stat tile ~419); Test `psychics.test.tsx`, `psychic-profile.test.tsx`.

- [ ] **Step 1: Failing vitest**: advisor with `reviews_count: 0, rating: 5` → card and profile show "New advisor", no "5.0", no "(0 reviews)", no star; `reviews_count: 4, rating: 4.5` unchanged. **Steps 2–4:** implement, PASS.

### Task C2: Pricing link and mobile Sign up

**Files:** Modify `frontend/src/components/layout/site-header.tsx`; Create `site-header.test.tsx`; Test `e2e/responsive-landing.spec.ts` (mobile project, `@responsive`).

- [ ] **Step 1: Failing vitest**: public header has a "Pricing" link to `/<lang>/pricing` (desktop nav and drawer); signed-out below `lg` shows a Sign up link carrying the current `?next=` behaviour; signed-in shows neither Sign up nor Log in.
- [ ] **Step 2: Failing Playwright** (390×844, signed out): Sign up visible in the bar without opening the drawer; Pricing in the drawer; no horizontal scroll (`expectNoHorizontalScroll`).
- [ ] **Steps 3–4:** implement, PASS.

### Task C3: Public frame for visitors on advisor profiles

**Files:** Modify `PsychicProfileView` in `frontend/src/components/psychics/psychics.tsx` (main, loading, error branches); Test `psychic-profile.test.tsx`, `e2e/psychics-gating.spec.ts`.

- [ ] **Step 1: Failing vitest**: anonymous session → `PublicFrame` header (hamburger trigger) and `SiteFooter` present, no `.mobile-navigation`; authenticated → current app frame, unchanged; loading and error states follow the same rule.
- [ ] **Step 2: Failing Playwright** in `psychics-gating.spec.ts` ("advisor profiles stay public"): signed-out phone visitor sees menu, Sign up and footer on `/en/psychics/<slug>`.
- [ ] **Steps 3–4:** implement, PASS; re-run `booking.spec.ts` and `merchant-readiness.spec.ts`.

---

## Slice D — Report next step, footnote, UTM, alternative advisors

### Task D1: `AdvisorStrip` exclude + alternatives in the chat room

**Files:** Modify `frontend/src/components/booking/advisor-strip.tsx`, `frontend/src/components/chat/session-panel.tsx`; Test `advisor-strip.test.tsx`, `session-panel.test.tsx`, `e2e/booking.spec.ts` or `chat-flow.spec.ts`.

**Interfaces:** `AdvisorStrip` props add `excludeSlug?: string` (fetch 4, filter, slice 3; render nothing when empty).

- [ ] **Step 1: Failing vitest**: strip with `excludeSlug="theo"` never shows theo; "hides when every advisor is excluded"; session panel shows heading "Other advisors online now" for client sessions ended `declined` or `request_timeout`, not for `completed`/`client_cancelled`, never on the advisor side.
- [ ] **Step 2: Failing Playwright**: client requests an advisor who declines → strip visible without that advisor.
- [ ] **Steps 3–4:** implement (placement `chat_alternatives`), PASS.

### Task D2: Report success next step + footnote

**Files:** Modify `frontend/src/components/calculators/report-email-dialog.tsx` (success branch ~162-173, footnote ~275-280), dictionaries; Test `report-email-dialog.test.tsx`, `e2e/calculator-report.spec.ts`.

- [ ] **Step 1: Failing vitest**: after a successful send the dialog shows `AdvisorStrip` heading "Ask an advisor about your results" (placement `report_success`); signed-out → "Create a free account" link to sign-up with `next` = current calculator path; signed-in → no sign-up link; footnote text equals the spec's en copy exactly and the Privacy Policy link remains; checkbox label unchanged.
- [ ] **Step 2: Failing Playwright** in `calculator-report.spec.ts`: success screen shows the strip and the account link.
- [ ] **Steps 3–4:** implement, PASS. Check `content/legal/privacy/*` report-request section agrees with the new footnote (no edit expected; report a mismatch instead of editing legal text).

### Task D3: UTM tags on report e-mail links (Go)

**Files:** Modify `backend/internal/mail/calculator_report.go` (view, advisor and sign-up CTAs; not unsubscribe), Test `backend/internal/mail/calculator_report_test.go`, `backend/tests/e2e/calculator_report_test.go`; update `backend/internal/mail/testdata/report_parity.json` if it pins URLs.

**Interfaces:** `func withUTM(rawURL, kind string) string` — adds `utm_source=staryield&utm_medium=email&utm_campaign=calculator_report&utm_content=<kind>`.

- [ ] **Step 1: Failing unit test** `TestReportLinksCarryUTM`: `https://staryield.net/en/home#psychics` → `https://staryield.net/en/home?utm_source=staryield&utm_medium=email&utm_campaign=calculator_report&utm_content=natal#psychics`; a URL with `?a=1` gets `&utm_…`; `#v=<payload>` fragment byte-identical; unsubscribe link has no UTM.
- [ ] **Steps 2–4:** run FAIL, implement with `net/url`, run PASS (plus the e2e report test checks one rendered link).

---

## Slice E — FAQ answers (web content)

### Task E1: Answers for the 22 questions + growth group + JSON-LD

**Files:** Modify `frontend/src/components/content/faq.tsx`, `frontend/src/content/faq/*` (new list next to `live-readings.ts`), `frontend/src/lib/seo/screen-json-ld.ts`, dictionaries; Test `faq.test.tsx`, `lib/seo/seo.test.ts`, `e2e/seo.spec.ts` or a new `e2e/faq.spec.ts`.

- [ ] **Step 1: Failing vitest**: no FAQ item renders "For help with this question, contact our support team" as its whole answer; each of the 22 questions plus the new group (free trial minutes, invite rewards, confirming your e-mail address, free calculators) has a non-empty answer in en, fr, es; FAQPage JSON-LD has one `Question` per answered item; existing banned-wording rules still pass.
- [ ] **Step 2: Failing Playwright** `e2e/faq.spec.ts`: open `/en/faq`, expand "How to reset password?" → answer mentions the reset link; JSON-LD script parses with ≥ 26 questions.
- [ ] **Step 3: Implement.** Answers come from the code and the legal texts, not invention: credits/minute billing (`pricing`, Terms §6), refunds (Refund Policy), password reset (auth flow), e-mail confirmation (48 h single-use link, resend in account), invites (+5 minutes per friend who signs up with the link, confirms their e-mail and completes a reading; granted at launch), free calculators. Where a fact cannot be confirmed in code or legal text, leave that question's placeholder and list it in the report.
- [ ] **Step 4: Run** → PASS, plus `npm run test:i18n` and `npm run test:content`.
