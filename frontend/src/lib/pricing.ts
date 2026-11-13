/**
 * Display helpers shared by the public pricing page and the top-up dialog
 * (docs/platform-extensions.md §24.7). Pure functions: the numbers come from
 * the API (`PricingPack`, `RateRange`), formatting happens here once.
 */

/** Approximate reading minutes a pack buys at a rate: credits ÷ rate. */
export function packMinutes(credits: number, ratePerMinute: number): number {
  if (!Number.isFinite(credits) || !Number.isFinite(ratePerMinute) || ratePerMinute <= 0 || credits <= 0) return 0;
  return credits / ratePerMinute;
}

/**
 * Minutes with at most two decimals and no trailing zeros, localized
 * ("10.5", "5.25", "13.33"; "13,33" in French). Truncated, never rounded up,
 * so an estimate never promises more time than the credits buy.
 */
export function formatMinutes(minutes: number, locale: string): string {
  const truncated = Math.floor(Math.max(0, minutes) * 100 + 1e-9) / 100;
  return new Intl.NumberFormat(locale, { maximumFractionDigits: 2, minimumFractionDigits: 0 }).format(truncated);
}

/** "$10" for whole amounts, "$10.50" otherwise (USD and any other ISO currency). */
export function formatPackPrice(cents: number, currency: string, locale: string): string {
  const whole = cents % 100 === 0;
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
    minimumFractionDigits: whole ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(cents / 100);
}
