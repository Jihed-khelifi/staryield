import "server-only"

import type { Locale } from "@/i18n/config"
import type { Dictionary } from "@/i18n/types"

const dictionaries: Record<Locale, () => Promise<Dictionary>> = {
  en: () => import("@/i18n/dictionaries/en.json").then((m) => m.default),
  fr: () => import("@/i18n/dictionaries/fr.json").then((m) => m.default),
  es: () => import("@/i18n/dictionaries/es.json").then((m) => m.default),
}

export const getDictionary = async (locale: Locale): Promise<Dictionary> =>
  dictionaries[locale]()

export type { Dictionary }
