/**
 * Language-independent facts of the tarot birth card pages: the 22 Major Arcana
 * slugs (from the calculator's own mapping), their numerals, the Golden Dawn
 * attributions and a real birth date that yields each card.
 */
import { MAJOR_ARCANA, birthCard, birthCardsOf, birthNumberOfSlug, constellationOf } from "@/lib/astro/tarot-birth-card";
import { romanNumeral } from "@/lib/calculators";

export const TAROT_SLUGS: readonly string[] = MAJOR_ARCANA;

export function isTarotSlug(slug: string): boolean {
  return TAROT_SLUGS.includes(slug);
}

/** Golden Dawn attribution of each card: a Hebrew letter and an element, planet or zodiac sign. */
export type Attribution =
  | { kind: "element"; value: "Air" | "Water" | "Fire" }
  | { kind: "planet"; value: "Sun" | "Moon" | "Mercury" | "Venus" | "Mars" | "Jupiter" | "Saturn" }
  | { kind: "sign"; value: "Aries" | "Taurus" | "Gemini" | "Cancer" | "Leo" | "Virgo" | "Libra" | "Scorpio" | "Sagittarius" | "Capricorn" | "Aquarius" | "Pisces" };

export const GOLDEN_DAWN: Record<string, { letter: string; attribution: Attribution }> = {
  "the-fool": { letter: "Aleph", attribution: { kind: "element", value: "Air" } },
  "the-magician": { letter: "Beth", attribution: { kind: "planet", value: "Mercury" } },
  "the-high-priestess": { letter: "Gimel", attribution: { kind: "planet", value: "Moon" } },
  "the-empress": { letter: "Daleth", attribution: { kind: "planet", value: "Venus" } },
  "the-emperor": { letter: "Heh", attribution: { kind: "sign", value: "Aries" } },
  "the-hierophant": { letter: "Vav", attribution: { kind: "sign", value: "Taurus" } },
  "the-lovers": { letter: "Zayin", attribution: { kind: "sign", value: "Gemini" } },
  "the-chariot": { letter: "Cheth", attribution: { kind: "sign", value: "Cancer" } },
  strength: { letter: "Teth", attribution: { kind: "sign", value: "Leo" } },
  "the-hermit": { letter: "Yod", attribution: { kind: "sign", value: "Virgo" } },
  "wheel-of-fortune": { letter: "Kaph", attribution: { kind: "planet", value: "Jupiter" } },
  justice: { letter: "Lamed", attribution: { kind: "sign", value: "Libra" } },
  "the-hanged-man": { letter: "Mem", attribution: { kind: "element", value: "Water" } },
  death: { letter: "Nun", attribution: { kind: "sign", value: "Scorpio" } },
  temperance: { letter: "Samekh", attribution: { kind: "sign", value: "Sagittarius" } },
  "the-devil": { letter: "Ayin", attribution: { kind: "sign", value: "Capricorn" } },
  "the-tower": { letter: "Peh", attribution: { kind: "planet", value: "Mars" } },
  "the-star": { letter: "Tzaddi", attribution: { kind: "sign", value: "Aquarius" } },
  "the-moon": { letter: "Qoph", attribution: { kind: "sign", value: "Pisces" } },
  "the-sun": { letter: "Resh", attribution: { kind: "planet", value: "Sun" } },
  judgement: { letter: "Shin", attribution: { kind: "element", value: "Fire" } },
  "the-world": { letter: "Tav", attribution: { kind: "planet", value: "Saturn" } },
};

/** A real birth date (YYYY-MM-DD) whose cards include each birth number (checked by the tests). */
export const TAROT_EXAMPLE_DATE: Record<number, string> = {
  1: "2002-02-13",
  2: "1975-05-20",
  3: "1983-03-24",
  4: "1990-05-16",
  5: "1993-05-14",
  6: "1992-07-23",
  7: "1990-08-16",
  8: "1988-09-27",
  9: "1986-11-10",
  10: "1984-12-12",
  11: "1979-09-21",
  12: "1997-09-13",
  13: "1989-12-28",
  14: "1999-12-28",
  15: "1939-01-10",
  16: "1948-02-10",
  17: "1959-01-10",
  18: "1968-02-10",
  19: "1979-01-10",
  20: "1979-02-10",
  21: "1981-01-10",
  22: "1981-02-10",
};

/** What the page of a card needs that does not depend on the language. */
export type TarotCardFacts = {
  slug: string;
  /** Major Arcana number 0–21 (The Fool is 0). */
  arcana: number;
  /** Birth number 1–22 (The Fool is 22). */
  birthNumber: number;
  /** "0" for The Fool, otherwise the Roman numeral. */
  numeral: string;
  image: string;
  letter: string;
  attribution: Attribution;
  exampleDate: string;
  /** This card as a personality card: the full chain ([19, 10, 1]) and what follows it. */
  chain: number[];
  /** Birth numbers whose chain contains this card (itself first). */
  constellation: number[];
};

export function tarotCardFacts(slug: string): TarotCardFacts | null {
  const number = birthNumberOfSlug(slug);
  const card = number === null ? null : birthCard(number);
  if (number === null || !card) return null;
  return {
    slug,
    arcana: card.arcana,
    birthNumber: number,
    numeral: card.arcana === 0 ? "0" : romanNumeral(card.arcana),
    image: card.image,
    letter: GOLDEN_DAWN[slug].letter,
    attribution: GOLDEN_DAWN[slug].attribution,
    exampleDate: TAROT_EXAMPLE_DATE[number],
    chain: birthCardsOf(number),
    constellation: constellationOf(number),
  };
}

/** The slug of a birth number 1–22. */
export function slugOfBirthNumber(number: number): string {
  return birthCard(number)?.slug ?? "";
}

/** Previous and next card in Major Arcana order (the Fool has only a next). */
export function tarotNeighbours(slug: string): string[] {
  const index = TAROT_SLUGS.indexOf(slug);
  return [TAROT_SLUGS[index - 1], TAROT_SLUGS[index + 1]].filter((item): item is string => Boolean(item));
}
