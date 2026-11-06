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
