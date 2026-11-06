/**
 * Advisor settings forms (Figma 1219:1284): the advisor profile (phone,
 * About, Modalities) and the Customs offering. Limits mirror the API
 * (backend/internal/service/psychics.go profileChanges and
 * advisor_settings.go); the API re-validates everything.
 */
import * as yup from "yup"

import type { TranslateCopy } from "@/i18n/translate-copy"
import type { AdvisorSettings, AdvisorSettingsPatch, CustomService, CustomServiceInput, CustomServiceLimits } from "@/lib/api/types"
import { phoneE164Rule, trimmedText } from "@/lib/schemas/common"

export const ABOUT_MAX_PARAGRAPHS = 6
export const ABOUT_PARAGRAPH_MAX = 2000
export const MODALITIES_MAX = 8
export const MODALITY_MAX = 40

/** Server defaults, used until GET /psychic/service answers. */
export const DEFAULT_CUSTOM_LIMITS: CustomServiceLimits = {
  name_max: 80,
  description_max: 1000,
  min_price_credits: 10,
  max_price_credits: 50_000,
  min_delivery_days: 1,
  max_delivery_days: 60,
}

const chars = (value: string) => Array.from(value).length

/** The About text as paragraphs: blank lines separate them, empty ones are dropped. */
export function aboutParagraphs(text: string): string[] {
  return text
    .replace(/\r\n?/g, "\n")
    .split(/\n[ \t]*\n/)
    .map((paragraph) => paragraph.trim())
    .filter(Boolean)
}

/** Paragraphs back to the textarea's text. */
export function aboutText(paragraphs: readonly string[]): string {
  return paragraphs.join("\n\n")
}

export type AdvisorProfileValues = {
  phone: string
  about: string
  modalities: string[]
}

export const advisorProfileSchema = (text: TranslateCopy) =>
  yup.object({
    phone: phoneE164Rule(text),
    about: yup
      .string()
      .default("")
      .test(
        "paragraphs",
        text("Use at most {value0} paragraphs.", { value0: ABOUT_MAX_PARAGRAPHS }),
        (value) => aboutParagraphs(value ?? "").length <= ABOUT_MAX_PARAGRAPHS
      )
      .test(
        "paragraph-length",
        text("Keep each paragraph under {value0} characters.", { value0: ABOUT_PARAGRAPH_MAX.toLocaleString("en") }),
        (value) => aboutParagraphs(value ?? "").every((paragraph) => chars(paragraph) <= ABOUT_PARAGRAPH_MAX)
      ),
    modalities: yup
      .array(yup.string().defined())
      .default([])
      .max(MODALITIES_MAX, text("Add up to {value0} modalities.", { value0: MODALITIES_MAX }))
      .test(
        "modality-length",
        text("Each modality must be between 1 and {value0} characters.", { value0: MODALITY_MAX }),
        (list) => (list ?? []).every((item) => chars(item.trim()) >= 1 && chars(item.trim()) <= MODALITY_MAX)
      ),
  })

/** Form values for the settings the API returned. */
export function advisorProfileValues(settings: Pick<AdvisorSettings, "phone" | "about" | "modalities">): AdvisorProfileValues {
  return { phone: settings.phone ?? "", about: aboutText(settings.about ?? []), modalities: [...(settings.modalities ?? [])] }
}

function sameList(a: readonly string[], b: readonly string[]) {
  return a.length === b.length && a.every((item, i) => item === b[i])
}

/**
 * The PATCH /psychic/settings body for the fields that changed since
 * `saved`, with the version the form was loaded at. Null when nothing
 * changed. Values are the schema's output (phone separators removed).
 */
export function advisorSettingsPatch(values: AdvisorProfileValues, saved: AdvisorSettings): AdvisorSettingsPatch | null {
  const patch: AdvisorSettingsPatch = {}
  const phone = values.phone.trim()
  if (phone !== saved.phone) patch.phone = phone
  const about = aboutParagraphs(values.about)
  if (!sameList(about, saved.about ?? [])) patch.about = about
  const modalities = values.modalities.map((item) => item.trim().replace(/\s+/g, " ")).filter(Boolean)
  if (!sameList(modalities, saved.modalities ?? [])) patch.modalities = modalities
  if (Object.keys(patch).length === 0) return null
  return { ...patch, expected_version: saved.version }
}

export type CustomServiceValues = {
  name: string
  price: string
  days: string
  description: string
}

export const emptyCustomService: CustomServiceValues = { name: "", price: "", days: "", description: "" }

/** Yup context: the server's bounds and a number formatter for messages. */
export type CustomServiceContext = {
  limits?: CustomServiceLimits | null
  format?: (value: number) => string
}

function contextOf(options: { context?: unknown }): { limits: CustomServiceLimits; format: (value: number) => string } {
  const ctx = (options.context as CustomServiceContext | undefined) ?? {}
  return { limits: ctx.limits ?? DEFAULT_CUSTOM_LIMITS, format: ctx.format ?? String }
}

const WHOLE = /^\d+$/

export const customServiceSchema = (text: TranslateCopy) =>
  yup.object({
    name: trimmedText(text, { max: DEFAULT_CUSTOM_LIMITS.name_max }),
    price: yup
      .string()
      .trim()
      .required(text("Enter a price in credits."))
      .test("whole", text("Enter a whole number of credits."), (value) => !value || WHOLE.test(value))
      .test("bounds", function (value) {
        if (!value || !WHOLE.test(value)) return true
        const { limits, format } = contextOf(this.options)
        const n = Number(value)
        if (n >= limits.min_price_credits && n <= limits.max_price_credits) return true
        return this.createError({
          message: text("Use a price between {value0} and {value1} credits.", {
            value0: format(limits.min_price_credits),
            value1: format(limits.max_price_credits),
          }),
        })
      }),
    days: yup
      .string()
      .trim()
      .required(text("Enter the average delivery time in days."))
      .test("whole", text("Enter a whole number of days."), (value) => !value || WHOLE.test(value))
      .test("bounds", function (value) {
        if (!value || !WHOLE.test(value)) return true
        const { limits } = contextOf(this.options)
        const n = Number(value)
        if (n >= limits.min_delivery_days && n <= limits.max_delivery_days) return true
        return this.createError({
          message: text("Use between {value0} and {value1} days.", {
            value0: limits.min_delivery_days,
            value1: limits.max_delivery_days,
          }),
        })
      }),
    description: trimmedText(text, { max: DEFAULT_CUSTOM_LIMITS.description_max, required: false }),
  })

/** Form values for an existing offering (or an empty form). */
export function customServiceValues(service: CustomService | null): CustomServiceValues {
  if (!service) return { ...emptyCustomService }
  return {
    name: service.name,
    price: String(service.price_credits),
    days: String(service.delivery_days),
    description: service.description,
  }
}

/** The PUT /psychic/service body for validated values. */
export function toCustomServiceInput(values: CustomServiceValues): CustomServiceInput {
  const description = values.description.trim()
  return {
    name: values.name.trim().replace(/\s+/g, " "),
    price_credits: Number(values.price.trim()),
    delivery_days: Number(values.days.trim()),
    ...(description ? { description } : {}),
  }
}
