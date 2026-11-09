/**
 * Revisions 3.0 calculator analytics, client side (§17): visits are
 * page_view events with the browser's random anonymous_id in every locale;
 * calculator events carry `kind`; the id is sent at sign-up only when
 * analytics is allowed; visitors who opt out send nothing.
 */
import { expect, test, type Page } from "@playwright/test";

type Ev = { name: string; path: string; anonymous_id: string; properties?: Record<string, unknown> };

/** Collects every analytics event the page sends (the tracker batches for up to 5 s). */
function collect(page: Page) {
  const events: Ev[] = [];
  page.on("request", (r) => {
    if (r.url().endsWith("/api/v1/analytics/events") && r.method() === "POST") events.push(...(r.postDataJSON().events as Ev[]));
  });
  return events;
}

for (const locale of ["en", "fr", "es"]) {
  test(`${locale}: a calculator visit sends page_view with the anonymous id`, async ({ page }) => {
    const events = collect(page);
    await page.goto(`/${locale}/calculators/natal-chart`);
    await expect.poll(() => events.find((e) => e.name === "page_view" && e.path === `/${locale}/calculators/natal-chart`), { timeout: 20_000 }).toBeTruthy();
    const aid = await page.evaluate(() => localStorage.getItem("staryield.aid"));
    expect(aid).toMatch(/^[A-Za-z0-9_-]{8,64}$/);
    expect(events.every((e) => e.anonymous_id === aid)).toBe(true);
  });
}

test("each calculator is a visit; campaign_landing carries the click id", async ({ page }) => {
  const events = collect(page);
  await page.goto("/en/calculators/path-of-life?utm_source=ig&utm_term=tarot&sy_click=e2eClick1");
  await expect.poll(() => events.find((e) => e.name === "campaign_landing"), { timeout: 20_000 }).toMatchObject({ properties: { has_click: true, utm_source: "ig", utm_term: "tarot", click_id: "e2eClick1" } });
  await page.goto("/en/calculators/astrology");
  await expect.poll(() => events.some((e) => e.name === "page_view" && e.path === "/en/calculators/astrology"), { timeout: 20_000 }).toBe(true);
});

test("calculator_started and calculator_completed carry the kind", async ({ page }) => {
  const events = collect(page);
  await page.goto("/en/calculators/path-of-life");
  const form = page.getByTestId("calculator-form");
  await form.locator('input[name="birth.day"]').fill("14");
  await form.locator('input[name="birth.month"]').fill("3");
  await form.locator('input[name="birth.year"]').fill("1990");
  await form.locator("button[type=submit]").click();
  await expect.poll(() => events.filter((e) => e.name.startsWith("calculator_")).map((e) => `${e.name}:${e.properties?.kind}`), { timeout: 20_000 }).toEqual(
    expect.arrayContaining(["calculator_started:path-of-life", "calculator_completed:path-of-life"]),
  );
});

test("visitors who send Do Not Track or Global Privacy Control send nothing and get no id", async ({ browser }) => {
  for (const signal of ["dnt", "gpc"] as const) {
    const context = await browser.newContext();
    const page = await context.newPage();
    await page.addInitScript((s) => {
      Object.defineProperty(navigator, s === "dnt" ? "doNotTrack" : "globalPrivacyControl", { get: () => (s === "dnt" ? "1" : true) });
    }, signal);
    const events = collect(page);
    await page.goto("/en/calculators/natal-chart");
    await page.waitForTimeout(6_500);
    expect(events).toEqual([]);
    expect(await page.evaluate(() => localStorage.getItem("staryield.aid"))).toBeNull();

    // Sign-up must not carry the id either.
    await page.goto("/en/signup");
    await page.locator("input[name=email]").fill(`e2e+optout-${signal}-${Date.now()}@staryield.test`);
    await page.getByRole("checkbox").check();
    await page.locator("form button[type=submit]").click();
    await page.waitForURL(/\/en\/signup\/password$/);
    await page.locator("input[name=password]").fill("E2e!Secure#2026x");
    await page.locator("input[name=confirm]").fill("E2e!Secure#2026x");
    const register = page.waitForRequest((r) => r.url().endsWith("/api/v1/auth/register") && r.method() === "POST");
    await page.locator("form button[type=submit]").click();
    const body = (await register).postDataJSON();
    expect(body.attribution?.anonymous_id).toBeUndefined();
    await context.close();
  }
});

test("sign-up without any UTM touch still sends the anonymous id when analytics is allowed", async ({ page }) => {
  await page.goto("/en/calculators/natal-chart");
  // The id is created on hydration, which can land after "load" on a dev server.
  const readAid = () => page.evaluate(() => localStorage.getItem("staryield.aid"));
  await expect.poll(readAid, { timeout: 20_000 }).toBeTruthy();
  const aid = await readAid();
  await page.goto("/en/signup");
  await page.locator("input[name=email]").fill(`e2e+aid-${Date.now()}@staryield.test`);
  await page.getByRole("checkbox").check();
  await page.locator("form button[type=submit]").click();
  await page.waitForURL(/\/en\/signup\/password$/);
  await page.locator("input[name=password]").fill("E2e!Secure#2026x");
  await page.locator("input[name=confirm]").fill("E2e!Secure#2026x");
  const register = page.waitForRequest((r) => r.url().endsWith("/api/v1/auth/register") && r.method() === "POST");
  await page.locator("form button[type=submit]").click();
  expect((await register).postDataJSON().attribution).toEqual({ anonymous_id: aid });
});
