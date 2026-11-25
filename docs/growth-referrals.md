# Growth: the referral loop

Staryield is in beta. Every client can invite friends with a personal link. For each invited friend who signs up with
the link, verifies the e-mail address and **completes a reading** (a settled live-chat session with at least one billed
or free minute), the inviter earns **free chat minutes** (5 by default), **granted at launch**. Before the launch the
minutes are "pending, added at launch"; the administrator launches the rewards once, the pending ones are granted
within a minute and later ones as soon as they qualify. Nothing depends on reviews or ratings. The invited friend gets
the normal sign-up offer only.

Code: `backend/internal/service/referrals*.go`, `site_referrals.go`, `wallet_free_minutes.go`,
`repository/referrals.go`, migration `0036_referrals.sql`; web `src/lib/referral.ts`, `src/lib/referrals/*`,
`src/components/referrals/*`; admin `src/features/referrals/*`.

## Rules

| Rule | Where |
| --- | --- |
| Code: 8 characters of `ABCDEFGHJKMNPQRSTVWXYZ23456789` (no 0/O/1/I/L/U), created on the first `GET /me/referrals`, collision retry | `EnsureCode` |
| A sign-up with `attribution.referral_code` (case-insensitive, trimmed) of an active client other than the new account, while the program is enabled, records the friend as `joined`; anything else is ignored silently and never fails the sign-up | `ReferralService.Attribute` (from `afterSignup`, password and Google) |
| One referral per friend (`referrals.referee_id` unique) | schema |
| Self-referral: the sign-up browser (`attribution.anonymous_id`) is first linked to the inviter in `web_visitors` → `rejected` / `self_referral` | `Attribute` |
| Held for review (`flagged`, never granted automatically): `same_network` (sign-up IP hash equals one of the inviter's active devices seen in the last 30 days), `signup_burst` (10 or more friends joined the inviter in the last 24 h), `shared_browser` (the sign-up browser already belongs to another account) | `Attribute` |
| `joined` → `qualified` when the friend's `users.email_verified_at` is set **and** they have a settled reading with ≥ 1 billed or free minute, in either order; `reward_minutes` is the setting at that moment | `Evaluate` (from `ChatService.OnSessionSettled`, `OnEmailVerified`, and the `referral-sweep` job every 5 min) |
| Per-inviter cap (default 20): a friend qualifying beyond it is `rejected` / `cap_reached` (the inviter's row is locked, so concurrent qualifications cannot both take the last place) | `Evaluate` |
| Program disabled: no new referral, no qualification (the sweep resumes them when re-enabled); earned rewards are still granted after launch | `Attribute`, `Evaluate`, `RunGrants` |
| Grants: after launch, the `referral-grants` job (every minute and woken by the launch, an approval or a new qualification) claims qualified, unflagged rows of active inviters with `FOR UPDATE SKIP LOCKED` in batches of 200 (45 s budget), adds the minutes to `wallets.free_minutes` and marks the rows `granted` in one transaction; `wallet.updated` and one `referral.reward_granted` per inviter per batch after commit | `RunGrants` |
| A decision clears the hold: approve (`flagged=false`) or reject (`rejected` with the admin's reason); the flags stay as history | `Approve`, `Reject` |
| Privacy: the inviter only sees counts; notifications name nobody; only the admin console shows who invited whom | everywhere |

## Settings key

`site_settings.key = 'referral_program'` (CHECK in 0036, value CHECK too):

```json
{ "enabled": true, "reward_minutes": 5, "max_rewards_per_referrer": 20, "launched_at": null }
```

`reward_minutes` 1–60, `max_rewards_per_referrer` 1–500, `launched_at` RFC 3339 once launched (never editable by PUT).
It travels in the cached site snapshot; an unreadable setting means "disabled, not launched".

## Client API

### `GET /api/v1/me/referrals` (clients only, `Cache-Control: private, no-store`)

Not behind the 18+ gate (listed in `tests/e2e/age_gate_routes_test.go`): the client's own code and counters, no money
moves; the rewards need a settled reading, which is gated.

```json
{
  "code": "ABCD2345",
  "link": "https://staryield.net/en?ref=ABCD2345",
  "program": { "enabled": true, "reward_minutes": 5, "max_rewards": 20, "launched": false, "launched_at": "…" },
  "stats": { "joined": 2, "qualified": 1, "pending_minutes": 5, "granted_minutes": 0, "remaining_rewards": 19 }
}
```

`joined` counts friends not rejected; `qualified` counts qualified + granted (what the cap counts). One aggregate query.
403 for advisors, 401 anonymous.

### `GET /api/v1/site/config` gains

```json
"referrals": { "enabled": true, "reward_minutes": 5, "launched": false }
```

### Sign-up

`POST /auth/register` and `POST /auth/google/start` accept `attribution.referral_code` (string). The web app keeps the
first valid `?ref=` code for 30 days (`localStorage["staryield.referral"]`), strips `ref` from the address bar and
forgets the code after a successful sign-up.

## Admin API (all audited writes need a fresh step-up)

| Route | Body / query | Answer |
| --- | --- | --- |
| `GET /admin/referrals` | `status` (joined, qualified, granted, rejected), `flagged` (true/false), `cursor`, `limit` ≤ 100 | `{items: AdminReferral[], next_cursor}` newest first |
| `GET /admin/referrals/summary` | – | counts per status, `flagged`, `pending_minutes`, `granted_minutes`, `ready_rewards`, `ready_minutes`, `top_inviters` (10), `program` |
| `POST /admin/referrals/:id/approve` | – | `AdminReferral`; 409 `invalid_state` unless held and joined/qualified; audit `referral.approved` |
| `POST /admin/referrals/:id/reject` | `{reason}` 3–200 chars | `AdminReferral`; 409 unless joined/qualified; audit `referral.rejected` |
| `GET /admin/site/referral-program` | – | setting with `version`, `updated_at`, `updated_by` |
| `PUT /admin/site/referral-program` | `{enabled, reward_minutes, max_rewards_per_referrer, version}` | setting; 422 field errors; 409 `version_conflict`; audit `site.referral_program.updated` |
| `POST /admin/referrals/launch` | `{confirmation: "LAUNCH"}` | `{launched_at, rewards, minutes}`; 422 without the phrase; 409 `referral_already_launched`; audit `site.referral_program.launched` |

`AdminReferral`: `{id, referrer{id,name}, referee?{id,name}, status, reward_minutes, flagged, flags[], reject_reason?,
joined_at, qualified_at?, granted_at?, decided_by?{id,name}, decided_at?, created_at}` (`referee` is absent once the
friend's account was erased).

## Notifications and realtime

| Kind | Data | Channels |
| --- | --- | --- |
| `referral.joined` | `{}` | in-app, push (not sent for held or rejected sign-ups) |
| `referral.qualified` | `{minutes, launched: false}` | in-app, push (before launch only; after launch the grant notifies) |
| `referral.reward_granted` | `{minutes, friends}` | in-app, push, e-mail (category `account`) |

Link: `/profile/invite`. No new realtime event: the grant publishes `wallet.updated`; the web app refreshes the invite
data on `notification.created` of a `referral.*` kind and on `wallet.updated` while minutes are pending.

## Data protection

- Account erasure deletes the user's code and the referrals they made; as a friend, an unfinished referral is deleted
  and a decided one stays anonymised (`referee_id`, `signup_ip_hash`, `first_session_id` cleared).
- The export (`referrals.json`) holds the user's code, counters and whether they joined with an invite — never who
  invited them or whom they invited.
- `signup_ip_hash` is a keyed hash, never an address.

## Load and query plans

See `backend/tests/load/README.md` ("Referral loop"): `TestExplainHotQueries` (`referrals: …`),
`TestExplainReferralsPopulated` (about 117,000 referrals in a rolled-back transaction, every plan on its index) and
`TestLoadReferrals`.
