import type { CalculatorKind } from "@/lib/schemas/calculator";

/**
 * The link that goes with a shared result: the calculator's own page in the
 * viewer's language, never the fragment (it holds the birth data). This is the
 * one place that builds it, so a referral code can be appended later.
 */
export function calculatorShareUrl(kind: CalculatorKind, locale: string, origin?: string): string {
  const base = origin ?? (typeof window !== "undefined" ? window.location.origin : "https://staryield.net");
  return `${base.replace(/\/+$/, "")}/${locale}/calculators/${kind}`;
}
