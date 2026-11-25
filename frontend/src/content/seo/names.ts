/**
 * Names used by the programmatic pages, in every locale. Slugs stay English
 * everywhere (like the rest of the site's routes); only the display names change.
 *
 * Signs, elements, modalities, planets, Major Arcana and "Home" are not
 * written here: they come from the site's dictionaries (copy-{en,fr,es}.json,
 * the copy every other screen shows through `text()`), so a card or a sign is
 * named the same on a search page and in a calculator result. Only grammar
 * the dictionaries cannot give lives here (articles, "du Bélier").
 *
 * Server and content modules only: this file bundles the three dictionaries.
 * Client components translate the same names with `useI18n().text`.
 */
import { locales, type Locale } from "@/i18n/config";
import copyEn from "@/i18n/dictionaries/copy-en.json";
import copyEs from "@/i18n/dictionaries/copy-es.json";
import copyFr from "@/i18n/dictionaries/copy-fr.json";
import { createTranslateCopy, type TranslateCopy } from "@/i18n/translate-copy";
import type { ZodiacSign } from "@/lib/api/astrology";
import { SIGNS, TAROT_CARDS } from "@/lib/astro/zodiac";

export type Element = "Fire" | "Earth" | "Air" | "Water";
export type Modality = "Cardinal" | "Fixed" | "Mutable";
export type Planet = "Sun" | "Moon" | "Mercury" | "Venus" | "Mars" | "Jupiter" | "Saturn" | "Uranus" | "Neptune" | "Pluto";

/** The site's display-copy translator of each locale (English source → that locale). */
export const SEO_TEXT: Record<Locale, TranslateCopy> = {
  en: createTranslateCopy(copyEn),
  fr: createTranslateCopy(copyFr),
  es: createTranslateCopy(copyEs),
};

/** `{ locale: { key: text(source(key)) } }` for every locale. */
function translated<K extends string>(keys: readonly K[], source: (key: K) => string = (key) => key): Record<Locale, Record<K, string>> {
  return Object.fromEntries(
    locales.map((locale) => [locale, Object.fromEntries(keys.map((key) => [key, SEO_TEXT[locale](source(key))]))]),
  ) as Record<Locale, Record<K, string>>;
}

export const SIGN_NAMES: Record<Locale, Record<ZodiacSign, string>> = translated(SIGNS);

export const ELEMENT_NAMES: Record<Locale, Record<Element, string>> = translated(["Fire", "Earth", "Air", "Water"]);

export const MODALITY_NAMES: Record<Locale, Record<Modality, string>> = translated(["Cardinal", "Fixed", "Mutable"]);

const PLANETS: readonly Planet[] = ["Sun", "Moon", "Mercury", "Venus", "Mars", "Jupiter", "Saturn", "Uranus", "Neptune", "Pluto"];

/** The article the Sun and the Moon take inside a sentence ("the Sun", "le Soleil", "la Luna"). */
const PLANET_ARTICLE: Record<Locale, Partial<Record<Planet, string>>> = {
  en: { Sun: "the", Moon: "the" },
  fr: { Sun: "le", Moon: "la" },
  es: { Sun: "el", Moon: "la" },
};

/** Planet names as they read inside a sentence: "the Sun", "Vénus", "la Luna". */
export const PLANET_NAMES: Record<Locale, Record<Planet, string>> = Object.fromEntries(
  locales.map((locale) => [
    locale,
    Object.fromEntries(
      PLANETS.map((planet) => {
        const article = PLANET_ARTICLE[locale][planet];
        const name = SEO_TEXT[locale](planet);
        return [planet, article ? `${article} ${name}` : name];
      }),
    ),
  ]),
) as Record<Locale, Record<Planet, string>>;

/** Major Arcana display names by slug (the names `TAROT_CARDS` gives the rest of the site). */
export const CARD_NAMES: Record<Locale, Record<string, string>> = translated(Object.keys(TAROT_CARDS), (slug) => TAROT_CARDS[slug]);

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
export const SEO_COMMON: Record<Locale, { home: string }> = Object.fromEntries(
  locales.map((locale) => [locale, { home: SEO_TEXT[locale]("Home") }]),
) as Record<Locale, { home: string }>;

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
