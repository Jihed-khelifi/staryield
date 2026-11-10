# Legal review checklist — Privacy Policy & Terms of Use (revisions 2.0)

The legal pages are now structured content in
`frontend/src/content/legal/{privacy,terms}/{en,es,fr}.ts`, rendered by
`frontend/src/components/content/legal/legal-document.tsx`. Clause ids below
are the stable anchors (e.g. `/en/privacy#advisor-notes`).

English prevails over the translations. The Spanish and French versions were
machine-assisted and need native legal review.

Status legend: **Decide**: owner or counsel decision required.
**Verify**: confirm the text matches the product or contracts.
**Removed**: template text deleted because it was not true of the product.

## Facts we did not invent (left as-is)

| # | Clause | Status | Why |
|---|---|---|---|
| 1 | Terms `#informal-dispute-resolution-procedures-and-arbitration` (§12, incl. 12.3, 12.5, 12.12) and `#for-eea-and-uk-residents` (§13) | Decide | Governing law and venue now consistently point to Wyoming (previously Cyprus law with Delaware courts), matching the Wyoming LLC. §13 still applies England & Wales law and EEA/UK courts for EEA/UK users: confirm with counsel. Wyoming choice made by Claude, not counsel. |
| 2 | Terms §12.11 (arbitration opt-out, "on or after April 2, 2026") | Decide | The date is tied to the old "Last updated" date and no longer matches 2026-11-04. |
| 3 | Privacy intro (`#privacy-policy`, uppercase paragraph) | Decide | Allows use at 18+ "or with a parent or guardian's agreement", which conflicts with §6 (`#age-limitation`, under-18s barred). Kept; only the translations' grammar and the App Store/Google Play wording were fixed. |
| 4 | Privacy §1 `#personal-data-controller`, Terms §1 | Decide | Registered address now shown. No company registration number or email contact for legal notices. Privacy still describes GDPR rights/DPO although the controller is now a US LLC: confirm whether an EU/UK representative (GDPR Art. 27) is needed. |
| 5 | Privacy §14 `#contact-us` | Decide | No DPO contact details; the "dedicated contact form" now points to `/contact?topic=privacy`. GDPR Art. 13 requires the DPO's contact details if one is appointed. |

## Owner decisions applied

| # | Clause | Status | Why |
|---|---|---|---|
| 6 | Privacy §1, Terms §1, Terms §12 (first paragraph) | Done | Entity is "Staryield Media LLC", a Wyoming LLC, 30 N Gould St, Ste N, Sheridan, WY 82801 (Articles of Organization filed 2026-10-01, ID 2026-002096924). Replaces "Staryield Limited" (Cyprus). Footer copyright now names the LLC. EIN and bank details are deliberately not published. |
| 7 | Privacy §2.7 `#advisor-notes`, §5 `#right-to-data-portability`, Terms §4.14 `#client-notes` | Verify | Notes are private to their author by default and can be shared with advisors who also had a session with the client. They are excluded from the self-service export (third-party opinions, GDPR Art. 15(4)) and provided through an admin-assisted, audited request in a structured format. Check this matches the admin "client notes" view (JSON download) and the support procedure. |
| 8 | Privacy §2.5 `#live-readings-ratings-and-notifications`, Terms §4.11 `#live-readings-messages-and-images` | Verify | Date of birth and birth city are shown to advisors only with the client's opt-in ("Share birth details with advisors"). Confirm the final Settings label and its default (off). |

## Needs legal assessment

| # | Clause | Status | Why |
|---|---|---|---|
| 9 | Privacy §2.7 `#advisor-notes`, §3 table (row "enable communication with advisors"), `#legitimate-interests` | Decide | Notes may still contain special-category data despite the instruction not to record it (GDPR Art. 9). Confirm the lawful basis (legitimate interest plus prohibition, or explicit consent) and run a legitimate-interest assessment for sharing between advisors. |
| 10 | Privacy §1 `#personal-data-controller` | Decide | Added: Staryield is controller of data stored on its systems (chats, client cards, notes). The original text made advisors controllers and Staryield a processor. Confirm the role allocation, and joint controllership for notes. |
| 11 | Terms §4.14 `#client-notes` | Decide | Advisor duties (confidentiality, no off-platform use, no sensitive data) sit in the consumer Terms. Consider a separate Advisor Agreement, which already holds §6.4 too. |
| 12 | Terms §3.6 `#account-deletion` | Decide | Unused credits and any remaining membership period are forfeited when closure becomes final "unless applicable law requires otherwise". Check consumer law on prepaid credits (EU/UK/CA). |
| 13 | Terms §3.7 `#disabling-your-account` | Verify | States that disabling the account does not cancel a Premium Membership. Confirm the billing implementation behaves this way. |
| 14 | Terms §6.1.3 `#limitations` | Decide | Forfeiture reason "account disabled due to inactivity": no inactivity policy exists. Remove or define it. "Balance Units" were renamed "Credits". |
| 15 | Terms §6.2.1 `#subscription-options-purchases-and-refunds` | Verify | Benefits: monthly credit allowance, chat discount, monthly free minutes (no roll-over), waitlist priority. Granted credits behave like purchased credits. Confirm against plan configuration. |
| 16 | Terms §6.2.2 `#purchases-and-payment-processing`, Privacy §4 `#how-we-share-your-personal-data` | Verify | Processors are described generically ("Stripe or another card processor"). Name PayKings/QuadraPay (or others) once contracts are signed, and list each as a processor. |
| 17 | Terms §6.2.3 `#auto-renewal-subscription-continuity-and-cancellation` | Verify | Kept the "small temporary verification charge" sentence; confirm the processors actually do this. Check auto-renewal disclosure rules (e.g. California ARL, EU). |
| 18 | Terms §6.2.7 `#changes-to-subscription-fees` | Decide | Generic notice period for price changes on renewing memberships; confirm the required periods per market. |
| 19 | Terms §6.2.8 `#failure-to-pay-and-service-termination` | Verify | Retries over several days, benefits may pause, membership ends after 7 days. Matches the billing design (+0h/24h/72h/120h, 7-day grace). |
| 20 | Terms §6.2.9 `#right-of-withdrawal` (new) | Decide | Drafted EU/UK/CH 14-day withdrawal with an immediate-performance waiver and proportionate payment for memberships. Counsel must confirm, and checkout must capture express consent and the acknowledgement. |
| 21 | Terms §6.3.1 `#refunds` | Decide | Rewritten without the non-existent "Money-Back Policy". Undelivered custom readings return held credits (BR-07); everything else is non-refundable unless the law requires. |
| 22 | Terms §4.3 `#user-generated-content` | Verify | Added carve-out: private chat messages and images and advisor notes are excluded from the content licence and never published. |
| 23 | Terms §4.13 `#notifications`, Privacy §3 table (marketing row) | Decide | "We only send marketing communications if you opt in." Cloudflare Email is transactional only, so marketing email is not sent today. Confirm the consent model before enabling marketing. |
| 24 | Privacy §9.6 `#third-party-tracking` | Decide | Advertising use removed, but the broad "your continued use constitutes explicit consent" to monitoring/recording language was kept. Review for CIPA/wiretap risk. |
| 25 | Privacy §11 `#cookies` | Decide | There is no consent banner design for anonymous EU visitors (only Settings → Privacy preferences). Analytics and attribution storage must be gated on prior consent where ePrivacy requires it before relying on this text. |
| 26 | Privacy §11 `#cookies` | Verify | `sy_theme` (theme) and `sy_consent` (privacy choices) are planned in M2 and not yet in code; confirm the final names and durations. `sy_csrf` duration shows "Session"; confirm. |
| 27 | Privacy §8 `#data-retention` | Decide | Device records are kept "for a limited period" after sign-out and consent records "as long as the account exists"; set exact periods. Chats are kept while the conversation exists, which is open-ended; consider a fixed period. |
| 28 | Privacy §7 `#international-data-transfers` | Verify | Rewritten without dead "here" links. Confirm DPF certification or SCCs (plus UK addendum) for Cloudflare, Railway and the payment processors. |
| 29 | Privacy §4 `#how-we-share-your-personal-data` | Verify | States that application servers and databases run on Railway; confirm the hosting provider and region. |
| 30 | Privacy §5 `#your-privacy-rights` | Verify | Self-service export (ZIP, single-use link, 7-day expiry) and closure (30-day grace). Confirm response deadlines (1 month GDPR / 45 days CCPA) and the identity verification steps. |
| 31 | Privacy §9 `#categories-of-personal-information-we-disclose-and-share-with-third-parties` | Verify | Disclosure table rewritten for the actual providers; now says no sale and no cross-context behavioural sharing. Confirm before publishing. |

## Removed template text (not true of the product)

| # | Location | Removed |
|---|---|---|
| 32 | Privacy intro | IDFA/GAID/Facebook ID collection, advertising use, and sharing with Meta, TikTok, Google, Apple, Appsflyer, Amplitude, Firebase, Customer.io and Twilio. |
| 33 | Privacy §2.1 | "Palm or face image" collection. |
| 34 | Privacy §2.2 | Sign-in with Apple, Facebook and Google; app-marketplace data. |
| 35 | Privacy §2.3 | Advertising performance data, advertising IDs, and Meta/TikTok/Google pixels; the "Cookie Policy" reference now points to `#cookies`. |
| 36 | Privacy §2.4 (old) | Online store (Shopify, Shop login). Later subsections were renumbered. |
| 37 | Privacy §3 | Purposes "process your orders (online store)", "serve personalized advertising" and "analyze performance of advertising", plus their legitimate-interest descriptions. |
| 38 | Privacy §4 | Firebase, Appsflyer, Amplitude, GrowthBook, Meta Pixel, TikTok Pixel, Google Analytics, Tableau, Microsoft Clarity, Debounce.io, AstrologyAPI (charts are computed in-house), OneTrust, Zendesk, Intercom, Twilio, Customer.io, WhatsApp, OpenAI, Shopify, Meta/TikTok/Pinterest/Google/Bing ads, Solidgate, Pay.com, Checkout, Worldpay, Amazon Web Services; and the names Google Workspace, Slack, Notion and Jira (kept generically). Verify that none of these is in use. |
| 39 | Privacy §9 | "Sharing for cross-context behavioural advertising" table; "phone call" communications. |
| 40 | Terms intro notice | App Store / Google Play refund bullets; trials wording replaced by the membership notice. Cross-reference fixed from "Section 5" to §6.2 `#subscription-terms`. |
| 41 | Terms §6.1 (old 6.1.2, 6.1.4) | Auto-Refill and its cancellation (feature does not exist). Later clauses were renumbered. |
| 42 | Terms §6.2 | App Store / In-App Purchase mechanics. Free trials (§6.2.5) and promo codes (§6.2.6) kept generically. |

## Mechanical fixes (no review needed)

- **Fixed URL:** `staryield.com/app/privacy` → `https://staryield.net/{lang}/privacy` (each locale shows and links its own page).
- **TOC:** generated from the structure, with unique ids in every locale. This fixes the 16 duplicated Terms ids, the Privacy sub-entries that routed to `/psychics`, and the "17. Contact Information" vs "16.1" mismatch.
- **Placeholder links:**
  - "Support center" and complaint forms → `/contact`.
  - Privacy "request form" and "dedicated contact form" → `/contact?topic=privacy`.
  - "Trust & Safety Center" / "Community Standards" → `/standards` (Advisor Integrity Standards).
  - "Money-Back Policy" → `/terms#refunds`.
  - "Subscription Terms" → `/terms#subscription-terms`.
- **Supervisory authorities:** the dead "here" links now name the EDPB and the ICO in plain text.
- **New content:**
  - Privacy: §2.7 advisor notes, §2.8 account/devices/preferences/membership, §11 cookies.
  - Terms: §2.4 calculators, §3.7 disabling, §4.14 client notes, §4.15 waiting lists, §6.2.9 withdrawal.
  - Spanish blocks rewritten from informal "tú" to formal "usted" for consistency.
- **Dates:** effective and last-updated set to 2026-11-07 in all locales (revisions 3.0). Update them on publication.

## Revisions 3.0 additions (2026-11-07)

| # | Clause | Status | Why |
|---|---|---|---|
| 43 | Privacy §2.9 `#support-requests`, §8 `#data-retention` | Verify | Support desk: thread, staff replies and internal notes sealed at rest, e-mail keyed hash, private links valid 30 days (fragment token), deletion 24 months after closing, linked to the account when signed in, included in the data export. Matches `STARYIELD_SUPPORT_LINK_TTL` / `STARYIELD_SUPPORT_RETENTION`. |
| 44 | Privacy §2.10 `#calculator-usage-measurement` | Decide | Random browser id (`staryield.aid`) linked to the account at sign-up only when analytics is allowed; first-touch campaign data kept 25 months. Same open point as #25: confirm prior-consent handling for anonymous EU visitors. |
| 45 | Privacy §2.11 `#chat-notifications-and-drafts` | Verify | Push without message content; drafts only on the device for up to 7 days. |
| 46 | Privacy §2.12 `#free-access-periods`, Terms §6.2.5 | Verify | Sponsored sessions are recorded for billing and advisor-earnings statistics. Terms: introductory free minutes (amount set by Staryield, new accounts only, no cash value, used before credits) and promotional free access ("free mode": may start or stop at any time, chats requested during it stay free until they end, advisors still paid, custom requests and credit purchases excluded). |
| 47 | Terms §4.10 `#customer-support` | Verify | Support communications: private e-mailed links, closed requests cannot be answered. |


## Revisions 3.3 additions (2026-11-09)

Dates and versions bumped to 2026-11-09 in all six legal files.

| # | Clause (anchor id) | Status | Why |
|---|---|---|---|
| 48 | Privacy §2.5 `#live-readings-ratings-and-notifications`, §3 table (communication with advisors row), §4 `#how-we-share-your-personal-data`, Terms §4.11 `#live-readings-messages-and-images` | Decide | Advisors now always see the client's full date of birth (legal basis: performance of the contract). This reverses the previous promise that the date of birth was shown only with opt-in. Owner/counsel must decide the notice to existing users before it takes effect (in-app broadcast and/or e-mail; see also Privacy §13 policy-change obligation, #54) and confirm the legal basis and that no consent was relied on for the date. |
| 49 | Privacy §2.5, §3 table, §4, §5 consent withdrawal; Terms §4.11 | Verify | Switch label is now "Share my birth city with advisors" (off by default); verify the exact wording against Settings in en/fr/es. Birth time and coordinates are never shown. |
| 50 | Privacy §2.13 `#messages-outside-live-readings`, §2.11, Terms §4.16 `#messages-outside-live-readings`, §4.11 | Decide | New free messaging outside live readings: free, capped per conversation per rolling 24 h, limit set and changeable or switchable off by Staryield, unlimited in free mode, not a reading, advisors not obliged to reply, no guaranteed response time. Counsel to confirm the policy, and that advisors must not give readings or solicit off-platform contact or payment (cross-referenced to §4.7, §4.11 and the Advisor Integrity Standards rather than duplicated). |
| 51 | Privacy §2.13, Terms §4.16 | Verify | Limits shown in the app must match the admin setting `off_session_messages` (enabled, client and advisor daily limits). The policies deliberately state no number. Retention (same as §8), participants-only visibility, staff review rules and notifications without text match the API. Messages in a paused live reading belong to that reading and are not billed while paused. |
| 52 | Privacy §11 `#cookies` (`sy_csrf`, `sy_rt` rows) | Verify | `sy_csrf` now "Up to 30 days, renewed while you use the site" (was "Session"); the `sy_rt` row now says you stay signed in on that device for up to 30 days of use unless you sign out. Check against `STARYIELD_AUTH_REFRESH_TOKEN_TTL` and the cookie Max-Age. No new storage keys were added (cross-tab sign-in coordination is in memory only). |
| 53 | Privacy §2.5, Terms §4.11 | Verify | Images may be resized/compressed on the device (longest side up to 2048 px), metadata is removed, and HEIC/HEIF/AVIF photos are converted where the browser supports it. Confirm against the composer code (server still stores PNG/JPEG/WebP/GIF-derived images). |
| 54 | Privacy §13 (changes to this Policy) | Decide | The DOB change is a material change to how existing users' data is shown. Decide whether Privacy §13 requires advance notice or renewed acknowledgement (and for how long before effect), and ship that notice before or with this release. |


## Revisions 4.0 additions (2026-11-09)

Dates and versions stay at 2026-11-09 in all six legal files (the same release round as 3.3). New clauses cover Google sign-in, replies and likes, private client nicknames, notification and push changes, and the Trustpilot widget (docs/platform-extensions.md §23).

| # | Clause (anchor id) | Status | Why |
|---|---|---|---|
| 55 | Privacy §2.2 `#data-we-receive-from-third-parties`, §3 table, §4 `#how-we-share-your-personal-data`, §7, Terms §3.1 `#creating-an-account`, §3.5 `#account-security` | Verify | Google sign-in data (account identifier, e-mail address and the verified flag only; scope `openid email`; no access or refresh token stored; no name, picture or contacts) matches the implementation (§23.1). The earlier contradiction ("when you sign in via Google" next to "we do not currently offer sign-in with Google") was removed: Apple and Facebook remain not offered. |
| 56 | Privacy §2.2, §4 (Google) | Decide | Google OAuth app verification: before production the consent screen must show the verified brand, the home page and the Privacy Policy link on staryield.net. Decide who owns the Google Cloud project and the client secret (`STARYIELD_GOOGLE_CLIENT_SECRET`), and who is the contact for Google's review. |
| 57 | Privacy §2.2, Terms §3.1 | Verify | Accounts are never linked by an e-mail match alone: an existing password account is linked only after its password is confirmed (or from a signed-in session), and Google sign-up records acceptance of the Terms (checkbox on sign-up or on the "Create your Staryield account" consent step; `terms_accepted_at` is set exactly as for e-mail sign-up). The age requirement in Terms §3.3 applies identically. |
| 58 | Privacy §4 (Trustpilot A/S), §11 `#cookies`, §3 table (Trustpilot row) | Decide | Whether EU/UK visitors must consent before the Trustpilot widget loads (it may set or read data in Trustpilot's own frame, and Trustpilot receives the IP address, browser details and page address). There is still no anonymous consent banner (see row 25); today the widget loads when it scrolls into view and is suppressed only by Global Privacy Control or Do Not Track, which show a plain link instead. Legal basis stated: legitimate interests. |
| 59 | Home page (testimonial and statistics), `/reviews` (review cards), Terms §4.4 | Decide | The home page still shows a hard-coded testimonial ("H. Reynolds") and statistics ("4.8 Average Platform Rating", "50+ Countries", "10K+ Consultations"), and `/reviews` still shows three hard-coded review cards. The hard-coded Trustpilot figures ("17,419 reviews", "4.4 rating") were removed this round. Substantiate each remaining claim or remove it: FTC rule on consumer reviews and testimonials (16 CFR Part 465), UK DMCC Act 2024, EU Unfair Commercial Practices Directive Annex I points 23b and 23c, and Trustpilot's brand rules. |
| 60 | Terms §4.4 `#user-reviews-and-testimonials`, Privacy §4 | Verify | Trustpilot invitation rules: the widget is shown in the same four places to everyone (reviews page, home page, footer, end-of-chat summary of every client), never tied to a rating or to whether the client rated, no incentive of any kind, advisors excluded (Terms §4.4 also names employees, contractors and close relatives). Confirm no other channel (e-mail, push, in-app) invites only satisfied clients. |
| 61 | Privacy §2.7 `#advisor-notes`, §3 legitimate interests, Terms §4.14 `#client-notes` | Verify | Client nicknames written by advisors are personal data about the client (sealed at rest, deleted when either account is erased, not in the client's self-service export). Access requests and safety reviews now go through the admin console: in a session's transcript the nickname is hidden until an admin passes the step-up check and states a reason, and each view is audited as `client_alias.viewed` without the text (Privacy §2.7 says so). Confirm that wording and the conduct rule in Terms §4.14 (respectful, not discriminatory or insulting, no sensitive information, removable by Staryield); note that the console can show a nickname but has no button to remove one yet. |
| 62 | Privacy §2.5 `#live-readings-ratings-and-notifications`, §5 portability, Terms §4.11 `#live-readings-messages-and-images` | Verify | Replies and likes: kept as long as the messages they belong to (same retention as chat messages, §8), visible to both participants, included in the data export, deleted with the conversation or account; likes never notify, never push and are free; a like is not advice, a confirmation or a commitment by an advisor. A reply shows a short excerpt of the quoted message (derived at read time). |
| 63 | Privacy §2.8 `#account-devices-and-preferences`, §2.11 `#chat-notifications-and-drafts`, §11 `#cookies` (`sy_oa`), Terms §4.13 `#notifications` | Verify | Push subscriptions are removed at sign-out of that browser, and all of them at sign-out everywhere, password change or reset, deactivation or closure; read status is synchronised across devices; the app-icon badge uses no new data. The `sy_oa` cookie row (HttpOnly, "Up to 10 minutes" = `STARYIELD_GOOGLE_STATE_TTL`, essential) matches the API; verify the name (`__Secure-sy_oa` on HTTPS) and lifetime. Two storage rows were also added to the cookie table this round: `staryield.google.intent` (session storage, up to 15 minutes or until the tab closes) and `staryield.push.*` (local storage: `staryield.push.synced.*`, `staryield.push.resubscribe.*`, `staryield.push.endpoint.*`, removed at sign-out; the first two already existed in production but were missing from the table). This supersedes the "no new storage keys" note of row 52 for this round. |
| 64 | Privacy §13 (changes to this Policy), §4 | Decide | Whether this round's changes (two new third-party recipients, Google and Trustpilot A/S, and the new nickname, reply and like processing) need advance notice to existing users under Privacy §13, to be decided together with row 54 and shipped as one notice. |
