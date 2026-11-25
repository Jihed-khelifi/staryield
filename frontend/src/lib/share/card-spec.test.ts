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
import { SHARE_DOMAIN, natalHeadline, shareCardSpec, sunCuspOf } from "@/lib/share/card-spec";
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
    expect(spec.headline).toBe("My tarot birth card is Wheel of Fortune");
    expect(spec.lines).toEqual(["Soul card: The Magician"]);
    expect(spec.artwork.map((a) => [a.url, a.caption])).toEqual([
      ["https://assets.staryield.net/assets/tarot/wheel-of-fortune.png", "X"],
      ["https://assets.staryield.net/assets/tarot/the-magician.png", "I"],
    ]);
    expect(spec.shareText).toBe(`My tarot birth card is Wheel of Fortune — find yours at ${SHARE_DOMAIN}`);
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

  // A cusp day without a birth time: the API's sign is the one at noon, which may not be the visitor's.
  const cuspChart = {
    ...natalResult.chart,
    time_known: false,
    rising_sign: undefined,
    sun_sign_uncertain: true,
    sun_sign_alternatives: ["Aquarius", "Pisces"],
    moon_sign_uncertain: true,
    moon_sign_alternatives: ["Libra", "Scorpio"],
  };
  const cuspSun = { sun_sign: "Pisces", uncertain: true, alternatives: ["Aquarius", "Pisces"], element: "Water", modality: "Mutable" };

  it("natal: an uncertain Sun names both signs, never the noon guess alone, on the headline and the image", () => {
    expect(natalHeadline(cuspChart, text)).toBe("Sun in Aquarius or Pisces");
    const spec = shareCardSpec({ kind: "natal-chart", natal: { ...natalResult, chart: cuspChart } }, text);
    expect(spec.headline).toBe("Sun in Aquarius or Pisces");
    expect(spec.shareText).toBe(`Sun in Aquarius or Pisces — find yours at ${SHARE_DOMAIN}`);
    expect(spec.artwork.map((a) => a.style)).toEqual(["emblem", "emblem"]);
    expect(spec.artwork[0].url).not.toBe(spec.artwork[1].url);
    expect(natalHeadline(cuspChart, createTranslateCopy(fr))).toBe("Soleil en Verseau ou en Poissons");
    expect(natalHeadline(cuspChart, createTranslateCopy(es))).toBe("Sol en Acuario o en Piscis");
  });

  it("astrology: a cusp result says so instead of claiming one sign, without an element or modality", () => {
    const spec = shareCardSpec({ kind: "astrology", sun: cuspSun }, text);
    expect(spec.headline).toBe("I was born on the Aquarius–Pisces cusp");
    expect(spec.lines).toEqual([]);
    expect(spec.artwork).toHaveLength(2);
    expect(spec.alt).toBe("Share image: I was born on the Aquarius–Pisces cusp");
    expect(shareCardSpec({ kind: "astrology", sun: cuspSun }, createTranslateCopy(fr)).headline).toBe("Ma naissance tombe sur la cuspide Verseau–Poissons");
    expect(shareCardSpec({ kind: "astrology", sun: cuspSun }, createTranslateCopy(es)).headline).toBe("Nací en la cúspide Acuario–Piscis");
  });

  it("sunCuspOf: both signs in the order the Sun crossed them, only when the API says the sign is uncertain", () => {
    expect(sunCuspOf(cuspChart)).toEqual(["Aquarius", "Pisces"]);
    expect(sunCuspOf(cuspSun)).toEqual(["Aquarius", "Pisces"]);
    expect(sunCuspOf(natalResult.chart)).toBeNull();
    expect(sunCuspOf({ uncertain: false, alternatives: ["Aquarius", "Pisces"] })).toBeNull();
    expect(sunCuspOf({ uncertain: true, alternatives: [] })).toBeNull();
    expect(sunCuspOf({ sun_sign_uncertain: true })).toBeNull();
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
    expect(spanish.headline).toBe("Soy Leo");
    expect(shareCardSpec(results["natal-chart"], createTranslateCopy(es)).headline).toBe("Sol en Piscis · Luna en Libra · Ascendente Tauro");
  });
});

describe("calculatorShareUrl", () => {
  it("links to the calculator in the viewer's language, never with the fragment", () => {
    expect(calculatorShareUrl("tarot-birth-card", "fr", { origin: "https://staryield.net/" })).toBe("https://staryield.net/fr/calculators/tarot-birth-card");
    expect(calculatorShareUrl("natal-chart", "en", { origin: "http://localhost:3001" })).not.toContain("#");
  });

  it("carries the sharer's invite code, and only a well-formed one", () => {
    expect(calculatorShareUrl("path-of-life", "es", { origin: "https://staryield.net", referralCode: "abcd2345" })).toBe(
      "https://staryield.net/es/calculators/path-of-life?ref=ABCD2345",
    );
    for (const bad of [null, undefined, "", "abc", "ABCD234O", "ABCD2345&x=1"])
      expect(calculatorShareUrl("path-of-life", "es", { origin: "https://staryield.net", referralCode: bad })).toBe(
        "https://staryield.net/es/calculators/path-of-life",
      );
  });
});
