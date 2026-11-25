/**
 * Names used by the programmatic pages, in every locale. Slugs stay English
 * everywhere (like the rest of the site's routes); only the display names change.
 */
import type { Locale } from "@/i18n/config";
import type { ZodiacSign } from "@/lib/api/astrology";

export type Element = "Fire" | "Earth" | "Air" | "Water";
export type Modality = "Cardinal" | "Fixed" | "Mutable";
export type Planet = "Sun" | "Moon" | "Mercury" | "Venus" | "Mars" | "Jupiter" | "Saturn" | "Uranus" | "Neptune" | "Pluto";

export const SIGN_NAMES: Record<Locale, Record<ZodiacSign, string>> = {
  en: {
    Aries: "Aries", Taurus: "Taurus", Gemini: "Gemini", Cancer: "Cancer", Leo: "Leo", Virgo: "Virgo",
    Libra: "Libra", Scorpio: "Scorpio", Sagittarius: "Sagittarius", Capricorn: "Capricorn", Aquarius: "Aquarius", Pisces: "Pisces",
  },
  fr: {
    Aries: "Bélier", Taurus: "Taureau", Gemini: "Gémeaux", Cancer: "Cancer", Leo: "Lion", Virgo: "Vierge",
    Libra: "Balance", Scorpio: "Scorpion", Sagittarius: "Sagittaire", Capricorn: "Capricorne", Aquarius: "Verseau", Pisces: "Poissons",
  },
  es: {
    Aries: "Aries", Taurus: "Tauro", Gemini: "Géminis", Cancer: "Cáncer", Leo: "Leo", Virgo: "Virgo",
    Libra: "Libra", Scorpio: "Escorpio", Sagittarius: "Sagitario", Capricorn: "Capricornio", Aquarius: "Acuario", Pisces: "Piscis",
  },
};

export const ELEMENT_NAMES: Record<Locale, Record<Element, string>> = {
  en: { Fire: "Fire", Earth: "Earth", Air: "Air", Water: "Water" },
  fr: { Fire: "Feu", Earth: "Terre", Air: "Air", Water: "Eau" },
  es: { Fire: "Fuego", Earth: "Tierra", Air: "Aire", Water: "Agua" },
};

export const MODALITY_NAMES: Record<Locale, Record<Modality, string>> = {
  en: { Cardinal: "Cardinal", Fixed: "Fixed", Mutable: "Mutable" },
  fr: { Cardinal: "Cardinal", Fixed: "Fixe", Mutable: "Mutable" },
  es: { Cardinal: "Cardinal", Fixed: "Fijo", Mutable: "Mutable" },
};

export const PLANET_NAMES: Record<Locale, Record<Planet, string>> = {
  en: { Sun: "the Sun", Moon: "the Moon", Mercury: "Mercury", Venus: "Venus", Mars: "Mars", Jupiter: "Jupiter", Saturn: "Saturn", Uranus: "Uranus", Neptune: "Neptune", Pluto: "Pluto" },
  fr: { Sun: "le Soleil", Moon: "la Lune", Mercury: "Mercure", Venus: "Vénus", Mars: "Mars", Jupiter: "Jupiter", Saturn: "Saturne", Uranus: "Uranus", Neptune: "Neptune", Pluto: "Pluton" },
  es: { Sun: "el Sol", Moon: "la Luna", Mercury: "Mercurio", Venus: "Venus", Mars: "Marte", Jupiter: "Júpiter", Saturn: "Saturno", Uranus: "Urano", Neptune: "Neptuno", Pluto: "Plutón" },
};

/** Major Arcana display names by slug. */
export const CARD_NAMES: Record<Locale, Record<string, string>> = {
  en: {
    "the-fool": "The Fool", "the-magician": "The Magician", "the-high-priestess": "The High Priestess", "the-empress": "The Empress",
    "the-emperor": "The Emperor", "the-hierophant": "The Hierophant", "the-lovers": "The Lovers", "the-chariot": "The Chariot",
    strength: "Strength", "the-hermit": "The Hermit", "wheel-of-fortune": "Wheel of Fortune", justice: "Justice",
    "the-hanged-man": "The Hanged Man", death: "Death", temperance: "Temperance", "the-devil": "The Devil",
    "the-tower": "The Tower", "the-star": "The Star", "the-moon": "The Moon", "the-sun": "The Sun",
    judgement: "Judgement", "the-world": "The World",
  },
  fr: {
    "the-fool": "Le Mat", "the-magician": "Le Bateleur", "the-high-priestess": "La Papesse", "the-empress": "L’Impératrice",
    "the-emperor": "L’Empereur", "the-hierophant": "Le Pape", "the-lovers": "L’Amoureux", "the-chariot": "Le Chariot",
    strength: "La Force", "the-hermit": "L’Hermite", "wheel-of-fortune": "La Roue de Fortune", justice: "La Justice",
    "the-hanged-man": "Le Pendu", death: "La Mort", temperance: "La Tempérance", "the-devil": "Le Diable",
    "the-tower": "La Maison Dieu", "the-star": "L’Étoile", "the-moon": "La Lune", "the-sun": "Le Soleil",
    judgement: "Le Jugement", "the-world": "Le Monde",
  },
  es: {
    "the-fool": "El Loco", "the-magician": "El Mago", "the-high-priestess": "La Sacerdotisa", "the-empress": "La Emperatriz",
    "the-emperor": "El Emperador", "the-hierophant": "El Hierofante", "the-lovers": "Los Enamorados", "the-chariot": "El Carro",
    strength: "La Fuerza", "the-hermit": "El Ermitaño", "wheel-of-fortune": "La Rueda de la Fortuna", justice: "La Justicia",
    "the-hanged-man": "El Colgado", death: "La Muerte", temperance: "La Templanza", "the-devil": "El Diablo",
    "the-tower": "La Torre", "the-star": "La Estrella", "the-moon": "La Luna", "the-sun": "El Sol",
    judgement: "El Juicio", "the-world": "El Mundo",
  },
};

/** The sign after "of" in French: the contracted article it needs ("du Bélier", "de la Vierge", "des Gémeaux"). */
const SIGN_OF_FR: Record<ZodiacSign, string> = {
  Aries: "du Bélier", Taurus: "du Taureau", Gemini: "des Gémeaux", Cancer: "du Cancer", Leo: "du Lion", Virgo: "de la Vierge",
  Libra: "de la Balance", Scorpio: "du Scorpion", Sagittarius: "du Sagittaire", Capricorn: "du Capricorne", Aquarius: "du Verseau", Pisces: "des Poissons",
};

/** The sign after "of" in the language: "of Aries", "du Bélier", "de Aries". */
export function signOf(locale: Locale, sign: ZodiacSign): string {
  if (locale === "fr") return SIGN_OF_FR[sign];
  return `${locale === "es" ? "de" : "of"} ${SIGN_NAMES[locale][sign]}`;
}

/** Words every programmatic page shares (breadcrumbs). */
export const SEO_COMMON: Record<Locale, { home: string }> = {
  en: { home: "Home" },
  fr: { home: "Accueil" },
  es: { home: "Inicio" },
};

/** "3, 5 and 7" in the page's language. */
export function listOf(locale: Locale, items: string[]): string {
  return new Intl.ListFormat(locale, { style: "long", type: "conjunction" }).format(items);
}

/** "June 18, 1981" for an ISO date, in the page's language (UTC noon: no time-zone drift). */
export function longDate(locale: Locale, iso: string): string {
  return new Intl.DateTimeFormat(locale, { dateStyle: "long", timeZone: "UTC" }).format(new Date(`${iso}T12:00:00Z`));
}

/** "1+9+8+1=19" → "1 + 9 + 8 + 1 = 19". */
export function spacedSum(step: string): string {
  return step.replace(/\+/g, " + ").replace(/=/g, " = ");
}

/** "Label: value", with the space French typography puts before a colon. */
export function labelled(locale: Locale, label: string, value: string): string {
  return `${label}${locale === "fr" ? " :" : ":"} ${value}`;
}
