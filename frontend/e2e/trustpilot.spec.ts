import { expect, test, type Page } from "@playwright/test";

import { expectNoHorizontalScroll } from "./helpers";

/**
 * Trustpilot "Review Collector" widget (docs/platform-extensions.md §23.7).
 * Trustpilot is never contacted for real: `widget.trustpilot.com` is answered
 * by a stub bootstrap that defines `window.Trustpilot.loadFromElement` and
 * marks the element it is given.
 */
const REVIEW_URL = "https://www.trustpilot.com/review/staryield.net";
const STUB = `window.Trustpilot = { loadFromElement: function (el, full) { el.setAttribute("data-stub-loaded", full ? "full" : "partial"); } };`;
const TRUSTPILOT = /widget\.trustpilot\.com/;

const widget = (page: Page, placement: string) => page.locator(`[data-testid=trustpilot-widget][data-placement=${placement}]`);

async function stubTrustpilot(page: Page) {
  const requests: string[] = [];
  page.on("request", (request) => {
    if (/trustpilot\.com/.test(request.url())) requests.push(request.url());
  });
  await page.route("**/widget.trustpilot.com/**", (route) => route.fulfill({ status: 200, contentType: "application/javascript", body: STUB }));
  return requests;
}

async function settle(page: Page, path: string) {
  await page.goto(path);
  await page.waitForLoadState("networkidle");
}

test.describe("Trustpilot widget", () => {
  test("/en/reviews: the hero widget loads the script, the footer one waits until it is scrolled into view", async ({ page }) => {
    const requests = await stubTrustpilot(page);
    await settle(page, "/en/reviews");
    await expect(page.getByRole("heading", { level: 1, name: "Trustpilot" })).toBeVisible();
    // The fabricated figures are gone.
    await expect(page.getByText(/17,419|4\.4 rating/)).toHaveCount(0);

    const hero = widget(page, "reviews");
    await expect(hero).toHaveAttribute("data-state", "loaded");
    await expect(hero.locator(".trustpilot-widget")).toHaveAttribute("data-stub-loaded", "full");
    await expect(hero.locator(".trustpilot-widget")).toHaveAttribute("data-locale", "en-US");
    const footer = widget(page, "footer");
    await expect(footer).toHaveAttribute("data-state", "idle");
    expect(requests).toHaveLength(1);

    await footer.scrollIntoViewIfNeeded();
    await expect(footer).toHaveAttribute("data-state", "loaded");
    await expect(footer.locator(".trustpilot-widget")).toHaveAttribute("data-stub-loaded", "full");
    // One script for both widgets.
    expect(requests).toHaveLength(1);
  });

  test("home: nothing is requested until the widget under the testimonials scrolls into view", async ({ page }) => {
    const requests = await stubTrustpilot(page);
    await settle(page, "/en");
    expect(requests).toEqual([]);

    const home = widget(page, "home");
    await expect(home).toHaveAttribute("data-state", "idle");
    const request = page.waitForRequest(TRUSTPILOT);
    await home.scrollIntoViewIfNeeded();
    await request;
    await expect(home).toHaveAttribute("data-state", "loaded");
    await expect(home.locator(".trustpilot-widget")).toHaveAttribute("data-stub-loaded", "full");
    // It sits under the testimonials and statistics.
    await expect(page.locator('[data-name="Container / Testimonials & Stats"]').locator("[data-testid=trustpilot-widget]")).toHaveCount(1);
  });

  test("footer: the script is requested only after the footer widget comes into view", async ({ page }) => {
    const requests = await stubTrustpilot(page);
    await settle(page, "/en/terms");
    expect(requests).toEqual([]);

    const footer = widget(page, "footer");
    const request = page.waitForRequest(TRUSTPILOT);
    await footer.scrollIntoViewIfNeeded();
    await request;
    await expect(footer).toHaveAttribute("data-state", "loaded");
    await expect(footer.locator(".trustpilot-widget")).toHaveAttribute("data-stub-loaded", "full");
    // The row sits above the copyright line.
    const widgetBox = (await footer.boundingBox())!;
    const copyrightBox = (await page.getByText(/© 2026 Staryield Media LLC/).boundingBox())!;
    expect(widgetBox.y + widgetBox.height).toBeLessThanOrEqual(copyrightBox.y + 1);
  });

  test("when Trustpilot cannot be reached the plain link to the review page is shown", async ({ page }) => {
    await page.route("**/widget.trustpilot.com/**", (route) => route.abort());
    await settle(page, "/en/reviews");
    const hero = widget(page, "reviews");
    await expect(hero).toHaveAttribute("data-state", "failed");
    const link = hero.getByRole("link", { name: "Review us on Trustpilot (opens in a new tab)" });
    await expect(link).toBeVisible();
    await expect(link).toHaveText("Review us on Trustpilot");
    await expect(link).toHaveAttribute("href", REVIEW_URL);
    await expect(link).toHaveAttribute("target", "_blank");
    await expect(link).toHaveAttribute("rel", "noopener");
  });

  test("Global Privacy Control: Trustpilot is never contacted and a link is shown instead", async ({ page }) => {
    await page.addInitScript(() => {
      Object.defineProperty(navigator, "globalPrivacyControl", { value: true, configurable: true });
    });
    const requests = await stubTrustpilot(page);
    await settle(page, "/en/reviews");
    const hero = widget(page, "reviews");
    await expect(hero).toHaveAttribute("data-state", "blocked");
    await expect(hero.getByRole("link", { name: /Review us on Trustpilot/ })).toHaveAttribute("href", REVIEW_URL);
    const footer = widget(page, "footer");
    await footer.scrollIntoViewIfNeeded();
    await expect(footer).toHaveAttribute("data-state", "blocked");
    await page.waitForTimeout(500);
    expect(requests).toEqual([]);
    await expect(page.locator("script[src*=trustpilot]")).toHaveCount(0);
    await expect(page.locator("iframe[src*=trustpilot]")).toHaveCount(0);
  });

  test("Do Not Track behaves the same as Global Privacy Control", async ({ page }) => {
    await page.addInitScript(() => {
      Object.defineProperty(navigator, "doNotTrack", { value: "1", configurable: true });
    });
    const requests = await stubTrustpilot(page);
    await settle(page, "/en");
    await widget(page, "home").scrollIntoViewIfNeeded();
    await expect(widget(page, "home")).toHaveAttribute("data-state", "blocked");
    expect(requests).toEqual([]);
  });

  test("@responsive the reviews hero does not overflow on phones or desktop, loaded or not", async ({ page }) => {
    await stubTrustpilot(page);
    await settle(page, "/en/reviews");
    await expect(widget(page, "reviews")).toHaveAttribute("data-state", "loaded");
    await expectNoHorizontalScroll(page);
    await page.unroute("**/widget.trustpilot.com/**");
    await page.route("**/widget.trustpilot.com/**", (route) => route.abort());
    await page.reload();
    await expect(widget(page, "reviews")).toHaveAttribute("data-state", "failed");
    await expectNoHorizontalScroll(page);
  });
});
