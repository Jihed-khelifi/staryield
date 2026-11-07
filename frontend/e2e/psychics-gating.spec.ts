/**
 * Revisions 3.0: "Psychics" leads visitors to the home section "Psychic
 * readings built around your journey"; the directory (/psychics) is for
 * signed-in customers. Advisor profiles stay public.
 */
import { expect, test } from "@playwright/test";

import { CLIENT, signIn } from "./helpers";

test.describe("visitors", { tag: "@responsive" }, () => {
  test("every public Psychics link points to the home section", async ({ page }) => {
    await page.goto("/en/about");
    const header = page.locator("header.site-header");
    // The mobile project hides the desktop nav behind the menu sheet.
    const desktop = await header.getByRole("navigation").getByRole("link", { name: "Psychics" }).isVisible();
    if (desktop) await expect(header.getByRole("link", { name: "Psychics" })).toHaveAttribute("href", "/en#psychics");
    await expect(page.getByRole("contentinfo").getByRole("link", { name: "Browse Psychics" })).toHaveAttribute("href", "/en#psychics");
    await page.goto("/en");
    await expect(page.getByRole("link", { name: "Explore Advisors" }).first()).toHaveAttribute("href", "/en#psychics");
    await page.goto("/fr");
    await expect(page.getByRole("contentinfo").getByRole("link").filter({ hasText: "Parcourir les médiums" }).first()).toHaveAttribute("href", "/fr#psychics");
  });

  test("/psychics sends an anonymous visitor to the section without ever showing the directory", async ({ page }) => {
    const cards: number[] = [];
    await page.exposeFunction("sawCard", (n: number) => cards.push(n));
    await page.addInitScript(() => {
      new MutationObserver(() => {
        const n = document.querySelectorAll("article.advisor-card").length;
        if (n) (window as unknown as { sawCard: (n: number) => void }).sawCard(n);
      }).observe(document, { childList: true, subtree: true });
    });
    await page.goto("/en/psychics");
    await page.waitForURL(/\/en#psychics$/);
    await expect(page.locator("#psychics")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Psychic readings built around your journey" })).toBeVisible();
    expect(cards).toEqual([]);
  });

  test("the directory page is not indexable", async ({ request }) => {
    const html = await (await request.get("/en/psychics")).text();
    expect(html).toMatch(/<meta name="robots" content="noindex/);
  });

  test("clicking Psychics scrolls to the section and focuses its heading", async ({ page }) => {
    await page.goto("/en");
    const link = page.getByRole("contentinfo").getByRole("link", { name: "Browse Psychics" });
    await link.click();
    await expect(page).toHaveURL(/\/en#psychics$/);
    await expect(page.locator("#psychics-heading")).toBeFocused();
    await expect(page.locator("#psychics")).toBeInViewport();
    const margin = await page.locator("#psychics").evaluate((el) => getComputedStyle(el).scrollMarginTop);
    expect(parseInt(margin)).toBeGreaterThan(0);
  });

  test("arriving with #psychics from another page lands on the section", async ({ page }) => {
    await page.goto("/en/about");
    await page.goto("/en#psychics");
    await expect(page.locator("#psychics-heading")).toBeFocused();
    await expect(page.locator("#psychics")).toBeInViewport();
  });

  test("advisor profiles stay public", async ({ page }) => {
    await page.goto("/en/psychics/theo");
    await expect(page.getByRole("heading", { level: 1 })).toContainText("Theo");
    expect(new URL(page.url()).pathname).toBe("/en/psychics/theo");
  });
});

test.describe("signed-in customers", () => {
  test("see the directory and the links to it", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en");
    await expect(page.getByRole("contentinfo").getByRole("link", { name: "Browse Psychics" })).toHaveAttribute("href", "/en/psychics");
    await page.goto("/en/psychics");
    await expect(page.locator("article.advisor-card").first()).toBeVisible();
    expect(new URL(page.url()).pathname).toBe("/en/psychics");
    await expect(page.locator("header.site-header").getByRole("link", { name: "Psychics" }).first()).toHaveAttribute("href", "/en/psychics");
  });
});
