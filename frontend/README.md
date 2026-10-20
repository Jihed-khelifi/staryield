# Staryield frontend

Next.js App Router implementation of the Staryield Figma file `epPf5w6QoMtfcFuOIdkzEp`. The Project Overview and production handoff guide define the scope; Design System supplies palette, typography, corner radii, and paper texture. Canonical All Screens frames are implemented; WIP, Archive, and the alternate password screen are excluded.

## Run and check

```bash
npm install
npm run dev
npm run lint
npm run typecheck
npm run test:calculators
npm test                 # Vitest unit tests (API client, realtime client, chat model, i18n, redirects)
npm run build
```

Open http://localhost:3000/en. The existing `/en`, `/fr`, and `/es` locale routing is preserved. New Figma copy is in English; the existing chat and shared navigation retain dictionary translations.

## Screen coverage

| Area           | Routes (after the locale)                                                                                               |
| -------------- | ----------------------------------------------------------------------------------------------------------------------- |
| Public         | `/`, `/about`, `/reviews`, `/standards`, `/faq`, `/contact`, `/disclaimer`, `/terms`, `/privacy`                        |
| Authentication | `/signup`, `/signup/password`, `/signup/birth`, `/signup/offer`, `/login`, `/forgot-password`                           |
| Calculators    | `/calculators/natal-chart`, `/calculators/path-of-life`, `/calculators/astrology`; each has a `/results` reference view |
| Psychics       | `/psychics`, `/psychics/ramone`, `/psychics/theo`, `/psychics/solomon`                                                  |
| Account        | `/profile`, `/profile/chart`, `/profile/settings`                                                                       |
| Credits        | `/credits` opens the credit checkout dialog                                                                             |
| Chat           | `/chatroom?reader=ramone`, `?reader=theo`, `?reader=solomon`                                                            |

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
* **Errors and telemetry**: route and global error boundaries (`error.tsx`, `global-error.tsx`), uncaught error and rejection reporting, toasts, and inline field errors. Warnings and errors are reported to the API (`/client-logs`) with request IDs. Product analytics (`src/lib/analytics.ts`) are batched, honour Do Not Track and Global Privacy Control, and never include message content.
* **Remaining fixtures**: the static home, about, review and legal content, the illustrative chart wheel artwork, and the tarot spread on calculator results.

## Validation

Production builds (Next.js and vinext), TypeScript, ESLint, i18n, calculator assertions and Vitest unit tests pass. Browser checks cover the canonical routes, public R2 image loading, legal anchors, calculator errors/results, FAQ expansion, and planetary dialogs. Responsive checks include 320px, 390px, 768px, and desktop layouts.
