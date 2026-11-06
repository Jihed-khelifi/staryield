# Staryield platform extensions — API contract

This document is the single source of truth for the features added on top of
the v1 API (`backend/README.md`): the admin console (`admin.staryield.net`,
repository `staryield-admin`), growth tooling (campaigns, short links, UTM
attribution, audience insights), notifications, chat attachments, ratings,
psychic analytics and withdrawals, and server-side localization.

Conventions are unchanged from v1:

* Base path `/api/v1`, JSON bodies (`Content-Type: application/json`), unknown
  fields rejected, cookies for sessions, CSRF double-submit header
  `X-CSRF-Token` on every non-GET request, `X-Request-ID` correlation.
* Errors: `{"error":{"code","message","details","request_id"}}`. `message`
  and `details.fields.*` are **localized** (see §10). UIs should still prefer
  their own copy keyed by `code`.
* Lists: `{"items": [...], "next_cursor": "..."}` (cursor is opaque; pass it
  back as `?cursor=`). Default/max `limit` given per endpoint.
* Times are RFC 3339 UTC. Money is integer cents; credits are integers.
* Date ranges: `?from=&to=` (RFC 3339, `to` exclusive). Defaults: last 30
  days. Maximum span 366 days.
* `interval` (series): `hour | day | week | month` (default `day`).

New error codes (add to client catalogues):

| code | HTTP | meaning |
| --- | --- | --- |
| `step_up_required` | 403 | Admin must re-enter a TOTP code (`POST /admin/auth/step-up`) and retry. |
| `invalid_code` | 401 | Wrong/expired TOTP or recovery code. `details.attempts_left` may be present. |

---

## 1. Admin authentication and session security

Admin endpoints (`/api/v1/admin/**`) no longer accept the regular access
cookie. They require an **admin session**, obtained with password + TOTP.

Guards applied to every `/api/v1/admin/**` route, in order:

1. **Admin origin**: the `Origin` header must be in `STARYIELD_SECURITY_ADMIN_ORIGINS`
   (prod: `https://admin.staryield.net`, dev: `http://localhost:3002`).
   Otherwise `403 csrf_failed`.
2. **IP allow-list** (optional, `STARYIELD_SECURITY_ADMIN_IP_ALLOWLIST` CIDRs): `403 forbidden`.
3. **Admin session cookie** (`__Host-sy_adm` in production, `sy_adm` in dev;
   HttpOnly, Secure, SameSite=Strict, Path=/). Server-side session, idle
   timeout 30 min (sliding), absolute lifetime 8 h. Missing/expired →
   `401 unauthenticated` / `401 session_expired`. The account must still be an
   active admin; role/status changes and logout revoke sessions immediately.
4. **CSRF** double-submit (unchanged) for non-GET.
5. **Step-up** for sensitive routes (marked 🔐 below): the session's last TOTP
   verification must be within `STARYIELD_ADMIN_STEP_UP_WINDOW` (10 min), else
   `403 step_up_required`.

Rate limits: auth endpoints use the `auth` bucket (per IP) plus per-account
lockout (5 failures → 15 min). Each login challenge allows 5 code attempts.

### 1.1 Flow

```
POST /admin/auth/login {email,password}
  → {status:"enrollment_required", challenge, expires_at}   (no TOTP yet)
       POST /admin/auth/enroll {challenge} → {secret, otpauth_url}
       POST /admin/auth/enroll/confirm {challenge, code} → AdminSession + recovery_codes
  → {status:"mfa_required", challenge, expires_at}
       POST /admin/auth/verify {challenge, code} | {challenge, recovery_code} → AdminSession
```

Non-admin, deactivated, banned or unknown accounts get the same generic
`401 invalid_credentials` as a wrong password.

### 1.2 Endpoints

| Method & path | Body | Response |
| --- | --- | --- |
| `POST /admin/auth/login` | `{email, password}` | `LoginChallenge` |
| `POST /admin/auth/enroll` | `{challenge}` | `{secret, otpauth_url}` (same secret for repeated calls with one challenge) |
| `POST /admin/auth/enroll/confirm` | `{challenge, code}` | `AdminSession & {recovery_codes: string[10]}` (shown once) |
| `POST /admin/auth/verify` | `{challenge, code}` or `{challenge, recovery_code}` | `AdminSession` |
| `GET /admin/auth/session` | – | `AdminSession` (also extends idle timer) |
| `POST /admin/auth/step-up` | `{code}` | `AdminSession` |
| `POST /admin/auth/logout` | – | `204` (revokes the session, clears cookie) |
| `GET /admin/auth/sessions` | – | `{items: AdminSessionRow[]}` (own active sessions) |
| `DELETE /admin/auth/sessions/:id` | – | `204` |
| 🔐 `POST /admin/auth/recovery-codes` | – | `{recovery_codes: string[10]}` (replaces old ones) |

```ts
type LoginChallenge = { status: "mfa_required" | "enrollment_required"; challenge: string; expires_at: string };
type AdminSession = {
  admin: { id: string; email: string; display_name: string };
  csrf_token: string;            // also rotated in X-CSRF-Token header
  session_id: string;
  expires_at: string;            // absolute
  idle_expires_at: string;       // last activity + idle timeout
  idle_timeout_seconds: number;
  step_up_until: string;         // sensitive actions allowed until
  recovery_codes_left: number;
};
type AdminSessionRow = { id: string; current: boolean; user_agent: string; created_at: string; last_seen_at: string; expires_at: string };
```

TOTP: RFC 6238, SHA-1, 6 digits, 30 s period, ±1 step tolerance, a code can
be used only once. Secrets and payout destinations are encrypted at rest
(AES-256-GCM, `STARYIELD_SECURITY_DATA_ENCRYPTION_KEY`). Recovery codes are stored as
hashes and are single-use (format `xxxxx-xxxxx`, lowercase base32).

---

## 2. Admin: users

All under `/api/v1/admin`. Every mutation is audit-logged.

| Method & path | Notes |
| --- | --- |
| `GET /users?q=&role=&status=&cursor=&limit=` | limit 50/200. `status`: `active|deactivated|banned|deleted`. Returns `AdminUserRow` page. |
| `POST /users` | `{email, password, role, display_name?, locale?}` → `AdminUser` (201). Creating `role:"admin"` is 🔐. Use `/psychics` to create advisors. |
| `GET /users/:id` | `AdminUser` |
| `PATCH /users/:id` | `{display_name?, locale?, email?, role?, status?, status_reason?}`. `status:"banned"` requires `status_reason` and is 🔐; changing `role` to/from `admin` is 🔐. Cannot target yourself. Deactivate/ban revokes all sessions, ends live chats, closes realtime connections, hides a psychic from the directory. |
| 🔐 `DELETE /users/:id` | `{reason}` → `204`. Soft delete + anonymisation: email becomes `deleted+<id>@deleted.invalid`, profile PII and avatar removed, sessions revoked, status `deleted`. Ledger/earnings history is kept (append-only). |
| 🔐 `POST /users/:id/credits` | unchanged body `{amount, reason, idempotency_key}` → `Wallet`. |
| `POST /users/:id/logout-all` | `204`. Revokes all main-site sessions. |

```ts
type AdminUserRow = {
  id: string; email: string; role: "client"|"psychic"|"admin";
  status: "active"|"deactivated"|"banned"|"deleted"; status_reason?: string;
  display_name: string; locale: string; created_at: string; last_login_at?: string;
};
type AdminUser = AdminUserRow & {
  deleted_at?: string;
  wallet?: { balance: number; held: number; free_minutes: number };
  profile?: { nickname: string; birth_date?: string; country_code?: string; location: string; avatar_url: string };
  attribution?: Attribution;
  stats: { purchases: number; revenue_cents: number; credits_purchased: number; credits_spent: number;
           sessions: number; chat_minutes: number; custom_requests: number; last_active_at?: string };
  mfa_enabled?: boolean;              // admins only
};
type Attribution = {
  campaign_id?: string; campaign_name?: string; link_id?: string; link_code?: string;
  utm_source: string; utm_medium: string; utm_campaign: string; utm_content: string; utm_term: string;
  referrer_host: string; landing_path: string; first_seen_at?: string; created_at: string;
};
```

---

## 3. Admin: psychics (advisors)

| Method & path | Notes |
| --- | --- |
| `GET /psychics?q=&status=&listed=&featured=` | `{items: AdminPsychic[]}` (all, including unlisted/banned). |
| `POST /psychics` | unchanged + optional `featured`, `featured_rank` → `AdminPsychic` (201). |
| `GET /psychics/:id` | `AdminPsychic` (with all services, active or not). |
| `PATCH /psychics/:id` | v1 fields + `featured?: boolean`, `featured_rank?: number` (0–1000, lower first). **Hide** = `listed:false`. **Favor** = `featured:true`. **Ban** = `PATCH /users/:id {status:"banned"}`. |
| 🔐 `DELETE /psychics/:id` | `{reason}` → `204`. Soft delete (unlisted, status `deleted`, live chat ended). |
| `POST /psychics/:id/image` | `multipart/form-data`, field `image` (PNG/JPEG/WebP, ≤ 5 MB, ≤ 40 MP). Re-encoded to JPEG (max side 1200 px, EXIF stripped) and stored in the **`R2_ADVISOR_IMAGE`** bucket under a random key; previous image deleted. → `AdminPsychic`. |
| `DELETE /psychics/:id/image` | → `AdminPsychic` (image_url = ""). |
| `GET /psychics/:id/earnings` | unchanged (v1 `EarningsReport`). |
| `GET /psychics/:id/analytics?from&to&tz&interval` | `PsychicAnalytics` (§8). |
| `POST /psychics/:id/services` | unchanged. |
| `PATCH /services/:id` | unchanged. |
| `DELETE /services/:id` | `204`, or `409 conflict` if orders reference it (deactivate instead). |

Public directory order: `featured DESC, featured_rank ASC, sort_order ASC, name ASC`;
only `listed` advisors whose account is `active` are public.

```ts
type AdminPsychic = Psychic /* v1 public view + featured */ & {
  featured: boolean; featured_rank: number; sort_order: number; availability: "online"|"away"|"offline";
  email: string; user_status: AdminUserRow["status"]; status_reason?: string; created_at: string;
};
```

`Psychic` (public view, `GET /psychics`) gains `featured: boolean`.

---

## 4. Admin: catalogue, moderation, operations

| Method & path | Notes |
| --- | --- |
| `GET/POST /credit-packs`, `PATCH /credit-packs/:id` | unchanged. |
| `DELETE /credit-packs/:id` | `204` or `409 conflict` when purchases reference it. |
| `GET /custom-requests?status=`, `POST /custom-requests/:id/resolve` | unchanged (dispute resolution). |
| `GET /chat/sessions?status=&client_id=&psychic_id=`, `GET /chat/sessions/:id` | unchanged (monitoring). |
| 🔐 `GET /chat/sessions/:id/transcript?reason=&before=` | unchanged, audited. Image messages appear with `attachments` metadata only: admins cannot open chat images (sender and receiver only). |
| `POST /chat/sessions/:id/terminate` | unchanged. |
| `GET /reviews?psychic_id=&status=&rating=&cursor=` | `Page<AdminReview>` (limit 50/200). |
| `PATCH /reviews/:id` | `{status: "visible"|"hidden", note?}` → `AdminReview`. Hiding recomputes the advisor rating. |
| `GET /withdrawals?status=&psychic_id=&cursor=` | `Page<AdminWithdrawal>`. |
| 🔐 `GET /withdrawals/:id/destination` | `{destination}` (decrypted payout details, audited). |
| 🔐 `POST /withdrawals/:id/approve` | `{note?}` → `AdminWithdrawal` (`pending → approved`). |
| 🔐 `POST /withdrawals/:id/pay` | `{reference?, note?}` → `AdminWithdrawal` (`approved → paid`). |
| `POST /withdrawals/:id/reject` | `{note}` (required) → `AdminWithdrawal` (`pending|approved → rejected`, credits become available again). |
| `GET /audit-logs?actor_id=&target_type=&target_id=&action=&cursor=` | unchanged + `action` filter; items gain `actor_email`. |
| `GET /presence` | `{online_clients, online_psychics, users: [{id, role, display_name, email}]}` (first 500 online). |

```ts
type AdminReview = Review & { client: {id: string; name: string}; psychic: {id: string; name: string; slug: string}; moderation_note?: string };
type AdminWithdrawal = Withdrawal & { psychic: {id: string; name: string; slug: string}; processed_by?: string; reference?: string };
```

---

## 5. Growth: campaigns, short links, UTM, attribution

### 5.1 Campaigns (admin)

| Method & path | Notes |
| --- | --- |
| `GET /campaigns?status=` | `{items: Campaign[]}` with `metrics` for the last 30 days. |
| `POST /campaigns` | `{name, slug, channel?, status?, budget_cents?, currency?, starts_at?, ends_at?, notes?}` → `Campaign` (201). `slug` is the `utm_campaign` value (`^[a-z0-9]+([-_][a-z0-9]+)*$`, ≤ 64, unique). |
| `GET /campaigns/:id` | `Campaign` |
| `PATCH /campaigns/:id` | same fields, all optional. |
| `DELETE /campaigns/:id` | `204`. Archived (status `archived`) when it has links or attributed users, otherwise deleted. |
| `GET /campaigns/:id/report?from&to&interval` | `GrowthReport` |

```ts
type Campaign = {
  id: string; name: string; slug: string; channel: string;
  status: "draft"|"active"|"paused"|"ended"|"archived";
  budget_cents: number; currency: string; starts_at?: string; ends_at?: string; notes: string;
  created_at: string; updated_at: string;
  metrics?: { clicks: number; unique_clicks: number; signups: number; purchasers: number; revenue_cents: number };
};
```

### 5.2 Short links + UTM builder (admin)

| Method & path | Notes |
| --- | --- |
| `GET /links?campaign_id=&q=&active=&cursor=` | `Page<ShortLink>` |
| `POST /links` | `{destination, code?, campaign_id?, utm_source, utm_medium, utm_campaign?, utm_content?, utm_term?, label?, expires_at?, active?}` → `ShortLink` (201). `destination` is a path on the main site (`/en/psychics`) or an absolute `https://` URL whose host is in `STARYIELD_LINKS_ALLOWED_HOSTS`. `code` optional (`^[A-Za-z0-9_-]{3,32}$`, case-insensitive unique); generated (7 chars) when omitted. `utm_campaign` defaults to the campaign slug. UTM values: `^[A-Za-z0-9._\-+ ]{1,100}$`. |
| `GET /links/:id` | `ShortLink` |
| `PATCH /links/:id` | any create field except `code`. |
| `DELETE /links/:id` | `204` (deactivated if it has clicks). |
| `GET /links/:id/report?from&to&interval` | `GrowthReport` |

```ts
type ShortLink = {
  id: string; code: string; short_url: string;   // e.g. https://staryield.net/go/aB3dE9x
  destination: string; full_url: string;         // destination + utm_* query (no click id)
  campaign_id?: string; campaign_name?: string;
  utm_source: string; utm_medium: string; utm_campaign: string; utm_content: string; utm_term: string;
  label: string; active: boolean; expires_at?: string; clicks: number; created_at: string; updated_at: string;
};
```

### 5.3 Redirect and click tracking (public)

* `GET /l/:code` (API root, not under `/api/v1`): records a click and
  responds `302` to `full_url` + `sy_click=<click id>`. Unknown, inactive or
  expired codes redirect to the frontend home (`302`). Known crawlers/link
  unfurlers are recorded as bot clicks and excluded from metrics. Visitors
  are counted uniquely with a keyed hash of IP + user agent per day (no raw
  IPs stored). Rate-limited by IP.
* Frontend `https://staryield.net/go/:code` → `302` to `${API}/l/:code`.

### 5.4 Attribution capture (main site)

The frontend reads `utm_source, utm_medium, utm_campaign, utm_content,
utm_term, sy_click` from any landing URL, keeps the **first touch** for 30
days in `localStorage` (`staryield.attribution`), removes `sy_click` from the
address bar, and sends a `campaign_landing` analytics event. On sign-up it
sends:

```ts
POST /auth/register { ..., attribution?: {
  click_id?: string; utm_source?: string; utm_medium?: string; utm_campaign?: string;
  utm_content?: string; utm_term?: string; referrer?: string; landing_path?: string; first_seen_at?: string } }
```

The server validates and stores it in `user_attributions` (one row per user):
a valid `click_id` links the user to its link and campaign; otherwise
`utm_campaign` is matched to a campaign slug. Purchases are attributed through
the user's attribution (click → sign-up → credit purchase funnel).

New allowed client event: `campaign_landing` (properties: `utm_source,
utm_medium, utm_campaign, utm_content, has_click`).

### 5.5 Reports (admin)

| Method & path | Response |
| --- | --- |
| `GET /acquisition/overview?from&to&interval` | `AcquisitionOverview` |

```ts
type GrowthTotals = {
  clicks: number; unique_clicks: number; bot_clicks: number;
  signups: number; purchasers: number; purchases: number; revenue_cents: number; currency: string;
  click_to_signup_rate: number;      // signups / unique_clicks (0..1)
  click_to_purchase_rate: number;    // purchasers / unique_clicks
  signup_to_purchase_rate: number;   // purchasers / signups
  cost_per_signup_cents?: number; cost_per_purchaser_cents?: number; roas?: number;   // campaigns with a budget
};
type GrowthPoint = { bucket: string; clicks: number; unique_clicks: number; signups: number; purchasers: number; revenue_cents: number };
type GrowthReport = {
  totals: GrowthTotals; series: GrowthPoint[];
  by_link: { link_id: string; code: string; label: string; clicks: number; unique_clicks: number; signups: number; purchasers: number; revenue_cents: number }[];
  by_source: { source: string; medium: string; clicks: number; signups: number; purchasers: number; revenue_cents: number }[];
  devices: { key: string; count: number }[];   // mobile | tablet | desktop
  referrers: { key: string; count: number }[]; // referrer host
};
type AcquisitionOverview = GrowthReport & {
  campaigns: { id: string; name: string; slug: string; clicks: number; signups: number; purchasers: number; revenue_cents: number }[];
  organic_signups: number;                       // sign-ups without attribution
};
```

---

## 6. Admin: audience and user insights

| Method & path | Response |
| --- | --- |
| `GET /insights/audience?from&to` | `Audience` |
| `GET /insights/engagement?from&to&interval` | `Engagement` |
| `GET /insights/retention?weeks=8` | `{cohorts: {cohort: string; size: number; weeks: number[] /* 0..1, week 0 first */}[]}` |
| `GET /insights/top-users?metric=spend|minutes|sessions&from&to&limit=20` | `{items: {user: AdminUserRow; value: number}[]}` |
| `GET /insights/users/:id` | `UserInsights` |

```ts
type KeyCount = { key: string; count: number };
type Audience = {
  total_users: number; new_users: number;
  by_role: KeyCount[]; by_status: KeyCount[]; by_locale: KeyCount[];
  by_country: KeyCount[];          // ISO 3166-1 alpha-2 from the birth place, "" = unknown
  by_age: KeyCount[];              // "18-24","25-34","35-44","45-54","55-64","65+","unknown"
  by_sign: KeyCount[];             // sun sign from birth date
  by_device: KeyCount[];           // from web events in range (distinct visitors)
  by_source: KeyCount[];           // utm_source of attributed users ("direct" when none)
  signups: { bucket: string; count: number }[];
};
type Engagement = {
  active_users: { bucket: string; users: number }[];   // distinct signed-in users with any event
  dau: number; wau: number; mau: number; stickiness: number;   // dau/mau
  paying_users: number; arppu_cents: number; repeat_purchase_rate: number;
  avg_session_minutes: number; sessions_per_paying_user: number; chat_conversion_rate: number; // clients with ≥1 session / active clients
};
type UserInsights = {
  user: AdminUser;
  psychics: { id: string; name: string; slug: string; sessions: number; minutes: number }[];
  devices: KeyCount[];
  activity: { at: string; type: "signup"|"purchase"|"session"|"request"|"review"|"ledger"|"event"|"admin"; summary: string; ref_id?: string }[]; // newest 100
};
```

---

## 7. Notifications (main site) and broadcasts (admin)

### 7.1 User notifications

| Method & path | Notes |
| --- | --- |
| `GET /me/notifications?cursor=&limit=` | `Page<Notification>` (limit 20/100), newest first. |
| `GET /me/notifications/unread-count` | `{count}` |
| `POST /me/notifications/read` | `{ids?: string[]}` (≤ 100) or `{all: true}` → `{unread}` |

Realtime: `notification.created` `{notification: Notification}` (durable event).

```ts
type Notification = { id: string; kind: NotificationKind; data: Record<string, unknown>; link: string; read_at?: string; created_at: string };
```

Notification text is rendered **by the client** in the viewer's current
language from `kind` + `data` (never stored as prose), except broadcasts,
which carry admin-written copy per locale.

| kind | recipient | data |
| --- | --- | --- |
| `chat.requested` | psychic | `{session_id, client_name}` |
| `chat.missed` | psychic | `{session_id, client_name}` (request expired unanswered) |
| `chat.review_prompt` | client | `{session_id, psychic_name, psychic_slug}` |
| `review.received` | psychic | `{review_id, rating}` |
| `request.created` | psychic | `{request_id, service_name, client_name}` |
| `request.updated` | client & psychic | `{request_id, service_name, status}` |
| `purchase.succeeded` | client | `{purchase_id, credits}` |
| `credits.adjusted` | client | `{amount}` |
| `withdrawal.updated` | psychic | `{withdrawal_id, status, amount_credits}` |
| `broadcast` | any | `{broadcast_id, title: {en,fr?,es?}, body: {en,fr?,es?}}` (fallback to `en`) |

`link` is a locale-less app path (`/chatroom?reader=theo&review=<session>`,
`/profile/analytics`, `/profile`); clients prefix the current locale.

Delivery never blocks the triggering request: notifications are queued to an
in-process async dispatcher (bounded buffer, batch insert, then realtime
publish). A full buffer drops with an error log rather than slowing the API.

### 7.2 Broadcasts (admin)

| Method & path | Notes |
| --- | --- |
| `POST /broadcasts` | `BroadcastInput` → `202 Broadcast` (status `queued`; delivery is asynchronous). `audience:"all"` is 🔐. |
| `GET /broadcasts?cursor=` | `Page<Broadcast>` |
| `GET /broadcasts/:id` | `Broadcast` |

```ts
type BroadcastInput = {
  audience: "all" | "clients" | "psychics" | "users";
  online_only?: boolean;                 // only users connected right now
  user_ids?: string[]; emails?: string[]; // audience "users" (≤ 1000 total)
  campaign_id?: string;                  // narrow to users attributed to a campaign
  title: { en: string; fr?: string; es?: string };   // ≤ 120 chars each
  body: { en: string; fr?: string; es?: string };    // ≤ 1000 chars each
  link?: string;                         // locale-less app path starting with "/"
};
type Broadcast = BroadcastInput & { id: string; status: "queued"|"sending"|"sent"|"failed"; recipients: number; created_by?: string; created_at: string; sent_at?: string };
```

---

## 8. Psychic analytics, reviews and withdrawals (main site)

| Method & path | Role | Notes |
| --- | --- | --- |
| `GET /psychic/analytics?from&to&tz&interval` | psychic | `PsychicAnalytics`. `tz` = IANA zone for hour/day bucketing (default UTC). |
| `GET /psychic/balance` | psychic | `{available_credits, available_cents, currency, reserved_credits, lifetime_net_credits}`: withdrawable earnings for the header wallet pill (`available_cents` = credits × payout cents per credit). |
| `GET /psychic/stats/overview?from&to&tz` | psychic | `StatsOverview`: the "Advisor stats overview" cards for the window and for the window of the same length just before it. Dates are calendar days in `tz` (`to` inclusive); default the last 30 days; window ≤ 400 days; cached 60 s. |
| `GET /psychic/reviews?cursor=` | psychic | `Page<Review>` (own visible reviews). |
| `GET /psychic/withdrawals?cursor=` | psychic | `Page<Withdrawal>` |
| `POST /psychic/withdrawals` | psychic | `{amount_credits, method: "bank_transfer"|"paypal", destination, idempotency_key}` → `Withdrawal` (201). One open (pending/approved) withdrawal at a time; amount ≥ `min_withdrawal_credits` and ≤ available. `destination` ≤ 200 chars (IBAN/account or PayPal email), stored encrypted; only a masked hint is returned. |
| `POST /psychic/withdrawals/:id/cancel` | psychic | pending only → `Withdrawal`. |

```ts
type PsychicAnalytics = {
  range: { from: string; to: string; tz: string; interval: string };
  earnings: { gross_credits: number; commission_credits: number; net_credits: number; sessions: number; custom_requests: number;
              series: { bucket: string; net_credits: number; gross_credits: number; sessions: number }[] };
  payout: { available_credits: number; reserved_credits: number; paid_credits: number; lifetime_net_credits: number;
            cents_per_credit: number; currency: string; min_withdrawal_credits: number };
  sessions: { requested: number; accepted: number; declined: number; expired: number; cancelled: number; completed: number;
              acceptance_rate: number; avg_response_seconds: number; avg_duration_seconds: number; total_minutes: number };
  clients: { unique: number; new: number; returning: number; returning_rate: number;
             top: { name: string; sessions: number; minutes: number }[] };   // first names only
  rating: { average: number; count: number; distribution: Record<"1"|"2"|"3"|"4"|"5", number> };
  score: { value: number /* 0..100 */; grade: "excellent"|"good"|"fair"|"needs_attention";
           components: { acceptance: number; rating: number; retention: number; completion: number; responsiveness: number } };
  hours: { grid: number[][];        // [7][24] chat requests received (client demand); weekday 0 = Monday, in `tz`
           levels: ("peak"|"medium"|"low"|"none")[][];
           best: { weekday: number; hour: number; sessions: number }[] };   // top 3 slots by requests
};
type Withdrawal = { id: string; amount_credits: number; amount_cents: number; currency: string;
  method: "bank_transfer"|"paypal"; destination_hint: string; status: "pending"|"approved"|"paid"|"rejected"|"cancelled";
  note: string; requested_at: string; processed_at?: string };
```

Success score = weighted mean of: acceptance rate (25%), rating/5 (30%),
returning-client rate (15%), completion = sessions not ended by
disconnect/grace (15%), responsiveness = 1 − min(avg accept seconds / 90, 1) (15%).
Grades: ≥ 85 excellent, ≥ 70 good, ≥ 50 fair, else needs_attention.
Hour levels: slots with requests are split into terciles by value (top = peak,
equal counts share a level); the grid shows when clients want readings, which
is when advisors should be online. Components without data (e.g. no ratings
yet) are left out and the remaining weights are renormalised.

---

### 8.1 Advisor stats overview

```ts
type StatsRatio = { pct: number | null; numerator: number; denominator: number }; // pct null = nobody qualified
type StatsKPIs = {
  free_to_paid: StatsRatio;       // clients whose FIRST started session with the advisor fell in the window and was free → share who later paid
  paid_to_retained: StatsRatio;   // clients whose first PAID session fell in the window → share with a later paid session
  paid_minutes: number;           // Σ(billed_minutes − free_minutes_used) over sessions started in the window
  retained_repeat_days: { median_days: number | null; sample: number }; // median gap between consecutive paid sessions (later one in the window)
  active_hours: { hours: number; in_session_hours: number; tracked_since?: string }; // online+busy availability overlap; history starts at deploy
};
type StatsOverview = { range: { from: string; to: string; tz: string }; kpis: StatsKPIs; previous: StatsKPIs };
```

### 8.2 Advisor settings: contact, photo review, custom offering

The advisor Settings page (Figma 1219:1284). Every route acts on the
signed-in advisor only (role `psychic`; clients get `403`, anonymous
callers `401`), so there is no identifier to probe. Writes are CSRF
protected and rate limited (`writes`; photo uploads have their own bucket,
see below).

**Settings.**

```ts
type AdvisorSettings = {
  version: number;          // bumps on every save; send it back as expected_version
  phone: string;            // E.164 ("+14155552671") or ""; only ever returned to its owner
  phone_masked: string;     // "+•••••••••71" or ""
  headline: string; about: string[]; philosophy: string;
  modalities: string[];     // the public profile's skills
};
type AdvisorSettingsPatch = Partial<Omit<AdvisorSettings, "version" | "phone_masked">> & { expected_version?: number };
```

| Method | Path | Notes |
|---|---|---|
| GET | `/psychic/settings` | `AdvisorSettings` |
| PATCH | `/psychic/settings` | `AdvisorSettingsPatch` → `AdvisorSettings`. Absent fields are unchanged. 409 `version_conflict` (`details.current_version`) when `expected_version` is stale; the version is checked under a row lock. Saving identical values writes nothing. |

- **phone:** separators (spaces, `-`, `.`, brackets) are stripped, then it must match E.164 (`+`, 7–15 digits, no leading 0); `""` removes it. Sealed at rest with AES-GCM (associated data `advisor-phone:<advisor id>`, so a copied ciphertext does not open). Never on the public profile, cards or admin views; included in the advisor's own data export.
- **headline, about, philosophy, modalities** share `PATCH /psychic/profile`'s validation: headline ≤ 200, philosophy ≤ 600, about ≤ 6 paragraphs of ≤ 2,000 characters (blank ones dropped), modalities ≤ 8 of 1–40 characters, de-duplicated case-insensitively (first spelling kept). Changes to them refresh the public directory. Both endpoints bump `version`.
- Saves are audited (`psychic.settings_updated`, field names only, never values).

**Profile photo (self-upload with review).** A photo stays **private** until
an administrator approves it: it is stored in the private bucket
(`R2_CHAT_IMAGES_PRIVATE`, prefix `advisor-photos/`) and streamed only to its
owner and to administrators after a step-up. On approval it is copied to
the public advisor bucket and becomes `image_url`.

```ts
type PhotoSubmission = { id: string; status: "pending" | "approved" | "rejected"; reason?: string;
  width: number; height: number; bytes: number;
  has_image: boolean;          // GET /psychic/photo/image can still stream it (pending or rejected)
  submitted_at: string; reviewed_at?: string };
type PhotoState = { image_url: string;            // the live public photo ("" when none)
  submission: PhotoSubmission | null;              // the latest submission
  limits: { max_bytes: number; max_pixels: number; min_side: number; daily_uploads: number } };
```

| Method | Path | Notes |
|---|---|---|
| GET | `/psychic/photo` | `PhotoState` |
| POST | `/psychic/photo` | `multipart/form-data`, field `photo`: PNG/JPEG/WebP, ≤ 5 MB, ≤ 40 MP, sniffed by content. EXIF orientation applied, metadata stripped, longest side scaled to 1,200 px, re-encoded to JPEG; after that both sides must be ≥ 256 px (422 `fields.photo`). → 201 `PhotoState`. A pending submission is superseded (withdrawn, its image deleted). Rate limits: bucket `advisor_photo` (6/h, burst 3, fails closed) and at most 10 uploads per 24 h counted in the database (withdrawn ones included) → 429 `rate_limited`. 503 `feature_unavailable` without both buckets. |
| DELETE | `/psychic/photo` | Withdraws the pending submission (its image is deleted) → 204; 404 when none is pending. |
| GET | `/psychic/photo/image` | The owner's pending or rejected photo, `Cache-Control: no-store, private`; 404 otherwise. |

The web app crops the photo to a square in the browser (with a face guide
in the upper half) before uploading; the server does not trust the crop and
re-validates the image. `psychics.photo_review_status` mirrors the latest
submission (`pending`/`rejected`, else `approved`).

Notifications (in-app only): `photo.approved` `{submission_id}` and
`photo.rejected` `{submission_id, reason}`, both linking to `/profile/settings`.

**Retention.** Withdrawn and superseded submissions are soft-deleted at
once. Rejected ones stay visible to the advisor for 7 days; the
`advisor photo submissions` clean-up then deletes the image and soft-deletes
the row, retries image deletions that failed earlier (approved, withdrawn or
removed rows still holding a key) and purges soft-deleted rows after 90 days.
Account erasure deletes every submission and its image, the published photo
and the phone number (images first: when storage fails the erasure fails and
can be retried).

**Custom offering ("Customs").** One `psychic_services` row per advisor with
`kind = 'custom'`, listed on the public profile after the services curated by
Staryield (`kind = 'standard'`, which advisors cannot edit) and ordered
through the usual custom-request flow.

```ts
type CustomService = { id: string; name: string; description: string; price_credits: number;
  delivery_days: number; active: boolean;  // false when the Staryield team hid it
  created_at: string; updated_at: string };
type CustomServiceState = { service: CustomService | null;
  limits: { name_max: 80; description_max: 1000; min_price_credits: 10; max_price_credits: 50000;
            min_delivery_days: 1; max_delivery_days: 60 } };
```

| Method | Path | Notes |
|---|---|---|
| GET | `/psychic/service` | `CustomServiceState` |
| PUT | `/psychic/service` | `{name, price_credits, delivery_days, description?}` (full replacement; name 1–80, price 10–50,000 credits, delivery 1–60 days, description ≤ 1,000) → 201 when created, 200 when replaced (same id). A hidden offering stays hidden. Audited. |
| DELETE | `/psychic/service` | Soft delete → 204 (orders keep it); 404 when none. Audited. |

`Psychic.services[]` (public profile) gains `kind: "standard" | "custom"` and
`delivery_days`. When an order is accepted, the delivery deadline is the
platform's (`STARYIELD_CUSTOM_REQUESTS_DELIVERY_TIMEOUT`, 7 days) or the
offering's `delivery_days`, whichever is longer.

#### 8.2.1 Admin: photo reviews (`/api/v1/admin`, 🔐 step-up on every route)

| Method | Path | Notes |
|---|---|---|
| 🔐 GET | `/photo-submissions?status=pending\|approved\|rejected&cursor&limit≤100` | `Page<AdminPhotoSubmission>` (default 20). Pending ones oldest first (a queue), the others newest first; omit `status` for every live submission. |
| 🔐 GET | `/photo-submissions/:id/image` | Streams the private image (`no-store, private`); 404 once approved or cleaned up. Audited `photo_submission.viewed`. Must be fetched by the console (admin origin and session), never linked. |
| 🔐 POST | `/photo-submissions/:id/approve` | → `AdminPhotoSubmission`. Publishes the photo (public bucket, `image_url`/`image_key`), deletes the previous public photo and the private copy, refreshes the directory, broadcasts the status and notifies the advisor. 409 `invalid_state` when already reviewed or the image is gone. Audited. |
| 🔐 POST | `/photo-submissions/:id/reject` | `{reason}` (1–500, shown to the advisor) → `AdminPhotoSubmission`; notifies the advisor. 409 `invalid_state` when already reviewed. Audited with the reason. |

```ts
type AdminPhotoSubmission = { id: string; status: "pending" | "approved" | "rejected";
  psychic: { id: string; name: string; slug: string; image_url: string };
  content_type: string; bytes: number; width: number; height: number;
  reason?: string; has_image: boolean; reviewed_by?: string; reviewed_at?: string; submitted_at: string };
```

## 9. Chat: attachments, ratings, client profile, presence

### 9.1 Image attachments

| Method & path | Notes |
| --- | --- |
| `POST /chat/sessions/:id/attachments` | `multipart/form-data`: `image` (PNG/JPEG/WebP/GIF first frame, ≤ 8 MB, ≤ 40 MP), `caption?` (≤ max message length), `client_msg_id?`. Session must be `active`, caller a participant. Re-encoded (JPEG, or PNG when the source has transparency; max side 2048 px; metadata stripped), stored in the private **`R2_CHAT_IMAGES_PRIVATE`** bucket. → `{message: Message}` (201); also published as `message.created`. Rate-limited (`upload` bucket + chat message rate). |
| `GET /chat/attachments/:id` | Image bytes. Only the two conversation participants (sender and receiver) — everyone else gets `404`. `Cache-Control: private, max-age=86400`, `Content-Disposition: inline`. |

`Message` gains: `kind: "text" | "system" | "image"` and
`attachments?: {id, content_type, width, height, bytes}[]`. For image
messages `body` is the (possibly empty) caption. Text bodies keep newlines and
spacing exactly; clients must render them as text (`white-space: pre-wrap`),
linkifying only `http(s)://` and `www.` URLs with
`rel="noopener noreferrer nofollow ugc"` and `target="_blank"`.

### 9.2 Ratings

| Method & path | Notes |
| --- | --- |
| `POST /chat/sessions/:id/review` | client of the session; `{rating: 1..5, comment?: string ≤ 1000}` → `Review` (201). Session must have started and ended/settled, within 7 days of the end, once per session (`409 conflict`). Updates the advisor's rating and review count; notifies the advisor. |
| `GET /psychics/:slug/reviews?cursor=&limit=` | public `Page<Review>` (visible only, limit 10/50). |

`SessionSummary` (end-of-chat recap, client viewer) gains
`can_review: boolean` and `review?: Review`.

```ts
type Review = { id: string; session_id?: string; rating: number; comment: string; status?: "visible"|"hidden";
                author: { name: string /* first name or nickname */; avatar_url?: string }; created_at: string };
```

### 9.3 Client profile for psychics

`GET /chat/conversations/:id/client` (psychic of that conversation only) →

```ts
type ClientProfile = {
  id: string; name: string; avatar_url?: string; online: boolean; locale: string;
  country_code?: string;            // from birth place (ISO alpha-2)
  age?: number; member_since: string;
  // Same engine as the client's own chart: sun sign from the Sun's longitude
  // (sun_sign_uncertain when the unknown birth time straddles an ingress),
  // moon_sign only when it cannot have changed that day, rising_sign only
  // with a birth time and place.
  astro?: { sun_sign: string; sun_sign_uncertain: boolean; moon_sign?: string; rising_sign?: string; life_path?: number; time_known: boolean };
  horoscope?: { sign: string; sign_name: string; date: string; text: string; focus: string; mood: string; affirmation: string };
  history: { sessions: number; minutes: number; first_session_at?: string; last_session_at?: string };
};
```

Never included: email, balance, wallet, purchases, birth place/time, ledger.

### 9.4 Presence

* Conversation list (`GET /chat/conversations`) for psychics now fills
  `counterpart.status` for clients with `online | offline`.
* Realtime `presence.updated` `{user_id, online}` (ephemeral) is sent to a
  psychic when a client they have an open session with connects or disconnects.

### 9.5 Client notes (psychics) and oversight (admin)

Advisors keep notes about the clients they read for. Notes are **private to
their author** unless the author ticks *share*; shared notes are readable
(never editable) by other advisors **who have had at least one started
session** with the same client. Bodies are sealed at rest (AES-GCM, associated
data `note:<id>:<author_id>`). Every write and every admin read is audited
(`note.created|edited|shared|unshared|deleted|removed`, `notes.viewed`); the
audit trail never contains note text.

```ts
type Note = { id: string; body: string; shared: boolean; created_at: string; updated_at: string };
type SharedNote = { id: string; body: string; author: { name: string; slug: string; avatar_url?: string };
  created_at: string; updated_at: string };
```

| Method | Path | Notes |
|---|---|---|
| GET | `/chat/conversations/:id/notes?cursor&limit≤50` | psychic of that conversation; own notes about its client, newest first; `Page<Note>` (default 5) |
| POST | `/chat/conversations/:id/notes` | `{body: 1–2000 chars, shared?: false}` → 201 `Note`; 403 until the advisor has had a started session with the client; 409 `limit_reached` after 500 live notes per client; rate limit `notes` 30/h |
| PATCH | `/chat/notes/:id` | author only (404 otherwise); `{body?, shared?}` (at least one) → `Note` |
| DELETE | `/chat/notes/:id` | author only; soft delete → 204 |
| GET | `/chat/conversations/:id/notes/shared?cursor&limit≤50` | psychic who has met the client; other advisors' shared notes → `Page<SharedNote>` |
| GET | `/admin/client-notes?client_id\|author_id&reason&include_deleted&cursor&limit≤100` | **step-up**; a client or advisor filter and a `reason` (≤300) are required; includes private notes; `Page<AdminNote>` with `client{id,name,email}` and `author{id,name,slug}`; audited `notes.viewed` |
| DELETE | `/admin/client-notes/:id` | **step-up**; `{reason}` → 204 (soft delete, audited `note.removed`) |

Pages fetch one extra row, so `next_cursor` is only present when another page
exists. Account erasure (`DELETE /admin/users/:id`) hard-deletes every note
written by or about the user. Notes are not part of the client's self-service
data export; access requests are handled by an admin through the endpoint
above (the console offers a JSON download).

### 9.6 Advisor directory, home selection and presence history

The public directory and the home page cards are served from an in-memory
snapshot of every listed advisor of an active account (card fields only, with
the computed status). The snapshot is rebuilt at most every 10 s, with one
rebuild in flight per instance; a failed rebuild keeps serving the previous
snapshot. Status changes published on the instance are patched into the
snapshot in place, and profile, listing, image and account-status edits mark
it stale. Other instances, rating changes and presence that simply expired
catch up within 10 s. `PublishStatus` is serialised per advisor, so the last
broadcast always carries the latest state.

```ts
type AdvisorCard = { slug: string; name: string; headline: string; image_url: string; zodiac: string; skills: string[];
  rating: number; reviews_count: number; readings_count: number; years_experience: number; rate_per_minute: number;
  status: "online" | "busy" | "away" | "offline"; featured: boolean };
type DirectoryPage = { items: AdvisorCard[]; page: number; page_size: number; total: number; total_pages: number };
type FeaturedList = { items: (AdvisorCard & { cta: "chat" | "waitlist" })[]; generated_at: string };
```

| Method | Path | Notes |
|---|---|---|
| GET | `/psychics?page&page_size≤48&status&q≤60&skill≤40` | public; numbered page in the curated order (default size 24); `status` is `online\|busy\|away\|offline`; `q` matches name or headline, `skill` one skill (case-insensitive); `Cache-Control: public, max-age=10` |
| GET | `/psychics/featured?limit=1–6` | public, per-IP rate limit (`featured`); default 3. Order: status (online, busy, away, offline), then Bayesian rating `(v·R + 20·C)/(v + 20)` with `C` the platform mean, then review count, then last time online. Skills trimmed to 3. `Cache-Control: public, max-age=10, stale-while-revalidate=30`, weak `ETag` over the cards (304 on `If-None-Match`) |

Cards never carry account identifiers, contact details, long-form profile
text or exact presence timestamps.

**Presence history.**
- Every published status opens or closes a `psychic_presence_intervals` row (online, busy or away; offline closes the open one).
- `psychics.last_online_at` moves forward whenever an online or busy interval opens or closes.
- The `presence-history` job (every 30 s) closes intervals whose advisor went offline without an event, for example after a crashed connection.
- The history feeds the Active Hours KPI (§8.1); `last_online_at` feeds the featured order.

### 9.7 Advisor waiting lists (clients)

A client can wait for an advisor who cannot chat right now and is told
**once** when the advisor is online.
- When the advisor comes online, their open entries are stamped.
- Members whose plan has `waitlist_priority` are notified straight away (on the next run of the `waitlist` job, which is woken immediately).
- Everyone else is notified 2 minutes later, and only if the advisor is still online.
- If the advisor goes away first, the entry waits for the next time.

The notification uses kind `waitlist.available` with data `{psychic_name, psychic_slug}` and link `/chatroom?reader=<slug>`. It goes in-app, by push and by e-mail, following the user's notification switches.

```ts
type WaitlistEntry = { slug: string; name: string; image_url: string; status: string; joined_at: string };
```

| Method | Path | Notes |
|---|---|---|
| POST | `/psychics/:slug/waitlist` | client only; idempotent → `WaitlistEntry`. 404 for unknown, hidden or banned advisors; 409 `advisor_available` when the advisor is online now (start a chat instead); 409 `limit_reached` with 20 open lists; rate limit `waitlist` 30/h |
| DELETE | `/psychics/:slug/waitlist` | → 204 whether or not the client was on the list |
| GET | `/me/waitlist?cursor&limit≤50` | open entries for public advisors, newest first, with live status; `Page<WaitlistEntry>` (default 20) |

**Retention.**
- A notified entry is finished, and the client can join again later.
- Open entries expire after 7 days.
- Finished entries are deleted after 90 days.
- Account erasure deletes the user's entries and every entry waiting for them as an advisor.

### 9.8 Saved messages, read receipts and live advisor earnings

| Method | Path | Notes |
|---|---|---|
| GET | `/chat/saved?conversation_id&cursor&limit` | the caller's saved messages, most recently saved first, as `Page<Message>`. Keyset cursor; `next_cursor` only when another page exists. `limit` defaults to and is capped at 200, so older callers see no change; the chat panels ask for 5. A bad `cursor` or out-of-range `limit` returns 422 |

- **Read receipts:** conversation views (including `/chat/psychics/:slug`) carry `peer_read_seq`, how far the *other* participant has read. "Seen" therefore survives a reload; live updates still arrive as `message.read` frames with `seq`.
- **Live earnings:** the advisor's `session.billed` event carries `psychic_earnings`, `psychic_earnings_cents` and `currency`, the same values as the advisor's session view. The client's copy of the event never includes them.

---

## 10. Localization of server messages

Every user-facing server message (error `message`, validation field messages
in `details.fields`, realtime error frames) is returned in the caller's
language: `X-Locale` header (sent by the clients), else `Accept-Language`,
else English. Supported: `en`, `fr`, `es`. WebSocket connections pass
`&locale=xx` on connect. A Go test fails the build if any server message
literal lacks a French or Spanish translation.

---

## 11. Configuration added

Configuration is environment variables only (a local `.env` file in
development); `backend/.env.example` lists every variable with its default.

| Environment variable | Default | Purpose |
| --- | --- | --- |
| **`R2_ADVISOR_IMAGE`** (or `STARYIELD_STORAGE_ADVISOR_BUCKET`) | "" | Public R2 bucket for advisor images (uploads disabled when empty with `STARYIELD_STORAGE_DRIVER=r2`). |
| `R2_ADVISOR_IMAGE_PUBLIC_URL` (or `STARYIELD_STORAGE_ADVISOR_PUBLIC_BASE_URL`) | `https://advisorsassets.staryield.net` | Public URL of that bucket. |
| **`R2_CHAT_IMAGES_PRIVATE`** (or `STARYIELD_STORAGE_CHAT_BUCKET`) | "" | Private R2 bucket for chat images (never public). |
| `STARYIELD_SECURITY_ADMIN_ORIGINS` | `http://localhost:3002` | Origins allowed to call `/api/v1/admin/**`. |
| `STARYIELD_SECURITY_ADMIN_IP_ALLOWLIST` | "" | Optional comma-separated CIDRs for admin routes. |
| `STARYIELD_SECURITY_DATA_ENCRYPTION_KEY` | dev-derived | 32-byte key (hex/base64) for TOTP secrets and payout details. Required in production. |
| `STARYIELD_ADMIN_SESSION_TTL` / `STARYIELD_ADMIN_IDLE_TIMEOUT` / `STARYIELD_ADMIN_STEP_UP_WINDOW` | 8h / 30m / 10m | Admin session policy. |
| `STARYIELD_LINKS_PUBLIC_BASE_URL` | `http://localhost:3000/go` | Base of short URLs. |
| `STARYIELD_LINKS_ALLOWED_HOSTS` | frontend host | Hosts allowed as absolute destinations. |
| `STARYIELD_BILLING_PAYOUT_CENTS_PER_CREDIT` | 2 | Value of one earned credit at payout. |
| `STARYIELD_BILLING_MIN_WITHDRAWAL_CREDITS` | 2500 | Minimum withdrawal. |
| `STARYIELD_REVIEWS_WINDOW` | 168h | Rating window after a session ends. |
| `STARYIELD_NOTIFICATIONS_BUFFER_SIZE` | 10000 | Async dispatcher queue. |
| `R2_EXPORTS_PRIVATE` (or `STARYIELD_STORAGE_EXPORT_BUCKET`) | "" (falls back to the private chat bucket) | Private R2 bucket for personal-data export archives (§13.5). |
| `STARYIELD_MAIL_SUPPORT_INBOX` | `support@staryield.net` | Receives contact-form requests (§14); empty disables the team copy. |
| `STARYIELD_AUTH_ARGON2_CONCURRENCY` | 0 (≈ one per CPU: GOMAXPROCS ÷ parallelism, min 2) | Password hashes (Argon2id, 64 MiB each) allowed at once. Bounds memory under login bursts and credential stuffing. |
| `STARYIELD_AUTH_PASSWORD_HASH_WAIT` | 3s | How long a login, sign-up or password check waits for a hashing slot before a retryable `503 service_unavailable` (with `Retry-After`). It is never counted as a failed login. |

Development: with `STARYIELD_STORAGE_DRIVER=memory` the API serves the public buckets
at `/_dev/storage/{avatars,advisors}/…` so uploaded images display locally
(set `R2_ADVISOR_IMAGE_PUBLIC_URL=http://localhost:8080/_dev/storage/advisors`).
Never enabled in production and never for the private chat bucket.

---

## 12. Billing: Premium Membership, saved cards, payments

Card data never reaches the API. The browser loads the routed provider's
card fields (`GET /billing/client-config`), the provider returns a token, and
only that token is posted. Bodies with unknown fields (`number`, `cvc`, …) are
rejected, and any 13–19 digit run inside a token is refused. Prices always
come from the server-side plan version, never from the client.
`Idempotency-Key` (header or `idempotency_key` body field, 8–128 characters)
is required wherever noted. A replay returns the original result.

Error codes: `payment_failed` (402, `details.reason` = decline code such as
`card_declined`, `expired_card`, `processing_error`), `subscription_exists`
(409), `payment_method_required` (409), `limit_reached` (409, card limit),
`invalid_state` (409).

### 12.1 Client endpoints (role client)

| Method & path | Notes |
| --- | --- |
| `GET /billing/plans` | Public. `{items: Plan[]}` in the caller's locale. Cached 5 min. |
| `GET /billing/client-config?purpose=credits\|subscription&plan_version_id=` | `ClientConfig` of the provider routed for this user/country. |
| `GET /billing/payment-methods` | `{items: PaymentMethod[]}` (default first). |
| `POST /billing/payment-methods` | `{provider, token, billing_country?, make_default?, idempotency_key}` → `201 PaymentMethod` (`200` on replay or when the same card is already saved). Rate limited. |
| `DELETE /billing/payment-methods/:id` | `204`. `409 payment_method_required` if it is the only card of an auto-renewing membership. |
| `POST /billing/payment-methods/:id/default` | `PaymentMethod` |
| `GET /billing/subscription` | `{subscription: Subscription \| null}` (the open membership, else the latest ended one). |
| `POST /billing/subscriptions` | `{plan_version_id, payment_method_id, idempotency_key}` → `201 SubscriptionResult`. The first period is charged now. A decline returns `402` and the membership is `expired`. An unknown provider outcome returns `incomplete`, which the worker resolves. |
| `PATCH /billing/subscription` | `{payment_method_id}` → `SubscriptionResult`. A `past_due` membership is charged immediately. |
| `POST /billing/subscription/cancel` | Optional `{reason?}` → `{subscription}`. An `active` membership keeps its benefits until `current_period_end`. An unpaid membership ends now. `at_period_end:false` is rejected. |
| `POST /billing/subscription/resume` | `{subscription}`. Undoes a cancellation before the period ends. |
| `GET /billing/transactions?cursor=&limit=` | `Page<Transaction>` (limit 20/100): credit purchases, membership charges and refunds, newest first. |

```ts
type Plan = { id: string; code: string; name: string; description: string; version_id: string; version: number;
  price_cents: number; currency: string; interval: "month" | "year"; interval_count: number;
  monthly_credit_grant: number; chat_discount_bps: number; chat_discount_pct: number;
  monthly_free_minutes: number; waitlist_priority: boolean; benefits: string[] };
type ClientConfig = { provider: string; type: "sandbox" | "stripe_elements" | "nmi_collectjs" | "authnet_accept_hosted";
  publishable_key?: string; tokenization_key?: string; hosted_form_url?: string; client_secret?: string;
  token_format?: string; test_cards?: { brand: string; last4: string; behaviour: string }[]; expires_at?: string };
type PaymentMethod = { id: string; provider: string; brand: string; last4: string; exp_month: number; exp_year: number;
  is_default: boolean; status: "active" | "expired" | "failed"; expired: boolean; created_at: string };
type Subscription = { id: string; status: "incomplete" | "active" | "past_due" | "canceled" | "expired"; plan: Plan;
  cancel_at_period_end: boolean; current_period_start?: string; current_period_end?: string; renews_at?: string;
  next_retry_at?: string; grace_until?: string; canceled_at?: string; ended_at?: string;
  payment_method?: PaymentMethod; created_at: string };
type Transaction = { id: string; kind: "credit_purchase" | "subscription_initial" | "subscription_renewal" | "refund";
  status: "pending" | "succeeded" | "failed" | "refunded" | "partially_refunded" | "disputed";
  amount_cents: number; refunded_amount_cents: number; currency: string; description: string; credits?: number;
  payment_method?: { brand: string; last4: string }; failure_code?: string; created_at: string };
type NextAction = { type: string; url?: string; client_secret?: string };   // e.g. 3-D Secure
type SubscriptionResult = { subscription: Subscription; transaction?: Transaction; next_action?: NextAction };
```

Lifecycle:

* Renewals bill on the anchor day: a membership started on Jan 31 renews
  Feb 28 and then Mar 31.
* Benefits are granted monthly, including on yearly plans. Unused free
  minutes from the previous grant are removed, so they do not roll over.
* A failed renewal makes the membership `past_due` and is retried after
  24 h, 72 h and 120 h. After the 7-day grace period the membership is
  `expired`. Changing the card retries at once.
* A hard decline marks the card `failed`.
* Account closure (`BillingService.CancelForClosure`) ends the membership and
  forgets the saved cards, both locally and at the provider.

Credit purchases (`POST /credits/purchases`) also accept
`payment_method_id` (a saved card), or `payment_token` with `provider` and
optionally `save_method`. The response may contain `next_action`.

Webhooks: `POST /webhooks/payments/:provider` must be signed (sandbox: header
`X-Staryield-Signature: t=<unix>,v1=<hmac-sha256>`). Receipts are de-duplicated
by event id and only a payload hash is stored. Handled events are
`charge.succeeded`, `charge.failed`, `charge.refunded` (with `refund_ref`,
refunds issued from the admin console are recognised) and `charge.disputed`,
which cancels the membership.

Notifications (§7.1):

| kind | data |
| --- | --- |
| `subscription.renewed` | `{subscription_id, period_end, amount_cents, currency}` |
| `subscription.payment_failed` | `{subscription_id, reason, retry, grace_until?}` |
| `subscription.canceled` | `{subscription_id, reason}`. `reason` is one of `period_ended`, `payment_failed` (grace period expired), `canceled_by_user`, `admin`, `dispute` or `account_closed`. |

Sandboxes (development only, never in production without
`STARYIELD_PAYMENTS_ALLOW_SANDBOX_IN_PRODUCTION`). Tokens have the form
`sbx_<a|b>_<brand>_<last4>_<mm>_<yyyy>[_<country>]`.

* `sandbox_a` supports everything. `sandbox_b` supports vault, stored charges
  and refunds.
* Card last4 `0002` is always declined. Last4 `9995` is a soft decline.
* Amounts ending in `.13` are declined, `.14` soft-declined, and `.15` time
  out (the charge succeeds but the response is lost).

### 12.2 Admin endpoints (`/api/v1/admin/billing`, 🔐 = step-up, all writes audited)

| Method & path | Notes |
| --- | --- |
| `GET /plans` | `{items: AdminPlan[]}` |
| `POST /plans` 🔐 | `{code, name: {en,fr?,es?}, description?, active?, sort_order?, version: PlanVersionInput}` → `201 AdminPlan` |
| `PATCH /plans/:id` 🔐 | `{name?, description?, active?, sort_order?}` (prices are never edited in place) |
| `DELETE /plans/:id` 🔐 | `204`. Returns `409` while it has open memberships. |
| `POST /plans/:id/versions` 🔐 | `PlanVersionInput` → `201 AdminPlan`. Versions are immutable: existing members keep their version, and new sign-ups get the current one. |
| `GET /routing-rules` · `POST` 🔐 · `PATCH /:id` 🔐 · `DELETE /:id` 🔐 | `RoutingRule` CRUD (partial PATCH) |
| `POST /routing-rules/simulate` | `{user_id?, country?, currency?, purpose?, plan_id?}` → `{providers, rule_id?}` |
| `GET /providers` | `{items: {name, capabilities: string[], sandbox}[]}` |
| `GET /subscriptions?user_id=&status=&cursor=&limit=` | `Page<Subscription & {user_id, provider, attempt_count, cancel_reason?}>` |
| `POST /subscriptions/:id/cancel` 🔐 | `{immediate, reason}`. `reason` is required. |
| `GET /transactions?user_id=&status=&kind=&cursor=&limit=` | `Page<Transaction & {user_id, provider, provider_ref?, subscription_id?, credit_purchase_id?, parent_transaction_id?, attempt_no, refundable_cents}>` |
| `POST /transactions/:id/refund` 🔐 | `{amount_cents, reason}`. Purchased credits are clawed back proportionally. Membership credits granted for the refunded period are reversed, but never below the user's balance. If a refund of this payment is still pending, the request is rejected with `409` unless it repeats the same amount, which completes it. |

```ts
type PlanVersionInput = { price_cents: number; currency?: string; interval: "month" | "year"; interval_count?: number;
  monthly_credit_grant: number; chat_discount_bps: number /* 0–5000 */; monthly_free_minutes: number /* 0–600 */;
  waitlist_priority: boolean; benefits?: { en?: string[]; fr?: string[]; es?: string[] }; make_current?: boolean /* default true */ };
type AdminPlan = { id: string; code: string; name: Record<string, string>; description: Record<string, string>; active: boolean;
  sort_order: number; open_subscriptions: number; versions: (PlanVersionInput & { id: string; version: number;
  is_current: boolean; created_at: string })[]; created_at: string; updated_at: string };
type RoutingRule = { id: string; name: string; priority: number; enabled: boolean; purpose: "any" | "credits" | "subscription";
  providers: string[]; match_countries: string[]; match_currencies: string[]; match_plan_ids: string[];
  match_user_ids: string[]; created_at: string; updated_at: string };
```

Routing:

* Rules are evaluated by ascending `priority`. Empty match lists match
  everything.
* The first matching rule decides, and its providers are tried in order.
  If none of its providers can serve the payment, the request fails; it
  does not fall through to the next rule.
* The country comes from Cloudflare's `CF-IPCountry` header, trusted only
  from configured proxies, and from the card's issuing country. A rule
  restricted to certain countries matches only when every known country is
  in its list.

Configuration (in addition to §11): `STARYIELD_PAYMENTS_PROVIDERS`,
`_DEFAULT_PROVIDER`, `_RENEWAL_LEASE`, `_GRACE_PERIOD`, `_INCOMPLETE_TTL`,
`_MAX_PAYMENT_METHODS` and `_RATE_LIMIT_{RATE,PERIOD,BURST}`. See
`backend/.env.example`.

---

## 13. Account settings, devices, e-mail change, consents, exports and closure

Everything is under `/api/v1`. "Sensitive" routes ask for the current password and use the `sensitive` rate-limit bucket.

### 13.1 Settings
- `GET /me/settings` returns `{version, account, profile, appearance, notifications, privacy}`.
  - `privacy` holds `share_birth_details_with_advisors`, `consents{analytics, marketing_attribution, personalization}` and `policy_version`.
- `PATCH /me/settings` takes the same sections, all optional, and saves them in one transaction.
  - Send `expected_version` to detect another tab's change: 409 `version_conflict` with `details.current_version`.
  - Field errors use dotted keys (`profile.birth_time`, `privacy.consents.x`).
- `share_birth_details_with_advisors` (off by default) decides whether advisors see the client's date and place of birth on the client card (§9.3). The birth time and coordinates are never shown.

### 13.2 Devices
- Each refresh-token family is one device (`user_sessions`).
- `GET /me/sessions?limit&cursor` (alias `/me/devices`) returns `{items[{id, label, device_type, browser, os, current, signed_in_at, last_seen_at, expires_at}], next_cursor}`.
- `DELETE /me/sessions/:id` signs one device out; it returns 404 for another user's device.
- `DELETE /me/sessions?scope=others` signs every other device out and returns `{signed_out}`.
- Revocation:
  - Sets `auth:famrev:<family>` in Redis, so that device's access token fails at once (the database is the fallback).
  - Closes that device's WebSockets.
  - A realtime ticket issued to a device that was signed out afterwards is refused at the handshake (401 `session_expired`).

### 13.3 E-mail change
- `POST /me/email-change {email, password}` always answers 202, whether or not the address is in use.
- A single-use, hashed link goes to the new address, and a cancel link to the current one.
- `POST /auth/email-change/{confirm|cancel} {token}` returns `{status: confirmed|cancelled|restored}`.
- The previous address can restore the change from its notice.
- Frontend pages: `/{lang}/confirm-email`, `/{lang}/cancel-email-change` and `/{lang}/keep-account`, each with `?token=`.

### 13.4 Consents
- `GET /consent-policy` is public.
- `GET|PUT /me/consents {consents{analytics, marketing_attribution, personalization}, source?}`.
- Defaults:
  - Analytics and attribution are off by default.
  - Personalisation is on by default (legitimate interest); withdrawing it hides the client's shared advisor notes from other advisors.
- Every change is recorded in the append-only `consent_events`, which is kept when the account is erased.

### 13.5 Personal-data export
- `POST /me/exports {password}` returns 202 with the job.
  - 409 `export_in_progress` if a job is already running.
  - 429 `limit_reached` after one export per 24 h.
- `GET /me/exports` and `GET /me/exports/:id` report the status.
- `POST /me/exports/:id/download-token {password}` returns `{token, expires_at, url}`.
- `GET /me/exports/:id/download?token=` streams the ZIP with `no-store`.
- Archives live in private storage (`STARYIELD_STORAGE_EXPORT_BUCKET`, or the private chat bucket under `exports/`) and expire after 7 days.
- Advisors' notes about the client are not included (Art. 15(4)); they are handled through §9.5.

### 13.6 Deactivate, reactivate and close
- `POST /me/deactivate {password}` returns 204 and revokes every session.
  - Signing in afterwards answers 403 `account_deactivated` with `{reactivation_token, reactivation_expires_at}`.
  - `POST /auth/reactivate {token}` signs the user in.
- `POST /me/close {password, confirmation: "DELETE", reason?}` returns 202 `{status, deletion_scheduled_at}`, 30 days later.
  - Advisors get 409 `closure_blocked` with `reasons` (`earnings_balance`, `pending_withdrawals`, `open_requests`).
  - Signing in during the grace period answers `account_closure_pending` with `deletion_scheduled_at`.
  - `POST /auth/closure/cancel {token}` keeps the account.
- When the grace period ends, the account eraser anonymises the account. It also:
  - ends the membership and forgets saved cards (billing);
  - deletes notes, push subscriptions, devices, consents, exports, waiting lists and the user's support requests.

---

## 14. Support requests (contact form)

`POST /support/tickets`:
- Public: a signed-in user is optional and the ticket is then linked to the account. CSRF is required, the body must be JSON, and unknown fields are rejected.
- Body: `{topic: general|privacy|billing|advisor|technical, subject (3–120), email, message (10–5000), consent: true, name? (≤80)}`.
- Returns 201 `{reference: "SY-XXXXXX", topic, created_at}`.
- Rate limited to 5 per hour, per IP and per user.

Data handling:
- The e-mail address and message are sealed at rest.
- The address and the IP are also stored as keyed hashes, so the team can group requests without decrypting them.

E-mail, through the outbox:
- The support inbox (`STARYIELD_MAIL_SUPPORT_INBOX`) receives the full request.
- The requester receives a localized acknowledgement with the reference only. It never repeats the submitted text, so the form cannot relay content to arbitrary addresses.
- Requests linked to an account are deleted when the account is erased.
