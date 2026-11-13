import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { expectNoHorizontalScroll } from "./helpers";

/**
 * Refund & Cancellation Policy (`/refund-policy`, src/content/legal/refund, rendered with the
 * policy layout of components/content/legal/policy-document.tsx). Content parity across locales
 * is unit-tested (npm run test:content, revisions-5-0.test.ts); this spec checks the rendered page.
 */
const WIDTHS = [320, 360, 390, 768, 1024, 1199, 1200, 1440, 1920, 2560];
const LOCALES = ["en", "fr", "es"] as const;
const doc = (page: Page) => page.locator("[data-legal-document=refund]");

test.describe("refund policy", () => {
  test("@responsive shows the policy with header actions, dates, placeholders and related links", async ({ page }) => {
    await page.goto("/en/refund-policy");
    await expect(page).toHaveTitle(/^Refund Policy — /);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Refund & Cancellation Policy");
    await expect(doc(page)).toContainText("Effective date: 13.11.2026");
    await expect(doc(page)).toContainText("Last updated: 13.11.2026");
    await expect(doc(page).getByRole("link", { name: "Contact Support" }).first()).toHaveAttribute("href", "/en/contact");
    await expect(doc(page).getByRole("link", { name: "Go to Pricing" })).toHaveAttribute("href", "/en/pricing");
    for (const value of ["within 2 business days", "within 5 business days", "as STARYIELD MEDIA LLC"]) await expect(doc(page).locator("article")).toContainText(value);
    await expect(doc(page).locator("article")).not.toContainText("NOTDEFINEDYET");
    await expect(doc(page).getByRole("link", { name: "support@staryield.net" }).first()).toHaveAttribute("href", "mailto:support@staryield.net");
    await expect(doc(page).getByRole("link", { name: "Contact page" })).toHaveAttribute("href", "/en/contact");
    const related = doc(page).getByRole("navigation", { name: "Related links:" });
    await expect(related.getByRole("link")).toHaveText(["Terms of Service", "Privacy Policy", "Pricing and Credits"]);
    await expect(related.getByRole("link", { name: "Pricing and Credits" })).toHaveAttribute("href", "/en/pricing");
  });

  for (const locale of LOCALES)
    test(`${locale}: contents anchors target unique ids; only internal and mailto links`, async ({ page }) => {
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto(`/${locale}/refund-policy`);
      const toc = await doc(page).locator("nav#toc a").evaluateAll((links) => links.map((a) => a.getAttribute("href") ?? ""));
      expect(toc).toHaveLength(14);
      const counts = await page.evaluate((list) => list.map((h) => document.querySelectorAll(`[id="${CSS.escape(h.slice(1))}"]`).length), toc);
      expect(counts.every((n) => n === 1)).toBe(true);
      const ids = await doc(page).locator("[id]").evaluateAll((els) => els.map((el) => el.id));
      expect(ids.filter((id, i) => ids.indexOf(id) !== i)).toEqual([]);
      const links = await doc(page).locator("a[href]").evaluateAll((els) => els.map((a) => a.getAttribute("href") ?? ""));
      expect(links.filter((h) => /^([a-z]+:)?\/\//i.test(h) || h.startsWith("javascript:"))).toEqual([]);
      expect(links.filter((h) => h.startsWith("/") && h !== `/${locale}` && !h.startsWith(`/${locale}/`))).toEqual([]);
    });

  test("anchors are identical in English, French and Spanish", async ({ page }) => {
    const all: string[][] = [];
    for (const locale of LOCALES) {
      await page.goto(`/${locale}/refund-policy`);
      all.push(await doc(page).locator("[id]").evaluateAll((els) => els.map((el) => el.id)));
    }
    expect(all[1]).toEqual(all[0]);
    expect(all[2]).toEqual(all[0]);
  });

  test("desktop: sticky contents beside a 920px card (760px of text); a section link scrolls to its heading", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/en/refund-policy");
    const aside = doc(page).locator("aside");
    await expect(aside).toBeVisible();
    await expect(doc(page).locator("details")).toBeHidden();
    const geometry = await page.evaluate(() => {
      const a = document.querySelector("aside")!.getBoundingClientRect();
      const card = document.querySelector("article")!;
      const c = card.getBoundingClientRect();
      const s = getComputedStyle(card);
      return { asideWidth: a.width, cardWidth: c.width, text: c.width - parseFloat(s.paddingLeft) - parseFloat(s.paddingRight), radius: s.borderTopLeftRadius, font: parseFloat(getComputedStyle(card.querySelector("section p")!).fontSize), sticky: getComputedStyle(document.querySelector("aside")!).position };
    });
    expect(geometry.asideWidth).toBe(232);
    expect(geometry.cardWidth).toBe(920);
    expect(geometry.text).toBe(760);
    expect(geometry.radius).toBe("8px");
    expect(geometry.font).toBeGreaterThanOrEqual(16);
    expect(geometry.sticky).toBe("sticky");
    // Still in view after scrolling down.
    await page.evaluate(() => window.scrollTo(0, 1500));
    await expect(aside.locator("nav h2")).toBeInViewport();
    await aside.getByRole("link", { name: "9. Requesting a refund" }).click();
    await expect(page.locator("#requesting-a-refund")).toBeInViewport();
  });

  test("@responsive phone: stacked layout with a collapsible contents list", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/en/refund-policy");
    await expect(doc(page).locator("aside")).toBeHidden();
    const details = doc(page).locator("details");
    await expect(details).toBeVisible();
    await expect(details).not.toHaveAttribute("open", "");
    const summary = details.locator("summary");
    expect((await summary.boundingBox())!.height).toBeGreaterThanOrEqual(44);
    await summary.click();
    await expect(details).toHaveAttribute("open", "");
    await details.getByRole("link", { name: "11. Payment disputes" }).click();
    await expect(page.locator("#payment-disputes")).toBeInViewport();
    // Header actions are stacked full width.
    const [a, b] = await doc(page).locator("header ~ div a, .flex.w-full.flex-col.gap-4 a").evaluateAll((els) => els.slice(0, 2).map((el) => el.getBoundingClientRect().toJSON()));
    expect(b.top).toBeGreaterThan(a.bottom - 1);
    await expectNoHorizontalScroll(page);
  });

  for (const width of WIDTHS)
    test(`no horizontal overflow, 16px+ body text and 44px targets at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/en/refund-policy");
      await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
      await expectNoHorizontalScroll(page);
      const facts = await page.evaluate(() => {
        const card = document.querySelector("article")!.getBoundingClientRect();
        const aside = document.querySelector("aside")!;
        const small = [...document.querySelectorAll<HTMLElement>("article p, article li")].filter((el) => parseFloat(getComputedStyle(el).fontSize) < 16).length;
        const targets = [...document.querySelectorAll<HTMLElement>("article a, aside a, details summary, details[open] a")]
          .filter((el) => { const r = el.getBoundingClientRect(); return r.width > 0 && r.height > 0 && r.height < 44 && !(el.closest("p, li") && el.closest("article") && el.tagName === "A"); })
          .map((el) => `${el.textContent?.trim().slice(0, 30)} ${Math.round(el.getBoundingClientRect().height)}`);
        return { cardRight: card.right, viewport: window.innerWidth, small, targets, sidebar: getComputedStyle(aside).display !== "none" };
      });
      expect(facts.cardRight).toBeLessThanOrEqual(facts.viewport);
      expect(facts.small).toBe(0);
      expect(facts.targets).toEqual([]);
      expect(facts.sidebar).toBe(width >= 1024);
    });

  test("@responsive has no serious accessibility violations", async ({ page }) => {
    await page.goto("/en/refund-policy");
    await expectNoA11yViolations(page, ["main"]);
  });
});
