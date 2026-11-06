/**
 * Advisor home (revisions 2.0, Figma 1174:2294) and header: stats overview,
 * recent reviews pager and the availability menu. Desktop and phone.
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { advisor, expectNoHorizontalScroll, setAvailability, signIn } from "./helpers";

// Solomon is not used by the chat journeys, so his availability is free to toggle.
const SOLOMON = advisor("solomon");

const KPIS = ["free_to_paid", "paid_to_retained", "paid_minutes", "retained_repeat_days", "active_hours"];

/** Seven reviews served in pages of the requested size, newest first. */
async function fakeReviews(page: Page) {
  const all = Array.from({ length: 7 }, (_, i) => ({
    id: `e2e-review-${i + 1}`,
    rating: 5 - (i % 5),
    comment: `E2E review ${i + 1}`,
    author: { name: `Client ${i + 1}` },
    created_at: new Date(Date.UTC(2026, 9, 20 - i)).toISOString(),
  }));
  const limits: number[] = [];
  await page.route(/\/api\/v1\/psychic\/reviews(\?|$)/, async (route) => {
    const url = new URL(route.request().url());
    const limit = Number(url.searchParams.get("limit") ?? 20);
    const start = Number(url.searchParams.get("cursor") ?? 0);
    limits.push(limit);
    const items = all.slice(start, start + limit);
    const next = start + limit < all.length ? String(start + limit) : undefined;
    await route.fulfill({ json: { items, next_cursor: next } });
  });
  return limits;
}

test.describe("advisor overview @responsive", () => {
  test.afterEach(async ({ page }) => {
    await setAvailability(page, "offline").catch(() => undefined);
  });

  test("shows the five KPI cards, switches the period and passes axe", async ({ page }) => {
    await signIn(page, SOLOMON.email, SOLOMON.password);
    const first = page.waitForRequest((r) => r.url().includes("/api/v1/psychic/stats/overview"));
    await page.goto("/en/profile");
    const overview = page.getByTestId("psychic-overview");
    await expect(overview.getByRole("heading", { level: 1 })).toHaveText(/Advisor stats overview/i);
    await expect(overview.getByText("Performance snapshot for your recent sessions.")).toBeVisible();

    const query = new URL((await first).url()).searchParams;
    expect(query.get("tz")).toBe("Europe/Paris");
    expect(query.get("from")).toMatch(/^\d{4}-\d{2}-\d{2}$/);

    for (const kpi of KPIS) {
      const card = page.getByTestId(`kpi-${kpi}`);
      await expect(card).toBeVisible();
      // A value with its unit, or an honest "No data yet" — never a bare 0 %.
      await expect(card.locator(".kpi-value")).toHaveText(/\d|No data yet/);
    }
    await expect(page.getByTestId("kpi-free_to_paid").locator(".kpi-value")).not.toHaveText("0%");

    // The definition opens on click (works on touch too).
    await page.getByTestId("kpi-paid_minutes").getByRole("button", { name: "What does Paid minutes mean?" }).click();
    await expect(page.getByRole("dialog", { name: "Paid minutes" })).toContainText("not counting free minutes");
    await page.keyboard.press("Escape");

    // A different period is fetched for calendar days in the viewer's zone.
    const week = page.waitForRequest((r) => r.url().includes("/api/v1/psychic/stats/overview"));
    await page.getByTestId("overview-range").getByText("7 days", { exact: true }).click();
    const params = new URL((await week).url()).searchParams;
    const days = (Date.parse(params.get("to")!) - Date.parse(params.get("from")!)) / 86_400_000 + 1;
    expect(days).toBe(7);
    await expect(page.getByTestId("kpi-cards")).toBeVisible();

    await expectNoA11yViolations(page, ["[data-testid=psychic-overview]", "[data-testid=availability-menu]"]);
    await expectNoHorizontalScroll(page);
  });

  test("pages through recent reviews five at a time", async ({ page }) => {
    await signIn(page, SOLOMON.email, SOLOMON.password);
    const limits = await fakeReviews(page);
    await page.goto("/en/profile");
    const reviews = page.getByTestId("overview-reviews");
    await expect(reviews.getByTestId("overview-review")).toHaveCount(5);
    await expect(reviews.getByText("E2E review 1", { exact: true })).toBeVisible();
    expect(limits[0]).toBe(5);

    await reviews.getByRole("button", { name: /Next/ }).click();
    await expect(reviews.getByTestId("overview-review")).toHaveCount(2);
    await expect(reviews.getByText("Page 2")).toBeVisible();
    await expect(reviews.getByText("E2E review 6", { exact: true })).toBeVisible();
    await expect(reviews.getByRole("button", { name: /Next/ })).toBeDisabled();

    await reviews.getByRole("button", { name: /Previous/ }).click();
    await expect(reviews.getByText("E2E review 1", { exact: true })).toBeVisible();
    await expectNoA11yViolations(page, ["[data-testid=overview-reviews]"]);
  });

  test("the availability menu toggles online, away and offline", async ({ page }) => {
    await signIn(page, SOLOMON.email, SOLOMON.password);
    await page.goto("/en/profile");
    const trigger = page.getByTestId("availability-menu");
    await setAvailability(page, "offline");
    await expect(trigger).toContainText("Offline");

    await setAvailability(page, "away");
    await expect(trigger).toContainText("Away");
    // Saved on the server: still away after a reload.
    await page.reload();
    await expect(page.getByTestId("availability-menu")).toHaveAttribute("data-status", "away");

    await setAvailability(page, "online");
    await expect(trigger).toContainText("Online");
    await trigger.click();
    await expect(page.getByTestId("availability-online")).toHaveAttribute("aria-checked", "true");
    await expectNoA11yViolations(page, ["[role=menu]"]);
    await page.keyboard.press("Escape");

    await setAvailability(page, "offline");
    await expect(trigger).toContainText("Offline");
    await expectNoHorizontalScroll(page);
  });

  test("the avatar menu goes online and offline, with settings and log out", async ({ page }, info) => {
    test.skip(info.project.name === "mobile", "The avatar menu is the phone menu sheet below 768px.");
    await signIn(page, SOLOMON.email, SOLOMON.password);
    await page.goto("/en/profile");
    await setAvailability(page, "offline");
    await page.getByTestId("user-menu-trigger").click();
    const menu = page.getByRole("menu");
    await expect(menu).toContainText("Solomon");
    await expect(menu.getByRole("menuitem", { name: "Settings" })).toHaveAttribute("href", "/en/profile/settings");
    await expect(menu.getByRole("menuitem", { name: "Log out" })).toBeVisible();
    const saved = page.waitForResponse((r) => r.url().endsWith("/api/v1/psychic/availability") && r.ok());
    await menu.getByRole("menuitem", { name: "Go online" }).click();
    await saved;
    await expect(page.getByRole("menu")).toHaveCount(0);
    await expect(page.getByTestId("availability-menu")).toHaveAttribute("data-status", /online|busy/);
    await page.getByTestId("user-menu-trigger").click();
    await expect(page.getByRole("menuitem", { name: "Go offline" })).toBeVisible();
    await page.keyboard.press("Escape");
  });

  test("the header shows the wallet and advisor navigation", async ({ page }, info) => {
    await signIn(page, SOLOMON.email, SOLOMON.password);
    await page.goto("/en/profile");
    const nav = page.locator("nav.desktop-nav");
    if (info.project.name === "mobile") {
      // Phones: the wallet is in the account sheet.
      await page.getByTestId("mobile-menu-trigger").click();
      await expect(page.getByRole("dialog").getByTestId("wallet-pill")).toHaveText(/^Wallet : [\d,.]+ [A-Z]{3}$/);
      await page.keyboard.press("Escape");
      return;
    }
    await expect(page.getByTestId("wallet-pill")).toHaveText(/^Wallet : [\d,.]+ [A-Z]{3}$/);
    await expect(page.getByTestId("wallet-pill")).toHaveAttribute("href", "/en/profile/analytics");
    await expect(nav.getByRole("link")).toHaveText([/Chatroom/i, /Profile/i]);

    // Tablets: the header keeps to one line in every language (amount only, label for screen readers).
    await page.setViewportSize({ width: 768, height: 1024 });
    for (const locale of ["en", "fr", "es"]) {
      await page.goto(`/${locale}/profile`);
      await expect(page.getByTestId("availability-menu")).toBeVisible();
      await expect(page.getByTestId("wallet-pill")).toHaveAccessibleName(/\d/);
      await expectNoHorizontalScroll(page);
    }
  });
});
