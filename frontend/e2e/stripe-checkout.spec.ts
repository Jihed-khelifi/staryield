/**
 * Stripe Payment Element in test mode (revisions 6.0, docs/platform-extensions.md §26.5).
 *
 * Skipped unless E2E_STRIPE_PUBLISHABLE_KEY is set. It needs the API configured with
 * STARYIELD_PAYMENTS_PROVIDERS=stripe and the three Stripe test keys (the publishable key below
 * must be the API's STARYIELD_STRIPE_PUBLISHABLE_KEY), a webhook forwarded to the API
 * (`stripe listen --forward-to localhost:8080/webhooks/payments/stripe`) and Stripe's test cards.
 */
import { expect, test, type FrameLocator, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { expectNoHorizontalScroll, newClient, walletBalance } from "./helpers";
import { agreeToPolicies, openPayStep } from "./payments-helpers";

test.skip(!process.env.E2E_STRIPE_PUBLISHABLE_KEY, "set E2E_STRIPE_PUBLISHABLE_KEY (and run the API with Stripe test keys) to run the Stripe test-mode checkout");

/** Stripe's Payment Element iframe. */
const element = (page: Page): FrameLocator => page.frameLocator('iframe[title*="Secure payment input frame"]').first();

async function fillStripeCard(page: Page, number: string) {
  const frame = element(page);
  await frame.getByLabel("Card number").fill(number);
  await frame.getByLabel(/Expiration date/i).fill("12 / 34");
  await frame.getByLabel(/Security code/i).fill("123");
  const postal = frame.getByLabel(/ZIP|Postal/i);
  if (await postal.count()) await postal.fill("42424");
}

test.describe("Stripe test mode", { tag: "@responsive" }, () => {
  test("the Payment Element is secured by Stripe, in test mode, and a card pays through a ConfirmationToken", async ({ page }) => {
    await newClient(page, "stripe-ok");
    const before = await walletBalance(page);
    const pack = await openPayStep(page);

    await expect(page.getByTestId("secured-by-stripe-copy")).toHaveText(
      "Payments are processed securely by Stripe. Your card details are encrypted and sent directly to Stripe — Staryield never sees or stores them.",
    );
    await expect(page.getByAltText("Powered by Stripe")).toBeVisible();
    await expect(page.getByTestId("stripe-test-mode")).toHaveText("Test mode");
    await expectNoHorizontalScroll(page);

    await fillStripeCard(page, "4242 4242 4242 4242");
    await agreeToPolicies(page);
    const purchase = page.waitForRequest((r) => r.url().endsWith("/api/v1/credits/purchases") && r.method() === "POST");
    await page.getByTestId("checkout-pay").click();
    const body = (await purchase).postDataJSON() as { payment_token: string; provider: string };
    expect(body.provider).toBe("stripe");
    expect(body.payment_token).toMatch(/^ctoken_/);
    expect(JSON.stringify(body)).not.toContain("4242424242424242");

    await expect(page.getByText(/credits added to your balance\./)).toBeVisible({ timeout: 60_000 });
    await expect.poll(() => walletBalance(page)).toBe(before + pack.credits);
  });

  test("a card that needs 3-D Secure completes it in Stripe's modal and the stream reports the result", async ({ page }) => {
    await newClient(page, "stripe-3ds");
    const before = await walletBalance(page);
    const pack = await openPayStep(page);
    await fillStripeCard(page, "4000 0025 0000 3155");
    await agreeToPolicies(page);
    await page.getByTestId("checkout-pay").click();

    const challenge = page.frameLocator('iframe[src*="js.stripe.com"]').frameLocator('iframe[name="stripe-challenge-frame"]');
    await challenge.getByRole("button", { name: "Complete" }).click({ timeout: 60_000 });
    await expect(page.getByText(/credits added to your balance\./)).toBeVisible({ timeout: 60_000 });
    await expect.poll(() => walletBalance(page)).toBe(before + pack.credits);
    await expect(page).toHaveURL(/\/en\/credits/); // the dialog never navigates
  });

  test("a card the bank refuses shows the decline and stays on the pay step", async ({ page }) => {
    await newClient(page, "stripe-decline");
    const before = await walletBalance(page);
    await openPayStep(page);
    await fillStripeCard(page, "4000 0000 0000 0002");
    await agreeToPolicies(page);
    await page.getByTestId("checkout-pay").click();
    await expect(page.getByTestId("top-up")).toContainText(/declined/i, { timeout: 60_000 });
    expect(await walletBalance(page)).toBe(before);
    await expectNoA11yViolations(page, ["[data-testid=top-up]"]);
  });

  test("Settings → Add a card saves a card through a SetupIntent", async ({ page }) => {
    await newClient(page, "stripe-add");
    await page.goto("/en/profile/settings#billing");
    await page.getByTestId("add-card").click();
    await expect(page.getByTestId("secured-by-stripe")).toBeVisible();
    await fillStripeCard(page, "4242 4242 4242 4242");
    await page.getByTestId("add-card-save").click();
    await expect(page.getByText("Card saved.")).toBeVisible({ timeout: 60_000 });
  });
});
