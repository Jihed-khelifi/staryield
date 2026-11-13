/**
 * Refund requests in the billing history (revisions 5.0, §24.4): the quote,
 * the request with its reference and the two-business-day promise, the status
 * with Withdraw, the one-open-request rule and the e-mail fallback. Every
 * journey registers its own account and buys its own pack.
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { API_URL, buyStandardPack, expectNoHorizontalScroll, newClient } from "./helpers";

const money = (cents: number) => `$${(cents / 100).toFixed(2)}`;

async function openBilling(page: Page) {
  await page.goto("/en/profile/settings");
  await expect(page.getByTestId("settings-billing")).toBeVisible();
  await expect(page.getByTestId("transaction").first()).toBeVisible();
}

async function csrf(page: Page) {
  return ((await (await page.context().request.get(`${API_URL}/api/v1/auth/csrf`)).json()) as { csrf_token: string }).csrf_token;
}

async function requestRefund(page: Page) {
  await page.getByTestId("refund-request").first().click();
  await expect(page.getByTestId("refund-quote")).toBeVisible();
  await page.getByTestId("refund-reason").click();
  await page.getByRole("option", { name: "I have credits I haven't used" }).click();
  await page.getByTestId("refund-details").fill("I would like my unused credits refunded.");
  await page.getByTestId("refund-submit").click();
  await expect(page.getByTestId("refund-success")).toBeVisible();
}

test.describe("refund requests", { tag: "@responsive" }, () => {
  test("a credit purchase can be refunded: quote, request, reference, status and withdraw", async ({ page }) => {
    await newClient(page, "refund");
    const bought = await buyStandardPack(page, (packs) => packs[0]);
    await openBilling(page);

    // The row is described by the pack's name ("Quick Insight −$20.00").
    const row = page.getByTestId("transaction").filter({ hasText: bought.name }).first();
    await expect(row.getByTestId("refund-request")).toBeVisible();
    await row.getByTestId("refund-request").click();

    // The quote: what the payment bought, what is left, what could come back.
    const quote = page.getByTestId("refund-quote");
    await expect(quote).toBeVisible();
    await expect(quote.getByText("Credits purchased").locator("xpath=following-sibling::dd")).toHaveText(bought.credits.toLocaleString("en-US"));
    await expect(quote.getByText("Credits used").locator("xpath=following-sibling::dd")).toHaveText("0");
    await expect(quote.getByText("Credits remaining").locator("xpath=following-sibling::dd")).toHaveText(bought.credits.toLocaleString("en-US"));
    await expect(quote.getByText("Refundable estimate").locator("xpath=following-sibling::dd")).toHaveText(`up to ${money(bought.price_cents)}`);
    await expect(quote.getByText("Refund window ends").locator("xpath=following-sibling::dd")).not.toBeEmpty();
    await expect(page.getByTestId("refund-policy-link")).toHaveAttribute("target", "_blank");
    await expectNoHorizontalScroll(page);
    await expectNoA11yViolations(page, ["[data-testid=refund-dialog]"]);

    // A reason is needed; the details are counted.
    await page.getByTestId("refund-submit").click();
    await expect(page.getByText("Choose a reason.")).toBeVisible();
    await page.getByTestId("refund-reason").click();
    await page.getByRole("option", { name: "I have credits I haven't used" }).click();
    const details = "I would like my unused credits refunded.";
    await page.getByTestId("refund-details").fill(details);
    await expect(page.getByText(`${details.length}/2000`)).toBeVisible();
    await page.getByTestId("refund-submit").click();

    const success = page.getByTestId("refund-success");
    await expect(success).toBeVisible();
    await expect(success.getByTestId("refund-reference")).toHaveText(/^RR-[0-9A-HJKMNP-TV-Z]{8}$/);
    await expect(success).toContainText("We aim to acknowledge requests within 2 business days.");
    const reference = (await success.getByTestId("refund-reference").textContent())!;
    await page.getByTestId("refund-done").click();

    // The row now shows the request, and survives a reload.
    const status = page.getByTestId("refund-status").first();
    await expect(status).toContainText("Refund requested");
    await expect(status.getByTestId("refund-reference-label")).toHaveText(`Reference ${reference}`);
    await page.reload();
    await expect(page.getByTestId("refund-status").first()).toContainText(`Reference ${reference}`);
    await expect(page.getByTestId("refund-request")).toHaveCount(0);

    // Withdrawing asks first, then the request can be made again.
    await page.getByTestId("refund-withdraw").first().click();
    await expect(page.getByTestId("refund-withdraw-dialog")).toContainText(reference);
    await page.getByRole("button", { name: "Keep my request" }).click();
    await expect(page.getByTestId("refund-status").first()).toContainText("Refund requested");
    await page.getByTestId("refund-withdraw").first().click();
    await page.getByTestId("refund-withdraw-confirm").click();
    await expect(page.getByTestId("refund-status").first()).toContainText("Request withdrawn");
    await expect(page.getByTestId("refund-request").first()).toBeVisible();
  });

  test("only one request can be open for a payment (409 refund_request_exists)", async ({ page }) => {
    await newClient(page, "refund-once");
    await buyStandardPack(page, (packs) => packs[0]);
    await openBilling(page);
    await requestRefund(page);
    await page.getByTestId("refund-done").click();

    const transactionId = (
      (await (await page.context().request.get(`${API_URL}/api/v1/billing/transactions`)).json()) as { items: { id: string; kind: string }[] }
    ).items.find((t) => t.kind === "credit_purchase")!.id;
    const second = await page.context().request.post(`${API_URL}/api/v1/billing/refund-requests`, {
      headers: { "X-CSRF-Token": await csrf(page) },
      data: { transaction_id: transactionId, reason: "other", idempotency_key: `e2e-refund-${Date.now()}` },
    });
    expect(second.status()).toBe(409);
    expect(((await second.json()) as { error: { code: string } }).error.code).toBe("refund_request_exists");
  });
});

test.describe("refund requests (intercepted API)", () => {
  test("e-mail is offered when in-app requests are switched off, with the subject 'Refund Request'", async ({ page }) => {
    await newClient(page, "refund-mail");
    await buyStandardPack(page, (packs) => packs[0]);
    await page.route("**/api/v1/billing/refund-quote*", (route) =>
      route.fulfill({ status: 503, json: { error: { code: "feature_unavailable", message: "Refund requests are switched off." } } }),
    );
    await openBilling(page);
    await page.getByTestId("refund-request").first().click();
    const email = page.getByTestId("refund-email").getByRole("link", { name: /support@staryield\.net/ });
    await expect(email).toHaveAttribute("href", "mailto:support@staryield.net?subject=Refund%20Request");
  });

  test("a rate limit and an ineligible payment are explained, with e-mail as the way out", async ({ page }) => {
    await newClient(page, "refund-limits");
    await buyStandardPack(page, (packs) => packs[0]);
    await openBilling(page);
    await page.route("**/api/v1/billing/refund-requests", (route) =>
      route.request().method() === "POST"
        ? route.fulfill({ status: 429, headers: { "Retry-After": "3600" }, json: { error: { code: "rate_limited", message: "Too many requests." } } })
        : route.continue(),
    );
    await page.getByTestId("refund-request").first().click();
    await expect(page.getByTestId("refund-quote")).toBeVisible();
    await page.getByTestId("refund-reason").click();
    await page.getByRole("option", { name: "Something else" }).click();
    await page.getByTestId("refund-submit").click();
    await expect(page.getByText(/You have sent several refund requests recently/)).toBeVisible();
    await expect(page.getByRole("link", { name: /support@staryield\.net/ })).toBeVisible();

    await page.unroute("**/api/v1/billing/refund-requests");
    await page.route("**/api/v1/billing/refund-requests", (route) =>
      route.request().method() === "POST"
        ? route.fulfill({ status: 409, json: { error: { code: "refund_not_eligible", message: "x", details: { reason: "fully_refunded" } } } })
        : route.continue(),
    );
    await page.getByTestId("refund-submit").click();
    await expect(page.getByText("This payment has already been fully refunded.")).toBeVisible();
  });
});
