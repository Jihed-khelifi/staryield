// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render } from "@testing-library/react";

import { CalculatorArticle } from "@/components/content/article/calculator-article";
import { getCalculatorArticle } from "@/content/calculators";

afterEach(cleanup);

// growth (seo): the FAQ answers are in the HTML from the first render (what
// the FAQPage structured data of the calculator pages describes), hidden
// until their question is opened; the articles link to the search hubs.
describe("CalculatorArticle for search engines", () => {
  it("renders every FAQ answer, closed, before any interaction", () => {
    const { container } = render(<CalculatorArticle kind="path-of-life" locale="en" />);
    const faq = getCalculatorArticle("path-of-life", "en").sections.flatMap((s) => s.blocks).find((b) => b.type === "faq");
    if (faq?.type !== "faq") throw new Error("no FAQ");
    const panels = container.querySelectorAll("[data-testid=article-faq] [data-slot=accordion-content]");
    expect(panels).toHaveLength(faq.items.length);
    for (const panel of panels) expect(panel.getAttribute("data-state")).toBe("closed");
    expect(container.querySelector("[data-testid=article-faq]")?.className).toContain("[&_[data-slot=accordion-content][data-state=closed]]:hidden");
    const trigger = container.querySelector<HTMLButtonElement>("[data-testid=article-faq] button[aria-expanded]")!;
    fireEvent.click(trigger);
    expect(panels[0].getAttribute("data-state")).toBe("open");
  });

  it("links each article to its search hub", () => {
    const links = (kind: "path-of-life" | "astrology" | "natal-chart") =>
      [...render(<CalculatorArticle kind={kind} locale="fr" />).container.querySelectorAll("a")].map((a) => a.getAttribute("href"));
    expect(links("path-of-life")).toContain("/fr/life-path-number");
    expect(links("astrology")).toContain("/fr/compatibility");
    expect(links("natal-chart")).toContain("/fr/compatibility");
  });
});
