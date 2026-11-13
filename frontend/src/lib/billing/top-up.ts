/**
 * Pure rules of the top-up dialog (docs/platform-extensions.md §24.7): which
 * pack is preselected, the labels shown on a pack, the minute estimates and
 * how a failed purchase is told to the payer. No React here, so every rule is
 * unit-tested; the numbers come from the API, formatting from `lib/pricing`.
 */
import type { TranslateCopy } from "@/i18n/translate-copy";
import { errorMessage, type ApiError } from "@/lib/api/errors";
import type { OfferUnavailableReason, PackBadge, PricingPack, RateRange, TopUpCatalog } from "@/lib/api/pricing";
import { formatMinutes, packMinutes } from "@/lib/pricing";

/** Who the top-up is for when it opens from a live chat (their rate shapes the estimate). */
export type TopUpAdvisor = {
  advisorName?: string;
  /** Credits per minute the client pays this advisor. */
  ratePerMinute?: number;
};

export function isIntroPack(pack: PricingPack): boolean {
  return pack.kind === "intro";
}

/**
 * The pack selected before the payer chooses: the eligible Welcome Pack, else
 * the first standard pack. `skipIntro` drops the offer after the API refused
 * it (`offer_unavailable`).
 */
export function defaultPack(catalog: Pick<TopUpCatalog, "packs" | "welcome_offer">, skipIntro = false): PricingPack | null {
  const { packs, welcome_offer } = catalog;
  if (!skipIntro && welcome_offer.eligible) {
    const intro = packs.find((p) => (welcome_offer.pack_id ? p.id === welcome_offer.pack_id : isIntroPack(p)));
    if (intro) return intro;
  }
  return packs.find((p) => !isIntroPack(p)) ?? packs[0] ?? null;
}

/** The standard packs in the order the API sent them (the intro pack is shown apart). */
export function standardPacks(packs: readonly PricingPack[]): PricingPack[] {
  return packs.filter((p) => !isIntroPack(p));
}

function badgeLabel(badge: PackBadge, text: TranslateCopy): string {
  switch (badge.type) {
    case "new_client_offer":
      return text("New client offer");
    case "most_popular":
      return text("Most popular");
    case "best_value":
      return text("Best value");
    case "save":
      return text("Save {value0}%", { value0: badge.percent ?? 0 });
  }
}

/** "Most popular · Save 11%", else the pack's own label, else "One-time pack" (shown in capitals by CSS). */
export function packEyebrow(pack: Pick<PricingPack, "badges" | "label">, text: TranslateCopy): string {
  const badges = pack.badges.map((b) => badgeLabel(b, text));
  if (badges.length) return badges.join(" · ");
  return pack.label || text("One-time pack");
}

/** "≈ 22.5 min. at 30 credits/min. · ≈ 11.25 min. at 60 credits/min." */
export function minutesRange(credits: number, range: RateRange, locale: string, text: TranslateCopy): string {
  const at = (rate: number) =>
    text("≈ {value0} min. at {value1} credits/min.", { value0: formatMinutes(packMinutes(credits, rate), locale), value1: rate });
  return range.min === range.max ? at(range.min) : `${at(range.min)} · ${at(range.max)}`;
}

/** The dock sentence for a live reading, or null without a usable rate. */
export function advisorEstimate(credits: number, context: TopUpAdvisor | undefined, locale: string, text: TranslateCopy): string | null {
  const rate = context?.ratePerMinute;
  const name = context?.advisorName?.trim();
  if (!name || !rate || rate <= 0) return null;
  return text("At {value0}'s rate of {value1} credits/minute, this pack provides approximately {value2} minutes.", {
    value0: name,
    value1: rate,
    value2: formatMinutes(packMinutes(credits, rate), locale),
  });
}

/** Why the welcome offer cannot be bought (English source copy, translate with `text()`). */
export const OFFER_UNAVAILABLE_COPY: Record<OfferUnavailableReason, string> = {
  already_redeemed: "You have already used the Welcome Pack. Choose another pack to continue.",
  not_new_client: "The Welcome Pack is for new clients only. Choose another pack to continue.",
  payment_method_used: "The Welcome Pack was already used with this payment method. Choose another pack to continue.",
  card_check_unavailable: "We couldn't verify this card for the Welcome Pack. Choose another pack to continue.",
  no_offer: "The Welcome Pack is no longer available. Choose another pack to continue.",
};

const GENERIC_OFFER_UNAVAILABLE = "The Welcome Pack is no longer available. Choose another pack to continue.";

export function offerUnavailableMessage(error: ApiError): string {
  const reason = error.details?.reason;
  return (typeof reason === "string" && OFFER_UNAVAILABLE_COPY[reason as OfferUnavailableReason]) || GENERIC_OFFER_UNAVAILABLE;
}

/** What a failed purchase attempt asks of the top-up dialog. */
export type PurchaseFailure =
  /** Declined: nothing was charged; the next try is a new attempt. */
  | { kind: "declined"; message: string }
  /** The welcome offer was refused: back to the packs, reload them. */
  | { kind: "offer"; message: string }
  /** The outcome is unknown (network, timeout, 5xx): retry with the SAME attempt key. */
  | { kind: "retry"; message: string }
  /** Purchases are switched off: nothing to retry. */
  | { kind: "unavailable"; message: string }
  /** The age gate answers these; the global handler takes the payer away. */
  | { kind: "age_gate" }
  | { kind: "error"; message: string };

const RETRY_COPY = "We couldn't confirm your payment. Try again: you will not be charged twice.";

/**
 * Classify a purchase failure. `declineMessage` maps a decline reason to its
 * English copy (billing-format); the caller translates the returned message.
 */
export function classifyPurchaseFailure(error: ApiError, declineMessage: (reason: unknown) => string): PurchaseFailure {
  switch (error.code) {
    case "payment_failed":
      return { kind: "declined", message: declineMessage(error.details?.reason) };
    case "offer_unavailable":
      return { kind: "offer", message: offerUnavailableMessage(error) };
    case "feature_unavailable":
      return { kind: "unavailable", message: "Credit purchases are not available right now. Please try again later." };
    case "birth_date_required":
    case "age_restricted":
      return { kind: "age_gate" };
    case "network_error":
    case "timeout":
    case "service_unavailable":
      return { kind: "retry", message: RETRY_COPY };
  }
  if (error.status >= 500) return { kind: "retry", message: RETRY_COPY };
  return { kind: "error", message: errorMessage(error) };
}
