import { describe, expect, it } from "vitest";

import { fieldErrors, identityText } from "@/test/schema";
import { cardBrand, cardFieldsSchema, expiryInFuture, formatCardNumber, formatExpiry, luhnValid, parseExpiry } from "@/lib/payments/card";
import { sandboxToken, sandboxTokenPrefix } from "@/lib/payments/payment-fields";

describe("card helpers", () => {
  it("detects the brand from the number's prefix", () => {
    expect(cardBrand("4242 4242 4242 4242")).toBe("visa");
    expect(cardBrand("5555 5555 5555 4444")).toBe("mastercard");
    expect(cardBrand("2223 0031 2200 3222")).toBe("mastercard");
    expect(cardBrand("3782 822463 10005")).toBe("amex");
    expect(cardBrand("6011 1111 1111 1117")).toBe("discover");
    expect(cardBrand("9999 0000 0000 0000")).toBeNull();
  });

  it("checks the Luhn digit", () => {
    expect(luhnValid("4242424242424242")).toBe(true);
    expect(luhnValid("4000000000000002")).toBe(true);
    expect(luhnValid("4242424242424241")).toBe(false);
    expect(luhnValid("4242")).toBe(false);
  });

  it("formats numbers and expiry dates while typing", () => {
    expect(formatCardNumber("4242424242424242")).toBe("4242 4242 4242 4242");
    expect(formatCardNumber("378282246310005")).toBe("3782 822463 10005");
    expect(formatExpiry("0829")).toBe("08 / 29");
    expect(formatExpiry("1")).toBe("1");
  });

  it("parses MM / YY and MM/YYYY and knows when a card expired", () => {
    expect(parseExpiry("08 / 29")).toEqual({ month: 8, year: 2029 });
    expect(parseExpiry("12/2030")).toEqual({ month: 12, year: 2030 });
    expect(parseExpiry("13/29")).toBeNull();
    expect(expiryInFuture({ month: 11, year: 2026 }, new Date(2026, 10, 30))).toBe(true);
    expect(expiryInFuture({ month: 10, year: 2026 }, new Date(2026, 10, 1))).toBe(false);
  });
});

describe("cardFieldsSchema", () => {
  const s = cardFieldsSchema(identityText);
  const ok = { cardholder: "Cassandra Moon", number: "4242 4242 4242 4242", expiry: "08 / 39", cvc: "123" };
  it("accepts a valid card", () => {
    expect(fieldErrors(s, ok)).toEqual({});
  });
  it("explains each problem", () => {
    expect(fieldErrors(s, { cardholder: "", number: "", expiry: "", cvc: "" })).toEqual({
      cardholder: "This field is required.",
      number: "Enter your card number.",
      expiry: "Enter the expiry date as MM / YY.",
      cvc: "Enter the security code.",
    });
    expect(fieldErrors(s, { ...ok, number: "9999 9999 9999 9995" }).number).toBe("Use a Visa, Mastercard, American Express or Discover card.");
    expect(fieldErrors(s, { ...ok, number: "4242 4242 4242 4241" }).number).toBe("Check the card number.");
    expect(fieldErrors(s, { ...ok, expiry: "01 / 20" }).expiry).toBe("This card has expired.");
    expect(fieldErrors(s, { ...ok, number: "3782 822463 10005", cvc: "123" }).cvc).toBe("Enter the 3 or 4 digit security code.");
  });
});

describe("sandbox tokens", () => {
  it("derives the prefix from the client config and never contains the card number", () => {
    expect(sandboxTokenPrefix({ provider: "sandbox_a", token_format: "sbx_a_<brand>_<last4>_<mm>_<yyyy>[_<country>]" })).toBe("sbx_a");
    expect(sandboxTokenPrefix({ provider: "sandbox_b" })).toBe("sbx_b");
    expect(sandboxTokenPrefix({ provider: "stripe" })).toBeNull();
    const token = sandboxToken("sbx_a", { brand: "visa", last4: "4242", month: 8, year: 2039 });
    expect(token).toBe("sbx_a_visa_4242_08_2039");
    expect(token).not.toMatch(/\d{13,19}/);
  });
});
