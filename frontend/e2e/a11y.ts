import AxeBuilder from "@axe-core/playwright";
import { expect, type Page } from "@playwright/test";

/**
 * Fail on serious/critical axe violations (WCAG 2.1 A/AA rules) inside the
 * given selectors — new or changed surfaces are checked, pre-existing pages
 * are covered as they are migrated.
 */
export async function expectNoA11yViolations(page: Page, include: string[] = ["body"]) {
  // Pages fade in after a navigation or reload (.app-route-motion): colour
  // contrast is only meaningful once finite animations have settled.
  await page
    .waitForFunction(
      () => document.getAnimations().every((a) => a.playState !== "running" || a.effect?.getComputedTiming().iterations === Infinity),
      undefined,
      { timeout: 5_000 },
    )
    .catch(() => undefined);
  let builder = new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]);
  for (const selector of include) builder = builder.include(selector);
  const { violations } = await builder.analyze();
  const blocking = violations
    .filter((violation) => violation.impact === "serious" || violation.impact === "critical")
    .map((violation) => `${violation.id}: ${violation.help}\n  ${violation.nodes.map((node) => `${node.target.join(" ")} ${node.any[0]?.message ?? ""}`).join("\n  ")}`);
  expect(blocking, blocking.join("\n\n")).toEqual([]);
}
