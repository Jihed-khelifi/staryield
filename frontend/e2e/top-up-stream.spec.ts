/**
 * Payments confirmed over the purchase stream (revisions 6.0, docs/platform-extensions.md §26.5),
 * against the real stack with the sandbox provider: the sandbox card ending 3220 needs a
 * 3-D Secure-style confirmation (the purchase is `pending` with `next_action: sandbox_3ds`) and
 * settles about 2 s later; the card ending 3221 fails the bank confirmation. The dialog never
 * navigates, shows "Confirming your payment…" meanwhile and cannot be submitted twice.
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { expectNoHorizontalScroll, newClient, walletBalance } from "./helpers";
import { agreeToPolicies, fillCard, openPayStepInPlace, sandboxCardEndingIn } from "./payments-helpers";

const dialog = (page: Page) => page.getByTestId("top-up");

test.describe("purchase stream", { tag: "@responsive" }, () => {
  test("a card that needs confirmation shows 'Confirming your payment…', then the credits arrive through the stream", async ({ page }) => {
    await newClient(page, "stream-ok");
    const before = await walletBalance(page);
    const pack = await openPayStepInPlace(page);
    const here = page.url();
    await fillCard(page, sandboxCardEndingIn("3220"));
    await agreeToPolicies(page);

    const streams: string[] = [];
    page.on("request", (request) => {
      if (/\/credits\/purchases\/[^/]+\/stream/.test(request.url())) streams.push(request.url());
    });
    await page.getByTestId("checkout-pay").click();

    // Processing, visibly, with the button locked (no double submit); the dialog stays on the same page.
    await expect(page.getByTestId("top-up-confirming")).toContainText("Confirming your payment…");
    await expect(page.getByTestId("checkout-pay")).toBeDisabled();
    await expect(page.getByTestId("checkout-pay")).toHaveText("Confirming your payment…");
    await expectNoHorizontalScroll(page);
    await expectNoA11yViolations(page, ["[data-testid=top-up]"]);
    expect(page.url()).toBe(here);

    // About 2 s later the sandbox settles and the stream reports it.
    await expect(page.getByText(/credits added to your balance\./)).toBeVisible({ timeout: 30_000 });
    await expect(dialog(page)).toHaveCount(0);
    await expect.poll(() => walletBalance(page)).toBe(before + pack.credits);
    expect(streams.length).toBeGreaterThanOrEqual(1);
    expect(page.url()).toBe(here);
  });

  test("a bank that does not confirm shows the 3-D Secure message, keeps the dialog open and charges nothing", async ({ page }) => {
    await newClient(page, "stream-fail");
    const before = await walletBalance(page);
    await openPayStepInPlace(page);
    await fillCard(page, sandboxCardEndingIn("3221"));
    await agreeToPolicies(page);
    await page.getByTestId("checkout-pay").click();

    await expect(dialog(page).getByText("Your bank couldn't confirm this payment. Try again or use another card.")).toBeVisible({ timeout: 30_000 });
    await expect(dialog(page)).toBeVisible();
    await expect(page.getByTestId("top-up-confirming")).toHaveCount(0);
    await expect(page.getByTestId("checkout-pay")).toBeEnabled();
    expect(await walletBalance(page)).toBe(before);
    await expectNoHorizontalScroll(page);
  });

  test("the return page waits on the same stream for a purchase and shows the outcome", async ({ page }) => {
    await newClient(page, "stream-return");
    await openPayStepInPlace(page);
    // Start a purchase that needs confirmation, but leave the dialog by loading the return page for it.
    const created = page.waitForResponse((r) => r.url().endsWith("/api/v1/credits/purchases") && r.request().method() === "POST");
    await fillCard(page, sandboxCardEndingIn("3220"));
    await agreeToPolicies(page);
    await page.getByTestId("checkout-pay").click();
    const purchase = (await (await created).json()) as { id: string };
    await page.goto(`/en/credits/return?purchase=${encodeURIComponent(purchase.id)}`);
    await expect(page.getByTestId("return-success")).toContainText("credits added to your balance.", { timeout: 30_000 });
    await expect(page.getByRole("link", { name: "Back to my account" })).toHaveAttribute("href", "/en/profile");
  });

  test("the return page explains a missing or unknown purchase", async ({ page }) => {
    await newClient(page, "stream-missing");
    await page.goto("/en/credits/return");
    await expect(page.getByTestId("return-missing")).toBeVisible();
    await expectNoA11yViolations(page, ["[data-testid=credits-return]"]);
    await expectNoHorizontalScroll(page);
  });
});
