/**
 * Tarot birth cards (Mary K. Greer's method): the arithmetic lives in
 * `src/lib/calculators.ts` (import-free, like the Path of Life); this module
 * maps birth numbers to the Golden Dawn Major Arcana of the site's artwork
 * (`TAROT_CARDS`, R2 `assets/tarot/<slug>.png`).
 */
import { tarotBirthCards, type TarotBirthCards } from "@/lib/calculators";
import { TAROT_CARDS, tarotImage } from "@/lib/astro/zodiac";

/** The 22 Major Arcana in card-number order, 0 (The Fool) to 21 (The World). */
export const MAJOR_ARCANA: readonly string[] = [
  "the-fool",
  "the-magician",
  "the-high-priestess",
  "the-empress",
  "the-emperor",
  "the-hierophant",
  "the-lovers",
  "the-chariot",
  "strength",
  "the-hermit",
  "wheel-of-fortune",
  "justice",
  "the-hanged-man",
  "death",
  "temperance",
  "the-devil",
  "the-tower",
  "the-star",
  "the-moon",
  "the-sun",
  "judgement",
  "the-world",
];

/** Every card that can be a birth card, by birth number 1–22 (The Fool is 22). */
export const BIRTH_CARD_NUMBERS: readonly number[] = Array.from({ length: 22 }, (_, i) => i + 1);

export type BirthCard = {
  /** Birth number 1–22 (22 is The Fool). */
  number: number;
  /** Major Arcana number 0–21 (The Fool is 0). */
  arcana: number;
  slug: string;
  /** English display name (translate with `text()`). */
  name: string;
  /** Public artwork URL on R2. */
  image: string;
};

/** The card of a birth number (1–22); null outside that range. */
export function birthCard(number: number): BirthCard | null {
  if (!Number.isInteger(number) || number < 1 || number > 22) return null;
  const arcana = number === 22 ? 0 : number;
  const slug = MAJOR_ARCANA[arcana];
  return { number, arcana, slug, name: TAROT_CARDS[slug], image: tarotImage(slug) ?? "" };
}

/** The birth number of a card slug ("the-star" → 17, "the-fool" → 22); null for an unknown slug. */
export function birthNumberOfSlug(slug: string): number | null {
  const arcana = MAJOR_ARCANA.indexOf(slug);
  if (arcana < 0) return null;
  return arcana === 0 ? 22 : arcana;
}

/** Every birth number whose constellation contains a card ("the-sun": 19; "the-magician": 1, 10, 19). */
export function constellationOf(number: number): number[] {
  return BIRTH_CARD_NUMBERS.filter((n) => birthCardsOf(n).includes(number));
}

/** The cards that follow from a birth number (19 → [19, 10, 1]; 22 → [22, 4]; 5 → [5]). */
export function birthCardsOf(number: number): number[] {
  const cards = [number];
  if (number === 22) return [22, 4];
  let m = number;
  while (m >= 10) {
    m = String(m)
      .split("")
      .reduce((total, digit) => total + Number(digit), 0);
    cards.push(m);
  }
  return cards;
}

export type TarotBirthResult = TarotBirthCards & { birthCards: BirthCard[] };

/** The full result for a birth date ("YYYY-MM-DD"), or null for an invalid date string. */
export function tarotBirthResult(date: string): TarotBirthResult | null {
  const result = tarotBirthCards(date);
  if (!result) return null;
  return { ...result, birthCards: result.cards.map((n) => birthCard(n)).filter((c): c is BirthCard => c !== null) };
}
