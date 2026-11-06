import { expect, test, type Page } from "@playwright/test";

import { expectNoHorizontalScroll } from "./helpers";

/**
 * Privacy Policy and Terms of Use (src/content/legal, rendered by
 * components/content/legal/legal-document.tsx). Content parity is unit-tested
 * (npm run test:content); this spec checks the rendered pages.
 */
const DOCS = ["privacy", "terms"] as const;
const LOCALES = ["en", "es", "fr"] as const;
const LAST_UPDATED = { en: "Last updated", es: "Última actualización", fr: "Dernière mise à jour" };

const legal = (page: Page) => page.locator("[data-legal-document]");
const hrefs = (page: Page, selector: string) =>
  legal(page)
    .locator(selector)
    .evaluateAll((links) => links.map((link) => link.getAttribute("href") ?? ""));
const ids = (page: Page) =>
  legal(page)
    .locator("[id]")
    .evaluateAll((elements) => elements.map((element) => element.id));

/** How many elements in the whole page carry each fragment's id. */
const targetCounts = (page: Page, fragments: string[]) =>
  page.evaluate(
    (list) => list.map((href) => document.querySelectorAll(`[id="${CSS.escape(href.slice(1))}"]`).length),
    fragments,
  );

test.describe("legal documents", () => {
  for (const doc of DOCS)
    for (const locale of LOCALES)
      test(`${locale}/${doc}: TOC and in-page links target unique ids; no /psychics or external links`, async ({ page }) => {
        await page.goto(`/${locale}/${doc}`);
        const root = legal(page);
        await expect(root.locator("nav#toc")).toBeVisible();
        await expect(root.locator("header").getByText(LAST_UPDATED[locale], { exact: true })).toBeVisible();
        await expect(root.locator("header time")).toHaveCount(2);

        const toc = await hrefs(page, "nav#toc a");
        expect(toc.length).toBeGreaterThan(10);
        expect(toc.filter((href) => !href.startsWith("#"))).toEqual([]);

        const fragments = await hrefs(page, "a[href^='#']");
        const counts = await targetCounts(page, fragments);
        expect(fragments.filter((_, index) => counts[index] !== 1)).toEqual([]);

        const all = await ids(page);
        expect(all.filter((id, index) => all.indexOf(id) !== index)).toEqual([]);

        const links = await hrefs(page, "a[href]");
        expect(links.filter((href) => href.includes("/psychics"))).toEqual([]);
        expect(links.filter((href) => /^([a-z]+:)?\/\//i.test(href) || href.startsWith("javascript:"))).toEqual([]);
        expect(links.filter((href) => href.startsWith("/") && !href.startsWith(`/${locale}/`))).toEqual([]);
      });

  for (const doc of DOCS)
    test(`${doc}: Spanish and French anchors are identical to English`, async ({ page }) => {
      await page.goto(`/en/${doc}`);
      const english = await ids(page);
      expect(english.length).toBeGreaterThan(20);
      for (const locale of ["es", "fr"] as const) {
        await page.goto(`/${locale}/${doc}`);
        expect(await ids(page), locale).toEqual(english);
      }
    });

  test("a TOC entry scrolls to its clause and Back to top returns to the TOC", async ({ page }) => {
    await page.goto("/en/privacy");
    const root = legal(page);
    await root.locator("nav#toc a[href='#cookies']").click();
    await expect(page).toHaveURL(/#cookies$/);
    await expect(page.locator("h2#cookies")).toBeInViewport();
    await root.getByRole("link", { name: "Back to top" }).click();
    await expect(page).toHaveURL(/#toc$/);
    await expect(root.locator("nav#toc")).toBeInViewport();
  });

  test("Terms link to the Privacy Policy in the same locale", async ({ page }) => {
    await page.goto("/fr/terms");
    await legal(page).locator("a[href='/fr/privacy']").first().click();
    await expect(page).toHaveURL(/\/fr\/privacy$/);
    await expect(page.locator("[data-legal-document=privacy] h1")).toHaveText("Politique de confidentialité");
  });

  test("print styles drop the on-screen chrome", async ({ page }) => {
    await page.goto("/en/terms");
    const backToTop = legal(page).getByRole("link", { name: "Back to top" });
    await expect(backToTop).toBeVisible();
    await page.emulateMedia({ media: "print" });
    await expect(backToTop).toBeHidden();
  });
});

test.describe("legal documents on a phone", () => {
  test.use({ viewport: { width: 375, height: 812 } });

  for (const doc of DOCS)
    test(`${doc}: no horizontal page scroll in any locale`, async ({ page }) => {
      for (const locale of LOCALES) {
        await page.goto(`/${locale}/${doc}`);
        await expect(legal(page).locator("nav#toc")).toBeVisible();
        await expectNoHorizontalScroll(page);
      }
    });

  test("privacy tables scroll inside a focusable region instead of the page", async ({ page }) => {
    await page.goto("/en/privacy");
    const regions = legal(page).locator("[role=region]:has(table)");
    await expect(regions).toHaveCount(3);
    for (const region of await regions.all()) {
      await expect(region).toHaveAttribute("tabindex", "0");
      await expect(region).toHaveAttribute("aria-label", /.+/);
    }
    const cookies = page.locator("section[aria-labelledby=cookies] [role=region]");
    const overflow = await cookies.evaluate((element) => element.scrollWidth - element.clientWidth);
    expect(overflow).toBeGreaterThan(0);
    await expectNoHorizontalScroll(page);
  });
});
