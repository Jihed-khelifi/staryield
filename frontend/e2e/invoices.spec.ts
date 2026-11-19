/**
 * Invoices (revisions 6.0, docs/platform-extensions.md §26.6): a paid credit purchase gets an
 * invoice that is linked from the payment history, shown on its own page with print styles, and
 * downloadable as a PDF. Runs against the real stack with the sandbox provider.
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { API_URL, buyStandardPack, expectNoHorizontalScroll, newClient } from "./helpers";

type TransactionRow = { id: string; kind: string; invoice?: { id: string; number: string } };

/** The invoice is issued just after the payment completes: wait for the history to carry it. */
async function invoiceOfLatestPurchase(page: Page) {
  let found: { id: string; number: string } | undefined;
  await expect
    .poll(
      async () => {
        const res = await page.context().request.get(`${API_URL}/api/v1/billing/transactions?limit=5`);
        const body = (await res.json()) as { items: TransactionRow[] };
        found = body.items.find((t) => t.kind === "credit_purchase" && t.invoice)?.invoice;
        return found?.number;
      },
      { timeout: 60_000, intervals: [1_000, 2_000, 5_000] },
    )
    .toMatch(/^SY-\d{4}-\d{6}$/);
  return found!;
}

test.describe("invoices", { tag: "@responsive" }, () => {
  test("the payment history links to the invoice, the page shows it and the PDF downloads", async ({ page }) => {
    await newClient(page, "invoice");
    const pack = await buyStandardPack(page);
    const invoice = await invoiceOfLatestPurchase(page);

    await page.goto("/en/profile/settings#billing");
    const link = page.getByTestId("transaction-invoice").first();
    await expect(link).toBeVisible();
    await expect(link).toContainText("Invoice");
    await expect(link).toHaveAttribute("href", `/en/profile/invoices/${invoice.id}`);
    await link.click();

    // The page: seller, buyer, lines, totals, payment, descriptor.
    await expect(page.getByTestId("invoice-number")).toHaveText(invoice.number);
    await expect(page.getByTestId("invoice-seller")).toContainText("Staryield Media LLC");
    await expect(page.getByTestId("invoice-buyer")).toContainText("@staryield.test");
    await expect(page.getByTestId("invoice-lines")).toContainText(`${pack.credits.toLocaleString("en-US")} credits`);
    await expect(page.getByTestId("invoice-total")).toContainText("USD");
    await expect(page.getByTestId("invoice-totals")).toContainText("Not applicable");
    await expect(page.getByTestId("invoice-payment")).toBeVisible();
    await expect(page.getByTestId("invoice-descriptor")).toBeVisible();
    await expectNoHorizontalScroll(page);
    await expectNoA11yViolations(page, ["[data-testid=invoice]"]);

    // The PDF: the button downloads it, and the API answers application/pdf with the invoice number as file name.
    const href = await page.getByTestId("invoice-download").getAttribute("href");
    expect(href).toMatch(new RegExp(`/api/v1/billing/invoices/${invoice.id}/pdf$`));
    const download = page.waitForEvent("download");
    await page.getByTestId("invoice-download").click();
    expect((await download).suggestedFilename()).toBe(`${invoice.number}.pdf`);
    const pdf = await page.context().request.get(href!);
    expect(pdf.status()).toBe(200);
    expect(pdf.headers()["content-type"]).toContain("application/pdf");
    expect(pdf.headers()["content-disposition"]).toContain(`${invoice.number}.pdf`);
    expect((await pdf.body()).subarray(0, 5).toString()).toBe("%PDF-");
  });

  test("print styles hide the site chrome and the buttons", async ({ page }) => {
    await newClient(page, "invoice-print");
    await buyStandardPack(page);
    const invoice = await invoiceOfLatestPurchase(page);
    await page.goto(`/en/profile/invoices/${invoice.id}`);
    await expect(page.getByTestId("invoice")).toBeVisible();
    await page.emulateMedia({ media: "print" });
    await expect(page.getByTestId("invoice-actions")).toBeHidden();
    await expect(page.getByTestId("invoice")).toBeVisible();
    await page.emulateMedia({ media: "screen" });
    await expect(page.getByTestId("invoice-actions")).toBeVisible();
  });

  test("an invoice that is not yours (or does not exist) says so", async ({ page }) => {
    await newClient(page, "invoice-missing");
    await page.goto("/en/profile/invoices/00000000-0000-4000-8000-000000000000");
    await expect(page.getByTestId("invoice-not-found")).toContainText("We could not find this invoice");
    await expect(page.getByRole("link", { name: "Back to billing" }).first()).toHaveAttribute("href", "/en/profile/settings#billing");
    await expectNoA11yViolations(page, ["main"]);
  });

  test("a signed-out visitor is sent to the login", async ({ page }) => {
    await page.goto("/en/profile/invoices/00000000-0000-4000-8000-000000000000");
    await expect(page).toHaveURL(/\/en\/login\?next=/);
  });
});
