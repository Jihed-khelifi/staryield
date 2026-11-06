/**
 * Account settings forms. Rules mirror the API (`internal/service/account.go`,
 * `account_lifecycle.go`, `auth.go`) so most mistakes are caught before a
 * request; the server stays authoritative and its 422 `details.fields`
 * (dotted paths such as `profile.birth_time`) map onto the same field paths.
 */
import * as yup from "yup";

import { locales } from "@/i18n/config";
import type { TranslateCopy } from "@/i18n/translate-copy";
import { THEMES } from "@/lib/theme";
import { confirmPasswordRule, emailRule, passwordRule, placeRefRule, TIME_RULE, trimmedText } from "@/lib/schemas/common";

/** Text typed to confirm a permanent closure (the API checks the exact word). */
export const CLOSURE_CONFIRMATION = "DELETE";

const ISO_DATE = /^(\d{4})-(\d{2})-(\d{2})$/;

/** Local calendar date as YYYY-MM-DD. */
export function isoDay(date = new Date()): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}

/** The latest birth date of someone who is `years` old today. */
export function latestBirthDate(years = 18, today = new Date()): string {
  const cutoff = new Date(today);
  cutoff.setFullYear(cutoff.getFullYear() - years);
  return isoDay(cutoff);
}

/** A real calendar date (rejects 2023-02-30). */
export function isCalendarDate(value: string): boolean {
  const m = ISO_DATE.exec(value);
  if (!m) return false;
  const [y, mo, d] = [Number(m[1]), Number(m[2]), Number(m[3])];
  const date = new Date(Date.UTC(y, mo - 1, d));
  return date.getUTCFullYear() === y && date.getUTCMonth() === mo - 1 && date.getUTCDate() === d;
}

/** An IANA time zone the browser knows ("" = follow the device). */
export function isTimeZone(value: string): boolean {
  if (!value) return true;
  if (value.length > 64) return false;
  try {
    new Intl.DateTimeFormat("en", { timeZone: value });
    return true;
  } catch {
    return false;
  }
}

/**
 * The editable profile fields of the settings page, grouped like the API
 * sections so server errors land on the matching field.
 */
export const settingsSchema = (text: TranslateCopy) =>
  yup.object({
    account: yup.object({
      display_name: trimmedText(text, { max: 80 }),
    }),
    profile: yup.object({
      birth_date: yup
        .string()
        .default("")
        .test("valid", text("Enter a valid date."), (value) => !value || isCalendarDate(value))
        .test(
          "range",
          text("Enter a valid birth date that is not in the future."),
          (value) => !value || !isCalendarDate(value) || (value >= "1900-01-01" && value <= isoDay())
        )
        .test(
          "adult",
          text("You must be at least 18 years old."),
          (value) => !value || !isCalendarDate(value) || value > isoDay() || value <= latestBirthDate()
        )
        .when(["birth_time", "time_unknown"], ([time, unknown], schema) =>
          time && !unknown ? schema.test("needed", text("Add your date of birth to use a birth time."), (value) => Boolean(value)) : schema
        ),
      time_unknown: yup.boolean().default(false),
      birth_time: yup
        .string()
        .default("")
        .test("time24", text("Use the 24-hour format HH:MM."), (value) => !value || TIME_RULE.test(value)),
      place: placeRefRule(text, false).when(["birth_time", "time_unknown"], ([time, unknown], schema) =>
        time && !unknown
          ? schema.test("place-required", text("Choose where you were born so we can place your rising sign."), (value) => Boolean(value?.id))
          : schema
      ),
    }),
    notifications: yup.object({
      email: yup.boolean().default(true),
      push: yup.boolean().default(false),
      marketing: yup.boolean().default(false),
      horoscope: yup.boolean().default(false),
      events: yup.boolean().default(false),
      chart: yup.boolean().default(false),
    }),
    privacy: yup.object({
      share_birth_details_with_advisors: yup.boolean().default(false),
    }),
    appearance: yup.object({
      theme: yup.string().oneOf([...THEMES], text("Choose a theme from the list.")).required(text("Choose a theme from the list.")),
      locale: yup.string().oneOf([...locales], text("Choose English, French or Spanish.")).required(text("Choose English, French or Spanish.")),
      time_zone: yup.string().default("").test("tz", text("Choose a time zone from the list."), (value) => isTimeZone(value ?? "")),
    }),
  });

export type SettingsValues = yup.InferType<ReturnType<typeof settingsSchema>>;

/** POST /auth/password/change. */
export const changePasswordSchema = (text: TranslateCopy) =>
  yup.object({
    current_password: yup
      .string()
      .required(text("Enter your current password."))
      .max(128, text("Use at most {value0} characters.", { value0: 128 })),
    new_password: passwordRule(text).test(
      "different",
      text("Choose a password different from the current one."),
      (value, ctx) => !value || value !== ctx.parent.current_password
    ),
    confirm_password: confirmPasswordRule(text, "new_password"),
  });

export type ChangePasswordValues = yup.InferType<ReturnType<typeof changePasswordSchema>>;

/**
 * POST /me/email-change. Pass the current address as form context
 * (`{ currentEmail }`) to catch "this is already your address" early.
 */
export const emailChangeSchema = (text: TranslateCopy) =>
  yup.object({
    email: emailRule(text).test("different", text("This is already your email address."), (value, ctx) => {
      const current = (ctx.options.context as { currentEmail?: string } | undefined)?.currentEmail;
      return !value || !current || value.trim().toLowerCase() !== current.trim().toLowerCase();
    }),
    password: yup
      .string()
      .required(text("Enter your password."))
      .max(128, text("Use at most {value0} characters.", { value0: 128 })),
  });

export type EmailChangeValues = yup.InferType<ReturnType<typeof emailChangeSchema>>;

/** Re-authentication for sensitive actions (exports, deactivation). */
export const passwordConfirmSchema = (text: TranslateCopy) =>
  yup.object({
    password: yup
      .string()
      .required(text("Enter your password."))
      .max(128, text("Use at most {value0} characters.", { value0: 128 })),
  });

export type PasswordConfirmValues = yup.InferType<ReturnType<typeof passwordConfirmSchema>>;

/** POST /me/close: password, the typed confirmation and an optional reason. */
export const closeAccountSchema = (text: TranslateCopy) =>
  yup.object({
    password: yup
      .string()
      .required(text("Enter your password."))
      .max(128, text("Use at most {value0} characters.", { value0: 128 })),
    confirmation: yup
      .string()
      .required(text("Type DELETE to confirm."))
      .test("exact", text("Type DELETE to confirm."), (value) => value?.trim() === CLOSURE_CONFIRMATION),
    reason: trimmedText(text, { max: 500, required: false }),
  });

export type CloseAccountValues = yup.InferType<ReturnType<typeof closeAccountSchema>>;
