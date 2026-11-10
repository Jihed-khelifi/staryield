import { describe, expect, it } from "vitest";

import { ApiError, errorCopy, errorMessage, unreachableCopy } from "@/lib/api/errors";
import { isTempNote, noteErrorCopy, noteErrorMessage, placementsFor, sinceLabel } from "@/components/chat/client-panel/panel-helpers";

const placed = (list: string[], over: Record<string, unknown> = {}) => ({
  astro: {
    sun_sign: "Aries",
    time_known: true,
    placements: list.map((entry) => {
      const [body, sign] = entry.split(":");
      return { body, sign, retrograde: false };
    }),
    ...over,
  },
});
const bodies = (list: ReturnType<typeof placementsFor>) => list.map((p) => p.body);

describe("placementsFor (§23.5)", () => {
  const FULL = ["Pluto:Capricorn", "Neptune:Pisces", "Saturn:Leo", "Midheaven:Virgo", "Venus:Taurus", "Moon:Libra", "Sun:Aries", "Mars:Leo", "Uranus:Gemini", "Mercury:Pisces", "Jupiter:Aries", "Ascendant:Gemini", "Chiron:Leo"];

  it("returns fixed slots in order, three per row, whatever order the API lists them in", () => {
    const list = placementsFor(placed(FULL, { rising_sign: "Gemini", moon_sign: "Libra" }));
    expect(bodies(list)).toEqual(["Sun", "Moon", "Ascendant", "Mercury", "Venus", "Mars", "Jupiter", "Saturn", "Uranus", "Neptune", "Pluto"]);
    expect(list.map((p) => p.sign)).toEqual(["Aries", "Libra", "Gemini", "Pisces", "Taurus", "Leo", "Aries", "Leo", "Gemini", "Pisces", "Capricorn"]);
    expect(list[0]).toEqual({ body: "Sun", sign: "Aries", glyph: "☉", retrograde: false });
  });

  it("never shows the Midheaven (or other bodies)", () => {
    const list = placementsFor(placed(FULL));
    expect(bodies(list)).not.toContain("Midheaven");
    expect(bodies(list)).not.toContain("Chiron");
  });

  it("shows the Ascendant once when the API lists it and rising_sign repeats it", () => {
    const list = placementsFor(placed(["Sun:Aries", "Moon:Libra", "Ascendant:Gemini"], { rising_sign: "Gemini" }));
    expect(bodies(list).filter((b) => b === "Ascendant")).toHaveLength(1);
    expect(list.find((p) => p.body === "Ascendant")?.sign).toBe("Gemini");
  });

  it("places rising_sign in the Ascendant slot when the list has no entry for it", () => {
    const list = placementsFor(placed(["Sun:Aries", "Moon:Libra", "Mercury:Pisces"], { rising_sign: "Gemini" }));
    expect(bodies(list)).toEqual(["Sun", "Moon", "Ascendant", "Mercury"]);
    expect(list[2]).toEqual({ body: "Ascendant", sign: "Gemini", glyph: "↑" });
  });

  it("keeps the Moon and Ascendant slots as Unknown when the chart has no birth time", () => {
    const list = placementsFor(placed(["Sun:Aries", "Mercury:Pisces", "Venus:Taurus"], { time_known: false }));
    expect(bodies(list)).toEqual(["Sun", "Moon", "Ascendant", "Mercury", "Venus"]);
    expect(list[1]).toEqual({ body: "Moon", sign: "", glyph: "☽", unknown: true });
    expect(list[2]).toEqual({ body: "Ascendant", sign: "", glyph: "↑", unknown: true });
  });

  it("marks only the missing one: a Moon without a rising sign (and the reverse)", () => {
    const noRising = placementsFor(placed(["Sun:Aries", "Moon:Libra"]));
    expect(noRising.map((p) => [p.body, p.unknown ?? false])).toEqual([["Sun", false], ["Moon", false], ["Ascendant", true]]);
    const noMoon = placementsFor(placed(["Sun:Aries"], { rising_sign: "Gemini" }));
    expect(noMoon.map((p) => [p.body, p.unknown ?? false])).toEqual([["Sun", false], ["Moon", true], ["Ascendant", false]]);
  });

  it("does not invent slots for other planets the API did not send", () => {
    expect(bodies(placementsFor(placed(["Sun:Aries", "Moon:Libra", "Ascendant:Gemini"])))).toEqual(["Sun", "Moon", "Ascendant"]);
  });

  it("falls back to the Sun, Moon and rising signs of the summary, in that order", () => {
    expect(placementsFor({ astro: { sun_sign: "Aries", rising_sign: "Gemini", moon_sign: "Pisces", time_known: true } }).map((p) => [p.body, p.sign, p.glyph])).toEqual([
      ["Sun", "Aries", "☉"],
      ["Moon", "Pisces", "☽"],
      ["Ascendant", "Gemini", "↑"],
    ]);
  });

  it("summary only, no birth time: the Moon and Ascendant stay as Unknown", () => {
    const list = placementsFor({ astro: { sun_sign: "Leo", time_known: false } });
    expect(list.map((p) => [p.body, Boolean(p.unknown)])).toEqual([["Sun", false], ["Moon", true], ["Ascendant", true]]);
  });

  it("is empty without a chart", () => {
    expect(placementsFor({})).toEqual([]);
    expect(placementsFor(null)).toEqual([]);
    expect(placementsFor({ astro: { sun_sign: "", time_known: false } })).toEqual([]);
  });

  it("keeps the retrograde flag of listed bodies", () => {
    expect(placementsFor({ astro: { sun_sign: "Aries", time_known: true, placements: [{ body: "Mercury", sign: "Pisces", retrograde: true }] } }).find((p) => p.body === "Mercury")?.retrograde).toBe(true);
  });
});

describe("note errors", () => {
  it("explains the per-client limit and the first-session rule", () => {
    expect(noteErrorMessage(new ApiError({ code: "limit_reached", message: "x", status: 409 }))).toBe(noteErrorCopy.limit_reached);
    expect(noteErrorMessage(new ApiError({ code: "forbidden", message: "x", status: 403 }))).toBe(noteErrorCopy.forbidden);
  });

  it("uses the generic copy for anything else", () => {
    expect(noteErrorMessage(new TypeError("Failed to fetch"))).toBe(unreachableCopy);
    expect(noteErrorMessage(new ApiError({ code: "rate_limited", message: "x", status: 429 }))).toBe(errorCopy.rate_limited);
  });

  it("has generic copy for limit_reached elsewhere in the app", () => {
    expect(errorMessage(new ApiError({ code: "limit_reached", message: "server", status: 409 }))).toBe(errorCopy.limit_reached);
  });
});

describe("sinceLabel", () => {
  const now = Date.parse("2026-11-05T10:00:00Z");
  it("says just now for the first seconds", () => {
    expect(sinceLabel("2026-11-05T09:59:30Z", "en", now)).toEqual({ justNow: true, relative: "" });
  });
  it("gives a short relative time afterwards", () => {
    expect(sinceLabel("2026-11-05T09:58:00Z", "en", now)).toEqual({ justNow: false, relative: "2 min. ago" });
    expect(sinceLabel("2026-11-05T07:00:00Z", "fr", now).relative.replace(/\s/g, " ")).toBe("il y a 3 h");
  });
});

it("recognises optimistic notes", () => {
  expect(isTempNote("temp-3")).toBe(true);
  expect(isTempNote("6a1b0c39-0000-4000-8000-000000000000")).toBe(false);
});
