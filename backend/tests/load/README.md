# Load and strain tests

These tests drive a **running** API at a constant arrival rate (open model: requests keep arriving whether or not the server keeps up) and check latency and error-rate objectives. A separate check runs `EXPLAIN (ANALYZE)` on the SQL that the repository methods actually build, against a large dataset.

They are behind the `load` build tag, so `go test ./...` never runs them.

## 1. Prepare a large dataset

Use a dedicated database. Never point this at production.

```sh
export STARYIELD_DATABASE_DSN='postgres://postgres:postgres@127.0.0.1:5432/staryield_load?sslmode=disable'
go run ./cmd/migrate up
go run ./cmd/seed                 # advisors, packs, demo accounts
go run ./cmd/seed -profile=large  # hundreds of thousands of users, millions of messages

# A dataset seeded before a feature existed can gain that feature's data
# without being regenerated (runs only the named generators):
go run ./cmd/seed -profile=large -extend "waiting lists,payment transactions"

# Calculator usage analytics (200k visitors, about 1M daily visit rows, 5% converted):
go run ./cmd/seed -profile=large -extend "calculator visitors"

# revisions 3.0: support desk — about one ticket per two clients (100k with the
# default profile) with sealed addresses, bodies and short threads:
go run ./cmd/seed -profile=large -extend "support tickets"

# revisions 3.1: conversation reports — about one chat report for every ten
# tickets, each linked to a synthetic client's conversation, its latest session
# and the advisor (statuses mixed; at most one open or pending report per
# client and conversation; some conversations also get an older closed one):
go run ./cmd/seed -profile=large -extend "chat reports"
```

Synthetic accounts are `load-<tag>-pNNNNNN@load.staryield.test` (advisors) and `load-<tag>-cNNNNNNN@load.staryield.test` (clients), all with the password in `seed.LoadPassword`.

## 2. Start the API for load

The normal per-IP and per-user rate limits would answer most load traffic with 429, so scale them up. Use in-memory storage so uploads never reach R2.

```sh
STARYIELD_RATE_LIMIT_SCALE=1000 STARYIELD_STORAGE_DRIVER=memory go run ./cmd/api
```

Rate limiting itself is covered by the e2e suite (`make test-e2e`).

## 3. Run

```sh
# All scenarios, 20 s each, base rates
go test -tags load ./tests/load/ -run TestLoad -v

# Only the query-plan checks
go test -tags load ./tests/load/ -run TestExplain -v

# Heavier: double every rate, one minute per scenario, fail on SLO misses
LOAD_SCALE=2 LOAD_DURATION=1m LOAD_STRICT=1 go test -tags load ./tests/load/ -run TestLoad -v -timeout 30m
```

| Variable | Default | Meaning |
|---|---|---|
| `LOAD_API_URL` | `http://localhost:8080` | API under test |
| `LOAD_ORIGIN` | `http://localhost:3000` | an allowed browser origin (CSRF/Origin checks stay on) |
| `LOAD_DURATION` | `20s` | per scenario |
| `LOAD_SCALE` | `1` | multiplies every arrival rate |
| `LOAD_USERS` | `20` | signed-in virtual users created for client scenarios |
| `LOAD_CLIENT_EMAIL` / `_PASSWORD` | seed demo client | fallback account |
| `LOAD_PSYCHIC_EMAIL` / `_PASSWORD` | seed advisor `ramone` | advisor scenarios |
| `LOAD_DATABASE_DSN` | local `staryield` | database for `TestExplain*` |
| `LOAD_REPORT_DIR` | `results` | JSON reports (git-ignored) |
| `LOAD_STRICT` | off | `1` turns SLO misses into test failures |
| `LOAD_CHAT_PAIRS` | `50` | `TestLoadChatFanout`: live sessions (advisor and client pairs of the large seed) |
| `LOAD_CHAT_MESSAGES` | `20` | `TestLoadChatFanout`: messages each side sends, one every 250 ms |

## What is measured

- **Scenarios:**
  - `TestLoadPublicCatalog`: advisor directory (first page, deep pages with filters), the featured home selection, horoscopes, the aztro-compatible endpoint, place search, credit packs, and the natal calculator (full chart, spread and numerology per request).
  - `TestLoadSignedInClient`: profile, settings, devices, waiting lists, wallet and ledger, conversations, notifications, the personal horoscope and chart, card transactions.
  - `TestLoadAdvisorNotes`: the notes panel (own and shared) plus note writes.
  - `TestLoadChatFanout`: real live chats. Synthetic advisors go online over WebSockets, synthetic clients with credits request sessions, both join, and each side sends messages; the objective is on the time from sending a message to its delivery to the other participant (p95 < 250 ms, p99 < 500 ms, no lost messages). It needs `LOAD_DATABASE_DSN` to pick advisors and clients without an open session.
- **How they run:** scenarios in one test run concurrently, like mixed real traffic. Each reports sent, ok, errors and dropped counts, p50/p90/p95/p99/max latency and throughput.
  - Dropped means more than `MaxInFlight` requests were outstanding, which is a sign of saturation.
- **Objectives** (initial):

  | Request type | p95 | p99 |
  |---|---|---|
  | Simple reads | < 100 ms | < 250 ms |
  | Heavier lists | < 150 ms | < 400 ms |
  | Writes | < 200 ms | < 500 ms |
  | Calculators (compute) | < 300 ms | < 600 ms |
  | Chat delivery | < 250 ms | < 500 ms |

  The error rate must stay at or below 0.1 % everywhere.
- **`TestExplainHotQueries`:**
  - Captures each repository query through a GORM dry run and executes `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)` 20 times, with the busiest real IDs.
  - Fails on a sequential scan of any table above 10 000 rows, or when the query's p95 execution time exceeds its budget (25 ms; 50 ms for the catalogue snapshot and the advisor statistics).
  - Only read queries are checked: `EXPLAIN ANALYZE` executes the statement.

## Adding a scenario

Write a `Request` (`func(ctx, *Client, i) (status, error)`) and add a `Scenario` with an SLO to the relevant test. Use `Client.Do` for JSON calls; it handles CSRF the same way the web app does. When a new hot query lands, add it to `TestExplainHotQueries`.

### Calculator analytics (revisions 3.0)

`go test -tags load ./tests/load/ -run 'TestLoadCalculator' -v` needs `LOAD_DATABASE_DSN` (the database of the API under test) and the `calculator visitors` seed above. `TestLoadCalculatorIngestion` posts batches of calculator page views, tries, results and campaign landings at 250 + 200 + 100 requests/s (times `LOAD_SCALE`), checks the ingestion latency and that a marker event reaches `web_visitors` within 15 s, while the 12-month reports are read from the database. `TestLoadCalculatorReportsConcurrent` runs twelve 12-month reports at once. `LOAD_CALC_REPORTS=0` drops the uncached report reads from `TestLoadCalculatorIngestion`, to measure ingestion on its own (on one laptop, 6 uncached 300-day reports per second at `LOAD_SCALE=3` saturate the database CPU; production serves reports from a 60 s Redis cache). `TestExplainHotQueries` includes the report and conversions queries.

## Support desk (revisions 3.0)

* `TestLoadSupportRequesters` (`support_test.go`): signed-in requesters list and open their tickets and reply (`support_reply` is 20/hour per user: keep `STARYIELD_RATE_LIMIT_SCALE=1000`), plus wrong-token lookups of the private link.
* `TestExplainHotQueries` includes the admin queue (newest, oldest, waiting, every filter, reference/e-mail/subject search, deep keyset pages), the summary, the requester's list and the maintenance claim (`support: …`, see `support_explain_test.go`). The summary is one pass over the table by design: it is cached for 15 s.
* revisions 3.1: `TestLoadChatReports` (`support_reports_test.go`): up to `LOAD_REPORT_CLIENTS` (default 300) distinct synthetic clients that own a conversation file a report (`POST /me/support/chat-reports`; `support_report` is 10/hour per user: keep `STARYIELD_RATE_LIMIT_SCALE=1000`; a repeated report answers 409 `report_exists` and counts as served), poll `GET /me/support/chat-reports?conversation_id=` and list their tickets. `TestExplainHotQueries` adds the `kind=chat_report` queue for every sort and deep page, the open-report lookup and the latest-session lookup (`support: reports …`, `chat: latest session …`); the admin queue itself needs a TOTP console session, so its SQL is checked there.
* The e2e suite has a self-contained 100k-ticket check (`TestSupportDeskLargeDatasetFiltersSortsAndKeysetPagination`); run it with `SUPPORT_LARGE_KEEP=1` to leave the rows in the e2e database for the EXPLAIN checks.
