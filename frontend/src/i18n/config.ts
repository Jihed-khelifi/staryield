export const locales = ["en", "fr", "es"] as const

export type Locale = (typeof locales)[number]

export const defaultLocale: Locale = "en"

export const localeLabels: Record<Locale, { name: string; short: string }> = {
  en: { name: "English", short: "EN" },
  fr: { name: "Français", short: "FR" },
  es: { name: "Español", short: "ES" },
}

export function isLocale(value: string): value is Locale {
  return (locales as readonly string[]).includes(value)
}
