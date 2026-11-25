/**
 * Links from the search pages into the calculators. The birth date travels
 * in the URL fragment (`#v=1&d=YYYY-MM-DD`, see `lib/astro/fragment.ts`), so
 * it never reaches the server, its logs or the Referer header, and the
 * calculator validates it with the same schema as its own form.
 */
import { FRAGMENT_VERSION } from "@/lib/astro/fragment";

/** Calculators the embedded birth-date forms open. */
export type JumpCalculator = "path-of-life" | "tarot-birth-card" | "astrology";

/** `/{locale}/calculators/{kind}#v=1&d=YYYY-MM-DD`. */
export function calculatorResultHref(locale: string, calculator: JumpCalculator, isoDate: string): string {
  const params = new URLSearchParams({ v: FRAGMENT_VERSION, d: isoDate });
  return `/${locale}/calculators/${calculator}#${params.toString()}`;
}
