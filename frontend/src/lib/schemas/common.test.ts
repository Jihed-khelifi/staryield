import { describe, expect, it } from "vitest";
import * as yup from "yup";

import {
  birthPartsRule,
  confirmPasswordRule,
  consentRule,
  creditsRule,
  emailRule,
  passwordRule,
  phoneE164Rule,
  placeRefRule,
  time24Rule,
  trimmedText,
} from "@/lib/schemas/common";

// Identity translator: messages stay in English source form.
const text = <T,>(value: T, values?: Record<string, string | number>): T =>
  typeof value === "string" && values
    ? (value.replace(/\{(\w+)\}/g, (m, k: string) => (k in values ? String(values[k]) : m)) as T)
    : value;

async function errorOf(schema: yup.AnySchema, value: unknown): Promise<string | null> {
  try {
    await schema.validate(value);
    return null;
  } catch (error) {
    return (error as yup.ValidationError).message;
  }
}

describe("common schema rules", () => {
  it("email: required, trimmed, valid, ≤254", async () => {
    expect(await errorOf(emailRule(text), "")).toBe("Enter your email address.");
    expect(await errorOf(emailRule(text), "not-an-email")).toBe("Enter a valid email address.");
    expect(await errorOf(emailRule(text), "  jane@example.com ")).toBeNull();
    expect(await errorOf(emailRule(text), `${"a".repeat(250)}@x.io`)).toBe("Use at most 254 characters.");
  });

  it("password matches the API rule and confirm must match", async () => {
    expect(await errorOf(passwordRule(text), "short")).toMatch(/^Use 8\+ characters/);
    expect(await errorOf(passwordRule(text), "longenough1!")).toMatch(/^Use 8\+ characters/);
    expect(await errorOf(passwordRule(text), "Longenough1!")).toBeNull();
    const schema = yup.object({ password: passwordRule(text), confirm: confirmPasswordRule(text) });
    expect(await errorOf(schema, { password: "Longenough1!", confirm: "Longenough1?" })).toBe("The passwords do not match.");
    expect(await errorOf(schema, { password: "Longenough1!", confirm: "Longenough1!" })).toBeNull();
  });

  it("trimmedText counts characters, not UTF-16 units", async () => {
    const rule = trimmedText(text, { min: 2, max: 3 });
    expect(await errorOf(rule, "   ")).toBe("This field is required.");
    expect(await errorOf(rule, "a")).toBe("Use at least 2 characters.");
    expect(await errorOf(rule, "🌙🌙🌙")).toBeNull();
    expect(await errorOf(rule, "abcd")).toBe("Use at most 3 characters.");
    expect(await errorOf(trimmedText(text, { max: 5, required: false }), undefined)).toBeNull();
  });

  it("time24 accepts HH:MM only", async () => {
    expect(await errorOf(time24Rule(text), "")).toBeNull();
    expect(await errorOf(time24Rule(text), "23:59")).toBeNull();
    expect(await errorOf(time24Rule(text), "24:00")).toBe("Use the 24-hour format HH:MM.");
    expect(await errorOf(time24Rule(text, true), "")).toBe("Use the 24-hour format HH:MM.");
  });

  it("birth parts reject incomplete, impossible and future dates", async () => {
    const rule = birthPartsRule(text);
    expect(await errorOf(rule, { day: "", month: "11", year: "1990" })).toBe("Enter your date of birth.");
    expect(await errorOf(rule, { day: "29", month: "02", year: "1990" })).toBe("Enter a valid date.");
    expect(await errorOf(rule, { day: "29", month: "02", year: "1992" })).toBeNull();
    expect(await errorOf(rule, { day: "31", month: "04", year: "1990" })).toBe("Enter a valid date.");
    const nextYear = String(new Date().getFullYear() + 1);
    expect(await errorOf(rule, { day: "01", month: "01", year: nextYear })).toBe("Enter a valid date.");
    expect(await errorOf(rule, { day: "1", month: "1", year: "1990", time: "25:00" })).toBe("Use the 24-hour format HH:MM.");
  });

  it("adult birth dates require 18 years", async () => {
    const rule = birthPartsRule(text, { adult: true });
    const today = new Date();
    const year = String(today.getFullYear() - 17);
    const day = String(today.getDate()).padStart(2, "0");
    const month = String(today.getMonth() + 1).padStart(2, "0");
    expect(await errorOf(rule, { day, month, year })).toBe("You must be at least 18 years old.");
    expect(await errorOf(rule, { day, month, year: String(today.getFullYear() - 18) })).toBeNull();
  });

  it("places must be picked from the list", async () => {
    expect(await errorOf(placeRefRule(text), null)).toBe("Choose a place from the list.");
    expect(await errorOf(placeRefRule(text), { id: "2988507", label: "Paris, France" })).toBeNull();
    expect(await errorOf(placeRefRule(text, false), null)).toBeNull();
  });

  it("credits are whole numbers within bounds", async () => {
    const rule = creditsRule(text, { min: 10, max: 5000 });
    expect(await errorOf(rule, "")).toBe("Enter a whole number of credits.");
    expect(await errorOf(rule, "abc")).toBe("Enter a whole number of credits.");
    expect(await errorOf(rule, 12.5)).toBe("Enter a whole number of credits.");
    expect(await errorOf(rule, 5)).toBe("Use a value between 10 and 5000.");
    expect(await errorOf(rule, "2000")).toBeNull();
  });

  it("phone numbers must be E.164 (separators tolerated)", async () => {
    const rule = phoneE164Rule(text);
    expect(await errorOf(rule, "")).toBeNull();
    expect(await errorOf(rule, "+216 66 669 696")).toBeNull();
    expect(await rule.validate("+1 (415) 555-2671")).toBe("+14155552671");
    expect(await errorOf(rule, "0612345678")).toMatch(/^Enter a phone number in international format/);
    expect(await errorOf(phoneE164Rule(text, true), "")).toMatch(/^Enter a phone number/);
  });

  it("consent must be ticked", async () => {
    expect(await errorOf(consentRule(text), false)).toBe("Please confirm to continue.");
    expect(await errorOf(consentRule(text), true)).toBeNull();
  });
});
