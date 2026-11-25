import type { CalculatorKind } from "@/lib/schemas/calculator";
import { normalizeReferralCode, REFERRAL_PARAM } from "@/lib/referral";

export type ShareUrlOptions = {
  /** Defaults to the current origin (https://staryield.net on the server). */
  origin?: string;
  /** The sharer's invite code (growth: referrals); ignored unless well formed. */
  referralCode?: string | null;
};

/**
 * The link that goes with a shared result: the calculator's own page in the
 * viewer's language, never the fragment (it holds the birth data). This is the
 * one place that builds it. A signed-in client's link carries their invite
 * code (`?ref=`), so a friend who signs up from a shared result is counted as
 * invited.
 */
export function calculatorShareUrl(kind: CalculatorKind, locale: string, { origin, referralCode }: ShareUrlOptions = {}): string {
  const base = origin ?? (typeof window !== "undefined" ? window.location.origin : "https://staryield.net");
  const url = `${base.replace(/\/+$/, "")}/${locale}/calculators/${kind}`;
  const code = normalizeReferralCode(referralCode);
  return code ? `${url}?${REFERRAL_PARAM}=${encodeURIComponent(code)}` : url;
}
