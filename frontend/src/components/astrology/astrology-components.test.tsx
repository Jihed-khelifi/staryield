// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, screen } from "@testing-library/react";

import natal from "@/test/natal-fixture.json";
import type { NatalChart, NatalResult } from "@/lib/api/astrology";
import { NatalWheel } from "@/components/astrology/NatalWheel";
import { ZodiacWheel } from "@/components/astrology/ZodiacWheel";
import { SpreadCardList, TarotSpread } from "@/components/calculators/tarot-spread";
import { axeViolations, renderWithI18n } from "@/test/render";

afterEach(cleanup);
const result = natal as unknown as NatalResult;

describe("NatalWheel", () => {
  it("draws the chart from data with the Ascendant at 9 o'clock", async () => {
    const { container } = renderWithI18n(<NatalWheel chart={result.chart as NatalChart} />);
    const svg = screen.getByRole("img", { name: /Natal chart wheel/ });
    expect(Number(svg.getAttribute("data-ascendant"))).toBeCloseTo(result.chart.angles!.asc, 3);
    expect(container.querySelectorAll("[data-layer=cusps] line")).toHaveLength(12);
    expect(container.querySelectorAll("[data-layer=signs] [data-sign]")).toHaveLength(12);
    expect(container.querySelector('[data-body="Ascendant"]')!.getAttribute("data-angle")).toBe("0.000");
    expect(container.querySelectorAll("[data-layer=aspects] line").length).toBeGreaterThan(5);
    expect(container.querySelector("[data-retrograde]")).not.toBeNull(); // Pluto is retrograde in this chart
    expect(svg.querySelector("desc")!.textContent).toContain("Taurus");
    expect(await axeViolations()).toEqual([]);
  });
  it("shows no cusps without a birth time", () => {
    const noTime = { ...result.chart, angles: undefined, houses: undefined, time_known: false, rising_sign: undefined } as unknown as NatalChart;
    const { container } = renderWithI18n(<NatalWheel chart={noTime} />);
    expect(container.querySelectorAll("[data-layer=cusps] line")).toHaveLength(0);
    expect(screen.getByRole("img").querySelector("desc")!.textContent).toContain("0° Aries");
  });
});

describe("ZodiacWheel", () => {
  it("highlights the sun sign and flags the cusp alternative", async () => {
    const { container } = renderWithI18n(<ZodiacWheel sign="Gemini" alternatives={["Gemini", "Cancer"]} />);
    expect(container.querySelector('[data-highlighted] ')!.getAttribute("data-sign")).toBe("Gemini");
    expect(container.querySelector("[data-alternative]")!.getAttribute("data-sign")).toBe("Cancer");
    expect(screen.getByRole("img").querySelector("desc")!.textContent).toContain("counter-clockwise");
    expect(await axeViolations()).toEqual([]);
  });
  it("highlights nothing for an unknown sign", () => {
    const { container } = renderWithI18n(<ZodiacWheel sign="Ophiuchus" />);
    expect(container.querySelector("[data-highlighted]")).toBeNull();
  });
});

describe("TarotSpread", () => {
  it("draws twelve cards with house numbers and reverses retrograde ones", async () => {
    const spread = result.tarot!;
    const { container } = renderWithI18n(
      <>
        <TarotSpread spread={spread} />
        <SpreadCardList spread={spread} />
      </>,
    );
    const cards = container.querySelectorAll("[data-testid=tarot-spread] [data-card]");
    expect(cards).toHaveLength(12);
    expect([...cards].map((c) => c.getAttribute("data-house"))).toEqual(Array.from({ length: 12 }, (_, i) => String(i + 1)));
    const reversed = spread.cards.filter((c) => c.reversed).map((c) => String(c.house));
    expect([...container.querySelectorAll("[data-reversed]")].map((c) => c.getAttribute("data-house"))).toEqual(reversed);
    expect(container.querySelector("[data-house='7'] g[transform*='rotate(180)']")).not.toBeNull();
    const list = screen.getByRole("list", { name: "Your twelve cards" });
    expect(list.querySelectorAll("li")).toHaveLength(12);
    expect(list.textContent).toContain("Jupiter in Cancer, in the third house");
    expect(list.textContent).toContain("Cusp sign: Taurus (no planet in this house)");
    expect(list.textContent).toContain("Reversed");
    expect(container.querySelector("rect[rx]")!.getAttribute("rx")).toBe(String(11 * (105 / 210)));
    expect(await axeViolations()).toEqual([]);
  });
});
