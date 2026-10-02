export const locales = ["en", "fr", "es"] as const;

export type Locale = (typeof locales)[number];

export const defaultLocale: Locale = "en";

export const localeLabels: Record<Locale, { name: string; short: string }> = {
  en: { name: "English", short: "EN" },
  fr: { name: "Français", short: "FR" },
  es: { name: "Español", short: "ES" },
};

export function isLocale(value: string): value is Locale {
  return (locales as readonly string[]).includes(value);
}

/** Keep the page, query parameters, and section anchor when changing locale. */
export function localePath(href: string, locale: Locale): string {
  const path = href.startsWith("/") ? href : `/${href}`;
  const withoutLocale = path.replace(/^\/(en|fr|es)(?=\/|\?|#|$)/, "");
  return `/${locale}${withoutLocale === "/" ? "" : withoutLocale}`;
}
