/**
 * Revisions 2.0 foundations: 404 page, footer social links, faceless small
 * logos, blurred top-up backdrop and phone layouts. Runs on desktop and on
 * the mobile project (@responsive).
 */
import { expect, test } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { CLIENT, expectNoHorizontalScroll, signIn } from "./helpers";

test.describe("site foundations", { tag: "@responsive" }, () => {
  test("unknown URLs get a translated 404 page with a real 404 status", async ({ page }) => {
    const response = await page.goto("/en/this-page-does-not-exist");
    expect(response?.status()).toBe(404);
    await expect(page.getByRole("heading", { level: 1, name: "Page not found" })).toBeVisible();
    await expect(page).toHaveTitle(/Page not found/);
    await expect(page.getByRole("link", { name: "Back to home" })).toHaveAttribute("href", "/en");
    const helpful = page.getByRole("navigation", { name: "Helpful links" });
    await expect(helpful.getByRole("link", { name: "Psychics" })).toHaveAttribute("href", "/en/psychics");
    await expectNoA11yViolations(page, ['[data-testid="not-found"]']);

    await page.goto("/fr/cette-page-nexiste-pas");
    await expect(page.getByRole("heading", { level: 1 })).not.toHaveText("Page not found");
  });

  test("footers link every live social profile in a new tab", async ({ page }) => {
    await page.goto("/en");
    const social = page.getByRole("navigation", { name: "Social platforms" });
    const links = social.getByRole("link");
    await expect(links).toHaveCount(5);
    const expected = [
      ["Facebook", "https://www.facebook.com/profile.php?id=61595272673949"],
      ["X", "https://x.com/StaryieldCoven"],
      ["Instagram", "https://www.instagram.com/staryield/"],
      ["Reddit", "https://www.reddit.com/r/staryield/"],
      ["TikTok", "https://www.tiktok.com/@staryield.media"],
    ];
    for (const [index, [name, href]] of expected.entries()) {
      const link = links.nth(index);
      await expect(link).toHaveAccessibleName(`Staryield on ${name} (opens in a new tab)`);
      await expect(link).toHaveAttribute("href", href);
      await expect(link).toHaveAttribute("target", "_blank");
      await expect(link).toHaveAttribute("rel", "noopener noreferrer");
    }
    await social.scrollIntoViewIfNeeded();
    const loaded = await social
      .locator("img")
      .evaluateAll((imgs) => imgs.map((img) => (img as HTMLImageElement).complete && (img as HTMLImageElement).naturalWidth > 0));
    expect(loaded).toEqual([true, true, true, true, true]);
    await expectNoA11yViolations(page, ['[data-testid="social-links"]']);
  });

  test("small logos are the faceless sun", async ({ page }) => {
    // Public frame (header + footer) and app frame (header only).
    for (const [path, regions] of [
      ["/en", ["header", "footer"]],
      ["/en/psychics", ["header"]],
    ] as const) {
      await page.goto(path);
      for (const region of regions) {
        const logo = page.locator(`${region} .sun-logo`).first();
        await expect(logo.locator(".sun-shape")).toHaveCount(1);
        await expect(logo.locator(".sun-face")).toHaveCount(0);
      }
    }
    // The large logo-page sun keeps its face.
    await page.goto("/en/login");
    await expect(page.locator(".sun-logo-large .sun-face")).toHaveCount(1);
  });

  test("public pages fit the viewport without horizontal scrolling", async ({ page }) => {
    for (const path of ["/en", "/en/psychics", "/en/login", "/en/this-page-does-not-exist"]) {
      await page.goto(path);
      await expectNoHorizontalScroll(page);
    }
  });

  test("the top-up dialog blurs the page behind it", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en/credits");
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    const overlay = page.locator('[data-slot="dialog-overlay"]');
    await expect(overlay).toHaveAttribute("data-overlay", "blur");
    const filter = await overlay.evaluate((el) => {
      const style = getComputedStyle(el);
      return style.backdropFilter || style.getPropertyValue("-webkit-backdrop-filter");
    });
    expect(filter).toContain("blur(6px)");
    await page.keyboard.press("Escape");
    await page.waitForURL(/\/en\/profile/);
  });
});
