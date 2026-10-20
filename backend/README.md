# Staryield API

Go backend for Staryield: accounts and roles, credits and payments, psychic advisors, **real-time live chat billed per started minute**, custom requests, birth charts and horoscopes, an admin API, and business analytics. It implements the *Formal Specification v1.0* (roles, use cases, the live-chat sequence diagram, the session state machine, FR-01…FR-14 and BR-01…BR-10).

**Stack:** Go 1.26 · Echo v4 · GORM + PostgreSQL (pgx) · Redis (Streams, Pub/Sub, Lua) · Viper · Zap · gorilla/websocket · Cloudflare R2 (S3 API) · Argon2id · JWT.

## Quick start

```bash
cp .env.example .env              # local-only values, incl. seed accounts
make up                           # PostgreSQL + Redis (docker compose)
set -a; source .env; set +a
make seed                         # migrations + advisors, credit packs, accounts
make run                          # http://localhost:8080
```

Point the web app at it with `NEXT_PUBLIC_API_URL=http://localhost:8080` (see `frontend/README.md`).

```bash
make test-unit    # no infrastructure needed
make test-e2e     # full HTTP + WebSocket + SSE suite against PostgreSQL + Redis
make test-race    # everything under the race detector
make lint         # gofmt + go vet
```

## Architecture

```
cmd/api, cmd/migrate, cmd/seed        entry points
internal/app                          composition root: explicit dependency injection
internal/api                          router, middleware ordering, central error handler
internal/api/handler                  controllers: decode → validate → call service → encode
internal/api/middleware               request ID, scoped logging, recover, security headers,
                                      CORS, body limit, timeouts, rate limits, CSRF, auth, RBAC
internal/service                      business rules (the server is the authority)
internal/repository                   data layer (GORM), context-propagated transactions
internal/domain                       entities, enums, state machines, billing arithmetic
internal/realtime                     WebSocket/SSE transport, hub, Redis broker, presence, tickets
internal/worker                       billing ticks, expiries, grace periods, settlement retries
internal/analytics                    async batched business-event tracker
internal/astro                        ephemeris, natal charts, lunar events, horoscope content
internal/security, cache, ratelimit, payment, storage, mail, config, logger, apperror, clock
```

* **Layers.** Handlers never touch the database; services never see HTTP; repositories hold no business rules. Services depend on repository interfaces, so every collaborator is passed in through a constructor (`internal/app/app.go` wires the graph; tests wire it with in-memory doubles).
* **Context everywhere.** Every service and repository method takes a `context.Context`. The request timeout middleware sets a deadline that cascades to PostgreSQL (through `database.Conn`) and Redis; WebSocket frames get their own per-frame deadline. Transactions travel in the context (`TxManager.WithinTx`), and post-commit side effects receive a context detached from the finished transaction.
* **Structured, request-scoped logs.** A Zap logger enriched with `request_id`, `route`, anonymised `ip`, `ip_hash`, `user_id` and `role` is stored in the context and used by every layer, including GORM (SQL is logged without bound parameters). The frontend sends `X-Request-ID`, gets it back on every response and error, and reports its own errors to `/api/v1/client-logs`, so a browser error and its server logs share one ID.
* **Errors.** Services return `apperror.Error` values (stable `code`, safe `message`, optional `details`, wrapped cause). The central handler renders `{"error":{"code","message","details","request_id"}}`, adds `Retry-After` when relevant, logs 5xx with the cause and never leaks internals.

### Data model

Defined in `internal/migrations/sql/0001_init.sql` (the spec's ERD page was not included in the PDF, so the model was derived from the use cases and rules):

`users` (role, status, `token_version`) · `user_profiles` (birth data, preferences, avatar) · `refresh_tokens` (rotating families) · `password_reset_tokens` · `psychics` + `psychic_services` · `wallets` (balance, held, free minutes) · **`credit_ledger` (append-only, enforced by trigger — BR-10)** · `credit_packs` · `credit_purchases` · `conversations` · `chat_sessions` · `chat_messages` · `saved_messages` · `custom_requests` · **`psychic_earnings` (append-only — BR-08)** · `analytics_events` · **`audit_logs` (append-only)**.

Integrity is enforced in the database too: CHECK constraints (no negative balances), partial unique indexes (one open session per client and per psychic), unique idempotency keys for ledger entries, purchases, requests and messages.

### Live chat (spec §5, §6)

```
requested ──accept──▶ accepted ──both connected──▶ active ◀──reconnect within grace── disconnected
    │ decline/timeout/cancel                  │ end / credits exhausted        │ connection lost       │ grace expired
    ▼                                          ▼                                └──────────▶ ended ◀────┘
declined · expired · cancelled            ended ──calculate earnings──▶ settled
```

* **Request (FR-09).** Allowed only if the advisor is online *and connected*, not already busy, and the client can pay one minute (free minutes count). Requests are idempotent (`request_key`) and expire after `chat.request_timeout` (BR-04).
* **Start (FR-10).** A session becomes active only when *both* parties have joined it over the realtime connection; the first started minute is charged at that moment.
* **Billing (FR-11, BR-01–BR-03).** The worker charges each *started* minute at `next_billing_at` (rows claimed with `FOR UPDATE SKIP LOCKED`, so several instances can run it). Promotional free minutes are used first. Each charge is a ledger entry with key `chat:<session>:<minute>`, so a minute can never be charged twice. When a minute cannot be paid, the session ends immediately with `credits_exhausted`. Ending a session first charges any minute that has already started.
* **Disconnects (BR-05).** Losing every connection to the session moves it to `disconnected`, pauses billing and opens a grace period; rejoining resumes billing where the paid coverage left off. A presence sweep catches connections lost without a close (e.g. a crashed instance).
* **Settlement (BR-08).** Ending computes duration and credits; settlement records the psychic's earning (`billing.psychic_share_percent`, rounded down) and the summary is pushed to both sides.

**Rate per minute.** BR-01 states "one credit equals one minute", while the design shows per-advisor prices (40/55/45 credits/min). The engine bills `rate_per_minute` credits per started minute; it is admin-controlled (BR-09) and the seed uses the design's prices. Set every rate to `1` to apply BR-01 literally.

### Realtime protocol

* **WebSocket** `GET /api/v1/realtime/ws?ticket=…&resume=<last event id>` with subprotocol `staryield.v1`. Tickets come from `POST /api/v1/realtime/ticket`. They are single-use and short-lived, and only their hash is stored in Redis, so no long-lived credential ever appears in a URL. `Origin` is checked against the allow-list (prevents cross-site WebSocket hijacking).
* **SSE fallback** `GET /api/v1/realtime/stream` (cookie auth, `Last-Event-ID` resume); intents then go over REST (`/chat/sessions/:id/messages`, `/typing`, `/realtime/presence`).
* **Envelope** `{ "id", "type", "ts", "ref", "data", "error" }`. Durable events carry a Redis Stream ID as `id`; each user's stream is bounded and expires. Every instance subscribes to Pub/Sub and delivers to its local sockets. On reconnect the server replays from `resume`, merges live events without duplicates, or sends `resync_required` if history was trimmed. If the server's own Pub/Sub link drops, it asks its clients to resync.
* **Client frames** `session.join`, `session.leave`, `message.send` (idempotent `client_msg_id`), `typing`, `message.read`, `ping`. Each frame is acked or rejected with an error frame carrying its `ref`.
* **Server events** `hello`, `session.*` (requested, accepted, started, billed, low_balance, disconnected, resumed, ended, settled, declined, expired, cancelled, participant), `message.created`, `message.read`, `typing`, `wallet.updated`, `custom_request.*`, `psychic.status_changed`, `auth.revoked`, `server.shutdown`, `resync_required`.
* **Robustness.** Ping/pong with read deadlines. Bounded send buffers: slow consumers are closed with 4000. Per-connection frame rate limit, closing with 4008 after repeated abuse. Frame size limit (1009). Per-user connection cap (4009). Revocation closes with 4001. Graceful drain on shutdown with 1001 and a reconnect hint.

### Security

* **Passwords** use Argon2id (RFC 9106) with a policy that matches the sign-up form, a max length (DoS) and a common-password check. Unknown accounts get constant-time dummy verification. Login lockout keys on the account; generic errors prevent enumeration.
* **Sessions** use short-lived HS256 access tokens and rotating opaque refresh tokens; only hashes are stored. Reusing a refresh token after rotation revokes the whole family. A `token_version` claim gives exact, immediate revocation on logout-all, password change or reset, role change and deactivation. Realtime connections are closed on revocation as well.
* **Cookies** are HttpOnly; Secure with `__Host-`/`__Secure-` prefixes in production. The access cookie is SameSite=Lax; the refresh cookie is SameSite=Strict and scoped to `/api/v1/auth`.
* **CSRF** uses signed double-submit tokens plus an Origin allow-list on every state-changing request.
* **Input** is decoded as strict JSON (unknown fields rejected — no mass assignment), with body size limits, sanitised text (control and bidi characters stripped), parameterised SQL only, keyset cursors and allow-listed enums.
* **Uploads** are content-sniffed, dimension-checked (decompression bombs) and re-encoded to JPEG. Re-encoding strips EXIF and GPS data. Files go to R2 under random keys.
* **Hardening:** strict security headers (CSP `default-src 'none'`, HSTS, nosniff, frame-deny) and `no-store` caching. Rate limits are a Redis token bucket per IP and per user; auth rules fail closed. Proxy headers are trusted only from configured CIDRs. Production config validation refuses development secrets, wildcard origins, insecure cookies, the sandbox payment provider, the in-memory storage and the log mailer.
* **Privacy and audit:** IPs are stored only as keyed hashes, and analytics never contain message content. Admin actions are audit-logged, transcript access requires a stated reason, and the ledger, earnings and audit tables are immutable.

### Analytics (admin dashboard data)

Business events are recorded server-side and cannot be forged by clients. They cover registrations, logins and lockouts, purchases, every chat transition and billed minute, messages (length only), custom requests and profile changes. Browsers may only send allow-listed `web.*` events (page views, funnels, calculator use, errors), which are de-duplicated by `event_id`. Writes go through a non-blocking buffered tracker that batch-inserts.

Admin endpoints for the future dashboard:

* `GET /api/v1/admin/reports/overview` — users, revenue by currency, paying clients, credits purchased/spent/refunded, sessions by status and end reason, average duration, acceptance rate and response time, messages, requests, earnings vs commission, top psychics
* `GET /api/v1/admin/reports/daily`
* `GET /api/v1/admin/analytics/timeseries`
* `GET /api/v1/admin/analytics/funnel`
* `GET /api/v1/admin/analytics/top`
* `GET /api/v1/admin/analytics/events`

### Payments

`payment.Provider` abstracts the provider (spec: "Buy Credits → Payment Provider"). Card data never reaches this API. A `sandbox` provider approves checkouts for development; amounts ending in `13` cents decline, for tests. Purchases are idempotent, and wallets are credited exactly once through the ledger. Signed webhooks (`t=…,v1=hmac`, with a replay window) handle async confirmation, failure and refund at `POST /api/v1/webhooks/payments`. Production deployments must plug in a real provider adapter; the config refuses `sandbox` there. Until then, `PAYMENTS_PROVIDER=disabled` refuses purchases (and webhooks) with a non-retryable `feature_unavailable` error instead of granting free credits.

## API overview (`/api/v1`)

| Area | Endpoints |
| --- | --- |
| Auth | `GET auth/csrf` · `POST auth/register` · `auth/login` · `auth/refresh` · `auth/logout` · `auth/logout-all` · `auth/password/{forgot,reset,change}` |
| Me | `GET me` · `PATCH me/profile` · `POST/DELETE me/avatar` · `GET me/chart` · `GET me/horoscope` |
| Credits | `GET credit-packs` · `GET wallet` · `GET wallet/ledger` · `POST/GET credits/purchases` · `POST webhooks/payments` |
| Advisors | `GET psychics` · `GET psychics/:slug` · `PATCH psychic/availability` · `PATCH psychic/profile` · `GET psychic/earnings` |
| Chat | `GET chat/conversations` · `GET chat/conversations/:id/messages[/since]` · `POST chat/conversations/:id/read` · `GET chat/psychics/:slug` · `POST/GET chat/sessions` · `GET chat/sessions/:id[/summary]` · `POST chat/sessions/:id/{accept,decline,cancel,end,messages,typing}` · `GET/PUT/DELETE chat/saved[/:messageId]` |
| Realtime | `POST realtime/ticket` · `GET realtime/ws` · `GET realtime/stream` · `POST realtime/presence` |
| Requests | `GET/POST custom-requests` · `GET custom-requests/:id` · `POST custom-requests/:id/{cancel,dispute,accept,decline,deliver}` |
| Astrology | `GET astrology/places` · `POST astrology/chart` · `GET astrology/horoscope/:sign` · `GET astrology/events` |
| Telemetry | `POST analytics/events` · `POST client-logs` |
| Admin | `users`, `users/:id[/credits]`, `psychics[/:id[/earnings,/services]]`, `services/:id`, `credit-packs[/:id]`, `custom-requests[/:id/resolve]`, `chat/sessions[/:id[/transcript,/terminate]]`, `reports/*`, `analytics/*`, `audit-logs` |

Health: `GET /healthz` (liveness), `GET /readyz` (PostgreSQL + Redis).

## Configuration

Defaults live in `internal/config/config.go`; `config.example.yaml` documents the important keys. Any key can be set as `STARYIELD_<SECTION>_<KEY>`. The platform variables `DATABASE_URL`, `REDIS_URL`, `APP_ENV` and `PORT` are used when the `STARYIELD_*` equivalent is unset. On startup the API retries PostgreSQL and Redis with backoff for `DATABASE_CONNECT_TIMEOUT` / `REDIS_CONNECT_TIMEOUT` (default 60s), and logs the host it connects to (never credentials).

Production requires `STARYIELD_APP_ENV=production` and the following:

* `AUTH_JWT_SECRET`, `SECURITY_CSRF_SECRET`, `ANALYTICS_IP_HASH_SECRET`, `PAYMENTS_WEBHOOK_SECRET`
* `SECURITY_ALLOWED_ORIGINS` (https only) and `SECURITY_COOKIE_SECURE=true`
* R2 credentials (`STORAGE_DRIVER=r2`, `STORAGE_R2_*`), or `STORAGE_DRIVER=disabled` to refuse uploads
* a real payment provider (or `PAYMENTS_PROVIDER=disabled`), and `MAIL_DRIVER=noop` until a mailer adapter exists

### Deploying on Railway

The `server` service builds `backend/Dockerfile` (root directory `/backend`, watch pattern `/backend/**`) next to the Postgres and Redis services, in the same region.

* **Detection:** when `RAILWAY_ENVIRONMENT_ID` is present, the defaults become production, secure cookies, HSTS, JSON logs, and client IPs from the edge's `X-Real-IP` header (trusted only from private-range peers). The instance ID becomes `RAILWAY_REPLICA_ID`. A Railway deployment therefore never falls back to development secrets.
* **Variables:**
  * `DATABASE_URL=${{Postgres.DATABASE_URL}}` and `REDIS_URL=${{Redis.REDIS_URL}}`, which point at the private `*.railway.internal` network.
  * The four secrets above, each generated with `openssl rand -hex 48`.
  * `STARYIELD_SECURITY_ALLOWED_ORIGINS=https://staryield.net` and `STARYIELD_APP_FRONTEND_URL=https://staryield.net`.
  * The storage, payment and mail drivers.
* **Service settings:**
  * healthcheck `/readyz`
  * restart policy `ON_FAILURE` (10 retries)
  * draining 30s, so in-flight requests finish and WebSockets get a `1001` close before `SIGKILL`; clients resume from the Redis stream on the new deployment
  * migrations run at startup under an advisory lock, so several replicas are safe
* **Domain:** serve the API from `api.staryield.net`. Add it as a Railway custom domain and point a DNS-only CNAME at it; with Cloudflare's proxy on, `X-Real-IP` would be a Cloudflare address. Then build the frontend with `NEXT_PUBLIC_API_URL=https://api.staryield.net`. The session cookies are `SameSite=Lax/Strict`, so browsers only send them when the frontend and API share a registrable domain. `*.up.railway.app` is on the Public Suffix List, so sign-in cannot work from `staryield.net` against the default Railway domain.

## Tests

* **Unit tests:** state machines and billing arithmetic, Argon2id/JWT/CSRF, Redis and in-memory rate limiters, realtime stream IDs/replay/hub/presence/tickets and a WebSocket server against fakes, middleware and error rendering, config hardening, analytics batching, payments signatures, cache, storage, cursors, astrology (ephemeris accuracy vs JPL Horizons).
* **End-to-end (`tests/e2e`):** the real app on PostgreSQL + Redis with real WebSocket and SSE clients. A fake clock drives billing deterministically. Scenarios:
  * auth, CSRF/Origin, refresh rotation and reuse detection, lockout, password reset, RBAC, profile and avatar
  * the full live chat lifecycle: free and paid minutes, an immutable ledger, read receipts, bookmarks and the summary
  * credit exhaustion, request expiry, decline and cancel
  * disconnect → resume with event replay, and grace expiry
  * WebSocket security and revocation, presence broadcasts, the SSE fallback, concurrent billing vs end
  * purchases, webhooks and refunds, the custom request lifecycle with disputes
  * admin reports and audit, analytics funnels, astrology
