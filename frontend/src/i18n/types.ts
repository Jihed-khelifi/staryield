import type en from "@/i18n/dictionaries/en.json"

/** Shape of every dictionary — `en.json` is the source of truth. */
export type Dictionary = typeof en

export type TranslationValues = Record<string, string | number>
