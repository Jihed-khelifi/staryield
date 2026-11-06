import { describe, expect, it } from "vitest";

import registry from "@/data/r2-images.json";
import { planetIconUrl, planetIcons, zodiacIconUrl, zodiacIcons } from "@/lib/astro-icons";

const signs = ["aries", "taurus", "gemini", "cancer", "leo", "virgo", "libra", "scorpio", "sagittarius", "capricorn", "aquarius", "pisces"];
const bodies = ["sun", "moon", "mercury", "venus", "mars", "jupiter", "saturn", "uranus", "neptune", "pluto", "ascendant"];

describe("astro icons", () => {
  it("has a transparent SVG glyph for every sign and planet, registered in R2", () => {
    const urls = new Set((registry as { url: string }[]).map((e) => e.url));
    for (const s of signs) expect(urls.has(zodiacIcons[s]), s).toBe(true);
    for (const b of bodies) expect(urls.has(planetIcons[b]), b).toBe(true);
    for (const u of [...Object.values(zodiacIcons), ...Object.values(planetIcons)]) expect(u).toMatch(/^https:\/\/assets\.staryield\.net\/.+\.svg$/);
  });

  it("looks names up case-insensitively", () => {
    expect(zodiacIconUrl("Leo")).toBe(zodiacIcons.leo);
    expect(zodiacIconUrl(" SCORPIO ")).toBe(zodiacIcons.scorpio);
    expect(planetIconUrl("Ascendant")).toBe(planetIcons.ascendant);
    expect(zodiacIconUrl("Ophiuchus")).toBeUndefined();
    expect(zodiacIconUrl("")).toBeUndefined();
    expect(planetIconUrl(undefined)).toBeUndefined();
  });
});
