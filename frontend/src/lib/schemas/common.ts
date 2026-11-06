/**
 * Reusable yup rules. Every rule is a factory taking the copy translator
 * (`text` from useI18n), so error messages are localized and checked by
 * `npm run test:i18n` like any other display copy.
 *
 * Usage:
 *   export const profileSchema = (text: TranslateCopy) =>
 *     yup.object({ email: emailRule(text), name: trimmedText(text, { max: 80 }) })
 */
import * as yup from "yup"

import type { TranslateCopy } from "@/i18n/translate-copy"
import { minimumAdultDate, validBirthDate } from "@/lib/calculators"

/** Same rule the API enforces: 8–128 chars, an uppercase letter, a digit and a symbol. */
export const PASSWORD_RULE = /^(?=.*[A-Z])(?=.*\d)(?=.*[^A-Za-z0-9]).{8,128}$/

/** E.164: "+", country code, up to 15 digits in total. */
export const E164_RULE = /^\+[1-9]\d{6,14}$/

export const TIME_RULE = /^([01]\d|2[0-3]):[0-5]\d$/

export function emailRule(text: TranslateCopy) {
  return yup
    .string()
    .trim()
    .required(text("Enter your email address."))
    .max(254, text("Use at most {value0} characters.", { value0: 254 }))
    .email(text("Enter a valid email address."))
}

/** New password (sign-up, reset, change). */
export function passwordRule(text: TranslateCopy) {
  return yup
    .string()
    .required(text("Enter your password."))
    .matches(PASSWORD_RULE, text("Use 8+ characters, an uppercase letter, a number, and a special character."))
}

/** "Confirm password" matching the sibling field `ref`. */
export function confirmPasswordRule(text: TranslateCopy, ref = "password") {
  return yup
    .string()
    .required(text("Enter your password."))
    .oneOf([yup.ref(ref)], text("The passwords do not match."))
}

type TextOptions = { min?: number; max: number; required?: boolean }

/** Trimmed free text with rune-length limits (matches the API's character counts). */
export function trimmedText(text: TranslateCopy, { min = 1, max, required = true }: TextOptions) {
  const base = yup
    .string()
    .trim()
    .test("max-chars", text("Use at most {value0} characters.", { value0: max }), (value) =>
      value === undefined || Array.from(value).length <= max
    )
  if (!required) return base.default("")
  return base
    .required(text("This field is required."))
    .test("min-chars", text("Use at least {value0} characters.", { value0: min }), (value) =>
      min <= 1 || value === undefined || Array.from(value).length >= min
    )
}

/** HH:MM, 24-hour clock. Empty is allowed unless `required`. */
export function time24Rule(text: TranslateCopy, required = false) {
  const rule = yup
    .string()
    .trim()
    .test("time24", text("Use the 24-hour format HH:MM."), (value) => !value || TIME_RULE.test(value))
  return required ? rule.required(text("Use the 24-hour format HH:MM.")) : rule.default("")
}

export type BirthParts = { day: string; month: string; year: string; time?: string }

/**
 * Birth date as separate DD / MM / YYYY parts (DatePartsInput). Rejects
 * impossible and future dates; `adult` additionally requires 18+.
 */
export function birthPartsRule(text: TranslateCopy, { adult = false }: { adult?: boolean } = {}) {
  return yup
    .object({
      day: yup.string().default(""),
      month: yup.string().default(""),
      year: yup.string().default(""),
      time: yup.string().default(""),
    })
    .test("complete", text("Enter your date of birth."), (value) =>
      Boolean(value?.day && value?.month && value?.year)
    )
    .test("valid", text("Enter a valid date."), (value) => {
      if (!value?.day || !value?.month || !value?.year) return true
      return validBirthDate(Number(value.day), Number(value.month), Number(value.year))
    })
    .test("adult", text("You must be at least 18 years old."), (value) => {
      if (!adult || !value?.day || !value?.month || value?.year?.length !== 4) return true
      const iso = `${value.year}-${value.month.padStart(2, "0")}-${value.day.padStart(2, "0")}`
      return iso <= minimumAdultDate()
    })
    .test("time", text("Use the 24-hour format HH:MM."), (value) => !value?.time || TIME_RULE.test(value.time))
}

/** A place picked from the geocoder suggestions (never free text). */
export function placeRefRule(text: TranslateCopy, required = true) {
  const shape = yup
    .object({ id: yup.string().required(), label: yup.string().required() })
    .nullable()
    .default(null)
  return required
    ? shape.test("picked", text("Choose a place from the list."), (value) => Boolean(value?.id))
    : shape
}

/** Whole number of credits within bounds. */
export function creditsRule(text: TranslateCopy, { min = 1, max = 1_000_000 }: { min?: number; max?: number } = {}) {
  return yup
    .number()
    .transform((value, original) => (original === "" || original === null ? undefined : value))
    .typeError(text("Enter a whole number of credits."))
    .required(text("Enter a whole number of credits."))
    .integer(text("Enter a whole number of credits."))
    .min(min, text("Use a value between {value0} and {value1}.", { value0: min, value1: max }))
    .max(max, text("Use a value between {value0} and {value1}.", { value0: min, value1: max }))
}

/** Phone number in international (E.164) format; spaces and dashes are tolerated. */
export function phoneE164Rule(text: TranslateCopy, required = false) {
  const rule = yup
    .string()
    .trim()
    .transform((value: string | undefined) => (value ? value.replace(/[\s().-]/g, "") : value))
    .test("e164", text("Enter a phone number in international format, e.g. +14155552671."), (value) =>
      !value || E164_RULE.test(value)
    )
  return required ? rule.required(text("Enter a phone number in international format, e.g. +14155552671.")) : rule.default("")
}

/** A required consent checkbox. */
export function consentRule(text: TranslateCopy) {
  return yup.boolean().oneOf([true], text("Please confirm to continue.")).required(text("Please confirm to continue."))
}
