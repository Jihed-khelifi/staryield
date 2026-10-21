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
  astro?: { sun_sign: string; moon_sign?: string; rising_sign?: string; life_path?: number; time_known: boolean };
  horoscope?: { sign: string; date: string; text: string; focus: string; mood: string; affirmation: string };
  history: { sessions: number; minutes: number; first_session_at?: string; last_session_at?: string };
};
```

Never included: email, balance, wallet, purchases, birth place/time, ledger.

### 9.4 Presence

* Conversation list (`GET /chat/conversations`) for psychics now fills
  `counterpart.status` for clients with `online | offline`.
* Realtime `presence.updated` `{user_id, online}` (ephemeral) is sent to a
  psychic when a client they have an open session with connects or disconnects.

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
| `R2_ADVISOR_IMAGE_PUBLIC_URL` (or `STARYIELD_STORAGE_ADVISOR_PUBLIC_BASE_URL`) | `https://advisors.staryield.net` | Public URL of that bucket. |
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

Development: with `STARYIELD_STORAGE_DRIVER=memory` the API serves the public buckets
at `/_dev/storage/{avatars,advisors}/…` so uploaded images display locally
(set `R2_ADVISOR_IMAGE_PUBLIC_URL=http://localhost:8080/_dev/storage/advisors`).
Never enabled in production and never for the private chat bucket.
