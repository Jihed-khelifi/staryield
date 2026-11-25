import { describe, expect, it } from "vitest";

import { tarotBirthCards } from "@/lib/calculators";
import { BIRTH_CARD_NUMBERS, MAJOR_ARCANA, birthCard, birthCardsOf, birthNumberOfSlug, constellationOf, tarotBirthResult } from "@/lib/astro/tarot-birth-card";
import { TAROT_CARDS } from "@/lib/astro/zodiac";

describe("tarotBirthCards (Greer's method)", () => {
  it("adds month, day and year, then digits while above 22", () => {
    expect(tarotBirthCards("1990-11-29")).toEqual({ total: 2030, steps: ["2+0+3+0=5"], cards: [5] });
    expect(tarotBirthCards("1984-03-22")).toEqual({ total: 2009, steps: ["2+0+0+9=11"], cards: [11, 2] });
  });

  it("reduces twice when the first digit sum is still above 22", () => {
    // 12 + 31 + 1956 = 1999 → 28 → 10 → The Wheel of Fortune and The Magician.
    expect(tarotBirthCards("1956-12-31")).toEqual({ total: 1999, steps: ["1+9+9+9=28", "2+8=10"], cards: [10, 1] });
  });

  it("gives The Sun, the Wheel of Fortune and The Magician for 19", () => {
    // 1 + 3 + 1950 = 1954 → 1+9+5+4 = 19 → 1+9 = 10 → 1+0 = 1.
    expect(tarotBirthCards("1950-01-03")).toEqual({ total: 1954, steps: ["1+9+5+4=19"], cards: [19, 10, 1] });
    expect(birthCardsOf(19)).toEqual([19, 10, 1]);
    expect(tarotBirthCards("1976-12-31")).toEqual({ total: 2019, steps: ["2+0+1+9=12"], cards: [12, 3] });
  });

  it("pairs The Fool (22) with The Emperor", () => {
    // 9 + 9 + 1966 = 1984 → 1+9+8+4 = 22.
    expect(tarotBirthCards("1966-09-09")).toEqual({ total: 1984, steps: ["1+9+8+4=22"], cards: [22, 4] });
  });

  it("rejects malformed dates", () => {
    expect(tarotBirthCards("1990-1-1")).toBeNull();
    expect(tarotBirthCards("")).toBeNull();
  });

  it("keeps every result within 1–22 for a century of dates", () => {
    for (let year = 1920; year <= 2026; year++) {
      for (const [month, day] of [[1, 1], [6, 15], [9, 9], [12, 31]]) {
        const date = `${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
        const result = tarotBirthCards(date)!;
        expect(result.cards[0]).toBeGreaterThanOrEqual(1);
        expect(result.cards[0]).toBeLessThanOrEqual(22);
        expect(result.cards.at(-1)).toBeLessThanOrEqual(9);
      }
    }
  });
});

describe("birth card mapping", () => {
  it("covers the 22 Major Arcana of the site's artwork, in card order", () => {
    expect(MAJOR_ARCANA).toHaveLength(22);
    expect(new Set(MAJOR_ARCANA)).toEqual(new Set(Object.keys(TAROT_CARDS)));
    expect(BIRTH_CARD_NUMBERS).toHaveLength(22);
  });

  it("maps birth numbers to cards (22 is The Fool)", () => {
    expect(birthCard(17)).toMatchObject({ number: 17, arcana: 17, slug: "the-star", name: "The Star" });
    expect(birthCard(22)).toMatchObject({ number: 22, arcana: 0, slug: "the-fool", name: "The Fool" });
    expect(birthCard(8)?.slug).toBe("strength");
    expect(birthCard(11)?.slug).toBe("justice");
    expect(birthCard(17)?.image).toBe("https://assets.staryield.net/assets/tarot/the-star.png");
    expect(birthCard(0)).toBeNull();
    expect(birthCard(23)).toBeNull();
  });

  it("maps slugs back to birth numbers", () => {
    expect(birthNumberOfSlug("the-star")).toBe(17);
    expect(birthNumberOfSlug("the-fool")).toBe(22);
    expect(birthNumberOfSlug("nope")).toBeNull();
  });

  it("lists the constellations a card belongs to", () => {
    expect(constellationOf(1)).toEqual([1, 10, 19]);
    expect(constellationOf(4)).toEqual([4, 13, 22]);
    expect(constellationOf(19)).toEqual([19]);
  });

  it("builds the full result for a date", () => {
    const r = tarotBirthResult("1984-03-22")!;
    expect(r.birthCards.map((c) => c.slug)).toEqual(["justice", "the-high-priestess"]);
  });
});
