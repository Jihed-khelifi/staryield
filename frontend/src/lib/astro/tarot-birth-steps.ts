import { parseIsoDate, type TarotBirthCards } from "@/lib/calculators";
import { birthCardsOf, constellationOf } from "@/lib/astro/tarot-birth-card";

/**
 * "How it adds up" for a tarot birth card, written out:
 * "11 + 29 + 1990 = 2030 → 2+0+3+0 = 5" (month + day + year, then every digit
 * sum until the total is 22 or less). Numbers only, so it needs no translation.
 */
export function tarotBirthSteps(date: string, result: Pick<TarotBirthCards, "total" | "steps">): string {
  const parts = parseIsoDate(date);
  const sum = parts ? `${parts[1]} + ${parts[2]} + ${parts[0]} = ${result.total}` : String(result.total);
  return [sum, ...result.steps.map((step) => step.replace("=", " = "))].join(" → ");
}

/** A chain of birth numbers as written in tarot books: "19/10/1", "22/4", "5". */
export function chainLabel(cards: readonly number[]): string {
  return cards.join("/");
}

/**
 * The other chains of a constellation: every birth number whose chain ends in
 * `soul`, without the visitor's own (`first`) and without 1, which no date
 * can add up to (the smallest digit sum of month + day + year is 2).
 */
export function constellationPeers(first: number, soul: number): number[][] {
  return constellationOf(soul)
    .filter((n) => n !== first && n >= 2)
    .map((n) => birthCardsOf(n));
}
