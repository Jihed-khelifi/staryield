/**
 * Validation of the public pricing catalog (`GET /pricing`, docs §24.1),
 * shared by the server render and the client's Retry. A malformed body is
 * "unavailable", never a guess: prices are not repaired or invented.
 */
import type { PackBadge, PricingPack, PricingSummary } from "@/lib/api/pricing";

const BADGE_TYPES = new Set<PackBadge["type"]>(["new_client_offer", "most_popular", "best_value", "save"]);
const isNumber = (value: unknown): value is number => typeof value === "number" && Number.isFinite(value);
const isString = (value: unknown): value is string => typeof value === "string";

function parseBadges(value: unknown): PackBadge[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((item): PackBadge[] => {
    const badge = item as Partial<PackBadge> | null;
    if (!badge || !BADGE_TYPES.has(badge.type as PackBadge["type"])) return [];
    if (badge.type === "save") return isNumber(badge.percent) && badge.percent >= 1 ? [{ type: "save", percent: Math.floor(badge.percent) }] : [];
    return [{ type: badge.type as PackBadge["type"] }];
  });
}

function parsePack(value: unknown): PricingPack | null {
  const p = value as Partial<PricingPack> | null;
  if (!p || typeof p !== "object") return null;
  if (!isString(p.id) || !isString(p.name) || !isString(p.currency)) return null;
  if (!isNumber(p.credits) || p.credits <= 0 || !isNumber(p.price_cents) || p.price_cents <= 0) return null;
  return {
    id: p.id,
    code: isString(p.code) ? p.code : "",
    kind: p.kind === "intro" ? "intro" : "standard",
    name: p.name,
    tagline: isString(p.tagline) ? p.tagline : "",
    label: isString(p.label) ? p.label : "",
    credits: p.credits,
    price_cents: p.price_cents,
    currency: p.currency,
    featured: p.featured === true,
    new_clients_only: p.new_clients_only === true,
    sort_order: isNumber(p.sort_order) ? p.sort_order : 0,
    badges: parseBadges(p.badges),
  };
}

/**
 * Narrow an unknown JSON body to a PricingSummary (null when malformed or
 * when there is nothing to sell: an empty catalog is "unavailable", not
 * "free"). Malformed packs are dropped; the order the API sent is kept.
 */
export function parsePricingSummary(body: unknown): PricingSummary | null {
  const s = body as Partial<PricingSummary> | null;
  if (!s || typeof s !== "object" || !Array.isArray(s.packs)) return null;
  if (!isString(s.currency)) return null;
  const rate = s.rate_range;
  if (!rate || !isNumber(rate.min) || !isNumber(rate.max) || rate.min <= 0 || rate.max < rate.min) return null;
  const trial = s.free_trial;
  if (!trial || typeof trial.enabled !== "boolean" || !isNumber(trial.minutes)) return null;
  const packs = s.packs.map(parsePack).filter((pack): pack is PricingPack => pack !== null);
  if (packs.length === 0) return null;
  const ref = s.reference;
  const reference = ref && isString(ref.pack_id) && isNumber(ref.credits) && isNumber(ref.price_cents) ? { pack_id: ref.pack_id, credits: ref.credits, price_cents: ref.price_cents } : undefined;
  return {
    currency: s.currency,
    free_trial: { enabled: trial.enabled, minutes: trial.minutes },
    rate_range: { min: rate.min, max: rate.max },
    reference,
    packs,
    generated_at: isString(s.generated_at) ? s.generated_at : "",
  };
}

