/**
 * Pure display copy for a credit pack (docs/platform-extensions.md §24.7–24.8).
 * Everything numeric comes from the API (`PricingPack`, `RateRange`); the
 * wording goes through the translator it is given.
 */
import type { PackBadge, PricingPack, RateRange } from "@/lib/api/pricing";
import { formatMinutes, packMinutes } from "@/lib/pricing";
import type { TranslateCopy } from "@/i18n/translate-copy";

const BADGE_SEPARATOR = " · ";

function badgeText(badge: PackBadge, text: TranslateCopy): string {
  switch (badge.type) {
    case "new_client_offer":
      return text("NEW CLIENT OFFER");
    case "most_popular":
      return text("MOST POPULAR");
    case "best_value":
      return text("BEST VALUE");
    case "save":
      return text("SAVE {percent}%", { percent: badge.percent ?? 0 });
  }
}

/** The badges joined with " · "; else the pack's label; else nothing. */
export function packEyebrow(pack: Pick<PricingPack, "badges" | "label">, text: TranslateCopy): string | null {
  if (pack.badges.length > 0) return pack.badges.map((badge) => badgeText(badge, text)).join(BADGE_SEPARATOR);
  return pack.label.trim() || null;
}

/** "≈ 10 min. at 30 credits/min. · ≈ 5 min. at 60 credits/min." (§24.7: at most two decimals, prefixed ≈). */
export function packMinutesLine(credits: number, range: RateRange, text: TranslateCopy, locale: string): string {
  return text("≈ {a} min. at {min} credits/min. · ≈ {b} min. at {max} credits/min.", {
    a: formatMinutes(packMinutes(credits, range.min), locale),
    min: range.min,
    b: formatMinutes(packMinutes(credits, range.max), locale),
    max: range.max,
  });
}

export function formatCredits(credits: number, locale: string): string {
  return new Intl.NumberFormat(locale).format(credits);
}
