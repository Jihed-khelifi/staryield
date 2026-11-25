import { describe, expect, it } from "vitest";

import en from "@/i18n/dictionaries/copy-en.json";
import fr from "@/i18n/dictionaries/copy-fr.json";
import es from "@/i18n/dictionaries/copy-es.json";
import { createTranslateCopy } from "@/i18n/translate-copy";
import natal from "@/test/natal-fixture.json";
import type { NatalResult } from "@/lib/api/astrology";
import { LIFE_PATH_COPY } from "@/lib/astro/life-path-copy";
import { tarotBirthResult } from "@/lib/astro/tarot-birth-card";
import { lifePathReduction, personalCycles } from "@/lib/calculators";
import type { CalculatorResult } from "@/components/calculators/calculate";
import { SHARE_DOMAIN, natalHeadline, shareCardSpec } from "@/lib/share/card-spec";
import { calculatorShareUrl } from "@/lib/share/share-url";

const text = createTranslateCopy(en);
const natalResult = natal as unknown as NatalResult;

const results: Record<CalculatorResult["kind"], CalculatorResult> = {
  "tarot-birth-card": { kind: "tarot-birth-card", tarot: tarotBirthResult("1987-06-15")! },
  "path-of-life": { kind: "path-of-life", numbers: { lifePath: lifePathReduction("1990-03-14"), cycles: personalCycles("1990-03-14", "2026-11-25") } },
  astrology: { kind: "astrology", sun: { sun_sign: "Leo", uncertain: false, element: "Fire", modality: "Fixed" } },
  "natal-chart": { kind: "natal-chart", natal: natalResult },
};

// Everything personal the results above were computed from.
const PERSONAL = ["1987", "06-15", "1990-03-14", "1990", "08:45", "14/03", "Paris", "gn:", "2988507", natalResult.chart.instant];

describe("shareCardSpec", () => {
  it("tarot: the personality card in the headline, the soul card below, both artworks with numerals", () => {
    const spec = shareCardSpec(results["tarot-birth-card"], text);
    expect(spec.eyebrow).toBe("Tarot birth card");
    expect(spec.headline).toBe("My tarot birth card is Wheel Of Fortune");
    expect(spec.lines).toEqual(["Soul card: The Magician"]);
    expect(spec.artwork.map((a) => [a.url, a.caption])).toEqual([
      ["https://assets.staryield.net/assets/tarot/wheel-of-fortune.png", "X"],
      ["https://assets.staryield.net/assets/tarot/the-magician.png", "I"],
    ]);
    expect(spec.shareText).toBe(`My tarot birth card is Wheel Of Fortune — find yours at ${SHARE_DOMAIN}`);
    expect(spec.artwork.every((a) => a.style === "card")).toBe(true);
  });

  it("tarot: a single card, and The Fool shows 0", () => {
    const single = shareCardSpec({ kind: "tarot-birth-card", tarot: tarotBirthResult("1990-11-29")! }, text);
    expect(single.headline).toBe("My tarot birth card is The Hierophant");
    expect(single.lines).toEqual(["Personality and soul card in one"]);
    expect(single.artwork).toHaveLength(1);
    const fool = shareCardSpec({ kind: "tarot-birth-card", tarot: tarotBirthResult("1940-07-10")! }, text);
    expect(fool.headline).toBe("My tarot birth card is The Fool");
    expect(fool.artwork.map((a) => a.caption)).toEqual(["0", "IV"]);
  });

  it("path of life: the number and its title, as a large figure", () => {
    const spec = shareCardSpec(results["path-of-life"], text);
    expect(spec.headline).toBe("My life path number is 9");
    expect(spec.lines).toEqual([LIFE_PATH_COPY[9].title]);
    expect(spec.badge).toBe("9");
    expect(spec.artwork).toEqual([]);
  });

  it("astrology: the sign with its element and modality, and the sign's artwork", () => {
    const spec = shareCardSpec(results.astrology, text);
    expect(spec.headline).toBe("I'm a Leo");
    expect(spec.lines).toEqual(["Fire · Fixed"]);
    expect(spec.artwork).toEqual([{ url: expect.stringMatching(/^https:\/\/assets\.staryield\.net\//), aspect: expect.any(Number), style: "emblem" }]);
  });

  it("natal: sun, moon and rising, only the parts that are known", () => {
    expect(shareCardSpec(results["natal-chart"], text).headline).toBe("Sun in Pisces · Moon in Libra · Taurus rising");
    const chart = natalResult.chart;
    expect(natalHeadline({ ...chart, rising_sign: undefined }, text)).toBe("Sun in Pisces · Moon in Libra");
    expect(natalHeadline({ ...chart, rising_sign: undefined, moon_sign_uncertain: true }, text)).toBe("Sun in Pisces");
  });

  it("never carries a birth date, time, place or name", () => {
    for (const result of Object.values(results)) {
      const json = JSON.stringify(shareCardSpec(result, text));
      for (const value of PERSONAL) expect(json, `${result.kind}: ${value}`).not.toContain(value);
    }
  });

  it("is written in the viewer's language", () => {
    const french = shareCardSpec(results["tarot-birth-card"], createTranslateCopy(fr));
    expect(french.headline).toBe("Ma carte de naissance du tarot est La Roue de Fortune");
    expect(french.lines).toEqual(["Carte d’âme : Le Bateleur"]);
    const spanish = shareCardSpec(results.astrology, createTranslateCopy(es));
    expect(spanish.headline).toBe("Soy León");
    expect(shareCardSpec(results["natal-chart"], createTranslateCopy(es)).headline).toBe("Sol en Piscis · Luna en Libra · Ascendente Tauro");
  });
});

describe("calculatorShareUrl", () => {
  it("links to the calculator in the viewer's language, never with the fragment", () => {
    expect(calculatorShareUrl("tarot-birth-card", "fr", "https://staryield.net/")).toBe("https://staryield.net/fr/calculators/tarot-birth-card");
    expect(calculatorShareUrl("natal-chart", "en", "http://localhost:3001")).not.toContain("#");
  });
});
