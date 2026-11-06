import { describe, expect, it } from "vitest";

import en from "@/i18n/dictionaries/copy-en.json";
import es from "@/i18n/dictionaries/copy-es.json";
import fr from "@/i18n/dictionaries/copy-fr.json";
import { LIFE_PATH_COPY, NUMBER_CARDS } from "@/lib/astro/life-path-copy";
import { ASPECT_NAMES, HOUSE_KEYWORDS, HOUSE_LABELS, SIGN_INSIGHT, SIGNS, TAROT_CARDS, degreeInSign, signOfLongitude, tarotImage } from "@/lib/astro/zodiac";
import { aspectLineStyle } from "@/lib/astro/wheel-geometry";

describe("display copy exists in every locale", () => {
  const keys = [
    ...SIGNS, ...HOUSE_LABELS, ...HOUSE_KEYWORDS, ...Object.values(TAROT_CARDS), ...Object.values(ASPECT_NAMES), ...Object.values(SIGN_INSIGHT),
    ...Object.values(LIFE_PATH_COPY).flatMap((c) => [c.title, c.body]), ...NUMBER_CARDS.flatMap((c) => [c.label, c.body]),
    ...["square", "trine", "sextile"].map((t) => aspectLineStyle(t)!.label),
  ];
  for (const [name, dict] of [["en", en], ["es", es], ["fr", fr]] as const)
    it(name, () => {
      const known = new Set(Object.keys(dict).map((k) => k.trim().replace(/\s+/g, " ").toLowerCase()));
      expect(keys.filter((k) => !known.has(k.trim().replace(/\s+/g, " ").toLowerCase()))).toEqual([]);
    });
});

describe("zodiac helpers", () => {
  it("names the 22 Major Arcana and refuses unknown artwork", () => {
    expect(Object.keys(TAROT_CARDS)).toHaveLength(22);
    expect(tarotImage("the-sun")).toBe("https://assets.staryield.net/assets/tarot/the-sun.png");
    expect(tarotImage("../evil")).toBeNull();
  });
  it("maps longitudes", () => {
    expect(signOfLongitude(0)).toBe("Aries");
    expect(signOfLongitude(359.9)).toBe("Pisces");
    expect(degreeInSign(83.5)).toBe("23°30′");
  });
});
