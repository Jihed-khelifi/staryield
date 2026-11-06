/**
 * Calculators and the client's overview / chart tab (revisions 2.0, FE-CALC),
 * against the real API: fill in, result, refresh restores the result from the
 * URL fragment, server 422 on fields, accordions, axe. Runs on desktop and on
 * the mobile project (@responsive).
 */
import { mkdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { API_URL, expectNoHorizontalScroll } from "./helpers";

const STATE = join(tmpdir(), "staryield-e2e", `calc-user-${process.pid}.json`);
mkdirSync(join(tmpdir(), "staryield-e2e"), { recursive: true });

async function settle(page: Page) {
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => undefined))
    )
  );
}

async function fillDate(page: Page, day: string, month: string, year: string, time?: string) {
  const form = page.getByTestId("calculator-form");
  await form.locator('input[name="birth.day"]').fill(day);
  await form.locator('input[name="birth.month"]').fill(month);
  await form.locator('input[name="birth.year"]').fill(year);
  if (time) await form.locator('input[name="birth.time"]').fill(time);
}

async function pickParis(page: Page) {
  await page.getByRole("combobox", { name: /Birthplace/ }).fill("Paris");
  await page.getByRole("option", { name: /^Paris, (.+, )?France$/ }).first().click();
}

test.describe("natal chart calculator", { tag: "@responsive" }, () => {
  test("form validation, spread, refresh restores it from the fragment, edit date", async ({ page }) => {
    const requests: string[] = [];
    page.on("request", (r) => requests.push(`${r.method()} ${r.url()}`));
    await page.goto("/en/calculators/natal-chart");
    const form = page.getByTestId("calculator-form");
    await expect(form.getByText("Your birth data is used only to draw this spread and is never stored.")).toBeVisible();
    await expect(form.getByText("calculated locally")).toHaveCount(0);

    // Empty submit: visible errors, no request.
    await page.getByTestId("calculator-submit").click();
    await expect(form.getByText("Enter your date of birth.")).toBeVisible();
    await expect(form.locator('input[name="birth.day"]')).toHaveAttribute("aria-invalid", "true");

    // A time without a place is explained next to the place field.
    await fillDate(page, "14", "03", "1990", "08:45");
    await page.getByTestId("calculator-submit").click();
    await expect(form.getByText("Choose your birth place to use a birth time.")).toBeVisible();
    await expectNoA11yViolations(page, ['[data-testid="calculator-form"]']);

    // Advanced settings: the house system.
    await page.getByTestId("advanced-settings").click();
    await expect(page.getByRole("radio", { name: /Placidus/ })).toBeChecked();
    await page.getByTestId("house-whole_sign").click();

    await pickParis(page);
    const natal = page.waitForResponse((r) => r.url().endsWith("/api/v1/astrology/natal") && r.request().method() === "POST");
    await page.getByTestId("calculator-submit").click();
    const response = await natal;
    expect(response.status()).toBe(200);
    expect(response.request().postDataJSON()).toMatchObject({ birth_date: "1990-03-14", birth_time: "08:45", place_id: "gn:2988507", house_system: "whole_sign" });

    // Result: the twelve cards with reasons; the fragment holds the state.
    const result = page.getByTestId("calculator-result");
    await expect(result.getByTestId("tarot-spread")).toBeVisible();
    await expect(result.getByTestId("spread-cards").locator("li")).toHaveCount(12);
    await expect(result.getByText("Your birth data is used only to draw this spread and is never stored.")).toBeVisible();
    await expect(result.getByText("Place data © GeoNames")).toBeVisible();
    expect(page.url()).toContain("#v=1&d=1990-03-14&t=08%3A45&p=gn%3A2988507&h=whole_sign");
    expect(new URL(page.url()).search).toBe("");

    // FAQ-less natal article is rendered under the result.
    await expect(page.locator("[data-calculator-article=natal]")).toBeVisible();
    await settle(page);
    await expectNoA11yViolations(page, ['[data-testid="calculator-result"]']);
    await expectNoHorizontalScroll(page);

    // Refresh: the same result, restored from the fragment.
    await page.reload();
    await expect(page.getByTestId("calculator-result").getByTestId("spread-cards").locator("li")).toHaveCount(12);
    await expect(page.getByTestId("calculator-result").getByText("08:45")).toBeVisible();

    // Change the date: back to the form (fragment removed).
    await page.getByTestId("change-birth-date").click();
    await expect(page.getByTestId("calculator-form")).toBeVisible();
    expect(page.url()).not.toContain("#v=1");
    // Birth data never travelled in a URL to the server.
    expect(requests.filter((r) => r.includes("1990-03-14") && !r.startsWith("POST "))).toEqual([]);
  });

  test("unknown birth time: noon, no place needed", async ({ page }) => {
    await page.goto("/en/calculators/natal-chart");
    await fillDate(page, "05", "11", "1985");
    const unknown = page.getByRole("checkbox", { name: "I don't know my birth time" });
    await unknown.click();
    await expect(unknown).toHaveAttribute("aria-checked", "true");
    await expect(page.getByText(/we use noon and whole-sign houses/)).toBeVisible();
    await page.getByTestId("calculator-submit").click();
    await expect(page.getByTestId("tarot-spread")).toBeVisible();
    expect(page.url()).toContain("#v=1&d=1985-11-05");
    await expect(page.getByText("Birth time unknown", { exact: true })).toBeVisible();
  });

  test("a 422 from the server is shown on the birthplace field", async ({ page }) => {
    await page.route("**/api/v1/astrology/natal", (route) =>
      route.fulfill({
        status: 422,
        contentType: "application/json",
        body: JSON.stringify({ error: { code: "validation_failed", message: "Some fields need attention.", details: { fields: { place_id: "Choose your birth place to use a birth time." } } } }),
      })
    );
    await page.goto("/en/calculators/natal-chart");
    await fillDate(page, "14", "03", "1990", "08:45");
    await pickParis(page);
    await page.getByTestId("calculator-submit").click();
    const form = page.getByTestId("calculator-form");
    await expect(form.getByText("Choose your birth place to use a birth time.")).toBeVisible();
    await expect(form.getByRole("combobox", { name: /Birthplace/ })).toHaveAttribute("aria-invalid", "true");
  });

  test("a shared link the API rejects reopens the form with the server's message", async ({ page }) => {
    await page.goto("/en/calculators/natal-chart#v=1&d=1990-03-14&t=08:45&p=gn:999999999");
    await expect(page.getByTestId("calculator-form")).toBeVisible();
    await expect(page.getByText("Choose a birth place from the list.").first()).toBeVisible();
  });

  test("tampered fragments are ignored with a clear message", async ({ page }) => {
    for (const hash of ["#v=1&d=2999-01-01", "#v=9&d=1990-03-14", "#v=1&d=1990-03-14&evil=<script>"]) {
      await page.goto(`/en/calculators/natal-chart${hash}`);
      await expect(page.getByTestId("calculator-form")).toBeVisible();
      await expect(page.getByText("This link's birth details are incomplete or invalid.")).toBeVisible();
    }
  });

  test("birthplace combobox is keyboard operable and paginated", async ({ page }) => {
    await page.goto("/en/calculators/natal-chart");
    const input = page.getByRole("combobox", { name: /Birthplace/ });
    await input.fill("San");
    await expect(page.getByRole("option").first()).toBeVisible();
    await expect(page.getByText("Place data © GeoNames")).toBeVisible();
    const more = page.getByRole("option", { name: "Show more results" });
    await expect(more).toBeVisible();
    const before = await page.getByRole("option").count();
    await input.press("End");
    await input.press("Enter");
    await expect.poll(() => page.getByRole("option").count()).toBeGreaterThan(before);
    await input.press("Home");
    await input.press("Enter");
    await expect(input).not.toHaveValue("San");
    await expect(page.getByRole("listbox")).toHaveCount(0);
  });
});

test.describe("astrology calculator", { tag: "@responsive" }, () => {
  test("sun sign on the counter-clockwise wheel; cusp day offers both signs", async ({ page }) => {
    await page.goto("/en/calculators/astrology");
    await expect(page.getByTestId("calculator-form").getByText("Your data is processed securely & never stored.")).toBeVisible();
    await fillDate(page, "02", "05", "1990");
    const sun = page.waitForResponse((r) => r.url().endsWith("/api/v1/astrology/sun-sign"));
    await page.getByTestId("calculator-submit").click();
    expect((await sun).status()).toBe(200);
    await expect(page.getByTestId("sun-sign")).toHaveText("Taurus");
    await expect(page.getByTestId("zodiac-wheel")).toHaveAttribute("data-highlight", "Taurus");
    await expect(page.getByTestId("sun-sign-uncertain")).toHaveCount(0);
    expect(page.url()).toContain("#v=1&d=1990-05-02");
    await settle(page);
    await expectNoA11yViolations(page, ['[data-testid="calculator-result"]']);
    await page.reload();
    await expect(page.getByTestId("sun-sign")).toHaveText("Taurus");

    // 21 June 1990 is a cusp day.
    await page.goto("/en/calculators/astrology#v=1&d=1990-06-21");
    await expect(page.getByTestId("sun-sign-uncertain")).toBeVisible();
    await expect(page.locator("[data-testid=zodiac-wheel] [data-alternative]")).toHaveCount(1);
    await page.getByRole("button", { name: "Add birth time and place" }).click();
    await expect(page.getByRole("combobox", { name: /Birthplace/ })).toBeVisible();
  });

  test("the article's FAQs are accordions", async ({ page }) => {
    await page.goto("/en/calculators/astrology#v=1&d=1990-05-02");
    const faq = page.getByTestId("article-faq");
    const first = faq.getByRole("button", { name: "What if I do not know my exact birth time?" });
    await first.scrollIntoViewIfNeeded();
    await expect(first).toHaveAttribute("aria-expanded", "false");
    await first.click();
    await expect(first).toHaveAttribute("aria-expanded", "true");
    await expect(faq.getByText(/set your chart to 12:00 PM/)).toBeVisible();
    await first.click();
    await expect(first).toHaveAttribute("aria-expanded", "false");
    await expect(page.locator("math")).toHaveCount(1); // Ascendant formula as MathML
  });
});

test.describe("path of life calculator", { tag: "@responsive" }, () => {
  test("runs in the browser: no API call, shared vectors, refresh restores it", async ({ page }) => {
    const calls: string[] = [];
    page.on("request", (r) => {
      if (r.url().startsWith(API_URL) && /astrology/.test(r.url())) calls.push(r.url());
    });
    await page.goto("/en/calculators/path-of-life");
    await expect(page.getByTestId("calculator-form").getByText("Your data is calculated locally & never stored.")).toBeVisible();
    await fillDate(page, "29", "11", "1990");
    await page.getByTestId("calculator-submit").click();
    await expect(page.getByTestId("number-lifePath")).toHaveText("5");
    await expect(page.getByTestId("life-path-title")).toHaveText("Life path 5");
    await expect(page.getByTestId("life-path-steps")).toContainText("Total: 23 → 5");
    expect(page.url()).toContain("#v=1&d=1990-11-29");
    await settle(page);
    await expectNoA11yViolations(page, ['[data-testid="calculator-result"]']);
    await page.reload();
    await expect(page.getByTestId("number-lifePath")).toHaveText("5");
    expect(calls).toEqual([]);
    // FAQ accordions under the result.
    const q = page.getByTestId("article-faq").getByRole("button", { name: "Does an adopted name change my Life Path Number?" });
    await q.scrollIntoViewIfNeeded();
    await q.click();
    await expect(page.getByText(/permanently fixed regardless of name changes/)).toBeVisible();
  });
});

test.describe("client overview and chart tab", { tag: "@responsive" }, () => {
  test.describe.configure({ mode: "serial" });
  const email = `e2e+calc-${Date.now()}@staryield.test`;

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: undefined });
    const api = ctx.request;
    const csrf = ((await (await api.get(`${API_URL}/api/v1/auth/csrf`)).json()) as { csrf_token: string }).csrf_token;
    const reg = await api.post(`${API_URL}/api/v1/auth/register`, {
      headers: { "X-CSRF-Token": csrf },
      data: { email, password: "E2e!Secure#2026x", accept_terms: true, birth_date: "1990-03-14", display_name: "Cassandra Moon" },
    });
    expect(reg.ok()).toBeTruthy();
    const token = ((await reg.json()) as { csrf_token: string }).csrf_token;
    const patch = await api.patch(`${API_URL}/api/v1/me/profile`, { headers: { "X-CSRF-Token": token }, data: { birth_time: "08:45", birth_place_id: "gn:2988507" } });
    expect(patch.ok()).toBeTruthy();
    await ctx.storageState({ path: STATE });
    await ctx.close();
  });

  test.use({ storageState: STATE });

  test("overview: compatibility with label, lucky number, mood, horoscope, events, focus", async ({ page }) => {
    await page.goto("/en/profile");
    const overview = page.getByTestId("client-overview");
    await expect(overview.getByTestId("compatibility-score")).toHaveText(/^\d{1,3}%$/);
    await expect(overview.getByTestId("compatibility-label")).toBeVisible();
    await expect(overview.getByTestId("lucky-number")).toHaveText(/^\d+$/);
    await expect(overview.getByText("Mood", { exact: true })).toBeVisible();
    await expect(overview.getByTestId("focus-text")).toContainText("“");
    await expect(overview.getByTestId("cosmic-events").locator("li").first()).toBeVisible();
    await expect(overview.getByText("Upcoming Cosmic Events")).toBeVisible();
    await expect(overview.getByRole("heading", { name: "Your horoscope" }).first()).toBeVisible();
    await settle(page);
    await expectNoA11yViolations(page, ['[data-testid="client-overview"]']);
    await expectNoHorizontalScroll(page);
  });

  test("chart tab: wheel from data, planets are accordion rows (no popups), tables, house system", async ({ page }) => {
    await page.goto("/en/profile/chart");
    const wheel = page.getByTestId("natal-wheel");
    await expect(wheel).toBeVisible();
    await expect(page.locator("[data-testid=natal-wheel] [data-layer=cusps] line")).toHaveCount(12);

    const sun = page.getByTestId("planet-row-Sun").filter({ visible: true });
    await expect(sun).toHaveAttribute("aria-expanded", "false");
    await sun.click();
    await expect(sun).toHaveAttribute("aria-expanded", "true");
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(page.getByText(/The Sun reflects your core identity/)).toBeVisible();
    await expect(page.locator("[data-testid=natal-wheel] [data-body=Sun]")).toHaveAttribute("data-active", "true");
    await sun.click();
    await expect(sun).toHaveAttribute("aria-expanded", "false");

    await page.getByTestId("chart-details-toggle").click();
    await expect(page.getByRole("table", { name: "Positions" })).toBeVisible();
    await expect(page.getByRole("table", { name: "House cusps" })).toBeVisible();
    await expect(page.getByTestId("my-numbers")).toBeVisible();

    const system = page.getByTestId("house-system-select");
    await system.click();
    const chart = page.waitForResponse((r) => r.url().includes("/api/v1/me/chart") && r.url().includes("house_system=whole_sign"));
    await page.getByRole("option", { name: "Whole Sign" }).click();
    expect((await chart).status()).toBe(200);
    await settle(page);
    await expectNoA11yViolations(page, ['[data-testid="chart-tab"]']);
    await expectNoHorizontalScroll(page);
  });
});
