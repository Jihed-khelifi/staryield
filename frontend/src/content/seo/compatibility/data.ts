/**
 * Language-independent facts of the sign compatibility pages: the 78 unordered
 * pairs, their canonical slugs, the angular relationship of the two signs and
 * the deterministic love / friendship / work indicators.
 *
 * The indicators are a traditional-astrology heuristic (aspect + element
 * harmony), not a measurement: the pages say so.
 */
import { locales } from "@/i18n/config";
import type { ZodiacSign } from "@/lib/api/astrology";
import { SIGNS, SIGN_FACTS } from "@/lib/astro/zodiac";
import type { Element, Modality, Planet } from "@/content/seo/names";

export const MODALITIES: Record<ZodiacSign, Modality> = {
  Aries: "Cardinal", Taurus: "Fixed", Gemini: "Mutable", Cancer: "Cardinal", Leo: "Fixed", Virgo: "Mutable",
  Libra: "Cardinal", Scorpio: "Fixed", Sagittarius: "Mutable", Capricorn: "Cardinal", Aquarius: "Fixed", Pisces: "Mutable",
};

/** Modern ruling planet of each sign (the one `SIGN_FACTS` uses). */
export function rulerOf(sign: ZodiacSign): Planet {
  return SIGN_FACTS[sign].ruler as Planet;
}

export function elementOf(sign: ZodiacSign): Element {
  return SIGN_FACTS[sign].element;
}

export type AspectKey = "same-sign" | "semi-sextile" | "sextile" | "square" | "trine" | "quincunx" | "opposition";

export const ASPECTS: readonly AspectKey[] = ["same-sign", "semi-sextile", "sextile", "square", "trine", "quincunx", "opposition"];

/** Angle between the signs' starting points, in degrees (0, 30, 60, 90, 120, 150, 180). */
export const ASPECT_DEGREES: Record<AspectKey, number> = {
  "same-sign": 0, "semi-sextile": 30, sextile: 60, square: 90, trine: 120, quincunx: 150, opposition: 180,
};

const BY_DISTANCE: AspectKey[] = ["same-sign", "semi-sextile", "sextile", "square", "trine", "quincunx", "opposition"];

/** The angular relationship of two signs (symmetric): 0–6 signs apart around the wheel. */
export function aspectBetween(a: ZodiacSign, b: ZodiacSign): AspectKey {
  const diff = Math.abs(SIGNS.indexOf(a) - SIGNS.indexOf(b));
  return BY_DISTANCE[Math.min(diff, 12 - diff)];
}

export type ElementPairKey = "Fire-Fire" | "Fire-Earth" | "Fire-Air" | "Fire-Water" | "Earth-Earth" | "Earth-Air" | "Earth-Water" | "Air-Air" | "Air-Water" | "Water-Water";
export type ModalityPairKey = "Cardinal-Cardinal" | "Cardinal-Fixed" | "Cardinal-Mutable" | "Fixed-Fixed" | "Fixed-Mutable" | "Mutable-Mutable";

const ELEMENT_ORDER: Element[] = ["Fire", "Earth", "Air", "Water"];
const MODALITY_ORDER: Modality[] = ["Cardinal", "Fixed", "Mutable"];

export function elementPairKey(a: Element, b: Element): ElementPairKey {
  const [first, second] = [a, b].sort((x, y) => ELEMENT_ORDER.indexOf(x) - ELEMENT_ORDER.indexOf(y));
  return `${first}-${second}` as ElementPairKey;
}

export function modalityPairKey(a: Modality, b: Modality): ModalityPairKey {
  const [first, second] = [a, b].sort((x, y) => MODALITY_ORDER.indexOf(x) - MODALITY_ORDER.indexOf(y));
  return `${first}-${second}` as ModalityPairKey;
}

export const ELEMENT_PAIRS: readonly ElementPairKey[] = [
  "Fire-Fire", "Fire-Earth", "Fire-Air", "Fire-Water", "Earth-Earth", "Earth-Air", "Earth-Water", "Air-Air", "Air-Water", "Water-Water",
];
export const MODALITY_PAIRS: readonly ModalityPairKey[] = [
  "Cardinal-Cardinal", "Cardinal-Fixed", "Cardinal-Mutable", "Fixed-Fixed", "Fixed-Mutable", "Mutable-Mutable",
];

export const SIGN_SLUGS: readonly string[] = SIGNS.map((sign) => sign.toLowerCase());

export function signOfSlug(slug: string): ZodiacSign | null {
  return SIGNS.find((sign) => sign.toLowerCase() === slug) ?? null;
}

/** The canonical slug of a pair: both signs in zodiac order, "aries-and-leo". */
export function pairSlug(a: ZodiacSign, b: ZodiacSign): string {
  const [first, second] = SIGNS.indexOf(a) <= SIGNS.indexOf(b) ? [a, b] : [b, a];
  return `${first.toLowerCase()}-and-${second.toLowerCase()}`;
}

export type SignPair = { slug: string; a: ZodiacSign; b: ZodiacSign };

/** A pair slug in either order; `canonical` is false for "leo-and-aries". */
export function parsePairSlug(slug: string): (SignPair & { canonical: boolean }) | null {
  const match = /^([a-z]+)-and-([a-z]+)$/.exec(slug);
  if (!match) return null;
  const a = signOfSlug(match[1]);
  const b = signOfSlug(match[2]);
  if (!a || !b) return null;
  const canonical = pairSlug(a, b);
  const [first, second] = SIGNS.indexOf(a) <= SIGNS.indexOf(b) ? [a, b] : [b, a];
  return { slug: canonical, a: first, b: second, canonical: canonical === slug };
}

/** The 78 unordered pairs (same-sign pairs included) in zodiac order. */
export const ALL_PAIRS: readonly SignPair[] = SIGNS.flatMap((a, i) => SIGNS.slice(i).map((b) => ({ slug: pairSlug(a, b), a, b })));

/** Every slug that has a page: the canonical slugs. */
export const PAIR_SLUGS: readonly string[] = ALL_PAIRS.map((pair) => pair.slug);

/** Both orders of every pair, for the redirecting route (`dynamicParams = false`). */
export const PAIR_SLUGS_BOTH_ORDERS: readonly string[] = [
  ...PAIR_SLUGS,
  ...ALL_PAIRS.filter((pair) => pair.a !== pair.b).map((pair) => `${pair.b.toLowerCase()}-and-${pair.a.toLowerCase()}`),
];

export type Indicator = 1 | 2 | 3 | 4 | 5;
export type Indicators = { love: Indicator; friendship: Indicator; work: Indicator };

/** Starting point of each angle (love, friendship, work), before the element bonus and the adjustments. */
const BASE: Record<AspectKey, Indicators> = {
  "same-sign": { love: 3, friendship: 4, work: 3 },
  "semi-sextile": { love: 2, friendship: 3, work: 2 },
  sextile: { love: 3, friendship: 3, work: 4 },
  square: { love: 2, friendship: 2, work: 3 },
  trine: { love: 4, friendship: 4, work: 3 },
  quincunx: { love: 2, friendship: 2, work: 2 },
  opposition: { love: 3, friendship: 2, work: 3 },
};

/** Elements that traditionally support one another (Fire with Air, Earth with Water) or match. */
export function elementsHarmonise(a: Element, b: Element): boolean {
  return a === b || (a === "Fire" && b === "Air") || (a === "Air" && b === "Fire") || (a === "Earth" && b === "Water") || (a === "Water" && b === "Earth");
}

function clamp(value: number): Indicator {
  return Math.max(1, Math.min(5, value)) as Indicator;
}

/**
 * Love, friendship and work indicators, from four traditional factors:
 *  1. the angle between the signs (the starting point),
 *  2. +1 to all three when the elements support one another (Fire/Air, Earth/Water or the same element),
 *  3. love +1 when a Water sign meets Water or Earth (a shared emotional language),
 *     friendship +1 when both signs share a ruling planet,
 *     work +1 for Cardinal with Fixed (one starts, the other sustains) and -1 for Fixed with Fixed or Mutable with Mutable.
 * Deterministic and symmetric: the same two signs always give the same answer.
 */
export function indicatorsFor(a: ZodiacSign, b: ZodiacSign): Indicators {
  const base = BASE[aspectBetween(a, b)];
  const [ea, eb] = [elementOf(a), elementOf(b)];
  const harmony = elementsHarmonise(ea, eb) ? 1 : 0;
  const emotional = (ea === "Water" || eb === "Water") && elementsHarmonise(ea, eb) ? 1 : 0;
  const sharedRuler = rulerOf(a) === rulerOf(b) ? 1 : 0;
  const modalities = modalityPairKey(MODALITIES[a], MODALITIES[b]);
  const workShift = modalities === "Cardinal-Fixed" ? 1 : modalities === "Fixed-Fixed" || modalities === "Mutable-Mutable" ? -1 : 0;
  return {
    love: clamp(base.love + harmony + emotional),
    friendship: clamp(base.friendship + harmony + sharedRuler),
    work: clamp(base.work + harmony + workShift),
  };
}

export type OverallLevel = "harmonious" | "good" | "mixed" | "demanding";

/** A plain-language overall level from the three indicators' average. */
export function overallLevel(indicators: Indicators): OverallLevel {
  const average = (indicators.love + indicators.friendship + indicators.work) / 3;
  if (average >= 4.3) return "harmonious";
  if (average >= 3.4) return "good";
  if (average >= 2.6) return "mixed";
  return "demanding";
}

/** Pairs to link from a pair page: each sign with the other sign's zodiac neighbours, without the page itself. */
export function relatedPairs(a: ZodiacSign, b: ZodiacSign): SignPair[] {
  const own = pairSlug(a, b);
  const around = (sign: ZodiacSign) => [SIGNS[(SIGNS.indexOf(sign) + 11) % 12], SIGNS[(SIGNS.indexOf(sign) + 1) % 12]];
  const candidates = [...around(b).map((other) => [a, other] as const), ...around(a).map((other) => [b, other] as const)];
  const seen = new Set<string>([own]);
  const result: SignPair[] = [];
  for (const [x, y] of candidates) {
    const slug = pairSlug(x, y);
    if (seen.has(slug)) continue;
    seen.add(slug);
    const parsed = parsePairSlug(slug)!;
    result.push({ slug, a: parsed.a, b: parsed.b });
  }
  return result;
}

const PAIR_PATH = new RegExp(`^(/(?:${locales.join("|")}))?/compatibility/([a-z]+-and-[a-z]+)/?$`);

/**
 * The canonical URL path of a reversed pair page, for a permanent redirect:
 * "/compatibility/leo-and-aries" → "/compatibility/aries-and-leo" (the locale
 * prefix, when present, is kept). Null for anything else, canonical pairs included.
 */
export function canonicalPairPath(pathname: string): string | null {
  const match = PAIR_PATH.exec(pathname);
  if (!match) return null;
  const parsed = parsePairSlug(match[2]);
  if (!parsed || parsed.canonical) return null;
  return `${match[1] ?? ""}/compatibility/${parsed.slug}`;
}
