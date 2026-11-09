import { describe, expect, it } from "vitest";

import { ApiError, errorCopy, errorMessage, unreachableCopy } from "@/lib/api/errors";
import { isTempNote, noteErrorCopy, noteErrorMessage, placementsFor, sinceLabel } from "@/components/chat/client-panel/panel-helpers";

describe("placementsFor", () => {
  it("uses the full natal list when the API sends one, rising sign after the Sun", () => {
    const list = placementsFor({
      astro: {
        sun_sign: "Aries",
        rising_sign: "Gemini",
        time_known: true,
        placements: [
          { body: "Sun", sign: "Aries", retrograde: false },
          { body: "Mercury", sign: "Pisces", retrograde: true },
          { body: "Chiron", sign: "Leo", retrograde: false },
        ],
      },
    });
    expect(list).toEqual([
      { body: "Sun", sign: "Aries", glyph: "☉", retrograde: false },
      { body: "Ascendant", sign: "Gemini", glyph: "↑" },
      { body: "Mercury", sign: "Pisces", glyph: "☿", retrograde: true },
      { body: "Chiron", sign: "Leo", glyph: "✦", retrograde: false },
    ]);
  });

  it("falls back to the Sun, rising and Moon signs", () => {
    expect(placementsFor({ astro: { sun_sign: "Aries", rising_sign: "Gemini", moon_sign: "Pisces", time_known: true } }).map((p) => [p.body, p.sign, p.glyph])).toEqual([
      ["Sun", "Aries", "☉"],
      ["Ascendant", "Gemini", "↑"],
      ["Moon", "Pisces", "☽"],
    ]);
    expect(placementsFor({ astro: { sun_sign: "Leo", time_known: false } })).toHaveLength(1);
    expect(placementsFor({})).toEqual([]);
    expect(placementsFor(null)).toEqual([]);
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
