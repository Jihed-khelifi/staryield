import { expect, test, type Page } from "@playwright/test";
import { expectNoA11yViolations } from "./a11y";
import { expectNoHorizontalScroll } from "./helpers";

const WIDTHS = [320, 360, 375, 390, 414, 600, 768, 820, 1024, 1280, 1440];
const LOCALES = ["en", "fr", "es"];

/** Overflow, overlap and mid-word-break findings for the home sections and the footer. */
async function auditLayout(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const out: string[] = [];
    const label = (el: Element) =>
      `${el.tagName.toLowerCase()}${el.getAttribute("data-name") ? `[${el.getAttribute("data-name")}]` : ""}${typeof el.className === "string" && el.className ? "." + el.className.split(" ")[0] : ""}`;
    const containers = [...document.querySelectorAll<HTMLElement>(".figma-content .figma-section, .site-footer")];
    for (const box of containers) {
      const rect = box.getBoundingClientRect();
      const name = label(box);
      if (box.scrollHeight > box.clientHeight + 1) out.push(`${name} content taller than box (${box.scrollHeight} > ${box.clientHeight})`);
      if (box.scrollWidth > box.clientWidth + 1) out.push(`${name} content wider than box (${box.scrollWidth} > ${box.clientWidth})`);
      for (const el of box.querySelectorAll<HTMLElement>("*")) {
        if (el.closest("[aria-hidden=true], svg, picture")) continue;
        const r = el.getBoundingClientRect();
        if (!r.width || !r.height) continue;
        const style = getComputedStyle(el);
        if (!el.closest('[data-name="Card / Tarot Fan"]') && el.scrollWidth > el.clientWidth + 1 && style.display !== "inline" && el.clientWidth > 0 && style.overflowX === "visible") {
          out.push(`${label(el)} inside ${name} overflows horizontally (${el.scrollWidth} > ${el.clientWidth}): "${(el.textContent ?? "").trim().slice(0, 30)}"`);
        }
        if (r.right > rect.right + 1 || r.left < rect.left - 1) out.push(`${label(el)} escapes ${name} horizontally: "${(el.textContent ?? "").trim().slice(0, 30)}"`);
        if (el.children.length === 0 && r.bottom > rect.bottom + 1) out.push(`${label(el)} escapes ${name} at the bottom`);
      }
      // normal words must not be split across lines
      const walker = document.createTreeWalker(box, NodeFilter.SHOW_TEXT);
      for (let node = walker.nextNode(); node; node = walker.nextNode()) {
        const parent = node.parentElement;
        if (!parent || parent.closest("[aria-hidden=true], svg")) continue;
        const text = node.textContent ?? "";
        for (const match of text.matchAll(/[\p{L}\p{N}']{4,}/gu)) {
          const range = document.createRange();
          range.setStart(node, match.index!);
          range.setEnd(node, match.index! + match[0].length);
          const tops = new Set([...range.getClientRects()].filter((q) => q.width > 0).map((q) => Math.round(q.top / 4)));
          if (tops.size > 1) out.push(`word "${match[0]}" broken mid-word in ${label(parent)}`);
        }
      }
    }
    const sections = [...document.querySelectorAll<HTMLElement>(".figma-content > *, .site-footer")];
    for (let i = 0; i + 1 < sections.length; i++) {
      const a = sections[i].getBoundingClientRect();
      const b = sections[i + 1].getBoundingClientRect();
      if (a.height && b.height && a.bottom > b.top + 1) out.push(`${label(sections[i])} overlaps ${label(sections[i + 1])} (${a.bottom} > ${b.top})`);
    }
    return out;
  });
}

/** Cards per row and narrowest card width ratio of How To Start. */
async function startCards(page: Page) {
  return page.evaluate(() => {
    const cards = [...document.querySelectorAll<HTMLElement>('[data-name^="Content Group / 0"]')].filter((c) => c.closest('[data-name="Section / How To Start"]'));
    const parent = cards[0].parentElement!.getBoundingClientRect();
    const rects = cards.map((c) => c.getBoundingClientRect());
    return { count: cards.length, perRow: rects.filter((r) => Math.abs(r.top - rects[0].top) < 2).length, minRatio: Math.min(...rects.map((r) => r.width / parent.width)) };
  });
}

/** Link groups per row (brand excluded) and the brand block width. */
async function footerColumns(page: Page) {
  return page.evaluate(() => {
    const cols = [...document.querySelectorAll<HTMLElement>(".footer-columns > *")];
    const firstTop = cols[1].getBoundingClientRect().top;
    return {
      groupsPerRow: cols.slice(1).filter((c) => Math.abs(c.getBoundingClientRect().top - firstTop) < 2).length,
      brandWidth: cols[0].getBoundingClientRect().width,
      container: cols[0].parentElement!.getBoundingClientRect().width,
    };
  });
}

test.describe("responsive landing page and footer @responsive", () => {
  for (const locale of LOCALES) {
    test(`home bottom sections and footer fit every width (${locale})`, async ({ page }) => {
      test.setTimeout(180_000);
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto(`/${locale}`);
      await page.locator('[data-name="Section / How To Start"]').waitFor();
      await page.evaluate(() => document.fonts.ready);
      const problems: string[] = [];
      for (const width of WIDTHS) {
        await page.setViewportSize({ width, height: 900 });
        await page.waitForTimeout(150);
        await expectNoHorizontalScroll(page);
        problems.push(...(await auditLayout(page)).map((finding) => `@${width} ${finding}`));

        const cards = await startCards(page);
        expect(cards.count).toBe(4);
        expect(cards.perRow, `${locale} @${width} cards per row`).toBe(width >= 1024 ? 4 : width >= 640 ? 2 : 1);
        if (width < 640) expect(cards.minRatio).toBeGreaterThanOrEqual(0.85);

        const footer = await footerColumns(page);
        expect(footer.groupsPerRow, `${locale} @${width} footer groups per row`).toBe(width >= 768 ? 3 : width >= 360 ? 2 : 1);
        if (width < 1024) expect(footer.brandWidth).toBeGreaterThanOrEqual(footer.container - 1);
      }
      expect(problems, problems.join("\n")).toEqual([]);
    });
  }

  test("home bottom section and footer have no a11y violations on a phone", async ({ page }) => {
    await page.setViewportSize({ width: 360, height: 800 });
    await page.goto("/en");
    await page.locator('[data-name="Section / How To Start"]').waitFor();
    // The step cards (numbers use the accessible gold, 4.8:1 on cream), the CTA and the footer.
    await expectNoA11yViolations(page, [".site-footer", '[data-name="Control / Get started"]', '[data-name="Layout / How To Start / 02"]']);
  });
});
