import { describe, expect, it } from "vitest";

import en from "@/i18n/dictionaries/copy-en.json";
import fr from "@/i18n/dictionaries/copy-fr.json";
import es from "@/i18n/dictionaries/copy-es.json";
import { createTranslateCopy } from "@/i18n/translate-copy";
import { decodeFragment, encodeFragment } from "@/lib/astro/fragment";
import { BIRTH_CARD_NUMBERS, birthCard, tarotBirthResult } from "@/lib/astro/tarot-birth-card";
import { TAROT_BIRTH_COPY } from "@/lib/astro/tarot-birth-copy";
import { chainLabel, constellationPeers, tarotBirthSteps } from "@/lib/astro/tarot-birth-steps";

const text = createTranslateCopy(en);

describe("tarotBirthSteps", () => {
  it("writes the sum out as month + day + year, then every digit sum", () => {
    expect(tarotBirthSteps("1990-11-29", tarotBirthResult("1990-11-29")!)).toBe("11 + 29 + 1990 = 2030 → 2+0+3+0 = 5");
    expect(tarotBirthSteps("1985-07-04", tarotBirthResult("1985-07-04")!)).toBe("7 + 4 + 1985 = 1996 → 1+9+9+6 = 25 → 2+5 = 7");
    expect(tarotBirthSteps("1987-06-15", tarotBirthResult("1987-06-15")!)).toBe("6 + 15 + 1987 = 2008 → 2+0+0+8 = 10");
  });
});

describe("constellationPeers", () => {
  it("lists the other chains of the soul card, never the visitor's own nor 1", () => {
    expect(constellationPeers(19, 1).map(chainLabel)).toEqual(["10/1"]);
    expect(constellationPeers(10, 1).map(chainLabel)).toEqual(["19/10/1"]);
    expect(constellationPeers(5, 5).map(chainLabel)).toEqual(["14/5"]);
    expect(constellationPeers(14, 5).map(chainLabel)).toEqual(["5"]);
    expect(constellationPeers(22, 4).map(chainLabel)).toEqual(["4", "13/4"]);
    expect(constellationPeers(11, 2).map(chainLabel)).toEqual(["2", "20/2"]);
  });
});

describe("tarot birth card copy", () => {
  it("has keywords and a reading for each of the 22 cards, in English, French and Spanish", () => {
    const fromFr = createTranslateCopy(fr);
    const dictionaries: Record<string, string>[] = [en, fr, es];
    for (const n of BIRTH_CARD_NUMBERS) {
      const copy = TAROT_BIRTH_COPY[n];
      expect(copy, `card ${n}`).toBeDefined();
      for (const value of [copy.keywords, copy.meaning])
        for (const dictionary of dictionaries) expect(dictionary[value]?.trim(), `${n}: ${value}`).toBeTruthy();
      // The card's own name is translated too (the result and the share image use it).
      expect(text(birthCard(n)!.name)).toBeTruthy();
      expect(fromFr(birthCard(n)!.name)).not.toBe(birthCard(n)!.name);
    }
    // The meanings are real translations, not English copies.
    expect(fr[TAROT_BIRTH_COPY[17].meaning as keyof typeof fr]).toContain("espoir");
    expect(es[TAROT_BIRTH_COPY[17].meaning as keyof typeof es]).toContain("esperanza");
    // Two to three sentences per reading.
    for (const { meaning } of Object.values(TAROT_BIRTH_COPY)) expect(meaning.split(/(?<=\.)\s/).length).toBeGreaterThanOrEqual(2);
  });
});

describe("tarot birth card fragment", () => {
  it("round-trips the birth date as #v=1&d=YYYY-MM-DD and rejects anything else", () => {
    const values = { birth: { day: "15", month: "6", year: "1987", time: "" } };
    const hash = encodeFragment("tarot-birth-card", values);
    expect(hash).toBe("#v=1&d=1987-06-15");
    const decoded = decodeFragment("tarot-birth-card", hash, text);
    expect(decoded).toEqual({ status: "ok", values: { birth: { day: "15", month: "06", year: "1987", time: "" } } });
    expect(decodeFragment("tarot-birth-card", "#v=1&d=1987-06-15&p=gn:2988507", text)).toEqual({ status: "invalid", reason: "unknown_key" });
    expect(decodeFragment("tarot-birth-card", "#v=1&d=2999-01-01", text)).toEqual({ status: "invalid", reason: "schema" });
    expect(decodeFragment("tarot-birth-card", "#v=1&d=1990-02-30", text)).toEqual({ status: "invalid", reason: "schema" });
    expect(decodeFragment("tarot-birth-card", "", text)).toEqual({ status: "empty" });
  });
});
