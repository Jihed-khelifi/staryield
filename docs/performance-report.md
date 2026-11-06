# Staryield revisions 2.0: load and strain test report

Date: 6 November 2026. Branch `revisions-2.0` (uncommitted working tree).
Harness: `backend/tests/load`, build tag `load` (see its README).

## Setup

- **One machine for everything:** a 10-core Apple-silicon laptop ran the API, PostgreSQL 17, Redis and the load generator together. The figures below are therefore conservative for latency and say nothing about horizontal scale.
- **API configuration:**
  - production defaults: Argon2id at 64 MiB, t=3, p=2, and every migration up to 0015;
  - `STARYIELD_RATE_LIMIT_SCALE=1000`, so rate limits do not answer the load traffic;
  - in-memory object storage.
- **Dataset** (`staryield_load2`, `cmd/seed -profile=large` plus `-extend`):

| Table | Rows |
|---|---|
| users | 202,048 |
| advisors | 2,003 |
| conversations | 100,286 |
| chat sessions | 164,983 |
| chat messages | 1,780,619 |
| credit ledger | 1,000,085 |
| payment transactions | 389,179 |
| presence intervals | 240,560 |
| client notes | 161,118 |
| reviews | 118,369 |
| waiting-list entries | 48,169 |

Not generated at scale: subscriptions (0) and notifications (610). Their hot queries are covered by the query-plan checks below, not by large data.

## Results

All runs are in strict mode: an SLO miss fails the test. Base-rate runs last 20 s per scenario; strain runs use **4× the arrival rates for 60 s**. Scenarios within a test run concurrently, like mixed real traffic.

### Public pages: strain (4×, 60 s), 0 errors in 225,608 requests

| Request | Rate | p95 | p99 | Target p95 |
|---|---|---|---|---|
| GET /psychics | 600/s | 0.5 ms | 0.7 ms | 100 ms |
| GET /psychics (paged, filtered) | 400/s | 0.6 ms | 0.9 ms | 100 ms |
| GET /psychics/featured | 800/s | 0.6 ms | 0.8 ms | 100 ms |
| GET /astrology/horoscope | 600/s | 0.7 ms | 0.9 ms | 100 ms |
| GET /astrology/places | 400/s | 0.6 ms | 0.8 ms | 100 ms |
| GET /aztro | 400/s | 0.6 ms | 0.8 ms | 100 ms |
| POST /astrology/natal (full chart compute) | 160/s | 1.1 ms | 1.4 ms | 300 ms |

### Signed-in client: strain (4×, 60 s), 0 errors in 151,211 requests

The slowest endpoint was `/me/chart` (natal chart from stored birth data), at p95 2.8 ms and p99 3.2 ms. Every other endpoint had p95 at or under 1.5 ms. The endpoints covered:
- `/me`, `/me/settings`, `/me/sessions`, `/me/notifications`, `/me/horoscope`, `/me/waitlist`;
- `/wallet`, `/wallet/ledger`;
- `/chat/conversations`;
- `/billing/transactions`.

### Advisor notes: strain (4×, 60 s), 0 errors

| Request | Rate | p95 | p99 |
|---|---|---|---|
| GET own notes | 320/s | 1.5 ms | 1.9 ms |
| GET shared notes | 240/s | 1.9 ms | 2.2 ms |
| POST + DELETE note | 20/s | 5.4 ms | 5.8 ms |

### Live chat fan-out (real WebSocket sessions)

Synthetic advisors go online over WebSockets and synthetic clients request and join sessions. Each side then sends 20 messages, one every 250 ms. The measure is the time from send to delivery to the other participant.

| Live sessions | Messages | Lost | p50 | p95 | p99 | Target |
|---|---|---|---|---|---|---|
| 50 | 2,000 | 0 | 3.5 ms | 64 ms | 87 ms | p95 < 250 ms ✓ |
| 250 (500 sockets, about 1,200 msg/s) | 10,000 | 0 | 146 ms | **263 ms** | 311 ms | p95 < 250 ms ✗ (5% over) |

At 250 sessions the median rises to 146 ms. Two things contribute: the ordered-publish gate (one Redis Lua call per message) and a load generator that shares the 10 cores with the API. Next step: profile with the generator on another machine before changing the delivery path. p99 stays within its 500 ms target and no message was lost.

### Query plans (`TestExplainHotQueries`, 25 checks)

The check fails on any sequential scan of a table above 10,000 rows, or when p95 execution exceeds its budget (25 ms; 50 ms for the catalogue snapshot and advisor statistics). All 25 checks pass. The slowest is the catalogue snapshot at p95 4.3 ms; the others run in microseconds.

Coverage:
- notes;
- ledger;
- reviews;
- advisor statistics;
- inbox;
- message history and catch-up;
- the billing claim;
- waiting lists;
- devices;
- transactions;
- saved messages;
- the photo-review queue.

## Problems found and fixed

1. **Password hashing could take the API down (fixed).**
   - At 250 chat pairs, 500 users signed in at once. Argon2id needs 64 MiB per hash, and nothing limited how many ran in parallel: 500 hashes asked for about 32 GB.
   - The host swapped, logins timed out, and the API stayed degraded (about 2 s for any request; `/readyz` failing) for 15+ minutes after the burst ended. A credential-stuffing burst would do the same.
   - **Fix:** `security.PasswordHasher` now admits at most about one hash per CPU.
     - A request waits for a slot only while its own deadline allows, and at most `STARYIELD_AUTH_PASSWORD_HASH_WAIT` (3 s).
     - After that the caller gets a retryable `503 service_unavailable` with `Retry-After`. Login treats it as "busy", never as a failed attempt, and answers known and unknown accounts the same way.
     - Tests: unit tests for the gate (under the race detector) and the e2e test `TestLoginWhileHashingIsSaturated`.
   - **After the fix:** the same 250-pair run signs everyone in (the harness retries 503s as the web app does), and the API answers `/readyz` in under 1 ms right after.
2. **The advisor directory collapsed under load (fixed earlier in this release).**
   - `GET /psychics` returned about 90% errors at 150 requests/s with 2,000 advisors. Causes: no pagination, a presence lookup per request, and a scan of open sessions.
   - **Fix:** an in-memory catalogue snapshot (10 s TTL, single-flight rebuild, patched in place on status changes), pagination, and batched status.
   - Now 600 requests/s at p95 0.5 ms.
3. **The catalogue snapshot query scanned the users table (fixed).**
   - The query took 45 ms.
   - **Fix:** migration `0015_perf_indexes` adds the partial index `users_active_psychics_idx`, plus a role predicate in the query. Now 4.3 ms.
4. **Deduplicated e-mails failed after a few sends (fixed; found by the browser tests).**
   - The outbox upsert's conflict target used a bind parameter for the partial-index predicate. Postgres cannot match the index once the cached statement switches to a generic plan, which happens after about 5 executions, and every deduplicated e-mail then failed with `42P10`.
   - Affected: e-mail-change confirmations, support acknowledgements, and similar mail.
   - **Fix:** a literal predicate.
   - Regression test: `TestEmailOutboxDedupeSurvivesGenericPlans`, which forces generic plans.
5. **Harness fixes.**
   - A run whose sessions all failed during setup reported success. It is now a failure.
   - Sign-ins now retry retryable answers.
   - The notes write scenario deletes what it creates, so scaled runs never reach the per-client note cap.

## Operational notes

- **Migration 0007** (per-conversation message ordering) took about 7.7 minutes on 1.78 M messages and holds an exclusive lock while it runs. Run it in a maintenance window, or backfill in batches if production has grown by then. The other new migrations each applied in under a second on this dataset.
- **Login capacity per instance** is about one Argon2id hash per core at a time (each needs 64 MiB and three passes at the defaults). Size instances, or lower `STARYIELD_AUTH_ARGON2_CONCURRENCY`, against expected sign-in peaks; a burst beyond that gets fast, retryable 503s instead of exhausting memory.
- **Reproduce with:**
  ```
  LOAD_SCALE=4 LOAD_DURATION=1m LOAD_STRICT=1 LOAD_CHAT_PAIRS=250 go test -tags load ./tests/load/ -run TestLoad -v
  ```
  Use `-run TestExplain` for the query-plan checks. JSON reports go to `tests/load/results/` (git-ignored).
