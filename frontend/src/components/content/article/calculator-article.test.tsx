// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import { CalculatorArticle } from "@/components/content/article/calculator-article";
import type { Locale } from "@/i18n/config";

afterEach(cleanup);

const kinds = ["natal-chart", "path-of-life", "astrology"] as const;
const locales: Locale[] = ["en", "es", "fr"];

describe("CalculatorArticle", () => {
  for (const kind of kinds)
    for (const locale of locales)
      it(`${kind}/${locale}: renders a labelled section with unique ids and safe links`, () => {
        const { container } = render(<CalculatorArticle kind={kind} locale={locale} />);
        const section = container.querySelector("section[aria-label]");
        expect(section).not.toBeNull();
        const ids = [...container.querySelectorAll("[id]")].map((el) => el.id);
        expect(new Set(ids).size).toBe(ids.length);
        expect(container.querySelectorAll("h2, h3").length).toBeGreaterThan(2);
        for (const anchor of container.querySelectorAll("a"))
          expect(anchor.getAttribute("href")).toMatch(/^(#|\/(en|es|fr)\/|mailto:)/);
        for (const img of container.querySelectorAll("img")) {
          expect(img.getAttribute("src")).toMatch(/^https:\/\/assets\.staryield\.net\/assets\//);
          expect(img.getAttribute("width")).toBeTruthy();
          expect(img.getAttribute("height")).toBeTruthy();
          expect(img.hasAttribute("alt")).toBe(true);
        }
      });

  it("renders the Ascendant formula as MathML with an accessible label", () => {
    const { container } = render(<CalculatorArticle kind="astrology" locale="en" />);
    const math = container.querySelector("math");
    expect(math).not.toBeNull();
    expect(math!.querySelector("mfrac")).not.toBeNull();
    expect(math!.textContent).toContain("RAMC");
    expect(
      screen.getByRole("img", { name: /tangent of the Ascendant equals minus the cosine of RAMC/ }),
    ).toBeTruthy();
  });

  it("renders FAQs as accordions: question buttons that reveal the answer", () => {
    const { container } = render(<CalculatorArticle kind="path-of-life" locale="es" />);
    const triggers = container.querySelectorAll("[data-testid=article-faq] button[aria-expanded]");
    expect(triggers).toHaveLength(3);
    for (const trigger of triggers) {
      expect(trigger.closest("h3")).not.toBeNull();
      expect(trigger.textContent?.trim()).toBeTruthy();
      expect(trigger.getAttribute("aria-expanded")).toBe("false");
    }
    fireEvent.click(triggers[0]);
    expect(triggers[0].getAttribute("aria-expanded")).toBe("true");
    const panel = container.querySelector(`#${CSS.escape(triggers[0].getAttribute("aria-controls")!)}`);
    expect(panel?.textContent?.trim()).toBeTruthy();
  });

  it("renders the natal history timeline, both images and the twelve houses", () => {
    const { container } = render(<CalculatorArticle kind="natal-chart" locale="fr" />);
    const timeline = container.querySelector("figure[aria-label]");
    expect(timeline?.querySelectorAll("li")).toHaveLength(4);
    const informative = [...container.querySelectorAll("img")].filter((img) => img.getAttribute("alt"));
    expect(informative).toHaveLength(2);
    const houses = [...container.querySelectorAll("ul li strong")].filter((el) => /^Maison \d+$/.test(el.textContent ?? ""));
    expect(houses).toHaveLength(12);
  });

  it("prefixes heading anchors with the article kind", () => {
    const { container } = render(<CalculatorArticle kind="astrology" locale="en" />);
    expect(container.querySelector("#astrology-faq")).not.toBeNull();
    expect(container.querySelector("#astrology-house-systems")).not.toBeNull();
  });
});
