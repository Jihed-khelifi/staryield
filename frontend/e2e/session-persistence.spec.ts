/**
 * Sessions stay signed in (revisions 3.3, docs/platform-extensions.md §22.1)
 * and the calculators do not blame the network for everything (§22.2).
 */
import { expect, test, type Page } from "@playwright/test";

import { CLIENT, accountTrigger, signIn } from "./helpers";

const REFRESH = "**/api/v1/auth/refresh";

/** The access cookie only: the refresh cookie keeps the session alive. */
const clearAccess = (page: Page) => page.context().clearCookies({ name: /sy_at/ });

const authHint = (page: Page) => page.evaluate(() => localStorage.getItem("staryield.auth-hint"));

test.describe("sessions stay signed in", () => {
  test("a second tab refreshing does not sign the first tab out", async ({ browser }) => {
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "en-US" });
    const tabA = await context.newPage();
    await signIn(tabA, CLIENT.email, CLIENT.password); // tab A caches the CSRF token from sign-in
    const tabB = await context.newPage();
    await tabB.goto("/en");
    await expect(accountTrigger(tabB)).toBeVisible();

    // Tab B finds the access token gone and refreshes (a reload forces it).
    await clearAccess(tabB);
    const refreshedB = tabB.waitForResponse((r) => r.url().endsWith("/api/v1/auth/refresh") && r.request().method() === "POST");
    await tabB.reload();
    expect((await refreshedB).status()).toBe(200);
    await expect(accountTrigger(tabB)).toBeVisible();

    // The token expires again; tab A (older CSRF token in memory) makes an
    // authenticated request: it must refresh and stay signed in.
    await clearAccess(tabA);
    const refreshedA = tabA.waitForResponse((r) => r.url().endsWith("/api/v1/auth/refresh") && r.request().method() === "POST");
    await tabA.evaluate(() => document.dispatchEvent(new Event("visibilitychange"))); // unread counters reload when the tab is shown
    expect((await refreshedA).status()).toBe(200);
    await expect(tabA.getByText("Your session has expired")).toHaveCount(0);
    await expect(accountTrigger(tabA)).toBeVisible();
    expect(await authHint(tabA)).toBe("1");

    // Both tabs keep working after another navigation.
    await tabA.goto("/en");
    await expect(accountTrigger(tabA)).toBeVisible();
    await context.close();
  });

  test("a 503 from /auth/refresh does not sign the user out and a later success recovers", async ({ browser }) => {
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "en-US" });
    const page = await context.newPage();
    await signIn(page, CLIENT.email, CLIENT.password);

    let refreshStatus = 503;
    let refreshAttempts = 0;
    await page.route(REFRESH, (route) => {
      refreshAttempts++;
      return refreshStatus === 503
        ? route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "service_unavailable", message: "down" } }) })
        : route.continue();
    });

    await clearAccess(page);
    await page.reload(); // /me answers 401, the refresh answers 503
    await expect.poll(() => refreshAttempts).toBeGreaterThan(0);
    await expect(page.getByText("Your session has expired")).toHaveCount(0);
    expect(await authHint(page)).toBe("1"); // nothing cleared: refresh stays enabled everywhere

    refreshStatus = 200; // the API is back: the session read retries by itself
    await expect(accountTrigger(page)).toBeVisible({ timeout: 30_000 });
    await expect(page.getByText("Your session has expired")).toHaveCount(0);
    expect(await authHint(page)).toBe("1");
    await context.close();
  });

  test("a real expiry (401 from /auth/refresh) still signs out and tells the user", async ({ browser }) => {
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "en-US" });
    const page = await context.newPage();
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.route(REFRESH, (route) =>
      route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ error: { code: "session_expired", message: "gone" } }) }),
    );
    await clearAccess(page);
    await page.evaluate(() => document.dispatchEvent(new Event("visibilitychange")));
    await expect(page.getByText("Your session has expired. Please sign in again.")).toBeVisible();
    expect(await authHint(page)).toBeNull();
    await context.close();
  });
});

async function fillNatalForm(page: Page) {
  const form = page.getByTestId("calculator-form");
  await form.locator('input[name="birth.day"]').fill("14");
  await form.locator('input[name="birth.month"]').fill("03");
  await form.locator('input[name="birth.year"]').fill("1990");
  await page.getByRole("checkbox", { name: "I don't know my birth time" }).click(); // no place needed
  await page.getByTestId("calculator-submit").click();
}

test.describe("calculators and network failures", () => {
  test("the natal calculator works when /auth/csrf is blocked (it is exempt from CSRF)", async ({ page }) => {
    await page.route("**/api/v1/auth/csrf", (route) => route.abort());
    await page.goto("/en/calculators/natal-chart");
    const natal = page.waitForResponse((r) => r.url().endsWith("/api/v1/astrology/natal") && r.request().method() === "POST");
    await fillNatalForm(page);
    expect((await natal).status()).toBe(200);
    await expect(page.getByTestId("calculator-result")).toBeVisible();
    await expect(page.getByText("You appear to be offline")).toHaveCount(0);
  });

  test("an unreachable API while the browser is online says so (not 'offline')", async ({ page }) => {
    await page.goto("/en/calculators/natal-chart");
    expect(await page.evaluate(() => navigator.onLine)).toBe(true);
    await page.route("**/api/v1/astrology/natal", (route) => route.abort());
    await fillNatalForm(page);
    const alert = page.getByTestId("calculator-form").getByRole("alert");
    await expect(alert).toContainText("We couldn't reach Staryield. Please try again in a moment.");
    await expect(page.getByText("You appear to be offline")).toHaveCount(0);
  });
});
