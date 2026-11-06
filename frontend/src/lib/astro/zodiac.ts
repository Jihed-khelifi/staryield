/**
 * Display data for the astrology screens: sign order, glyphs, tarot card
 * names and house labels. Every English string here is display copy that is
 * translated with `text()` (src/i18n/dictionaries/copy-*.json); the
 * `zodiac.test.ts` suite checks that every one exists in all three locales.
 */
import type { ZodiacSign } from "@/lib/api/astrology";

/** Tropical zodiac order: Aries starts at 0°. */
export const SIGNS: readonly ZodiacSign[] = [
  "Aries",
  "Taurus",
  "Gemini",
  "Cancer",
  "Leo",
  "Virgo",
  "Libra",
  "Scorpio",
  "Sagittarius",
  "Capricorn",
  "Aquarius",
  "Pisces",
];

export function signIndex(sign: string): number {
  return SIGNS.findIndex((s) => s.toLowerCase() === sign.trim().toLowerCase());
}

export function isSign(sign: string): sign is ZodiacSign {
  return signIndex(sign) >= 0;
}

/** Element and traditional ruler of each sign (for the sun-sign insight). */
export const SIGN_FACTS: Record<ZodiacSign, { element: "Fire" | "Earth" | "Air" | "Water"; ruler: string }> = {
  Aries: { element: "Fire", ruler: "Mars" },
  Taurus: { element: "Earth", ruler: "Venus" },
  Gemini: { element: "Air", ruler: "Mercury" },
  Cancer: { element: "Water", ruler: "Moon" },
  Leo: { element: "Fire", ruler: "Sun" },
  Virgo: { element: "Earth", ruler: "Mercury" },
  Libra: { element: "Air", ruler: "Venus" },
  Scorpio: { element: "Water", ruler: "Pluto" },
  Sagittarius: { element: "Fire", ruler: "Jupiter" },
  Capricorn: { element: "Earth", ruler: "Saturn" },
  Aquarius: { element: "Air", ruler: "Uranus" },
  Pisces: { element: "Water", ruler: "Neptune" },
};

/** One-line portrait of each sun sign (shown under the sun-sign result). */
export const SIGN_INSIGHT: Record<ZodiacSign, string> = {
  Aries: "This highlighted sign reveals a pioneering spark: direct, courageous and eager to begin.",
  Taurus: "This highlighted sign reveals a steady nature that seeks comfort, loyalty and lasting value.",
  Gemini: "This highlighted sign reveals your native energy flow, communicative versatility, and intellectual curiosity.",
  Cancer: "This highlighted sign reveals a protective, intuitive heart rooted in care and belonging.",
  Leo: "This highlighted sign reveals warmth, creativity and generous self-expression.",
  Virgo: "This highlighted sign reveals a precise, practical mind devoted to improvement and service.",
  Libra: "This highlighted sign reveals a search for harmony, fairness and meaningful partnership.",
  Scorpio: "This highlighted sign reveals depth, intensity and a quiet talent for transformation.",
  Sagittarius: "This highlighted sign reveals an adventurous, hopeful spirit searching for meaning.",
  Capricorn: "This highlighted sign reveals patience, discipline and a focus on what endures.",
  Aquarius: "This highlighted sign reveals an independent, inventive and community-minded spirit.",
  Pisces: "This highlighted sign reveals a gentle, imaginative and deeply empathetic soul.",
};

/** Uncoloured text presentation (U+FE0E) so no platform swaps in an emoji. */
const TEXT = "︎";

export const PLANET_GLYPHS: Record<string, string> = {
  Ascendant: "↑",
  Midheaven: "MC",
  Sun: `☉${TEXT}`,
  Moon: `☽${TEXT}`,
  Mercury: `☿${TEXT}`,
  Venus: `♀${TEXT}`,
  Mars: `♂${TEXT}`,
  Jupiter: `♃${TEXT}`,
  Saturn: `♄${TEXT}`,
  Uranus: `♅${TEXT}`,
  Neptune: `♆${TEXT}`,
  Pluto: `♇${TEXT}`,
};

export const SIGN_GLYPHS: Record<ZodiacSign, string> = {
  Aries: `♈${TEXT}`,
  Taurus: `♉${TEXT}`,
  Gemini: `♊${TEXT}`,
  Cancer: `♋${TEXT}`,
  Leo: `♌${TEXT}`,
  Virgo: `♍${TEXT}`,
  Libra: `♎${TEXT}`,
  Scorpio: `♏${TEXT}`,
  Sagittarius: `♐${TEXT}`,
  Capricorn: `♑${TEXT}`,
  Aquarius: `♒${TEXT}`,
  Pisces: `♓${TEXT}`,
};

/** The zodiac glyph artwork from the Figma "Instance / <Sign>" components (R2). */
export const SIGN_ARTWORK: Record<ZodiacSign, { src: string; width: number; height: number }> = {
  Aries: { src: "https://assets.staryield.net/assets/figma/9a293.svg", width: 50.65, height: 51 },
  Taurus: { src: "https://assets.staryield.net/assets/figma/1e525.svg", width: 45.45, height: 51 },
  Gemini: { src: "https://assets.staryield.net/assets/figma/dd3fa.svg", width: 47, height: 51 },
  Cancer: { src: "https://assets.staryield.net/assets/figma/6e31f.svg", width: 51, height: 42.51 },
  Leo: { src: "https://assets.staryield.net/assets/figma/c6980.svg", width: 37.4, height: 51 },
  Virgo: { src: "https://assets.staryield.net/assets/figma/341a8.svg", width: 40.02, height: 51 },
  Libra: { src: "https://assets.staryield.net/assets/figma/66a4a.svg", width: 51, height: 45.3 },
  Scorpio: { src: "https://assets.staryield.net/assets/figma/7f87b.svg", width: 42.58, height: 51 },
  Sagittarius: { src: "https://assets.staryield.net/assets/figma/87e4b.svg", width: 48.09, height: 51 },
  Capricorn: { src: "https://assets.staryield.net/assets/figma/12acb.svg", width: 50.3, height: 51 },
  Aquarius: { src: "https://assets.staryield.net/assets/figma/98d0a.svg", width: 51, height: 36.42 },
  Pisces: { src: "https://assets.staryield.net/assets/figma/2f624.svg", width: 38.56, height: 51 },
};

/** Planet order used by every list (traditional order, angles first). */
export const BODY_ORDER = [
  "Ascendant",
  "Sun",
  "Moon",
  "Mercury",
  "Venus",
  "Mars",
  "Jupiter",
  "Saturn",
  "Uranus",
  "Neptune",
  "Pluto",
  "Midheaven",
] as const;

/** English house labels the API uses ("First house" …), 1-based. */
export const HOUSE_LABELS = [
  "First house",
  "Second house",
  "Third house",
  "Fourth house",
  "Fifth house",
  "Sixth house",
  "Seventh house",
  "Eighth house",
  "Ninth house",
  "Tenth house",
  "Eleventh house",
  "Twelfth house",
] as const;

export function houseLabel(house: number): string {
  return HOUSE_LABELS[house - 1] ?? "";
}

/** A one-word theme per house (the spread's centre labels). */
export const HOUSE_KEYWORDS = [
  "Self",
  "Values",
  "Communication",
  "Home",
  "Creativity",
  "Health",
  "Partnership",
  "Transformation",
  "Beliefs",
  "Career",
  "Community",
  "Inner world",
] as const;

/** Golden Dawn Major Arcana artwork slugs → display names (R2: assets/tarot/<slug>.png). */
export const TAROT_CARDS: Record<string, string> = {
  "the-fool": "The Fool",
  "the-magician": "The Magician",
  "the-high-priestess": "The High Priestess",
  "the-empress": "The Empress",
  "the-emperor": "The Emperor",
  "the-hierophant": "The Hierophant",
  "the-lovers": "The Lovers",
  "the-chariot": "The Chariot",
  strength: "Strength",
  "the-hermit": "The Hermit",
  "wheel-of-fortune": "Wheel Of Fortune",
  justice: "Justice",
  "the-hanged-man": "The Hanged Man",
  death: "Death",
  temperance: "Temperance",
  "the-devil": "The Devil",
  "the-tower": "The Tower",
  "the-star": "The Star",
  "the-moon": "The Moon",
  "the-sun": "The Sun",
  judgement: "Judgement",
  "the-world": "The World",
};

/** Public R2 host (tarot artwork is registered in r2-images.json). */
const ASSET_HOST = "https://assets.staryield.net";

/** Artwork URL for a known card slug; anything else gets no image. */
export function tarotImage(slug: string): string | null {
  return Object.prototype.hasOwnProperty.call(TAROT_CARDS, slug) ? `${ASSET_HOST}/assets/tarot/${slug}.png` : null;
}

export const ASPECT_NAMES: Record<string, string> = {
  conjunction: "Conjunction",
  sextile: "Sextile",
  square: "Square",
  trine: "Trine",
  opposition: "Opposition",
};

/** Ordinal house names used inside sentences ("in the {value0}"). */
export function houseOrdinal(house: number): string {
  return houseLabel(house).toLowerCase();
}

/** "8°22′" from a longitude within its sign. */
export function degreeInSign(longitude: number): string {
  const within = ((longitude % 30) + 30) % 30;
  let degree = Math.floor(within);
  let minute = Math.round((within - degree) * 60);
  if (minute === 60) {
    degree += 1;
    minute = 0;
  }
  return `${degree}°${String(minute).padStart(2, "0")}′`;
}

/** The sign a longitude falls in. */
export function signOfLongitude(longitude: number): ZodiacSign {
  const lon = ((longitude % 360) + 360) % 360;
  return SIGNS[Math.floor(lon / 30) % 12];
}
