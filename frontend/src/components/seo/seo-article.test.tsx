// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, screen, within } from "@testing-library/react";

import { axeViolations, dictionary, renderWithI18n, stubResizeObserver } from "@/test/render";

vi.mock("@/i18n/dictionaries", () => ({ getDictionary: async () => dictionary }));
vi.mock("@/components/layout/public-frame", () => ({ PublicFrame: ({ children }: { children: React.ReactNode }) => <main>{children}</main> }));
vi.mock("@/lib/psychics-link", () => ({ usePsychicsHref: () => "/en#psychics" }));
// The advisor strip (growth: booking) reads the live advisors; it has its own tests.
vi.mock("@/components/booking/advisor-strip", () => ({
  AdvisorStrip: ({ placement }: { placement: string }) => <div data-testid="advisor-strip" data-placement={placement} />,
}));
vi.mock("@/lib/analytics", () => ({ track: vi.fn() }));

import { SeoArticle } from "@/components/seo/seo-article";
import { lifePathHubPage, lifePathPage } from "@/content/seo/life-path";
import { compatibilityPage } from "@/content/seo/compatibility";

beforeAll(stubResizeObserver);
afterEach(cleanup);

async function renderPage(...args: Parameters<typeof SeoArticle>) {
  return renderWithI18n(await SeoArticle(...args));
}

function jsonLd(container: HTMLElement) {
  return [...container.querySelectorAll('script[type="application/ld+json"]')].flatMap((script) => JSON.parse(script.textContent ?? ""));
}

describe("SeoArticle", () => {
  it("renders a detail page: H1, direct answer, sections, visible FAQ, links, CTA, and the calculator last", async () => {
    const page = lifePathPage("en", 7);
    const { container } = await renderPage({ page, locale: "en", family: "life-path", calculator: <div data-testid="calc-slot" /> });
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe("Life Path Number 7: The Seeker");
    expect(screen.getByTestId("seo-answer").textContent).toBe(page.intro);
    expect([...container.querySelectorAll("[data-section]")].map((s) => s.getAttribute("data-section"))).toEqual(page.sections.map((s) => s.id));
    // The FAQ answers are plain text on the page (what the FAQPage data describes).
    const faq = screen.getByTestId("seo-faq");
    for (const item of page.faq) {
      expect(within(faq).getByRole("heading", { level: 3, name: item.q })).toBeTruthy();
      expect(faq.textContent).toContain(item.a);
    }
    // Locale-prefixed internal links, the current page marked in the breadcrumbs.
    const crumbs = screen.getByTestId("seo-breadcrumbs");
    expect(within(crumbs).getByRole("link", { name: "Life Path numbers" }).getAttribute("href")).toBe("/en/life-path-number");
    expect(within(crumbs).getByText("Life Path 7").getAttribute("aria-current")).toBe("page");
    expect(within(screen.getByTestId("seo-related")).getByRole("link", { name: /Life Path 6/ }).getAttribute("href")).toBe("/en/life-path-number/6");
    expect(screen.getByTestId("seo-entertainment-note").textContent).toContain("for entertainment purposes only");
    expect(within(screen.getByTestId("seo-advisor-cta")).getByRole("link", { name: "Meet our advisors" }).getAttribute("href")).toBe("/en#psychics");
    expect(within(screen.getByTestId("seo-advisor-cta")).getByTestId("advisor-strip").dataset.placement).toMatch(/^seo_/);
    // The page ends with the calculator.
    const article = screen.getByTestId("seo-page");
    expect(article.lastElementChild?.getAttribute("data-testid")).toBe("seo-calculator");
    expect(within(screen.getByTestId("seo-calculator")).getByTestId("calc-slot")).toBeTruthy();
  });

  it("puts the calculator right under the hub's introduction, before the grid", async () => {
    const page = lifePathHubPage("en");
    await renderPage({ page, locale: "en", family: "life-path", calculator: <div data-testid="calc-slot" /> });
    const calculator = screen.getByTestId("seo-calculator");
    const grid = screen.getByTestId("seo-grid");
    expect(calculator.compareDocumentPosition(grid) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(within(grid).getAllByRole("link")).toHaveLength(12);
    expect(within(grid).getByRole("link", { name: /Life Path 33/ }).getAttribute("href")).toBe("/en/life-path-number/33");
  });

  it("emits parseable BreadcrumbList and FAQPage data matching the page", async () => {
    const page = compatibilityPage("en", "aries-and-leo")!;
    const { container } = await renderPage({ page, locale: "en", family: "compatibility", calculator: null });
    const [breadcrumbs, faq] = jsonLd(container);
    expect(breadcrumbs["@type"]).toBe("BreadcrumbList");
    expect(breadcrumbs.itemListElement.map((item: { item: string }) => item.item)).toEqual([
      "https://staryield.net/en",
      "https://staryield.net/en/compatibility",
      "https://staryield.net/en/compatibility/aries-and-leo",
    ]);
    expect(faq["@type"]).toBe("FAQPage");
    expect(faq.mainEntity.map((q: { name: string }) => q.name)).toEqual(page.faq.map((item) => item.q));
    // The scores are shown with text, not only dots.
    const indicators = screen.getByTestId("seo-indicators");
    expect(indicators.textContent).toContain("Love");
    expect(indicators.textContent).toContain("5 out of 5");
  });

  it("is accessible", async () => {
    const { container } = await renderPage({ page: compatibilityPage("fr", "cancer-and-pisces")!, locale: "fr", family: "compatibility", calculator: null });
    expect(await axeViolations(container)).toEqual([]);
  });
});
