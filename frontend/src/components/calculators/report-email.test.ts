import { describe, expect, it } from "vitest";

import { lastPromptedAt, mayPrompt, PROMPT_AFTER_MS, PROMPT_QUIET_MS, PROMPT_SCROLL_SHARE, rememberPrompt, reportInput, scrolledShare } from "@/components/calculators/report-email";
import type { NatalFormValues, SunSignFormValues } from "@/lib/schemas/calculator";

describe("what the report e-mail asks the API for", () => {
  const natal: NatalFormValues = {
    birth: { day: "14", month: "03", year: "1990", time: "08:45" },
    timeUnknown: false,
    place: { id: "gn:2988507", label: "Paris, France" },
    houseSystem: "whole_sign",
  };

  it("is the natal calculator's own request (the place's label never leaves the browser)", () => {
    expect(reportInput("natal-chart", natal)).toEqual({
      birth_date: "1990-03-14",
      birth_time: "08:45",
      place_id: "gn:2988507",
      house_system: "whole_sign",
      include: ["tarot"],
    });
    expect(reportInput("natal-chart", { ...natal, timeUnknown: true, place: null })).toEqual({
      birth_date: "1990-03-14",
      house_system: "whole_sign",
      include: ["tarot"],
    });
  });

  it("is the sun sign calculator's request, with the optional time and place", () => {
    const sun: SunSignFormValues = { birth: natal.birth, place: natal.place };
    expect(reportInput("astrology", sun)).toEqual({ birth_date: "1990-03-14", birth_time: "08:45", place_id: "gn:2988507" });
    expect(reportInput("astrology", { birth: { ...natal.birth, time: "" }, place: null })).toEqual({ birth_date: "1990-03-14" });
  });

  it("is the numerology request for the path of life, with the visitor's zone for 'today'", () => {
    const life = { birth: { day: "5", month: "9", year: "1987", time: "" } };
    expect(reportInput("path-of-life", life, "Europe/Paris")).toEqual({ birth_date: "1987-09-05", tz: "Europe/Paris" });
    expect(reportInput("path-of-life", life, "")).toEqual({ birth_date: "1987-09-05" });
  });
});

function memory(initial: Record<string, string> = {}) {
  const data = { ...initial };
  return {
    data,
    getItem: (k: string) => (k in data ? data[k] : null),
    setItem: (k: string, v: string) => {
      data[k] = v;
    },
  };
}

describe("when the dialog may open by itself", () => {
  it("opens for a browser that never saw it, once per calculator", () => {
    const store = memory();
    const now = Date.UTC(2026, 10, 19);
    expect(mayPrompt("natal-chart", now, store)).toBe(true);
    rememberPrompt("natal-chart", now, store);
    expect(store.data["staryield.report-email.natal-chart"]).toBe(String(now));
    expect(mayPrompt("natal-chart", now + 1000, store)).toBe(false);
    // another calculator is its own prompt
    expect(mayPrompt("astrology", now + 1000, store)).toBe(true);
    expect(mayPrompt("path-of-life", now + 1000, store)).toBe(true);
  });

  it("comes back 30 days after it was shown, not before", () => {
    const store = memory();
    const t0 = Date.UTC(2026, 10, 19);
    rememberPrompt("astrology", t0, store);
    expect(mayPrompt("astrology", t0 + PROMPT_QUIET_MS - 1, store)).toBe(false);
    expect(mayPrompt("astrology", t0 + PROMPT_QUIET_MS, store)).toBe(true);
    expect(PROMPT_QUIET_MS).toBe(30 * 24 * 3600 * 1000);
  });

  it("ignores a remembered time in the future (a clock that was wrong) and garbage", () => {
    const now = Date.UTC(2026, 10, 19);
    expect(mayPrompt("astrology", now, memory({ "staryield.report-email.astrology": String(now + 86400000) }))).toBe(true);
    for (const junk of ["", "abc", "-5", "0", "NaN"]) {
      expect(lastPromptedAt("astrology", memory({ "staryield.report-email.astrology": junk }))).toBeNull();
    }
  });

  it("behaves as 'never shown' when storage is blocked, and never throws", () => {
    const blocked = {
      getItem: () => {
        throw new DOMException("denied", "SecurityError");
      },
      setItem: () => {
        throw new DOMException("denied", "SecurityError");
      },
    };
    expect(mayPrompt("natal-chart", Date.now(), blocked)).toBe(true);
    expect(() => rememberPrompt("natal-chart", Date.now(), blocked)).not.toThrow();
  });

  it("uses the contract's 6 seconds and 60 %", () => {
    expect(PROMPT_AFTER_MS).toBe(6000);
    expect(PROMPT_SCROLL_SHARE).toBe(0.6);
  });
});

describe("scrolledShare", () => {
  const doc = (scrollHeight: number) => ({ documentElement: { scrollHeight } }) as unknown as Document;
  it("is the scrolled part of the scrollable height", () => {
    expect(scrolledShare(doc(2000), { scrollY: 600, innerHeight: 800 })).toBeCloseTo(0.5);
    expect(scrolledShare(doc(2000), { scrollY: 1200, innerHeight: 800 })).toBe(1);
    expect(scrolledShare(doc(2000), { scrollY: -20, innerHeight: 800 })).toBe(0);
  });
  it("is 0 when the page does not scroll (a short page never triggers it)", () => {
    expect(scrolledShare(doc(700), { scrollY: 0, innerHeight: 800 })).toBe(0);
    expect(scrolledShare(doc(800), { scrollY: 0, innerHeight: 800 })).toBe(0);
  });
});
