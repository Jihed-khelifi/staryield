# Growth: the referral loop

Every client can invite friends with a personal link. For each invited friend who signs up with the link, verifies the
e-mail address and **completes a reading** (a settled live-chat session with at least one billed or free minute), the
inviter earns **free chat minutes** (5 by default), added to their wallet by the grant job, usually within a minute of
the friend qualifying. Nothing depends on reviews or ratings. The invited friend gets the normal sign-up offer only.

Until 2026-11-25 the rewards waited for a one-time "launch" by an administrator (Staryield was in beta). The launch is
gone: migration `0042_referral_rewards_always_on.sql` removed `launched_at` from the setting, and whatever was still
pending was granted by the next run of the grant job.

Code: `backend/internal/service/referrals*.go`, `site_referrals.go`, `wallet_free_minutes.go`,
`repository/referrals.go`, migrations `0036_referrals.sql` and `0042_referral_rewards_always_on.sql`; web
`src/lib/referral.ts`, `src/lib/referrals/*`, `src/components/referrals/*`; admin `src/features/referrals/*`.

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
| Program disabled: no new referral, no qualification (the sweep resumes them when re-enabled); earned rewards are still granted | `Attribute`, `Evaluate`, `RunGrants` |
| Grants: the `referral-grants` job (every minute, and woken by an approval or a new qualification) claims qualified, unflagged rows of active inviters with `FOR UPDATE SKIP LOCKED` in batches of 200 (45 s budget), adds the minutes to `wallets.free_minutes` and marks the rows `granted` in one transaction; `wallet.updated` after each batch's commit, then one `referral.reward_granted` per inviter per run with the run's total (however many batches the inviter's friends were spread over; also when a later batch fails or the budget ends the run) | `RunGrants` |
| A decision clears the hold: approve (`flagged=false`) or reject (`rejected` with the admin's reason); the flags stay as history | `Approve`, `Reject` |
| Privacy: the inviter only sees counts; notifications name nobody; only the admin console shows who invited whom | everywhere |

## Settings key

`site_settings.key = 'referral_program'` (CHECK in 0036, value CHECK too):

```json
{ "enabled": true, "reward_minutes": 5, "max_rewards_per_referrer": 20 }
```

`reward_minutes` 1–60, `max_rewards_per_referrer` 1–500. It travels in the cached site snapshot; an unreadable setting
means "disabled".

## Client API

### `GET /api/v1/me/referrals` (clients only, `Cache-Control: private, no-store`)

Not behind the 18+ gate (listed in `tests/e2e/age_gate_routes_test.go`): the client's own code and counters, no money
moves; the rewards need a settled reading, which is gated.

```json
{
  "code": "ABCD2345",
  "link": "https://staryield.net/en?ref=ABCD2345",
  "program": { "enabled": true, "reward_minutes": 5, "max_rewards": 20 },
  "stats": { "joined": 2, "qualified": 1, "pending_minutes": 5, "granted_minutes": 0, "remaining_rewards": 19 }
}
```

`joined` counts friends not rejected; `qualified` counts qualified + granted (what the cap counts); `pending_minutes`
are earned but not added yet (on their way). One aggregate query. 403 for advisors, 401 anonymous.

### `GET /api/v1/site/config` gains

```json
"referrals": { "enabled": true, "reward_minutes": 5 }
```

### Sign-up

`POST /auth/register` and `POST /auth/google/start` accept `attribution.referral_code` (string). The web app keeps the
first valid `?ref=` code for 30 days (`localStorage["staryield.referral"]`), strips `ref` from the address bar and
forgets the code after a successful sign-up (password, Google callback `signed_up` and the Google consent step). On
Google, the code sent to `/auth/google/start` is kept with the attempt and used by the callback or, after the consent
step, by `POST /auth/google/complete` (whose own `attribution`, when sent, replaces it). Google accounts start with a
verified address, so a reading is all such a friend still needs.

## Admin API (all audited writes need a fresh step-up)

| Route | Body / query | Answer |
| --- | --- | --- |
| `GET /admin/referrals` | `status` (joined, qualified, granted, rejected), `flagged` (true/false), `cursor`, `limit` ≤ 100 | `{items: AdminReferral[], next_cursor}` newest first |
| `GET /admin/referrals/summary` | – | counts per status, `flagged`, `pending_minutes`, `granted_minutes`, `ready_rewards`, `ready_minutes` (exactly what the grant job claims: qualified, not held, inviter active), `top_inviters` (10), `program` |
| `POST /admin/referrals/:id/approve` | – | `AdminReferral`; 409 `invalid_state` unless held and joined/qualified; audit `referral.approved` |
| `POST /admin/referrals/:id/reject` | `{reason}` 3–200 chars | `AdminReferral`; 409 unless joined/qualified; audit `referral.rejected` |
| `GET /admin/site/referral-program` | – | setting with `version`, `updated_at`, `updated_by` |
| `PUT /admin/site/referral-program` | `{enabled, reward_minutes, max_rewards_per_referrer, version}` | setting; 422 field errors; 409 `version_conflict`; audit `site.referral_program.updated` |

`AdminReferral`: `{id, referrer{id,name}, referee?{id,name}, status, reward_minutes, flagged, flags[], reject_reason?,
joined_at, qualified_at?, granted_at?, decided_by?{id,name}, decided_at?, created_at}` (`referee` is absent once the
friend's account was erased).

The summary aggregates the whole `referrals` table (`referrals_summary_idx`, index-only once vacuumed) plus the top
inviters, so the console polls it only while rewards are being added (`ready_rewards > 0`); otherwise it refreshes when
the admin returns to the tab and after every decision and setting change.

## Notifications and realtime

| Kind | Data | Channels |
| --- | --- | --- |
| `referral.joined` | `{}` | in-app, push (not sent for held or rejected sign-ups) |
| `referral.reward_granted` | `{minutes, friends}` | in-app, push, e-mail (category `account`) |

A qualification itself sends nothing: the grant job adds the minutes within a minute and `referral.reward_granted`
announces them. `referral.qualified` was only sent before the launch; the API no longer sends it, and the web app still
renders older rows of that kind (without any launch wording).

Link: `/profile/invite`. No new realtime event: the grant publishes `wallet.updated` (the wallet balance). The web app
reads `GET /me/referrals` only on the account views that show it (the invite page, and the overview while the public
configuration announces the program) and refreshes it on `notification.created` of a `referral.*` kind, once per burst
of events (the grant job always sends `referral.reward_granted`, so `wallet.updated` is not needed).

## Data protection

- Account erasure deletes the user's code and the referrals they made; as a friend, an unfinished referral is deleted
  and a decided one stays anonymised (`referee_id`, `signup_ip_hash`, `first_session_id` cleared). The purger is wired
  with `AccountEraser.AddPurger`, which keeps its own list (`EraserDeps.Extra`): no `Set…` slot call can drop it.
- The export (`referrals.json`) holds the user's code, counters and whether they joined with an invite — never who
  invited them or whom they invited.
- `signup_ip_hash` is a keyed hash, never an address.

## Load and query plans

See `backend/tests/load/README.md` ("Referral loop"): `TestExplainHotQueries` (`referrals: …`),
`TestExplainReferralsPopulated` (about 117,000 referrals in a rolled-back transaction, every plan on its index) and
`TestLoadReferrals`.

## Legal text (for the lead: what Terms, Privacy, Refund policy and Disclaimer must state)

The invite page tells clients "Invites are subject to our Terms of Use"; the legal pages must carry these rules.

**Terms of Use: an "Invite friends" (referral program) section**
- Who can invite: active client accounts only; advisors and staff are excluded.
- How a reward is earned, for each invited person who: creates a new Staryield account through the inviter's personal
  link; verifies their e-mail address; completes a live chat reading of at least one minute, paid or free. Existing
  accounts and the inviter's own accounts do not count.
- Reward size: free chat minutes, currently 5 (set by Staryield between 1 and 60 and shown in the app). A reward keeps
  the value in force when it was earned.
- Limits: one reward per invited person; a per-inviter maximum of rewarded friends, currently 20 (set between 1 and
  500). Friends who qualify beyond it earn nothing.
- When minutes are added: automatically, usually within a minute of a friend qualifying, while the inviter's account is
  active; a reward held for review is added only if approved.
- What the minutes are: promotional free chat minutes, usable only for live readings on Staryield. No cash value; they
  cannot be transferred, exchanged, sold or refunded.
- The invited friend gets only the standard welcome offer available to every new client; there is no extra reward for
  being invited.
- Prohibited: self-referrals and multiple or fake accounts; using the same person, device, browser or network to create
  referrals; automated or bulk sign-ups; spam or unsolicited messages; misleading claims (for example promising
  predictions or outcomes); paid ads that use Staryield's name. The inviter is responsible for how they share the link
  and must follow anti-spam laws; when posting publicly they must disclose that they get free minutes (FTC endorsement
  guides, EU UCPD).
- Review: Staryield may automatically hold suspicious referrals for manual review, then approve or reject them,
  withhold rewards, and suspend accounts that abuse the program.
- Changes: Staryield may change, pause or end the program at any time. Minutes already added to the wallet stay there;
  while the program is paused, rewards already earned are kept (granted unless rejected for abuse), and new sign-ups
  and readings earn nothing.
- Closing the account forfeits pending rewards and deletes the personal code.

**Privacy Policy: new processing**
- The inviter's personal referral code, and the link between an inviter and each friend who signed up with it.
- For each referral: its status and dates (joined, qualified, granted or rejected); the friend's first qualifying
  reading; a keyed hash of the friend's sign-up IP address (never the address itself); review flags; any staff
  decision, with the admin and the reason.
- Fraud-prevention checks (purpose: prevent abuse; legal basis: legitimate interest): the sign-up IP hash is compared
  with the inviter's device IP hashes of the last 30 days; the sign-up browser's analytics id with browsers already
  linked to accounts; the number of sign-ups per inviter in 24 hours.
- What the inviter sees: only aggregate counts (friends joined, friends who completed a reading, minutes pending or
  added). They never see who joined or who had a reading, and notifications never name the friend. Staryield staff can
  see who invited whom in the admin console.
- Browser storage: localStorage `staryield.referral` (the invite code and when it was first seen, kept up to 30 days,
  removed after sign-up) and sessionStorage `staryield.referral.welcome-dismissed`.
- Analytics: that an invite link was opened (never the code), and that a link was copied or shared, with the share
  method only.
- Messages: in-app and push notifications when a friend joins and when minutes are added, plus an account-category
  e-mail when minutes are added (one per grant run). These are service messages, not marketing.
- Data export: `referrals.json` holds the user's code, counters and whether they joined through an invite (never the
  inviter's identity).
- Retention on erasure: erasing the inviter deletes their code and the referrals they made; erasing a friend deletes
  an unfinished referral, and a decided or rewarded one is kept anonymised (no link to the friend, no IP hash) for
  accounting and fraud records.

**Refund Policy**
- Referral minutes are promotional and non-refundable, with no cash value.
- Readings paid only with free minutes cannot be refunded in money.
- A friend's refund or dispute does not change minutes already added. Staryield may refuse or withhold rewards it finds
  fraudulent before they are granted; there is no automatic clawback after a grant.

**Disclaimer**
- The referral program does not change the entertainment-only nature of readings. Invitations shared by users must not
  promise outcomes, and Staryield is not responsible for what users write when they share their link.
