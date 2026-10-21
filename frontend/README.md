# Staryield frontend

Next.js App Router implementation of the Staryield Figma file `epPf5w6QoMtfcFuOIdkzEp`. The Project Overview and production handoff guide define the scope; Design System supplies palette, typography, corner radii, and paper texture. Canonical All Screens frames are implemented; WIP, Archive, and the alternate password screen are excluded.

## Run and check

```bash
npm install
npm run dev
npm run lint
npm run typecheck
npm run test:i18n        # dictionary parity, placeholders, no untranslated JSX text
npm run test:calculators
npm test                 # Vitest unit + component tests (jsdom / Testing Library)
npm run build
npm run build:vinext
npm run test:e2e         # Playwright against the real API (see "End-to-end tests")
```

Open http://localhost:3000/en. The existing `/en`, `/fr`, and `/es` locale routing is preserved. New Figma copy is in English; the existing chat and shared navigation retain dictionary translations.

## Screen coverage

| Area           | Routes (after the locale)                                                                                               |
| -------------- | ----------------------------------------------------------------------------------------------------------------------- |
| Public         | `/`, `/about`, `/reviews`, `/standards`, `/faq`, `/contact`, `/disclaimer`, `/terms`, `/privacy`                        |
| Authentication | `/signup`, `/signup/password`, `/signup/birth`, `/signup/offer`, `/login`, `/forgot-password`                           |
| Calculators    | `/calculators/natal-chart`, `/calculators/path-of-life`, `/calculators/astrology`; each has a `/results` reference view |
| Psychics       | `/psychics`, `/psychics/ramone`, `/psychics/theo`, `/psychics/solomon`                                                  |
| Account        | `/profile`, `/profile/chart`, `/profile/settings`, `/profile/analytics` (advisors only)                                 |
| Credits        | `/credits` opens the credit checkout dialog                                                                             |
| Chat           | `/chatroom?reader=ramone`, `?reader=theo`, `?reader=solomon`; `&review=<session_id>` opens the rating dialog            |
| Short links    | `/go/<code>` (outside the locale tree) → `${NEXT_PUBLIC_API_URL}/l/<code>`                                              |

Desktop and mobile references share reusable headers, navigation, footer, portraits, form controls, checkout, chart components, and advisor data. Public desktop layouts reflow on tablets and phones. Planet cards use keyboard-accessible dialogs; mobile conversation details and inbox use sheets. FAQ items expand and legal contents navigate to semantic section headings.

## Cloudflare Workers deployment

This Next.js 16 app uses vinext and the Cloudflare Vite plugin following the [official Next.js Workers guide](https://developers.cloudflare.com/workers/framework-guides/web-apps/nextjs/). `vite.config.ts` builds the App Router and locale proxy for the Workers runtime; `wrangler.jsonc` identifies the existing `staryield` Worker and its custom domain, `https://staryield.net`.

```bash
npm ci
npm run build:vinext
npm run start:vinext
npm run deploy
```

`npm run dev` and `npm run build` still use Next.js locally. `npm run dev:vinext` runs the Workers development environment on port 3001. Run `npm run cf:typegen` after changing Wrangler bindings. `npm run typecheck` regenerates the Next.js route types before checking TypeScript, because Next.js and vinext share `.next/types` with different formats.

The connected Cloudflare Git build uses root directory `/frontend`, build command `npm run build:vinext`, and deploy command `npx wrangler deploy --config dist/server/wrangler.json`. Commit these changes to the connected production branch to use subsequent automatic builds. Generated output is ignored by Git.

## Images and fonts

All 316 image files, including the favicon, live in the `staryield-app-assets` R2 bucket. Their public URLs begin with `https://assets.staryield.net/`. Components, advisor data, CSS backgrounds, and metadata reference that domain directly; Next Image serves the original public URL with `unoptimized: true`. No R2 credentials or signed URLs are used by the browser. JavaScript, CSS, fonts, and JSON remain part of the Workers build.

`src/data/r2-images.json` records every object key, public URL, MIME type, byte size, and SHA-256 checksum. `public/assets/manifest.json` retains the Figma node IDs and card/symbol names with public R2 URLs. Images have correct MIME types and object metadata `Cache-Control: public, max-age=3600` (the existing zone browser cache policy currently delivers `max-age=14400`); the stable filenames can be replaced without a year-long stale cache. The custom R2 domain provides production delivery; the rate-limited `r2.dev` endpoint stays disabled. `r2-cors.json` permits GET/HEAD from the production site and local development origins.

```bash
# Upload only image files from an external folder, preserving relative object keys.
npm run assets:upload -- /absolute/path/to/images
npm run test:r2
# Apply browser CORS settings after changing permitted origins.
npx wrangler r2 bucket cors set staryield-app-assets --file r2-cors.json --force
```

Keep original images outside the repository. Image binaries are ignored by Git, and `test:r2` also rejects binaries remaining under `public` or `src`. The upload command requires Wrangler authentication and never uploads non-image files. After changing images, update their inventory checksums and verify their public responses before deploying.

Source Serif 4 and Inter are loaded by `next/font`. Castellar is preferred when installed; Cinzel is the web fallback because no licensed Castellar webfont was supplied. To match display typography exactly, add a licensed local Castellar webfont using `next/font/local` in `src/app/[lang]/layout.tsx` and update `--font-display` in `globals.css`.

## Backend integration

The app talks to the Go API in `../backend` (see its README). Set the API origin at build time:

```bash
NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev     # local API
NEXT_PUBLIC_API_URL=https://api.staryield.net npm run build:vinext
```

The API must list the web origin in `STARYIELD_SECURITY_ALLOWED_ORIGINS`. In production, serve the API from a sub-domain of the site (for example `api.staryield.net`) so cookies stay same-site.

* **Transport** (`src/lib/api`): typed endpoints and a fetch client with cookies, CSRF double-submit, single-flight token refresh, per-request timeouts, idempotent retries with backoff and `Retry-After`, and `X-Request-ID` correlation. Every failure becomes an `ApiError` with a stable code, and `errorCopy` maps each code to localized copy.
* **Realtime** (`src/lib/realtime/client.ts`): WebSocket with single-use tickets, heartbeats, jittered reconnects, resume from the last event ID, queued intents with acknowledgements, online/offline and visibility handling, and an automatic SSE + REST fallback. `useRealtimeEvent` subscribes components to server events.
* **Session** (`src/lib/auth/session-provider.tsx`): the signed-in user and live wallet always come from the server; the realtime connection's lifecycle follows the session.
* **Screens**:
  * Sign-up, login and password reset (`/reset-password?token=`) are live.
  * Advisors, statuses and services come from the API, with live presence.
  * Credits use real packs and idempotent purchases. Card details never touch Staryield servers; the sandbox provider is used in development.
  * Profile shows the computed natal chart, the daily horoscope, cosmic events and orders.
  * Settings persist through the API, including avatar upload, birth time and birth-place search.
  * The chatroom implements the full live-chat lifecycle: request, accept or decline, the server-clocked timer, per-minute billing, low-balance warnings, disconnect and resume, the summary, message retry, read receipts, typing and bookmarks.
  * Advisors get availability controls, earnings and order fulfilment on their profile page, and handle chat requests in the chatroom.
* **Platform extensions** (contract: `../docs/platform-extensions.md`, §5.4, §7.1, §8–§10):
  * **Sign out everywhere**: header avatar menu on desktop (Profile, Settings, Analytics for advisors, Sign out), the account sidebar, a Session card in settings (also "sign out of all devices") and the phone menu sheet. Signing out revokes the session, stops realtime, drops cached chat images and lands on the locale home page.
  * **Notifications** (`src/lib/notifications`, `src/components/notifications`): unread count with the session, list on first open with infinite "load more", realtime `notification.created` with a toast, mark one / all as read, resync after `resync_required` or a non-resumed reconnect. Text is rendered client-side per `kind` + `data` in the viewer's language (broadcasts use `title/body[locale]`, falling back to English); times use `Intl.RelativeTimeFormat`; links are locale-prefixed app paths. Failures never block the UI. Popover on desktop, sheet on phones.
  * **Chat composer**: auto-growing textarea (6 lines, then scroll); Enter sends and Shift+Enter adds a line on desktop, Enter adds a line on touch devices (send button sends); Ctrl/Cmd+Enter always sends; typing indicators are throttled; a counter appears near the 1000-character limit; a dependency-free, keyboard-navigable emoji picker with recent emoji inserts at the caret.
  * **Image attachments**: picker button, paste and drag & drop; PNG/JPEG/WebP/GIF up to 8 MB (validated client-side, re-encoded server-side); thumbnail preview with remove; the text becomes the caption; multipart upload with the same optimistic pending/failed/retry flow as text. Images are fetched through the API client (cookies, refresh) into blob URLs cached per attachment and revoked when unused; aspect-ratio placeholders and an accessible lightbox.
  * **Message rendering**: bodies stay text (`white-space: pre-wrap`); only `http(s)://` and `www.` URLs are linked (`target="_blank" rel="noopener noreferrer nofollow ugc"`, trailing punctuation trimmed); short emoji-only messages are larger; saved messages show image thumbnails.
  * **Client panel for advisors**: presence, country (flag + localized name), age, language, member since, sun/moon/rising signs, life path, today's horoscope and history together, in the right panel and the phone details sheet; live online/offline dots in the header and inbox from `counterpart.status` and `presence.updated`. No balance or contact data is ever shown.
  * **Ratings**: 1–5 star radio group + optional comment on the client's chat summary (`can_review`, existing `review`, 409 handled), the `?review=` deep link dialog, and paginated public reviews on advisor profiles.
  * **Advisor analytics** (`/profile/analytics`, Chart.js + react-chartjs-2): KPIs, earnings series (line by day, bars by week/month, 7/30/90 days or 12 months), success score gauge with breakdown and guidance, a 7×24 working-hours heatmap in the viewer's time zone (`tz` from `Intl`), rating distribution, top clients, recent reviews, and payouts (balances with money value, a validated request form with a per-form idempotency key, history with cancel).
  * **Attribution** (`src/lib/attribution.ts`): `utm_*` and `sy_click` from the landing URL are kept as the first touch for 30 days in `localStorage` (`staryield.attribution`), `sy_click` is removed from the address bar, a `campaign_landing` event is recorded and sign-up sends `attribution` (retrying without it if the server rejects only that field).
  * **Localization**: every new string is in `copy-{en,fr,es}.json` (plural-aware counts in `analytics.*` of `{en,fr,es}.json`); REST sends `X-Locale` and the WebSocket connects with `&locale=`.
* **Errors and telemetry**: route and global error boundaries (`error.tsx`, `global-error.tsx`), uncaught error and rejection reporting, toasts, and inline field errors. Warnings and errors are reported to the API (`/client-logs`) with request IDs. Product analytics (`src/lib/analytics.ts`) are batched, honour Do Not Track and Global Privacy Control, and never include message content.
* **Remaining fixtures**: the static home, about, review and legal content, the illustrative chart wheel artwork, and the tarot spread on calculator results.

## End-to-end tests

`e2e/` contains Playwright specs that run against the real stack: the Go API (with the dev seed) and the Next.js dev server, which `playwright.config.ts` starts automatically (or reuses on port 3000).

```bash
# 1. Start the API with the seed (../backend README), e.g. on http://localhost:8080
# 2. Run the suite
E2E_API_URL=http://localhost:8080 npm run test:e2e
npm run test:e2e:ui      # interactive runner
```

| Variable       | Default                 | Purpose                                                       |
| -------------- | ----------------------- | ------------------------------------------------------------- |
| `E2E_API_URL`  | `http://localhost:8080` | API origin (also passed to the dev server as `NEXT_PUBLIC_API_URL`) |
| `E2E_BASE_URL` | `http://localhost:3000` | Site origin                                                    |
| `E2E_CHROMIUM` | –                       | Optional Chromium binary path                                  |

`@playwright/test` is pinned to 1.64.1, whose Chromium build (revision 1248) matches the cached browsers, so no download is needed. The API must allow the site origin (`STARYIELD_SECURITY_ALLOWED_ORIGINS`). Seed accounts: client `client@staryield.test` / `Cl1ent!Secure#2026`; advisors `<slug>@advisors.staryield.net` / `Psych1c!Secure#2026` (`ramone`, `theo`, `solomon`).

Coverage: `/go/<code>` redirects, UTM landing (storage and `sy_click` removal) and sign-up attribution, sign-in and sign-out (desktop menu, settings, phone sheet, French copy), the two-browser chat journey (advisor goes online, client requests, advisor accepts and sees the client panel, multiline message with link and emoji, emoji picker, typing, image upload and display on both sides, image privacy, end chat, rating), the advisor's notification and review, analytics charts and range selector, withdrawal validation, mark-all-read, and phone/tablet layouts without horizontal scrolling. Specs run serially with one worker because they share seed accounts.

## Validation

Production builds (Next.js and vinext), TypeScript, ESLint, i18n, calculator assertions and Vitest unit tests pass. Unit tests cover linkify edge cases, emoji-only detection, attribution capture/expiry/URL stripping, notification copy per kind and locale fallback, composer key handling and attachments, attachment validation and caching, chart data mappers and heatmap levels, withdrawal validation, multipart and blob requests, and the `/go/[code]` route handler. Browser checks cover the canonical routes, public R2 image loading, legal anchors, calculator errors/results, FAQ expansion, and planetary dialogs. Responsive checks include 320px, 390px, 768px, and desktop layouts.
