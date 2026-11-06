/**
 * End-to-end tests against the real stack (Go API + Next.js dev server).
 *
 *   # API running on http://localhost:8080 with the dev seed (see ../backend)
 *   npm run test:e2e
 *
 * E2E_API_URL      API origin (default http://localhost:8080)
 * E2E_BASE_URL     site origin (default http://localhost:3000); the dev server
 *                  is started automatically unless one is already running
 * E2E_CHROMIUM     optional path to a Chromium binary (otherwise Playwright's
 *                  cached chromium-1248 build for @playwright/test 1.64.1)
 *
 * Projects
 *   desktop  1440×900, every spec (the journey specs open their own phone
 *            contexts where they check mobile behaviour).
 *   mobile   390×844 touch phone (iPhone 13 metrics, Chromium). Runs only
 *            tests tagged @responsive (both projects) or @mobile-only, so the
 *            stateful journeys are not replayed twice against shared seed data.
 */
import { defineConfig, devices } from "@playwright/test";

const API_URL = process.env.E2E_API_URL ?? "http://localhost:8080";
const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:3000";
const PORT = new URL(BASE_URL).port || "3000";

export default defineConfig({
  testDir: "./e2e",
  // The seed accounts and advisor availability are shared state.
  fullyParallel: false,
  workers: 1,
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
    video: "retain-on-failure",
    launchOptions: process.env.E2E_CHROMIUM ? { executablePath: process.env.E2E_CHROMIUM } : {},
  },
  projects: [
    {
      name: "desktop",
      grepInvert: /@mobile-only/,
      use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } },
    },
    {
      name: "mobile",
      grep: /@responsive|@mobile-only/,
      use: {
        browserName: "chromium",
        viewport: { width: 390, height: 844 },
        deviceScaleFactor: 3,
        isMobile: true,
        hasTouch: true,
        userAgent: devices["iPhone 13"].userAgent,
      },
    },
  ],
  webServer: {
    command: "npm run dev",
    url: `${BASE_URL}/en`,
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
    env: { NEXT_PUBLIC_API_URL: API_URL, PORT },
  },
});
