/**
 * Sign-up, sign-in and password-reset forms. Rules mirror the API
 * (`internal/service/auth.go`): emails are normalised server-side, passwords
 * need 8–128 characters with an uppercase letter, a digit and a symbol.
 */
import * as yup from "yup"

import type { TranslateCopy } from "@/i18n/translate-copy"
import { confirmPasswordRule, emailRule, passwordRule } from "@/lib/schemas/common"

export const signupEmailSchema = (text: TranslateCopy) =>
  yup.object({
    email: emailRule(text),
    terms: yup
      .boolean()
      .default(false)
      .oneOf([true], text("Please accept the Terms of Service and Privacy Policy.")),
  })

/** The terms box on the "Create your Staryield account" step after Google (§23.1). */
export const googleConsentSchema = (text: TranslateCopy) =>
  yup.object({
    terms: yup
      .boolean()
      .default(false)
      .oneOf([true], text("Please accept the Terms of Service and Privacy Policy.")),
  })

export const newPasswordSchema = (text: TranslateCopy) =>
  yup.object({
    password: passwordRule(text),
    confirm: confirmPasswordRule(text, "password"),
  })

export const loginSchema = (text: TranslateCopy) =>
  yup.object({
    email: emailRule(text),
    password: yup
      .string()
      .required(text("Enter your password."))
      .max(128, text("Use at most {value0} characters.", { value0: 128 })),
  })

export const forgotPasswordSchema = (text: TranslateCopy) => yup.object({ email: emailRule(text) })

export type SignupEmailValues = yup.InferType<ReturnType<typeof signupEmailSchema>>
export type GoogleConsentValues = yup.InferType<ReturnType<typeof googleConsentSchema>>
export type NewPasswordValues = yup.InferType<ReturnType<typeof newPasswordSchema>>
export type LoginValues = yup.InferType<ReturnType<typeof loginSchema>>
export type ForgotPasswordValues = yup.InferType<ReturnType<typeof forgotPasswordSchema>>

export type PasswordCheckId = "length" | "upper" | "number" | "symbol"

/** Each password rule, for the live checklist under the field. */
export function passwordChecks(password: string): Record<PasswordCheckId, boolean> {
  const length = Array.from(password).length
  return {
    length: length >= 8 && length <= 128,
    upper: /[A-Z]/.test(password),
    number: /\d/.test(password),
    symbol: /[^A-Za-z0-9]/.test(password),
  }
}

/** Untranslated labels for the checklist (passed through `text()` when rendered). */
export const passwordCheckLabels: Record<PasswordCheckId, string> = {
  length: "At least 8 characters",
  upper: "One uppercase letter",
  number: "One number",
  symbol: "One special character",
}
