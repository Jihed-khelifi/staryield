/**
 * Stripe's Payment Element against the real Stripe.js, without Stripe keys (revisions 6.0, §26.5).
 *
 * Production once failed here: the Stripe.js release that @stripe/stripe-js pins threw an
 * IntegrationError for an Elements option it had renamed, and the whole site fell to the root
 * error page, whose "Try again" then threw too. Unit tests mock Stripe.js, so only a browser
 * loading the real script can catch that.
 *
 * The API runs the sandbox; these tests answer the card form's client configuration with a Stripe
 * one carrying a dummy test key. Stripe.js then builds and mounts the Payment Element with our
 * options, and Stripe's own frame rejects the key: that proves Stripe.js accepted the options.
 * The first test needs network access to js.stripe.com and api.stripe.com.
 */
import { expect, test, type Page } from "@playwright/test";

import { newClient } from "./helpers";
import { openPayStep } from "./payments-helpers";

/** Where @stripe/stripe-js loads Stripe.js from: `https://js.stripe.com/<release>/stripe.js`. */
const STRIPE_JS = /^https:\/\/js\.stripe\.com\/[a-z0-9]+\/stripe\.js/;

/** A Stripe.js stand-in that throws from `elements()`, as the real one did for the renamed option. */
const THROWING_STRIPE_JS = `
window.Stripe = function () {
  var later = function () { return Promise.resolve({}); };
  return {
    elements: function () {
      var error = new Error("Invalid value for elements(): stand-in for a Stripe.js IntegrationError.");
      error.name = "IntegrationError";
      throw error;
    },
    createToken: later,
    createPaymentMethod: later,
    confirmCardPayment: later,
  };
};`;

/** The card form gets a Stripe configuration (deferred payment, card only, test mode) with a dummy key. */
async function answerWithStripeConfig(page: Page) {
  await page.route(/\/api\/v1\/billing\/client-config\b/, (route) =>
    route.fulfill({
      json: {
        provider: "stripe",
        type: "stripe_elements",
        publishable_key: `pk_test_${"0".repeat(24)}`,
        mode: "payment",
        amount_cents: 1499,
        currency: "usd",
        payment_method_types: ["card"],
        livemode: false,
      },
    }),
  );
}

/** Errors nothing caught (an IntegrationError that escaped, or a broken "Try again"). */
function uncaughtErrors(page: Page): string[] {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(`${error.name}: ${error.message}`));
  return errors;
}

const cardFormAlert = (page: Page) => page.getByTestId("stripe-fields").getByRole("alert");

test.describe("Stripe Payment Element with the real Stripe.js", { tag: "@responsive" }, () => {
  test("Stripe.js accepts our Elements options and mounts the Payment Element in the top-up", async ({ page }) => {
    const errors = uncaughtErrors(page);
    await newClient(page, "stripe-js");
    await answerWithStripeConfig(page);
    const script = page.waitForRequest(STRIPE_JS);
    await openPayStep(page);
    await script;

    await expect(page.getByTestId("secured-by-stripe")).toBeVisible();
    await expect(page.getByTestId("stripe-test-mode")).toHaveText("Test mode");
    // Stripe's own frame loaded and rejected the dummy key: it got that far only because Stripe.js accepted
    // the options. A rejected option would have thrown before any frame existed ("crash").
    await expect(cardFormAlert(page)).toHaveAttribute("data-reason", "element", { timeout: 45_000 });
    await expect(page.getByTestId("checkout-pay")).toBeDisabled();
    await expect(page.getByTestId("top-up")).toBeVisible();
    expect(errors).toEqual([]);
  });

  test("anything Stripe.js throws stays inside the card form: logged, retryable, and the site keeps working", async ({ page }) => {
    const errors = uncaughtErrors(page);
    await newClient(page, "stripe-js-throws");
    await answerWithStripeConfig(page);
    await page.route(STRIPE_JS, (route) => route.fulfill({ contentType: "application/javascript", body: THROWING_STRIPE_JS }));
    const logged = page.waitForRequest((r) => r.url().endsWith("/api/v1/client-logs") && (r.postData() ?? "").includes("stripe_elements_failed"));
    await openPayStep(page);

    await expect(cardFormAlert(page)).toHaveAttribute("data-reason", "crash");
    await expect(cardFormAlert(page)).toContainText("The secure card form could not be loaded");
    await logged;
    await expect(page.getByTestId("checkout-pay")).toBeDisabled();
    await expect(page.getByTestId("secured-by-stripe")).toBeVisible();

    // Try again builds the form afresh; this stand-in throws again, and it is still contained.
    await cardFormAlert(page).getByRole("button", { name: "Try again" }).click();
    await expect(cardFormAlert(page)).toHaveAttribute("data-reason", "crash");

    // The root error page never took over: the dialog closes and the page under it works.
    await expect(page.getByText("Something went wrong. Please try again.")).toHaveCount(0);
    await page.keyboard.press("Escape");
    await expect(page.getByTestId("top-up")).toBeHidden();
    await expect(page.getByRole("main")).toBeVisible();
    expect(errors).toEqual([]);
  });
});
