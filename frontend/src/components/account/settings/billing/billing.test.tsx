// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { ApiError } from "@/lib/api/errors";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";
import { WithSiteConfig } from "@/test/site-config";

const m = vi.hoisted(() => ({
  billingApi: {
    plans: vi.fn(), subscription: vi.fn(), paymentMethods: vi.fn(), transactions: vi.fn(), clientConfig: vi.fn(),
    attachMethod: vi.fn(), subscribe: vi.fn(), cancel: vi.fn(), resume: vi.fn(), removeMethod: vi.fn(), setDefault: vi.fn(), changeMethod: vi.fn(),
  },
}));
// revisions 5.0: the history refreshes when a refund request is resolved (realtime notification).
const realtime = vi.hoisted(() => ({ on: vi.fn() }));
vi.mock("@/lib/api/billing", () => ({ billingApi: m.billingApi }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ user: { id: "u1" } }), useRealtimeEvent: realtime.on }));

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
/** Memberships are on unless a test says otherwise (they are switched off by default, §25.1). */
const view = (memberships: boolean | "absent" = true) =>
  renderWithI18n(
    <WithSiteConfig memberships={memberships === "absent" ? undefined : memberships}>
      <ToastProvider>
        <BillingCard />
      </ToastProvider>
    </WithSiteConfig>,
  );

describe("BillingCard", () => {
  it("shows the plan, empty states and is accessible", async () => {
    view();
    expect(await screen.findByText("You're not a member yet.")).toBeTruthy();
    expect(screen.getByText("$9.99 USD / mo")).toBeTruthy();
    expect(screen.getByText("No saved cards yet.")).toBeTruthy();
    expect(await screen.findByText("No payments yet.")).toBeTruthy();
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
    await u.click(screen.getByTestId("subscribe-renewal"));
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
    expect(await screen.findByText("Accept the Terms of Use, Privacy Policy and Refund & Cancellation Policy to continue.")).toBeTruthy();
    expect(await screen.findByText("Confirm that you understand your membership renews automatically.")).toBeTruthy();
    expect(m.billingApi.subscribe).not.toHaveBeenCalled();
    await u.click(screen.getByTestId("subscribe-consent"));
    await u.click(screen.getByTestId("subscribe-renewal"));
    await u.click(screen.getByTestId("subscribe-confirm"));
    expect(await screen.findByText("Your card was declined. Try another card.")).toBeTruthy();
  });

  it("shows the membership checkout terms: price with currency, rhythm, renewal date, how to cancel, descriptor", async () => {
    vi.useFakeTimers({ toFake: ["Date"], now: new Date(2026, 0, 31, 12, 0, 0) });
    try {
      const u = userEvent.setup({ advanceTimers: () => undefined });
      view();
      await u.click(await screen.findByTestId("membership-subscribe"));
      const dialog = await screen.findByTestId("subscribe-dialog");
      expect(within(dialog).getByText("Join Premium Membership")).toBeTruthy();
      expect(within(dialog).getByText("$9.99 USD / mo")).toBeTruthy();
      const terms = within(dialog).getByTestId("subscribe-terms");
      expect(terms.textContent).toContain("Billed every month until you cancel.");
      // Clamped to the last day of February, not rolled into March.
      expect(terms.textContent).toContain(`Next renewal: ${new Intl.DateTimeFormat("en", { dateStyle: "long" }).format(new Date(2026, 1, 28, 12))}.`);
      expect(terms.textContent).toContain("Cancel any time online in Settings → Billing.");
      expect(terms.textContent).toContain("This charge appears on your card statement as STARYIELD MEDIA LLC.");
      expect(within(dialog).getByRole("button", { name: "Subscribe for $9.99 USD / mo" })).toBeTruthy();
    } finally {
      vi.useRealTimers();
    }
  });

  it("has two required, unchecked agreements: the policies (links in new tabs) and automatic renewal", async () => {
    const u = userEvent.setup();
    view();
    await u.click(await screen.findByTestId("membership-subscribe"));
    const dialog = await screen.findByTestId("subscribe-dialog");
    const policies = within(dialog).getByTestId("subscribe-consent");
    const renewal = within(dialog).getByTestId("subscribe-renewal");
    expect(policies.getAttribute("aria-checked")).toBe("false");
    expect(renewal.getAttribute("aria-checked")).toBe("false");
    expect(within(dialog).getByRole("checkbox", { name: /^I agree to the Terms of Use.*, Privacy Policy.* and Refund & Cancellation Policy.*\.$/ })).toBe(policies);
    expect(within(dialog).getByRole("checkbox", { name: "I understand my membership renews automatically every month until I cancel." })).toBe(renewal);
    const links = within(policies.closest("[data-slot=field]") as HTMLElement).getAllByRole("link");
    expect(links.map((l) => l.getAttribute("href"))).toEqual(["/en/terms", "/en/privacy", "/en/refund-policy"]);
    for (const link of links) expect(link.getAttribute("target")).toBe("_blank");
    // Only one of the two is not enough.
    m.billingApi.paymentMethods.mockResolvedValue([card]);
    await u.click(policies);
    await u.click(within(dialog).getByTestId("subscribe-confirm"));
    expect(await within(dialog).findByText("Confirm that you understand your membership renews automatically.")).toBeTruthy();
    expect(m.billingApi.subscribe).not.toHaveBeenCalled();
    expect(await axeViolations()).toEqual([]);
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

describe("BillingCard with memberships switched off (§25.1)", () => {
  const offStates: [string, boolean | "absent"][] = [
    ["features absent (older API)", "absent"],
    ["features.memberships false", false],
  ];

  it.each(offStates)("shows no membership surface when %s, only saved cards and payments", async (_name, flag) => {
    view(flag);
    expect(await screen.findByText("No saved cards yet.")).toBeTruthy();
    expect(await screen.findByText("No payments yet.")).toBeTruthy();
    expect(screen.getByRole("heading", { level: 2, name: "Billing" })).toBeTruthy();
    for (const hidden of ["membership", "membership-subscribe", "membership-status", "membership-cancel", "membership-change-card", "membership-resume"])
      expect(screen.queryByTestId(hidden)).toBeNull();
    const card = screen.getByTestId("settings-billing");
    for (const word of [/Active Plan/i, /Premium Membership/i, /Subscribe/i, /member/i, /subscription/i]) expect(card.textContent).not.toMatch(word);
    expect(m.billingApi.plans).not.toHaveBeenCalled();
    expect(await axeViolations()).toEqual([]);
  });

  it("an account that still has a membership can see and cancel it, but not subscribe, change card or resume", async () => {
    const u = userEvent.setup();
    m.billingApi.subscription.mockResolvedValue(sub());
    m.billingApi.paymentMethods.mockResolvedValue([card]);
    m.billingApi.cancel.mockResolvedValue(sub({ cancel_at_period_end: true }));
    view(false);
    expect(await screen.findByTestId("membership-status")).toBeTruthy();
    expect(screen.getByText("Active Plan")).toBeTruthy();
    expect(screen.queryByTestId("membership-subscribe")).toBeNull();
    expect(screen.queryByTestId("membership-change-card")).toBeNull();
    await u.click(screen.getByTestId("membership-cancel"));
    await u.click(await screen.findByTestId("membership-cancel-confirm"));
    await waitFor(() => expect(m.billingApi.cancel).toHaveBeenCalled());
    expect(await screen.findByText(/Ends on/)).toBeTruthy();
    // Once it is set to end, there is nothing left to do (resuming is refused by the API while off).
    expect(screen.queryByTestId("membership-resume")).toBeNull();
    expect(screen.queryByTestId("membership-cancel")).toBeNull();
    expect(screen.queryByTestId("membership-subscribe")).toBeNull();
  });

  it("an ended membership shows nothing while memberships are off", async () => {
    m.billingApi.subscription.mockResolvedValue(sub({ status: "canceled", ended_at: "2026-11-01T00:00:00Z" }));
    view(false);
    expect(await screen.findByText("No saved cards yet.")).toBeTruthy();
    expect(screen.queryByTestId("membership")).toBeNull();
  });

  it("says billing (not memberships) is unavailable when the API refuses", async () => {
    m.billingApi.paymentMethods.mockRejectedValue(new ApiError({ code: "feature_unavailable", message: "x", status: 503 }));
    view(false);
    expect(await screen.findByText("Billing is not available yet.")).toBeTruthy();
  });

  it("shows the whole membership card when the API turns memberships on", async () => {
    view(true);
    expect(await screen.findByTestId("membership")).toBeTruthy();
    expect(screen.getByRole("heading", { level: 2, name: "Billing & Subscriptions" })).toBeTruthy();
    expect(screen.getByTestId("membership-subscribe")).toBeTruthy();
    expect(m.billingApi.plans).toHaveBeenCalled();
  });
});

