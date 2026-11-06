// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { ApiError } from "@/lib/api/errors";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const m = vi.hoisted(() => ({
  billingApi: {
    plans: vi.fn(), subscription: vi.fn(), paymentMethods: vi.fn(), transactions: vi.fn(), clientConfig: vi.fn(),
    attachMethod: vi.fn(), subscribe: vi.fn(), cancel: vi.fn(), resume: vi.fn(), removeMethod: vi.fn(), setDefault: vi.fn(), changeMethod: vi.fn(),
  },
}));
vi.mock("@/lib/api/billing", () => ({ billingApi: m.billingApi }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ user: { id: "u1" } }) }));

import { BillingCard } from "@/components/account/settings/billing/billing-card";

const plan = {
  id: "p1", code: "premium", name: "Premium Membership", description: "", version_id: "v1", version: 1, price_cents: 999, currency: "USD",
  interval: "month", interval_count: 1, monthly_credit_grant: 0, chat_discount_bps: 0, chat_discount_pct: 0, monthly_free_minutes: 0, waitlist_priority: false, benefits: [],
};
const card = { id: "m1", provider: "sandbox_a", brand: "visa", last4: "4242", exp_month: 12, exp_year: 2039, is_default: true, status: "active", expired: false, created_at: "2026-01-01T00:00:00Z" };
const sub = (over = {}) => ({ id: "s1", status: "active", plan, cancel_at_period_end: false, current_period_end: "2026-12-05T00:00:00Z", renews_at: "2026-12-05T00:00:00Z", payment_method: card, created_at: "2026-11-05T00:00:00Z", ...over });
const config = { provider: "sandbox_a", type: "sandbox", token_format: "sbx_a_<brand>_<last4>_<mm>_<yyyy>", test_cards: [{ brand: "visa", last4: "4242", behaviour: "approved" }] };

beforeAll(stubResizeObserver);
beforeEach(() => {
  m.billingApi.plans.mockResolvedValue([plan]);
  m.billingApi.subscription.mockResolvedValue(null);
  m.billingApi.paymentMethods.mockResolvedValue([]);
  m.billingApi.transactions.mockResolvedValue({ items: [] });
  m.billingApi.clientConfig.mockResolvedValue(config);
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

describe("BillingCard", () => {
  it("shows the plan, empty states and is accessible", async () => {
    view();
    expect(await screen.findByText("You're not a member yet.")).toBeTruthy();
    expect(screen.getByText("$9.99 / mo")).toBeTruthy();
    expect(screen.getByText("No saved cards yet.")).toBeTruthy();
    expect(screen.getByText("No payments yet.")).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
  });

  it("subscribes with a new sandbox card: only a token reaches the API", async () => {
    const u = userEvent.setup();
    m.billingApi.attachMethod.mockResolvedValue(card);
    m.billingApi.subscribe.mockResolvedValue({ subscription: sub() });
    view();
    await u.click(await screen.findByTestId("membership-subscribe"));
    await u.type(await screen.findByLabelText("Cardholder name"), "Jane Doe");
    await u.type(screen.getByLabelText("Card number"), "4242424242424242");
    await u.type(screen.getByLabelText("Expiry date"), "1239");
    await u.type(screen.getByLabelText("CVV"), "123");
    await u.click(screen.getByTestId("subscribe-consent"));
    await u.click(screen.getByTestId("subscribe-confirm"));
    await waitFor(() => expect(m.billingApi.subscribe).toHaveBeenCalled());
    const attach = m.billingApi.attachMethod.mock.calls[0][0];
    expect(attach).toMatchObject({ provider: "sandbox_a", token: "sbx_a_visa_4242_12_2039" });
    expect(JSON.stringify(m.billingApi.attachMethod.mock.calls)).not.toContain("4242424242424242");
    expect(m.billingApi.subscribe.mock.calls[0][0]).toEqual({ plan_version_id: "v1", payment_method_id: "m1" });
    expect(await screen.findByTestId("membership-status")).toBeTruthy();
  });

  it("requires the auto-renewal consent and shows a decline", async () => {
    const u = userEvent.setup();
    m.billingApi.paymentMethods.mockResolvedValue([card]);
    m.billingApi.subscribe.mockRejectedValue(new ApiError({ code: "payment_failed", message: "x", status: 402, details: { reason: "card_declined" } }));
    view();
    await u.click(await screen.findByTestId("membership-subscribe"));
    await u.click(await screen.findByTestId("subscribe-confirm"));
    expect(await screen.findByText("Please confirm to continue.")).toBeTruthy();
    expect(m.billingApi.subscribe).not.toHaveBeenCalled();
    await u.click(screen.getByTestId("subscribe-consent"));
    await u.click(screen.getByTestId("subscribe-confirm"));
    expect(await screen.findByText("Your card was declined. Try another card.")).toBeTruthy();
  });

  it("cancels at period end, then resumes", async () => {
    const u = userEvent.setup();
    m.billingApi.subscription.mockResolvedValue(sub());
    m.billingApi.paymentMethods.mockResolvedValue([card]);
    m.billingApi.cancel.mockResolvedValue(sub({ cancel_at_period_end: true }));
    m.billingApi.resume.mockResolvedValue(sub());
    view();
    await u.click(await screen.findByTestId("membership-cancel"));
    await u.click(await screen.findByTestId("membership-cancel-confirm"));
    expect(await screen.findByText(/Ends on/)).toBeTruthy();
    await u.click(screen.getByTestId("membership-resume"));
    await waitFor(() => expect(m.billingApi.resume).toHaveBeenCalled());
    expect(await screen.findByText(/Renews on/)).toBeTruthy();
  });

  it("lists, defaults and removes saved cards", async () => {
    const u = userEvent.setup();
    const second = { ...card, id: "m2", last4: "4444", brand: "mastercard", is_default: false };
    m.billingApi.paymentMethods.mockResolvedValue([card, second]);
    m.billingApi.setDefault.mockResolvedValue({ ...second, is_default: true });
    m.billingApi.removeMethod.mockResolvedValue(undefined);
    view();
    await screen.findAllByText("Mastercard ending in 4444");
    await u.click(screen.getByRole("button", { name: /Make default/ }));
    await waitFor(() => expect(m.billingApi.setDefault).toHaveBeenCalledWith("m2"));
    await u.click(screen.getAllByRole("button", { name: /^Remove/ })[0]);
    await u.click(await screen.findByRole("button", { name: "Remove card" }));
    await waitFor(() => expect(m.billingApi.removeMethod).toHaveBeenCalledWith("m1"));
  });

  it("pages the transaction history", async () => {
    const u = userEvent.setup();
    const t = (id: string, d: string) => ({ id, kind: "subscription_renewal", status: "succeeded", amount_cents: 999, refunded_amount_cents: 0, currency: "USD", description: d, created_at: "2026-10-24T10:00:00Z" });
    m.billingApi.transactions.mockResolvedValueOnce({ items: [t("t1", "First page")], next_cursor: "c2" }).mockResolvedValueOnce({ items: [t("t2", "Second page")] });
    view();
    expect(await screen.findByText("First page")).toBeTruthy();
    await u.click(screen.getByRole("button", { name: /Next/ }));
    expect(await screen.findByText("Second page")).toBeTruthy();
  });

  it("offers a retry when billing fails to load", async () => {
    m.billingApi.plans.mockRejectedValueOnce(new ApiError({ code: "service_unavailable", message: "x", status: 503 }));
    view();
    await userEvent.setup().click(await screen.findByRole("button", { name: "Try again" }));
    expect(await screen.findByText("You're not a member yet.")).toBeTruthy();
  });
});
