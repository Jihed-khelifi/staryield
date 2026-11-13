// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { ApiError } from "@/lib/api/errors";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const m = vi.hoisted(() => ({
  billingApi: { plans: vi.fn(), subscription: vi.fn(), paymentMethods: vi.fn(), transactions: vi.fn() },
  refundsApi: { quote: vi.fn(), create: vi.fn(), withdraw: vi.fn(), list: vi.fn() },
  handlers: {} as Record<string, (data: unknown) => void>,
}));
vi.mock("@/lib/api/billing", () => ({ billingApi: m.billingApi }));
vi.mock("@/lib/api/pricing", () => ({ refundsApi: m.refundsApi }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ user: { id: "u1" } }),
  useRealtimeEvent: (type: string, handler: (data: unknown) => void) => {
    m.handlers[type] = handler;
  },
}));

import { BillingCard } from "@/components/account/settings/billing/billing-card";

const tx = (id: string, refund?: unknown, over: Record<string, unknown> = {}) => ({
  id, kind: "credit_purchase", status: "succeeded", amount_cents: 4000, refunded_amount_cents: 0, currency: "USD",
  description: `Pack ${id}`, credits: 315, created_at: "2026-11-05T10:00:00Z", ...(refund ? { refund } : {}), ...over,
});
const info = (over: Record<string, unknown> = {}) => ({ refundable_cents: 3200, credits_remaining: 250, window_ends_at: "2026-11-19T10:00:00Z", can_request: true, ...over });
const quote = (over: Record<string, unknown> = {}) => ({
  transaction_id: "t1", pack_name: "Starter", currency: "USD", amount_paid_cents: 4000, already_refunded_cents: 0, credits_purchased: 315, credits_used: 65,
  credits_remaining: 250, refundable_cents: 3200, purchased_at: "2026-11-05T10:00:00Z", window_ends_at: "2026-11-19T10:00:00Z", within_window: true, ...over,
});
const request = (over: Record<string, unknown> = {}) => ({
  id: "r1", reference: "RR-AB12CD34", transaction_id: "t1", status: "open", reason: "unused_credits", details: "", channel: "web", quote: quote(),
  created_at: "2026-11-12T10:00:00Z", updated_at: "2026-11-12T10:00:00Z", ...over,
});
const err = (code: string, status: number, details?: Record<string, unknown>) => new ApiError({ code, message: code, status, details });

beforeAll(() => {
  stubResizeObserver();
  // Radix Select needs pointer capture and scrolling, which jsdom lacks.
  Element.prototype.hasPointerCapture ??= () => false;
  Element.prototype.setPointerCapture ??= () => undefined;
  Element.prototype.releasePointerCapture ??= () => undefined;
  Element.prototype.scrollIntoView ??= () => undefined;
});
beforeEach(() => {
  m.billingApi.plans.mockResolvedValue([]);
  m.billingApi.subscription.mockResolvedValue(null);
  m.billingApi.paymentMethods.mockResolvedValue([]);
  m.billingApi.transactions.mockResolvedValue({ items: [tx("t1", info())] });
  m.refundsApi.quote.mockResolvedValue(quote());
  for (const key of Object.keys(m.handlers)) delete m.handlers[key];
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
const view = () =>
  renderWithI18n(
    <ToastProvider>
      <BillingCard />
    </ToastProvider>,
  );

async function chooseReason(u: ReturnType<typeof userEvent.setup>, name: RegExp | string) {
  await u.click(await screen.findByTestId("refund-reason"));
  await u.click(await screen.findByRole("option", { name }));
}
async function openRequest(u: ReturnType<typeof userEvent.setup>) {
  await u.click(await screen.findByTestId("refund-request"));
  await screen.findByTestId("refund-quote");
}

describe("refund state in the billing history (§24.4)", () => {
  it("offers 'Request a refund' only where the API says one can be sent", async () => {
    m.billingApi.transactions.mockResolvedValue({
      items: [
        tx("t1", info()),
        tx("t2", info({ can_request: false })),
        tx("t3", undefined, { kind: "subscription_renewal", description: "Premium" }),
      ],
    });
    view();
    await screen.findByText("Pack t1");
    expect(screen.getAllByTestId("refund-request")).toHaveLength(1);
    expect(screen.getByTestId("refund-request").textContent).toContain("Request a refund");
    expect(screen.getByTestId("refund-request").textContent).toContain("for the payment on");
  });

  it("shows an open request's status, reference and Withdraw", async () => {
    m.billingApi.transactions.mockResolvedValue({ items: [tx("t1", info({ can_request: false, request: { id: "r1", reference: "RR-AB12CD34", status: "open" } }))] });
    view();
    const line = await screen.findByTestId("refund-status");
    expect(within(line).getByText("Refund requested")).toBeTruthy();
    expect(screen.getByTestId("refund-reference-label").textContent).toBe("Reference RR-AB12CD34");
    expect(within(line).getByTestId("refund-withdraw")).toBeTruthy();
    expect(screen.queryByTestId("refund-request")).toBeNull();
  });

  it.each([
    ["approved", "Refund approved"],
    ["denied", "Refund declined"],
    ["credited", "Credits granted"],
    ["withdrawn", "Request withdrawn"],
  ])("shows a %s request as %s, without Withdraw", async (status, label) => {
    m.billingApi.transactions.mockResolvedValue({ items: [tx("t1", info({ can_request: false, request: { id: "r1", reference: "RR-AB12CD34", status } }))] });
    view();
    const line = await screen.findByTestId("refund-status");
    expect(within(line).getByText(label)).toBeTruthy();
    expect(within(line).queryByTestId("refund-withdraw")).toBeNull();
  });

  it("refreshes the history when a refund request is resolved (realtime), and only then", async () => {
    view();
    await screen.findByText("Pack t1");
    expect(m.billingApi.transactions).toHaveBeenCalledTimes(1);
    m.handlers["notification.created"]?.({ notification: { id: "n1", kind: "chat.requested" } });
    m.handlers["notification.created"]?.({});
    expect(m.billingApi.transactions).toHaveBeenCalledTimes(1);
    m.billingApi.transactions.mockResolvedValue({ items: [tx("t1", info({ can_request: false, request: { id: "r1", reference: "RR-AB12CD34", status: "approved" } }))] });
    m.handlers["notification.created"]?.({ notification: { id: "n2", kind: "refund_request.resolved", data: { status: "approved" } } });
    expect(await screen.findByText("Refund approved")).toBeTruthy();
    expect(m.billingApi.transactions).toHaveBeenCalledTimes(2);
  });
});

describe("request a refund", () => {
  it("fetches a quote (loading first) and shows credits used and remaining, the estimate and the window", async () => {
    const u = userEvent.setup();
    let resolve: (q: unknown) => void = () => undefined;
    m.refundsApi.quote.mockReturnValue(new Promise((r) => (resolve = r)));
    view();
    await u.click(await screen.findByTestId("refund-request"));
    expect(screen.getByTestId("refund-quote-loading").getAttribute("aria-busy")).toBe("true");
    resolve(quote());
    const summary = await screen.findByTestId("refund-quote");
    expect(m.refundsApi.quote).toHaveBeenCalledWith("t1", expect.anything());
    expect(summary.textContent).toContain("Starter · $40.00");
    expect(within(summary).getByText("Credits used").nextSibling?.textContent).toBe("65");
    expect(within(summary).getByText("Credits remaining").nextSibling?.textContent).toBe("250");
    expect(within(summary).getByText("Refundable estimate").nextSibling?.textContent).toBe("up to $32.00");
    expect(within(summary).getByText("Refund window ends").nextSibling?.textContent).toBe("Nov 19, 2026");
    expect(await axeViolations()).toEqual([]);
  });

  it("offers the quote's error with a retry", async () => {
    const u = userEvent.setup();
    m.refundsApi.quote.mockRejectedValueOnce(err("service_unavailable", 503));
    view();
    await u.click(await screen.findByTestId("refund-request"));
    await u.click(await screen.findByRole("button", { name: "Try again" }));
    expect(await screen.findByTestId("refund-quote")).toBeTruthy();
    expect(m.refundsApi.quote).toHaveBeenCalledTimes(2);
  });

  it("offers e-mail when in-app requests are switched off", async () => {
    const u = userEvent.setup();
    m.refundsApi.quote.mockRejectedValue(err("feature_unavailable", 503));
    view();
    await u.click(await screen.findByTestId("refund-request"));
    const email = within(await screen.findByTestId("refund-email")).getByRole("link", { name: /support@staryield.net/ });
    expect(email.getAttribute("href")).toBe("mailto:support@staryield.net?subject=Refund%20Request");
  });

  it("outside the window the unused-credits reason is not offered", async () => {
    const u = userEvent.setup();
    m.refundsApi.quote.mockResolvedValue(quote({ within_window: false }));
    view();
    await openRequest(u);
    expect(screen.getByTestId("refund-outside-window")).toBeTruthy();
    await u.click(screen.getByTestId("refund-reason"));
    expect((await screen.findByRole("option", { name: "I have credits I haven't used" })).getAttribute("aria-disabled")).toBe("true");
    expect(screen.getAllByRole("option")).toHaveLength(6);
  });

  it("needs a reason, counts the details and links the policy in a new tab", async () => {
    const u = userEvent.setup();
    view();
    await openRequest(u);
    expect(screen.getByText("0/2000")).toBeTruthy();
    await u.type(screen.getByTestId("refund-details"), "Hello");
    expect(screen.getByText("5/2000")).toBeTruthy();
    const policy = screen.getByTestId("refund-policy-link");
    expect(policy.getAttribute("href")).toBe("/en/refund-policy");
    expect(policy.getAttribute("target")).toBe("_blank");
    await u.click(screen.getByTestId("refund-submit"));
    expect(await screen.findByText("Choose a reason.")).toBeTruthy();
    expect(m.refundsApi.create).not.toHaveBeenCalled();
  });

  it("sends the request with one idempotency key, then shows the reference and the acknowledgement promise, and the row updates", async () => {
    const u = userEvent.setup();
    m.refundsApi.create.mockResolvedValue(request());
    view();
    await openRequest(u);
    await chooseReason(u, "I have credits I haven't used");
    await u.type(screen.getByTestId("refund-details"), "  The reading never happened.  ");
    await u.click(screen.getByTestId("refund-submit"));
    const success = await screen.findByTestId("refund-success");
    expect(m.refundsApi.create).toHaveBeenCalledTimes(1);
    const [input, key] = m.refundsApi.create.mock.calls[0];
    expect(input).toEqual({ transaction_id: "t1", reason: "unused_credits", details: "The reading never happened." });
    expect(typeof key).toBe("string");
    expect(key.length).toBeGreaterThanOrEqual(8);
    expect(within(success).getByTestId("refund-reference").textContent).toBe("RR-AB12CD34");
    expect(success.textContent).toContain("We aim to acknowledge requests within 2 business days.");
    await u.click(screen.getByTestId("refund-done"));
    const line = await screen.findByTestId("refund-status");
    expect(within(line).getByText("Refund requested")).toBeTruthy();
    expect(screen.queryByTestId("refund-request")).toBeNull();
  });

  it("omits empty details", async () => {
    const u = userEvent.setup();
    m.refundsApi.create.mockResolvedValue(request());
    view();
    await openRequest(u);
    await chooseReason(u, "Something else");
    await u.click(screen.getByTestId("refund-submit"));
    await screen.findByTestId("refund-success");
    expect(m.refundsApi.create.mock.calls[0][0]).toEqual({ transaction_id: "t1", reason: "other" });
  });

  it("shows the existing request when one is already open", async () => {
    const u = userEvent.setup();
    m.refundsApi.create.mockRejectedValue(err("refund_request_exists", 409, { request_id: "r9" }));
    view();
    await openRequest(u);
    await chooseReason(u, "Something else");
    m.billingApi.transactions.mockResolvedValue({ items: [tx("t1", info({ can_request: false, request: { id: "r9", reference: "RR-ZZ00AA11", status: "open" } }))] });
    await u.click(screen.getByTestId("refund-submit"));
    expect(await screen.findByText(/You already have an open refund request for this payment/)).toBeTruthy();
    // The history behind the dialog was reloaded and shows it.
    await waitFor(() => expect(screen.getByTestId("refund-reference-label").textContent).toBe("Reference RR-ZZ00AA11"));
  });

  it.each([
    ["fully_refunded", "This payment has already been fully refunded."],
    ["not_paid", "This payment was not completed, so there is nothing to refund."],
    ["not_credit_purchase", "Only credit purchases can be refunded here."],
    ["outside_window", "The refund window for unused credits has ended. You can still tell us about a problem with this payment."],
  ])("says why a payment is not eligible (%s)", async (reason, message) => {
    const u = userEvent.setup();
    m.refundsApi.create.mockRejectedValue(err("refund_not_eligible", 409, { reason }));
    view();
    await openRequest(u);
    await chooseReason(u, "Something else");
    await u.click(screen.getByTestId("refund-submit"));
    expect(await screen.findByText(message, { selector: "p" })).toBeTruthy();
  });

  it("offers e-mail when requests are switched off at the moment of sending", async () => {
    const u = userEvent.setup();
    m.refundsApi.create.mockRejectedValue(err("feature_unavailable", 503));
    view();
    await openRequest(u);
    await chooseReason(u, "Something else");
    await u.click(screen.getByTestId("refund-submit"));
    expect(await screen.findByTestId("refund-email")).toBeTruthy();
    expect(screen.queryByTestId("refund-form")).toBeNull();
  });

  it("tells a rate limit and offers e-mail", async () => {
    const u = userEvent.setup();
    m.refundsApi.create.mockRejectedValue(err("rate_limited", 429));
    view();
    await openRequest(u);
    await chooseReason(u, "Something else");
    await u.click(screen.getByTestId("refund-submit"));
    expect(await screen.findByText(/You have sent several refund requests recently/)).toBeTruthy();
    expect(screen.getByRole("link", { name: /support@staryield.net/ })).toBeTruthy();
  });

  it("retries a network failure with the same key", async () => {
    const u = userEvent.setup();
    m.refundsApi.create.mockRejectedValueOnce(err("network_error", 0)).mockResolvedValueOnce(request());
    view();
    await openRequest(u);
    await chooseReason(u, "Something else");
    await u.click(screen.getByTestId("refund-submit"));
    await u.click(await screen.findByTestId("refund-retry"));
    expect(await screen.findByTestId("refund-success")).toBeTruthy();
    expect(m.refundsApi.create.mock.calls[1][1]).toBe(m.refundsApi.create.mock.calls[0][1]);
  });
});

describe("withdraw", () => {
  const openRow = () => m.billingApi.transactions.mockResolvedValue({ items: [tx("t1", info({ can_request: false, request: { id: "r1", reference: "RR-AB12CD34", status: "open" } }))] });

  it("asks first, then withdraws and refreshes the row", async () => {
    const u = userEvent.setup();
    openRow();
    m.refundsApi.withdraw.mockResolvedValue(request({ status: "withdrawn" }));
    view();
    await u.click(await screen.findByTestId("refund-withdraw"));
    const dialog = await screen.findByTestId("refund-withdraw-dialog");
    expect(dialog.textContent).toContain("Request RR-AB12CD34 will be closed.");
    expect(m.refundsApi.withdraw).not.toHaveBeenCalled();
    m.billingApi.transactions.mockResolvedValue({ items: [tx("t1", info({ can_request: true, request: { id: "r1", reference: "RR-AB12CD34", status: "withdrawn" } }))] });
    await u.click(screen.getByTestId("refund-withdraw-confirm"));
    await waitFor(() => expect(m.refundsApi.withdraw).toHaveBeenCalledWith("r1"));
    expect(await screen.findByText("Request withdrawn")).toBeTruthy();
    // It can be requested again.
    expect(await screen.findByTestId("refund-request")).toBeTruthy();
    expect(screen.queryByTestId("refund-withdraw-dialog")).toBeNull();
  });

  it("keeps the request when the person changes their mind", async () => {
    const u = userEvent.setup();
    openRow();
    view();
    await u.click(await screen.findByTestId("refund-withdraw"));
    await u.click(await screen.findByRole("button", { name: "Keep my request" }));
    expect(m.refundsApi.withdraw).not.toHaveBeenCalled();
    expect(screen.getByTestId("refund-status")).toBeTruthy();
  });

  it("a request that was decided meanwhile just refreshes the history", async () => {
    const u = userEvent.setup();
    openRow();
    m.refundsApi.withdraw.mockRejectedValue(err("invalid_state", 409));
    view();
    await u.click(await screen.findByTestId("refund-withdraw"));
    m.billingApi.transactions.mockResolvedValue({ items: [tx("t1", info({ can_request: false, request: { id: "r1", reference: "RR-AB12CD34", status: "approved" } }))] });
    await u.click(await screen.findByTestId("refund-withdraw-confirm"));
    expect(await screen.findByText("Refund approved")).toBeTruthy();
    expect(await screen.findByText("This request can no longer be withdrawn.")).toBeTruthy();
  });

  it("shows other failures in the dialog", async () => {
    const u = userEvent.setup();
    openRow();
    m.refundsApi.withdraw.mockRejectedValue(err("network_error", 0));
    view();
    await u.click(await screen.findByTestId("refund-withdraw"));
    await u.click(await screen.findByTestId("refund-withdraw-confirm"));
    expect((await within(await screen.findByTestId("refund-withdraw-dialog")).findByRole("alert")).textContent).toMatch(/offline|couldn't reach/i);
  });
});
