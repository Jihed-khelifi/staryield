import { describe, expect, it } from "vitest";

import {
  forgotPasswordSchema,
  loginSchema,
  newPasswordSchema,
  passwordCheckLabels,
  passwordChecks,
  signupEmailSchema,
} from "@/lib/schemas/auth";
import { PASSWORD_RULE } from "@/lib/schemas/common";
import { fieldErrors, identityText as text } from "@/test/schema";

describe("signupEmailSchema", () => {
  const schema = signupEmailSchema(text);
  it("needs a valid email and the accepted terms", () => {
    expect(fieldErrors(schema, { email: " jane@example.com ", terms: true })).toEqual({});
    expect(fieldErrors(schema, { email: "", terms: false })).toEqual({
      email: "Enter your email address.",
      terms: "Please accept the Terms of Service and Privacy Policy.",
    });
    expect(fieldErrors(schema, { email: "jane@", terms: true }).email).toBe("Enter a valid email address.");
  });
});

describe("newPasswordSchema", () => {
  const schema = newPasswordSchema(text);
  it("applies the API password rule and requires a matching confirmation", () => {
    expect(fieldErrors(schema, { password: "Longenough1!", confirm: "Longenough1!" })).toEqual({});
    expect(fieldErrors(schema, { password: "weakpass", confirm: "weakpass" }).password).toBe(
      "Use 8+ characters, an uppercase letter, a number, and a special character."
    );
    expect(fieldErrors(schema, { password: "Longenough1!", confirm: "Longenough1?" })).toEqual({
      confirm: "The passwords do not match.",
    });
    expect(fieldErrors(schema, { password: "", confirm: "" })).toEqual({
      password: "Enter your password.",
      confirm: "Enter your password.",
    });
  });
});

describe("loginSchema", () => {
  const schema = loginSchema(text);
  it("does not apply the strength rule to existing passwords", () => {
    expect(fieldErrors(schema, { email: "jane@example.com", password: "old-weak" })).toEqual({});
    expect(fieldErrors(schema, { email: "jane@example.com", password: "" }).password).toBe("Enter your password.");
    expect(fieldErrors(schema, { email: "jane@example.com", password: "x".repeat(129) }).password).toBe(
      "Use at most 128 characters."
    );
  });
});

describe("forgotPasswordSchema", () => {
  it("only needs an email address", () => {
    expect(fieldErrors(forgotPasswordSchema(text), { email: "jane@example.com" })).toEqual({});
    expect(fieldErrors(forgotPasswordSchema(text), { email: "nope" }).email).toBe("Enter a valid email address.");
  });
});

describe("passwordChecks", () => {
  it("reports each rule separately", () => {
    expect(passwordChecks("")).toEqual({ length: false, upper: false, number: false, symbol: false });
    expect(passwordChecks("abcdefgh")).toEqual({ length: true, upper: false, number: false, symbol: false });
    expect(passwordChecks("Abc1!")).toEqual({ length: false, upper: true, number: true, symbol: true });
    expect(passwordChecks("x".repeat(129)).length).toBe(false);
  });

  it("all checks met is exactly the API rule", () => {
    for (const sample of ["Longenough1!", "short1A!", "NoDigits!!", "nouppercase1!", "NoSymbol123", "Ünïcode1!x"]) {
      const allMet = Object.values(passwordChecks(sample)).every(Boolean);
      expect(allMet, sample).toBe(PASSWORD_RULE.test(sample));
    }
  });

  it("has a label for every rule", () => {
    expect(Object.keys(passwordCheckLabels).sort()).toEqual(Object.keys(passwordChecks("")).sort());
  });
});
