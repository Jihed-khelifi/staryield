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
| 1 | Terms `#informal-dispute-resolution-procedures-and-arbitration` (§12, incl. 12.3, 12.5, 12.12) and `#for-eea-and-uk-residents` (§13) | Decide | Governing law and venue contradict each other. §12.12 says Cyprus law but exclusive Delaware courts. §12.3 and §12.5 place relief and hearings in Delaware. §13 applies England & Wales law for EEA/UK users. Left untouched per owner decision. |
| 2 | Terms §12.11 (arbitration opt-out, "on or after April 2, 2026") | Decide | The date is tied to the old "Last updated" date and no longer matches 2026-11-04. |
| 3 | Privacy intro (`#privacy-policy`, uppercase paragraph) | Decide | Allows use at 18+ "or with a parent or guardian's agreement", which conflicts with §6 (`#age-limitation`, under-18s barred). Kept; only the translations' grammar and the App Store/Google Play wording were fixed. |
| 4 | Privacy §1 `#personal-data-controller`, Terms §1 | Decide | No company registration number or postal/email contact for legal notices. |
| 5 | Privacy §14 `#contact-us` | Decide | No DPO contact details; the "dedicated contact form" now points to `/contact?topic=privacy`. GDPR Art. 13 requires the DPO's contact details if one is appointed. |

## Owner decisions applied

| # | Clause | Status | Why |
|---|---|---|---|
| 6 | Privacy §1, Terms §1, Terms §12 (first paragraph) | Done | Entity name is "Staryield Limited" everywhere; "Spiritual Staryield Limited" replaced. |
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
- **Dates:** effective and last-updated set to 2026-11-04 in all locales. Update them on publication.
