/**
 * End-to-end tests against the real stack (Go API + Next.js).
 *
 *   # API running on http://localhost:8080 with the dev seed (see ../backend)
 *   npm run test:e2e          # one spec at a time against `next dev` (reused when running)
 *   npm run test:e2e:full     # the whole suite, fast: see scripts/e2e-run.mjs
 *
 * E2E_API_URL      API origin (default http://localhost:8080)
 * E2E_BASE_URL     site origin (default http://localhost:3000); the server
 *                  is started automatically unless one is already running
 * E2E_SERVER       "dev" (default): next dev. "prod": next build once, then
 *                  next start — no on-demand compilation, a fraction of the CPU
 *                  per page. The build inlines E2E_API_URL.
 * E2E_WORKERS      workers (default 1). Only the public-* projects use more
 *                  than one: run them on their own (test:e2e:full does).
 * E2E_VIDEO=1      also keep a video of failing tests (off by default: every
 *                  test's video is encoded, which costs a lot of CPU; traces and
 *                  screenshots of failures are always kept).
 * E2E_CHROMIUM     optional path to a Chromium binary (otherwise Playwright's
 *                  cached chromium-1248 build for @playwright/test 1.64.1)
 *
 * Projects
 *   desktop / mobile         the specs that share state: the seed accounts,
 *                            advisor availability, credits, site settings. One
 *                            worker each; never run them with another project.
 *   public-desktop / -mobile the specs of public pages (PUBLIC_SPECS): no
 *                            sign-in, no admin call, no shared state, so their
 *                            files run in parallel (tests within a file stay in
 *                            order).
 *   *-desktop is 1440×900 and runs every test of its specs; *-mobile is a
 *   390×844 touch phone (iPhone 13 metrics, Chromium) that runs only tests
 *   tagged @responsive (both) or @mobile-only, so the stateful journeys are not
 *   replayed twice against shared seed data.
 */
import { defineConfig, devices, type Project } from "@playwright/test";

const API_URL = process.env.E2E_API_URL ?? "http://localhost:8080";
const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:3000";
const PORT = new URL(BASE_URL).port || "3000";
const PROD = process.env.E2E_SERVER === "prod";
const WORKERS = Math.max(1, Number(process.env.E2E_WORKERS) || 1);

/** Specs that only visit public pages and register nothing shared (see Projects). */
export const PUBLIC_SPECS =
  /[\\/](calculator-analytics|calculator-explorer|calculator-report|calculator-share|calculators|faq|legal|merchant-readiness|redirects|refund-policy|responsive-landing|seo|tarot-birth-card|trustpilot)\.spec\.ts$/;

const desktop: Project["use"] = { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } };
const mobile: Project["use"] = {
  browserName: "chromium",
  viewport: { width: 390, height: 844 },
  deviceScaleFactor: 3,
  isMobile: true,
  hasTouch: true,
  userAgent: devices["iPhone 13"].userAgent,
};

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: WORKERS,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  timeout: 90_000,
  expect: { timeout: 15_000 },
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : [["list"]],
  use: {
    baseURL: BASE_URL,
    locale: "en-US",
    timezoneId: "Europe/Paris",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: process.env.E2E_VIDEO === "1" ? "retain-on-failure" : "off",
    launchOptions: process.env.E2E_CHROMIUM ? { executablePath: process.env.E2E_CHROMIUM } : {},
  },
  projects: [
    // The seed accounts, advisor availability and site settings are shared state.
    { name: "desktop", testIgnore: PUBLIC_SPECS, grepInvert: /@mobile-only/, workers: 1, use: desktop },
    { name: "mobile", testIgnore: PUBLIC_SPECS, grep: /@responsive|@mobile-only/, workers: 1, use: mobile },
    { name: "public-desktop", testMatch: PUBLIC_SPECS, grepInvert: /@mobile-only/, use: desktop },
    { name: "public-mobile", testMatch: PUBLIC_SPECS, grep: /@responsive|@mobile-only/, use: mobile },
  ],
  webServer: {
    command: PROD ? `npm run build && npx next start --port ${PORT}` : "npm run dev",
    url: `${BASE_URL}/en`,
    reuseExistingServer: !process.env.CI,
    timeout: PROD ? 600_000 : 180_000,
    env: { NEXT_PUBLIC_API_URL: API_URL, PORT, NEXT_TELEMETRY_DISABLED: "1" },
  },
});
