/**
 * Helpers shared by the revisions 6.0 payment specs (top-up stream, invoices,
 * Stripe checkout): sandbox cards with a chosen last four, and the walk to the
 * pay step. Kept apart from helpers.ts so that file stays untouched.
 */
import { expect, type Page } from "@playwright/test";

import { topUpCatalog } from "./helpers";

/** A Luhn-valid Visa number ending in `last4` (the sandbox keys its behaviour on the last four digits). */
export function sandboxCardEndingIn(last4: string): { number: string; expiry: string } {
  const luhn = (digits: string) => {
    let sum = 0;
    for (let i = 0; i < digits.length; i++) {
      let d = Number(digits[digits.length - 1 - i]);
      if (i % 2 === 1) d = d * 2 > 9 ? d * 2 - 9 : d * 2;
      sum += d;
    }
    return sum % 10 === 0;
  };
  for (let n = 0; n < 1000; n++) {
    const number = `4111111111${String(n).padStart(2, "0")}${last4}`;
    if (luhn(number)) {
      const month = String(1 + Math.floor(Math.random() * 12)).padStart(2, "0");
      const year = String(30 + Math.floor(Math.random() * 10));
      return { number, expiry: month + year };
    }
  }
  throw new Error(`no Luhn-valid number ending in ${last4}`);
}

/** Fill the sandbox card fields with `card`. */
export async function fillCard(page: Page, card: { number: string; expiry: string }) {
  await page.getByLabel("Card number").fill(card.number);
  await page.getByLabel("Expiry date").fill(card.expiry);
  await page.getByLabel("CVV").fill("123");
}

/** Tick the required policy agreement on the pay step. */
export async function agreeToPolicies(page: Page) {
  await page.getByTestId("top-up-consent").check();
  await expect(page.getByTestId("top-up-consent")).toBeChecked();
}

/** Open the top-up on `/credits`, choose a standard pack (never the Welcome Pack) and go on to the pay step. */
export async function openPayStep(page: Page) {
  const { packs } = await topUpCatalog(page);
  const standard = packs.find((p) => p.code === "starter_315") ?? packs.find((p) => p.kind === "standard")!;
  await page.goto("/en/credits");
  await expect(page.getByTestId("top-up")).toBeVisible();
  await page.getByRole("radio", { name: new RegExp(standard.name) }).click();
  await page.getByTestId("top-up-continue").click();
  await expect(page.getByTestId("checkout-pay")).toBeVisible();
  return standard;
}

/**
 * Open the top-up IN PLACE (from the header balance pill on the profile, no navigation) and go on to the pay step.
 * The `/credits` deep link leaves for the profile after a purchase (§24.7); only the in-place dialog never navigates.
 */
export async function openPayStepInPlace(page: Page) {
  const { packs } = await topUpCatalog(page);
  const standard = packs.find((p) => p.code === "starter_315") ?? packs.find((p) => p.kind === "standard")!;
  await page.goto("/en/profile");
  await page.locator("a.balance-pill").filter({ visible: true }).first().click();
  await expect(page.getByTestId("top-up")).toBeVisible();
  await page.getByRole("radio", { name: new RegExp(standard.name) }).click();
  await page.getByTestId("top-up-continue").click();
  await expect(page.getByTestId("checkout-pay")).toBeVisible();
  return standard;
}
