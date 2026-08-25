"use client"

import * as React from "react"

import type { Locale } from "@/i18n/config"
import type { Dictionary, TranslationValues } from "@/i18n/types"

type Translate = (path: string, values?: TranslationValues) => string

type I18nContextValue = {
  locale: Locale
  dictionary: Dictionary
  t: Translate
}

const I18nContext = React.createContext<I18nContextValue | null>(null)

function resolve(dictionary: Dictionary, path: string): unknown {
  return path
    .split(".")
    .reduce<unknown>(
      (node, key) =>
        node && typeof node === "object"
          ? (node as Record<string, unknown>)[key]
          : undefined,
      dictionary
    )
}

function interpolate(template: string, values?: TranslationValues) {
  if (!values) return template
  return template.replace(/\{(\w+)\}/g, (match, key: string) =>
    key in values ? String(values[key]) : match
  )
}

/**
 * Reads a translation by dot path, e.g. `chatroom.composer.placeholder`.
 * When `values.count` is present, `<path>_one` / `<path>_other` are preferred
 * so plural forms stay in the dictionary rather than in components.
 */
function createTranslate(dictionary: Dictionary, locale: Locale): Translate {
  const plurals = new Intl.PluralRules(locale)

  return (path, values) => {
    if (values && typeof values.count === "number") {
      const rule = plurals.select(values.count)
      const plural =
        resolve(dictionary, `${path}_${rule}`) ??
        resolve(dictionary, `${path}_other`)
      if (typeof plural === "string") return interpolate(plural, values)
    }

    const value = resolve(dictionary, path)
    return typeof value === "string" ? interpolate(value, values) : path
  }
}

export function I18nProvider({
  locale,
  dictionary,
  children,
}: {
  locale: Locale
  dictionary: Dictionary
  children: React.ReactNode
}) {
  const value = React.useMemo<I18nContextValue>(
    () => ({ locale, dictionary, t: createTranslate(dictionary, locale) }),
    [dictionary, locale]
  )

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n() {
  const context = React.useContext(I18nContext)
  if (!context) {
    throw new Error("useI18n must be used inside an <I18nProvider>")
  }
  return context
}
