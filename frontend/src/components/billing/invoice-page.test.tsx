// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { ApiError } from "@/lib/api/errors";
import type { Invoice } from "@/lib/api/invoices";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const m = vi.hoisted(() => ({ get: vi.fn(), replace: vi.fn(), status: { current: "authenticated" } }));
vi.mock("@/lib/api/invoices", async (importOriginal) => ({ ...(await importOriginal<typeof import("@/lib/api/invoices")>()), getInvoice: m.get }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: m.replace, push: vi.fn() }), usePathname: () => "/en" }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: m.status.current, user: { id: "u1" } }) }));
vi.mock("@/components/layout/public-frame", () => ({ PublicFrame: ({ children }: { children: React.ReactNode }) => <div className="public-frame"><main>{children}</main></div> }));

import { InvoicePage } from "@/components/billing/invoice-page";

const invoice = (over: Partial<Invoice> = {}): Invoice => ({
  id: "inv-1",
  number: "SY-2026-000001",
  kind: "invoice",
  transaction_id: "t1",
  currency: "USD",
  subtotal_cents: 4000,
  tax_cents: 0,
  total_cents: 4000,
  lines: [{ description: "Starter pack", quantity: 1, unit_amount_cents: 4000, amount_cents: 4000, credits: 315 }],
  buyer: { name: "Jo Tester", email: "jo@staryield.test", country: "FR" },
  seller: { legal_name: "Staryield Media LLC", address: ["30 N Gould St, Ste N", "Sheridan, WY 82801", "United States"], email: "support@staryield.net", website: "https://staryield.net" },
  payment: { provider: "stripe", card: { brand: "visa", last4: "4242" }, reference: "pi_3Abc", paid_at: "2026-11-19T10:00:00Z", statement_descriptor: "STARYIELD MEDIA LLC" },
  locale: "en",
  issued_at: "2026-11-19T10:00:05Z",
  ...over,
});

beforeAll(stubResizeObserver);
beforeEach(() => {
  m.status.current = "authenticated";
  m.get.mockResolvedValue(invoice());
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("InvoicePage", () => {
  it("shows a loading state, then the invoice: seller, buyer, lines, totals, payment and descriptor", async () => {
    let resolve: (i: Invoice) => void = () => undefined;
    m.get.mockReturnValue(new Promise<Invoice>((r) => (resolve = r)));
    renderWithI18n(<InvoicePage id="inv-1" />);
    expect(screen.getByTestId("invoice-skeleton").getAttribute("aria-busy")).toBe("true");
    expect(screen.getByRole("status").textContent).toBe("Loading your invoice…");
    resolve(invoice());
    expect((await screen.findByTestId("invoice-number")).textContent).toBe("SY-2026-000001");
    expect(m.get).toHaveBeenCalledWith("inv-1", expect.any(AbortSignal));
    const seller = screen.getByTestId("invoice-seller");
    expect(seller.textContent).toContain("Staryield Media LLC");
    expect(seller.textContent).toContain("30 N Gould St, Ste N");
    expect(seller.textContent).toContain("support@staryield.net");
    const buyer = screen.getByTestId("invoice-buyer");
    expect(buyer.textContent).toContain("Jo Tester");
    expect(buyer.textContent).toContain("jo@staryield.test");
    const lines = screen.getByTestId("invoice-lines");
    expect(within(lines).getByText(/Starter pack/)).toBeTruthy();
    expect(lines.textContent).toContain("315 credits");
    expect(screen.getByTestId("invoice-totals").textContent).toContain("Not applicable");
    expect(screen.getByTestId("invoice-total").textContent).toContain("$40.00 USD");
    const payment = screen.getByTestId("invoice-payment");
    expect(payment.textContent).toContain("Visa ending in 4242");
    expect(payment.textContent).toContain("pi_3Abc");
    expect(screen.getByTestId("invoice-descriptor").textContent).toContain("STARYIELD MEDIA LLC");
    expect(screen.getByTestId("invoice").getAttribute("data-kind")).toBe("invoice");
  });

  it("offers Download PDF (the API's PDF), Print and the way back to billing", async () => {
    const u = userEvent.setup();
    const print = vi.fn();
    vi.stubGlobal("print", print);
    renderWithI18n(<InvoicePage id="inv-1" />);
    const download = await screen.findByTestId("invoice-download");
    expect(download.getAttribute("href")).toMatch(/\/api\/v1\/billing\/invoices\/inv-1\/pdf$/);
    expect(download.hasAttribute("download")).toBe(true);
    expect(download.textContent).toContain("Download PDF");
    await u.click(screen.getByTestId("invoice-print"));
    expect(print).toHaveBeenCalledOnce();
    expect(screen.getByRole("link", { name: "Back to billing" }).getAttribute("href")).toBe("/en/profile/settings#billing");
    vi.unstubAllGlobals();
  });

  it("renders a credit note with the invoice it refunds", async () => {
    m.get.mockResolvedValue(
      invoice({
        id: "cn-1",
        number: "SYC-2026-000001",
        kind: "credit_note",
        original_invoice: { id: "inv-1", number: "SY-2026-000001" },
        payment: { provider: "stripe", refunded_at: "2026-11-20T10:00:00Z", card: { brand: "visa", last4: "4242" } },
      }),
    );
    renderWithI18n(<InvoicePage id="cn-1" />);
    expect((await screen.findByTestId("invoice-number")).textContent).toBe("SYC-2026-000001");
    expect(screen.getByTestId("invoice").getAttribute("data-kind")).toBe("credit_note");
    expect(screen.getByText("Credit note")).toBeTruthy();
    expect(screen.getByTestId("invoice-total").textContent).toContain("Total refunded");
    expect(screen.getByTestId("invoice-payment").textContent).toContain("Refunded on");
    const original = screen.getByRole("link", { name: "SY-2026-000001" });
    expect(original.getAttribute("href")).toBe("/en/profile/invoices/inv-1");
  });

  it("shows the tax line when tax was collected", async () => {
    m.get.mockResolvedValue(invoice({ subtotal_cents: 4000, tax_cents: 800, total_cents: 4800 }));
    renderWithI18n(<InvoicePage id="inv-1" />);
    await screen.findByTestId("invoice");
    expect(screen.getByTestId("invoice-totals").textContent).toContain("$8.00");
    expect(screen.getByTestId("invoice-totals").textContent).not.toContain("Not applicable");
  });

  it("says so when the invoice is not found (another account's, or a wrong link)", async () => {
    m.get.mockRejectedValue(new ApiError({ code: "not_found", message: "x", status: 404 }));
    renderWithI18n(<InvoicePage id="inv-9" />);
    expect((await screen.findByTestId("invoice-not-found")).textContent).toContain("We could not find this invoice");
    expect(screen.queryByTestId("invoice-download")).toBeNull();
    expect(screen.getAllByRole("link", { name: "Back to billing" }).length).toBeGreaterThan(0);
  });

  it("shows other errors with a retry that loads again", async () => {
    const u = userEvent.setup();
    m.get.mockRejectedValueOnce(new ApiError({ code: "service_unavailable", message: "down", status: 503 })).mockResolvedValueOnce(invoice());
    renderWithI18n(<InvoicePage id="inv-1" />);
    await u.click(await screen.findByRole("button", { name: "Try again" }));
    expect(await screen.findByTestId("invoice-number")).toBeTruthy();
    expect(m.get).toHaveBeenCalledTimes(2);
  });

  it("sends a signed-out visitor to the login and back", async () => {
    m.status.current = "anonymous";
    renderWithI18n(<InvoicePage id="inv-1" />);
    expect(m.replace).toHaveBeenCalledWith(`/en/login?next=${encodeURIComponent("/en/profile/invoices/inv-1")}`);
    expect(m.get).not.toHaveBeenCalled();
  });

  it("is accessible", async () => {
    renderWithI18n(<InvoicePage id="inv-1" />);
    await screen.findByTestId("invoice");
    expect(await axeViolations()).toEqual([]);
  });
});
