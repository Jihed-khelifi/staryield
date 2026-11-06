/**
 * Card-entry helpers for the sandbox payment fields: brand detection, Luhn
 * check, expiry parsing and display formatting. Real providers validate in
 * their own hosted fields; these only run in development sandboxes.
 */
import * as yup from "yup";

import type { TranslateCopy } from "@/i18n/translate-copy";
import { trimmedText } from "@/lib/schemas/common";

export type CardBrand = "visa" | "mastercard" | "amex" | "discover";

/** Digits only. */
export function digits(value: string): string {
  return value.replace(/\D/g, "");
}

/** Card brand from the number's prefix (IIN ranges), or null. */
export function cardBrand(value: string): CardBrand | null {
  const n = digits(value);
  if (/^4/.test(n)) return "visa";
  if (/^3[47]/.test(n)) return "amex";
  if (/^(6011|65|64[4-9])/.test(n)) return "discover";
  if (/^5[1-5]/.test(n)) return "mastercard";
  const two = Number(n.slice(0, 4));
  if (n.length >= 4 && two >= 2221 && two <= 2720) return "mastercard";
  return null;
}

/** Luhn (mod 10) checksum. */
export function luhnValid(value: string): boolean {
  const n = digits(value);
  if (n.length < 12) return false;
  let sum = 0;
  for (let i = 0; i < n.length; i++) {
    let d = Number(n[n.length - 1 - i]);
    if (i % 2 === 1) {
      d *= 2;
      if (d > 9) d -= 9;
    }
    sum += d;
  }
  return sum % 10 === 0;
}

/** Group the number for display: 4-4-4-4 (Amex 4-6-5). */
export function formatCardNumber(value: string): string {
  const n = digits(value).slice(0, 19);
  if (cardBrand(n) === "amex") return [n.slice(0, 4), n.slice(4, 10), n.slice(10, 15)].filter(Boolean).join(" ");
  return n.replace(/(\d{4})(?=\d)/g, "$1 ");
}

/** "MM / YY" while typing. */
export function formatExpiry(value: string): string {
  const n = digits(value).slice(0, 4);
  return n.length > 2 ? `${n.slice(0, 2)} / ${n.slice(2)}` : n;
}

/** Month and four-digit year from "MM / YY" (or "MM/YYYY"), or null. */
export function parseExpiry(value: string): { month: number; year: number } | null {
  const m = /^\s*(\d{1,2})\s*\/?\s*(\d{2}|\d{4})\s*$/.exec(value.replace(/\s+\/\s+/, "/"));
  if (!m) return null;
  const month = Number(m[1]);
  const year = m[2].length === 2 ? 2000 + Number(m[2]) : Number(m[2]);
  if (month < 1 || month > 12) return null;
  return { month, year };
}

/** True when the card still works this month. */
export function expiryInFuture(expiry: { month: number; year: number }, now = new Date()): boolean {
  const y = now.getFullYear();
  const m = now.getMonth() + 1;
  return expiry.year > y || (expiry.year === y && expiry.month >= m);
}

export const cardFieldsSchema = (text: TranslateCopy) =>
  yup.object({
    cardholder: trimmedText(text, { max: 100 }),
    number: yup
      .string()
      .required(text("Enter your card number."))
      .test("brand", text("Use a Visa, Mastercard, American Express or Discover card."), (value) => !value || cardBrand(value) !== null)
      .test("luhn", text("Check the card number."), (value) => !value || (digits(value).length >= 12 && digits(value).length <= 19 && luhnValid(value))),
    expiry: yup
      .string()
      .required(text("Enter the expiry date as MM / YY."))
      .test("format", text("Enter the expiry date as MM / YY."), (value) => !value || parseExpiry(value) !== null)
      .test("future", text("This card has expired."), (value) => {
        const parsed = value ? parseExpiry(value) : null;
        return !parsed || expiryInFuture(parsed);
      }),
    cvc: yup
      .string()
      .required(text("Enter the security code."))
      .test("cvc", text("Enter the 3 or 4 digit security code."), (value, ctx) => {
        if (!value) return true;
        const length = cardBrand(String(ctx.parent.number ?? "")) === "amex" ? 4 : 3;
        return new RegExp(`^\\d{${length}}$`).test(value);
      }),
  });

export type CardFieldsValues = yup.InferType<ReturnType<typeof cardFieldsSchema>>;
