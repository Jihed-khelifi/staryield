// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { ApiError } from "@/lib/api/errors";
import { LEGAL_VERSIONS, purchaseConsent } from "@/lib/legal/versions";
import type { PricingPack, TopUpCatalog } from "@/lib/api/pricing";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const m = vi.hoisted(() => ({
  catalog: vi.fn(),
  paymentMethods: vi.fn(),
  clientConfig: vi.fn(),
  purchase: vi.fn(),
  setWallet: vi.fn(),
  track: vi.fn(),
  free: { current: false },
}));
vi.mock("@/lib/api/pricing", () => ({ pricingApi: { catalog: m.catalog } }));
vi.mock("@/lib/api/billing", () => ({ billingApi: { paymentMethods: m.paymentMethods, clientConfig: m.clientConfig }, purchaseCredits: m.purchase }));
vi.mock("@/lib/analytics", () => ({ track: m.track }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ status: "authenticated", user: { id: "u1", email: "jo@staryield.test", display_name: "Jo" }, setWallet: m.setWallet }),
}));
vi.mock("@/lib/site-config/site-config-provider", () => ({ useFreeMode: () => m.free.current }));

import { CreditTopUp } from "@/components/billing/top-up/credit-top-up";

const pack = (code: string, over: Partial<PricingPack> = {}): PricingPack => ({
  id: `id_${code}`, code, kind: "standard", name: code, tagline: "", label: "", credits: 100, price_cents: 1000, currency: "USD",
  featured: false, new_clients_only: false, sort_order: 1, badges: [], ...over,
});
const welcome = pack("welcome_600", { kind: "intro", name: "Welcome Pack", credits: 600, price_cents: 1000, new_clients_only: true, badges: [{ type: "new_client_offer" }] });
const quick = pack("quick_150", { name: "Quick Insight", credits: 150, price_cents: 2000, label: "One focused question" });
const starter = pack("starter_315", { name: "Starter", credits: 315, price_cents: 4000, badges: [{ type: "save", percent: 4 }] });
const popular = pack("popular_675", { name: "Most Popular", credits: 675, price_cents: 8000, featured: true, badges: [{ type: "most_popular" }, { type: "save", percent: 11 }] });
const wallet = { balance: 120, held: 0, free_minutes: 0 };
const eligible: TopUpCatalog = { packs: [welcome, quick, starter, popular], welcome_offer: { eligible: true, pack_id: welcome.id }, rate_range: { min: 30, max: 60 }, wallet };
const redeemed: TopUpCatalog = { packs: [quick, starter, popular], welcome_offer: { eligible: false }, rate_range: { min: 30, max: 60 }, wallet };
const config = { provider: "sandbox_a", type: "sandbox", token_format: "sbx_a_<brand>_<last4>_<mm>_<yyyy>", test_cards: [] };
const bought = (credits = 600) => ({ id: "pu1", pack_id: "x", credits, price_cents: 1000, currency: "USD", status: "succeeded", provider: "sandbox_a", created_at: "2026-11-12T10:00:00Z", wallet: { balance: 720, held: 0, free_minutes: 0 } });
const err = (code: string, status: number, details?: Record<string, unknown>) => new ApiError({ code, message: code, status, details });

const onClose = vi.fn();
const view = (props: Partial<React.ComponentProps<typeof CreditTopUp>> = {}) =>
  renderWithI18n(
    <ToastProvider>
      <CreditTopUp onClose={onClose} {...props} />
    </ToastProvider>,
  );

async function fillCard(u: ReturnType<typeof userEvent.setup>) {
  await u.type(await screen.findByLabelText("Card number"), "4242424242424242");
  await u.type(screen.getByLabelText("Expiry date"), "1239");
  await u.type(screen.getByLabelText("CVV"), "123");
}
/** Ticks the required policy agreement (§25.6) unless it already is. */
async function agree(u: ReturnType<typeof userEvent.setup>) {
  const box = await screen.findByTestId("top-up-consent");
  if (box.getAttribute("aria-checked") !== "true") await u.click(box);
}
const radio = (name: RegExp | string) => screen.getByRole("radio", { name });
async function toPayment(u: ReturnType<typeof userEvent.setup>) {
  await u.click(await screen.findByTestId("top-up-continue"));
  await screen.findByTestId("checkout-pay");
}

beforeAll(stubResizeObserver);
beforeEach(() => {
  m.catalog.mockResolvedValue(eligible);
  m.paymentMethods.mockResolvedValue([]);
  m.clientConfig.mockResolvedValue(config);
  m.free.current = false;
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("step 1: choose a pack", () => {
  it("shows a loading skeleton, then the packs", async () => {
    let resolve: (c: TopUpCatalog) => void = () => undefined;
    m.catalog.mockReturnValue(new Promise<TopUpCatalog>((r) => (resolve = r)));
    view();
    expect(screen.getByTestId("top-up-loading").getAttribute("aria-busy")).toBe("true");
    expect(within(screen.getByTestId("top-up-loading")).getByRole("status").textContent).toBe("Loading credit packs…");
    resolve(eligible);
    expect(await screen.findByRole("radiogroup", { name: "Credit packs" })).toBeTruthy();
    expect(screen.queryByTestId("top-up-loading")).toBeNull();
  });

  it("an eligible client sees the Welcome Pack first and preselected, with the exact dock copy", async () => {
    view();
    const group = await screen.findByRole("radiogroup", { name: "Credit packs" });
    const radios = within(group).getAllByRole("radio");
    expect(radios).toHaveLength(4);
    expect(radios[0].textContent).toContain("Welcome Pack");
    expect(radios[0].getAttribute("aria-checked")).toBe("true");
    expect(radios[0].textContent).toContain("Selected");
    expect(radios[0].textContent).toContain("Your one-time new-client offer.");
    expect(radios[1].textContent).toContain("Select pack");
    expect(screen.getByTestId("top-up-dock-pack").textContent).toBe("Welcome Pack · 600 credits");
    expect(screen.getByTestId("top-up-total").textContent).toBe("Total: $10 USD");
    expect(screen.getByText("One-time payment")).toBeTruthy();
    expect(screen.getByTestId("top-up-continue").textContent).toBe("Continue to Checkout — $10");
    // The links also tell assistive technology that they open a new tab.
    expect(screen.getByTestId("top-up-ack").textContent?.replaceAll(" (opens in a new tab)", "")).toBe(
      "By continuing, you agree to purchase 600 credits for $10 USD as a one-time payment. No subscription or automatic top-up is included. Review our Terms and Refund Policy.",
    );
    expect(screen.getByTestId("top-up-dismiss").textContent).toBe("Not now");
    expect(m.purchase).not.toHaveBeenCalled();
  });

  it("each card shows its badges (or label), credits, USD price, minute range and 'One-time purchase'", async () => {
    view();
    await screen.findByRole("radiogroup");
    const popularCard = screen.getByTestId("top-up-pack-popular_675");
    expect(popularCard.textContent).toContain("Most popular · Save 11%");
    expect(popularCard.textContent).toContain("Most Popular");
    expect(popularCard.textContent).toContain("675 credits");
    expect(popularCard.textContent).toContain("$80 USD");
    expect(popularCard.textContent).toContain("≈ 22.5 min. at 30 credits/min. · ≈ 11.25 min. at 60 credits/min.");
    expect(popularCard.textContent).toContain("One-time purchase");
    expect(screen.getByTestId("top-up-pack-quick_150").textContent).toContain("One focused question");
    expect(screen.getByTestId("top-up-pack-welcome_600").textContent).toContain("New client offer");
  });

  it("a redeemed offer is absent (never disabled) and the first standard pack is preselected", async () => {
    m.catalog.mockResolvedValue(redeemed);
    view();
    await screen.findByRole("radiogroup");
    expect(screen.queryByText("Welcome Pack")).toBeNull();
    expect(screen.getAllByRole("radio")).toHaveLength(3);
    expect(radio(/Quick Insight/).getAttribute("aria-checked")).toBe("true");
    expect(screen.getByTestId("top-up-continue").textContent).toBe("Continue to Checkout — $20");
  });

  it("choosing a pack updates the total, the acknowledgement and the button together, and never charges", async () => {
    const u = userEvent.setup();
    view();
    await u.click(await screen.findByTestId("top-up-pack-popular_675"));
    expect(screen.getByTestId("top-up-dock-pack").textContent).toBe("Most Popular · 675 credits");
    expect(screen.getByTestId("top-up-total").textContent).toBe("Total: $80 USD");
    expect(screen.getByTestId("top-up-continue").textContent).toBe("Continue to Checkout — $80");
    expect(screen.getByTestId("top-up-ack").textContent).toContain("purchase 675 credits for $80 USD");
    expect(radio(/Most Popular/).getAttribute("aria-checked")).toBe("true");
    expect(radio(/Welcome Pack/).getAttribute("aria-checked")).toBe("false");
    expect(m.purchase).not.toHaveBeenCalled();
    expect(m.track).toHaveBeenCalledWith("checkout_pack_selected", { credits: 675 });
  });

  it("states the minutes at the live advisor's rate", async () => {
    view({ context: { advisorName: "Solomon", ratePerMinute: 45 } });
    await screen.findByRole("radiogroup");
    expect(screen.getByTestId("top-up-advisor-estimate").textContent).toBe("At Solomon's rate of 45 credits/minute, this pack provides approximately 13.33 minutes.");
  });

  it("has no advisor sentence outside a live reading", async () => {
    view();
    await screen.findByRole("radiogroup");
    expect(screen.queryByTestId("top-up-advisor-estimate")).toBeNull();
  });

  it("is a radio group: arrow keys move the choice and the focus, Tab leaves it", async () => {
    const u = userEvent.setup();
    view();
    await screen.findByRole("radiogroup");
    radio(/Welcome Pack/).focus();
    await u.keyboard("{ArrowRight}");
    expect(radio(/Quick Insight/).getAttribute("aria-checked")).toBe("true");
    expect(document.activeElement).toBe(radio(/Quick Insight/));
    expect(screen.getByTestId("top-up-continue").textContent).toBe("Continue to Checkout — $20");
    await u.keyboard("{ArrowDown}");
    expect(radio(/Starter/).getAttribute("aria-checked")).toBe("true");
    await u.keyboard("{ArrowLeft}");
    expect(radio(/Quick Insight/).getAttribute("aria-checked")).toBe("true");
    await u.keyboard("{ArrowUp}");
    expect(radio(/Welcome Pack/).getAttribute("aria-checked")).toBe("true");
    // Wraps around, and Home/End jump.
    await u.keyboard("{ArrowLeft}");
    expect(radio(/Most Popular/).getAttribute("aria-checked")).toBe("true");
    await u.keyboard("{ArrowRight}");
    expect(radio(/Welcome Pack/).getAttribute("aria-checked")).toBe("true");
    await u.keyboard("{End}");
    expect(radio(/Most Popular/).getAttribute("aria-checked")).toBe("true");
    await u.keyboard("{Home}");
    expect(radio(/Welcome Pack/).getAttribute("aria-checked")).toBe("true");
    expect(m.purchase).not.toHaveBeenCalled();
  });

  it("has one tab stop in the group: the selected card, then on to the summary", async () => {
    const u = userEvent.setup();
    view();
    await screen.findByRole("radiogroup");
    const radios = screen.getAllByRole("radio");
    expect(radios.map((r) => r.tabIndex)).toEqual([0, -1, -1, -1]);
    radio(/Welcome Pack/).focus();
    await u.tab();
    expect(radios.includes(document.activeElement as HTMLElement)).toBe(false);
  });

  it("selects with Space and Enter as well", async () => {
    const u = userEvent.setup();
    view();
    await screen.findByRole("radiogroup");
    radio(/Starter/).focus();
    await u.keyboard(" ");
    expect(radio(/Starter/).getAttribute("aria-checked")).toBe("true");
    radio(/Most Popular/).focus();
    await u.keyboard("{Enter}");
    expect(radio(/Most Popular/).getAttribute("aria-checked")).toBe("true");
  });

  it("opens the Terms and the Refund Policy in a new tab so a live chat is not left", async () => {
    view();
    await screen.findByRole("radiogroup");
    const ack = screen.getByTestId("top-up-ack");
    const terms = within(ack).getByRole("link", { name: /Terms/ });
    const refund = within(ack).getByRole("link", { name: /Refund Policy/ });
    expect(terms.getAttribute("href")).toBe("/en/terms");
    expect(refund.getAttribute("href")).toBe("/en/refund-policy");
    for (const link of [terms, refund]) {
      expect(link.getAttribute("target")).toBe("_blank");
      expect(link.getAttribute("rel")).toContain("noopener");
    }
  });

  it("closes with the close button, 'Not now' and Escape", async () => {
    const u = userEvent.setup();
    view();
    await screen.findByRole("radiogroup");
    await u.click(screen.getByRole("button", { name: "Close" }));
    await u.click(screen.getByTestId("top-up-dismiss"));
    await u.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledTimes(3);
  });

  it("keeps the free-mode notice", async () => {
    m.free.current = true;
    view();
    expect(await screen.findByTestId("credits-free-notice")).toBeTruthy();
  });

  it("is accessible, with the title as the dialog's name", async () => {
    view();
    await screen.findByRole("radiogroup");
    expect(screen.getByRole("dialog", { name: "Choose your credit pack" })).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
  });
});

describe("catalog states", () => {
  it("shows the error with Try again, then the packs", async () => {
    const u = userEvent.setup();
    m.catalog.mockRejectedValueOnce(err("service_unavailable", 503));
    view();
    expect(await screen.findByTestId("top-up-error")).toBeTruthy();
    expect(screen.getByRole("alert").textContent).toContain("The service is temporarily unavailable.");
    await u.click(screen.getByTestId("top-up-retry-catalog"));
    expect(await screen.findByRole("radiogroup")).toBeTruthy();
    expect(m.catalog).toHaveBeenCalledTimes(2);
  });

  it("offers no retry when purchases are switched off", async () => {
    m.catalog.mockRejectedValue(err("feature_unavailable", 503));
    view();
    expect(await screen.findByTestId("top-up-error")).toBeTruthy();
    expect(screen.queryByTestId("top-up-retry-catalog")).toBeNull();
  });

  it("says so when no pack is available", async () => {
    m.catalog.mockResolvedValue({ ...redeemed, packs: [] });
    view();
    expect((await screen.findByTestId("top-up-empty")).textContent).toBe("No credit packs are available right now. Please check back soon.");
    expect(screen.queryByTestId("top-up-continue")).toBeNull();
  });

  it("leaves the age gate's answers to the global handler (no message of its own)", async () => {
    m.catalog.mockRejectedValue(err("birth_date_required", 403, { next: "/signup/birth" }));
    view();
    await waitFor(() => expect(m.catalog).toHaveBeenCalled());
    expect(screen.queryByTestId("top-up-error")).toBeNull();
    expect(screen.queryByTestId("top-up-continue")).toBeNull();
  });
});

describe("step 2: pay", () => {
  it("keeps the selection when going on and back", async () => {
    const u = userEvent.setup();
    view();
    await u.click(await screen.findByTestId("top-up-pack-popular_675"));
    await toPayment(u);
    expect(screen.getByTestId("top-up-pay-summary").textContent).toContain("Most Popular · 675 credits");
    expect(screen.getByTestId("checkout-pay").textContent).toBe("Pay $80 USD");
    expect(screen.getByTestId("top-up-ack").textContent).toContain("purchase 675 credits for $80 USD");
    expect(screen.getByRole("dialog", { name: "Complete your purchase" })).toBeTruthy();
    await u.click(screen.getByTestId("top-up-back"));
    expect(radio(/Most Popular/).getAttribute("aria-checked")).toBe("true");
    expect(m.purchase).not.toHaveBeenCalled();
  });

  it("pays with a new card: only the provider token is sent, the wallet updates, the dialog closes", async () => {
    const u = userEvent.setup();
    m.purchase.mockResolvedValue(bought());
    view();
    await toPayment(u);
    await fillCard(u);
    await agree(u);
    await u.click(screen.getByTestId("checkout-pay"));
    await waitFor(() => expect(m.purchase).toHaveBeenCalled());
    const body = m.purchase.mock.calls[0][0];
    expect(body).toMatchObject({ pack_id: "id_welcome_600", payment_token: "sbx_a_visa_4242_12_2039", provider: "sandbox_a" });
    expect(body.idempotency_key).toBeTruthy();
    expect(JSON.stringify(body)).not.toContain("4242424242424242");
    expect(m.setWallet).toHaveBeenCalledWith({ balance: 720, held: 0, free_minutes: 0 });
    expect(await screen.findByText("600 credits added to your balance.")).toBeTruthy();
    expect(onClose).toHaveBeenCalled();
  });

  it("pays with a saved card and an optional receipt e-mail", async () => {
    const u = userEvent.setup();
    const saved = { id: "m1", provider: "sandbox_a", brand: "visa", last4: "4242", exp_month: 12, exp_year: 2039, is_default: true, status: "active", expired: false, created_at: "2026-01-01T00:00:00Z" };
    m.paymentMethods.mockResolvedValue([saved]);
    m.purchase.mockResolvedValue(bought());
    view();
    await toPayment(u);
    // The default saved card is preselected: no card fields are shown.
    expect(await screen.findByTestId("checkout-method")).toBeTruthy();
    expect(screen.queryByLabelText("Card number")).toBeNull();
    await u.click(screen.getByRole("checkbox", { name: /transaction confirmation/ }));
    expect((screen.getByLabelText("Notification email") as HTMLInputElement).value).toBe("jo@staryield.test");
    await agree(u);
    await u.click(screen.getByTestId("checkout-pay"));
    await waitFor(() => expect(m.purchase).toHaveBeenCalled());
    expect(m.purchase.mock.calls[0][0]).toMatchObject({ payment_method_id: "m1", receipt_email: "jo@staryield.test" });
    expect(m.purchase.mock.calls[0][0].payment_token).toBeUndefined();
  });

  it("asks for the policy agreement: an unchecked, required box with the three policy links in new tabs", async () => {
    const u = userEvent.setup();
    view();
    await toPayment(u);
    const box = screen.getByTestId("top-up-consent");
    expect(box.getAttribute("aria-checked")).toBe("false");
    expect(box.getAttribute("role")).toBe("checkbox");
    // The sentence is the box's accessible name (label association).
    const label = screen.getByRole("checkbox", { name: /^I agree to the Terms of Use.*, Privacy Policy.* and Refund & Cancellation Policy.*\.$/ });
    expect(label).toBe(box);
    const links = within(screen.getByTestId("top-up-payment")).getAllByRole("link");
    const policy = ["terms", "privacy", "refund-policy"].map((page) => links.find((l) => l.getAttribute("href") === `/en/${page}` && box.closest("[data-slot=field]")?.contains(l)));
    expect(policy.every(Boolean)).toBe(true);
    for (const link of policy as HTMLElement[]) {
      expect(link.getAttribute("target")).toBe("_blank");
      expect(link.getAttribute("rel")).toContain("noopener");
    }
    expect(policy.map((l) => l?.textContent?.replace(" (opens in a new tab)", ""))).toEqual(["Terms of Use", "Privacy Policy", "Refund & Cancellation Policy"]);
    // The acknowledgement of what is bought stays next to the button.
    expect(screen.getByTestId("top-up-ack")).toBeTruthy();
  });

  it("does not charge until the box is ticked: it shows the message and focuses the box", async () => {
    const u = userEvent.setup();
    view();
    await toPayment(u);
    await fillCard(u);
    await u.click(screen.getByTestId("checkout-pay"));
    expect(await screen.findByText("Accept the Terms of Use, Privacy Policy and Refund & Cancellation Policy to continue.")).toBeTruthy();
    expect(m.purchase).not.toHaveBeenCalled();
    expect(document.activeElement).toBe(screen.getByTestId("top-up-consent"));
    expect(screen.getByTestId("top-up-consent").getAttribute("aria-invalid")).toBe("true");
    expect(await axeViolations()).toEqual([]);
    // Ticking it clears the message and lets the payment through.
    m.purchase.mockResolvedValue(bought());
    await u.click(screen.getByTestId("top-up-consent"));
    await u.click(screen.getByTestId("checkout-pay"));
    await waitFor(() => expect(m.purchase).toHaveBeenCalledTimes(1));
    expect(screen.queryByText("Accept the Terms of Use, Privacy Policy and Refund & Cancellation Policy to continue.")).toBeNull();
  });

  it("toggles with the keyboard (Space) and sends the versions on screen as consent evidence", async () => {
    const u = userEvent.setup();
    m.purchase.mockResolvedValue(bought());
    view();
    await toPayment(u);
    await fillCard(u);
    screen.getByTestId("top-up-consent").focus();
    await u.keyboard(" ");
    expect(screen.getByTestId("top-up-consent").getAttribute("aria-checked")).toBe("true");
    await u.click(screen.getByTestId("checkout-pay"));
    await waitFor(() => expect(m.purchase).toHaveBeenCalled());
    expect(m.purchase.mock.calls[0][0].consent).toEqual(purchaseConsent());
    expect(m.purchase.mock.calls[0][0].consent).toEqual({ terms: LEGAL_VERSIONS.terms, privacy: LEGAL_VERSIONS.privacy, refund: LEGAL_VERSIONS.refund });
  });

  it("shows the API's 422 consent message on the box and focuses it", async () => {
    const u = userEvent.setup();
    m.purchase.mockRejectedValue(new ApiError({ code: "validation_failed", message: "Invalid", status: 422, details: { fields: { consent: "Accept the Terms of Use, Privacy Policy and Refund & Cancellation Policy to continue." } } }));
    view();
    await toPayment(u);
    await fillCard(u);
    await agree(u);
    await u.click(screen.getByTestId("checkout-pay"));
    await waitFor(() => expect(m.purchase).toHaveBeenCalled());
    const field = screen.getByTestId("top-up-consent").closest("[data-slot=field]") as HTMLElement;
    expect(await within(field).findByText("Accept the Terms of Use, Privacy Policy and Refund & Cancellation Policy to continue.")).toBeTruthy();
    expect(document.activeElement).toBe(screen.getByTestId("top-up-consent"));
    // Nothing leaked into a form-level alert.
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows the card marks, the statement descriptor and the support line near the pay button", async () => {
    const u = userEvent.setup();
    m.catalog.mockResolvedValue({ ...eligible, statement_descriptor: "STARYIELD MEDIA LLC" });
    view();
    await toPayment(u);
    const dock = screen.getByRole("region", { name: "Payment" });
    const marks = within(dock).getByTestId("accepted-cards");
    expect(within(marks).getByAltText("Visa")).toBeTruthy();
    expect(within(marks).getByAltText("Mastercard")).toBeTruthy();
    expect(marks.textContent).toContain("We accept");
    expect(screen.getByTestId("top-up-descriptor").textContent).toBe("This charge appears on your card statement as STARYIELD MEDIA LLC.");
    const support = within(dock).getByTestId("support-line");
    expect(support.textContent).toBe("Questions? support@staryield.net · Support is available 24/7. We reply within 1–2 days.");
    expect(within(support).getByRole("link", { name: "support@staryield.net" }).getAttribute("href")).toBe("mailto:support@staryield.net");
  });

  it("uses the API's statement descriptor, and the company's default when the API sends none", async () => {
    const u = userEvent.setup();
    m.catalog.mockResolvedValue({ ...eligible, statement_descriptor: "STARYIELD* READINGS" });
    view();
    await toPayment(u);
    expect(screen.getByTestId("top-up-descriptor").textContent).toBe("This charge appears on your card statement as STARYIELD* READINGS.");
    cleanup();
    m.catalog.mockResolvedValue(eligible);
    view();
    await toPayment(u);
    expect(screen.getByTestId("top-up-descriptor").textContent).toBe("This charge appears on your card statement as STARYIELD MEDIA LLC.");
  });

  it("the pay step is accessible", async () => {
    const u = userEvent.setup();
    view();
    await toPayment(u);
    expect(await axeViolations()).toEqual([]);
  });

  it("shows the decline's reason and starts a new attempt for the next try", async () => {
    const u = userEvent.setup();
    m.purchase
      .mockRejectedValueOnce(err("payment_failed", 402, { reason: "insufficient_funds" }))
      .mockResolvedValueOnce(bought());
    view();
    await toPayment(u);
    await fillCard(u);
    await agree(u);
    await u.click(screen.getByTestId("checkout-pay"));
    expect(await screen.findByText("Your card was declined for insufficient funds. Try another card.")).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
    await agree(u);
    await u.click(screen.getByTestId("checkout-pay"));
    await waitFor(() => expect(m.purchase).toHaveBeenCalledTimes(2));
    expect(m.purchase.mock.calls[1][0].idempotency_key).not.toBe(m.purchase.mock.calls[0][0].idempotency_key);
  });

  it("retries a network error with the SAME key, so nobody is charged twice", async () => {
    const u = userEvent.setup();
    m.purchase.mockRejectedValueOnce(err("network_error", 0)).mockResolvedValueOnce(bought());
    view();
    await toPayment(u);
    await fillCard(u);
    await agree(u);
    await u.click(screen.getByTestId("checkout-pay"));
    expect(await screen.findByText("We couldn't confirm your payment. Try again: you will not be charged twice.")).toBeTruthy();
    await u.click(screen.getByTestId("top-up-retry"));
    await waitFor(() => expect(m.purchase).toHaveBeenCalledTimes(2));
    expect(m.purchase.mock.calls[1][0].idempotency_key).toBe(m.purchase.mock.calls[0][0].idempotency_key);
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("keeps the key when going back to the packs and paying for the same pack again", async () => {
    const u = userEvent.setup();
    m.purchase.mockRejectedValueOnce(err("timeout", 0)).mockResolvedValueOnce(bought());
    view();
    await toPayment(u);
    await fillCard(u);
    await agree(u);
    await u.click(screen.getByTestId("checkout-pay"));
    await screen.findByTestId("top-up-retry");
    await u.click(screen.getByTestId("top-up-back"));
    await toPayment(u);
    await fillCard(u);
    await agree(u);
    await u.click(screen.getByTestId("checkout-pay"));
    await waitFor(() => expect(m.purchase).toHaveBeenCalledTimes(2));
    expect(m.purchase.mock.calls[1][0].idempotency_key).toBe(m.purchase.mock.calls[0][0].idempotency_key);
  });

  it("a refused Welcome Pack goes back to the packs with the reason, reloads the catalog and selects the first standard pack", async () => {
    const u = userEvent.setup();
    m.purchase.mockRejectedValueOnce(err("offer_unavailable", 409, { reason: "already_redeemed" }));
    m.catalog.mockResolvedValueOnce(eligible).mockResolvedValue(redeemed);
    view();
    await toPayment(u);
    await fillCard(u);
    await agree(u);
    await u.click(screen.getByTestId("checkout-pay"));
    expect((await screen.findByTestId("top-up-notice")).textContent).toContain("You have already used the Welcome Pack. Choose another pack to continue.");
    await waitFor(() => expect(m.catalog).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.queryByText("Welcome Pack")).toBeNull());
    expect(radio(/Quick Insight/).getAttribute("aria-checked")).toBe("true");
    expect(screen.getByTestId("top-up-continue").textContent).toBe("Continue to Checkout — $20");
  });

  it("selects the first standard pack even if the catalog still lists the refused offer", async () => {
    const u = userEvent.setup();
    m.purchase.mockRejectedValueOnce(err("offer_unavailable", 409, { reason: "card_check_unavailable" }));
    view();
    await toPayment(u);
    await fillCard(u);
    await agree(u);
    await u.click(screen.getByTestId("checkout-pay"));
    expect((await screen.findByTestId("top-up-notice")).textContent).toContain("We couldn't verify this card");
    await waitFor(() => expect(radio(/Quick Insight/).getAttribute("aria-checked")).toBe("true"));
  });

  it("tells a payment that is still processing and closes", async () => {
    const u = userEvent.setup();
    m.purchase.mockResolvedValue({ ...bought(), status: "pending", wallet: undefined });
    view();
    await toPayment(u);
    await fillCard(u);
    await agree(u);
    await u.click(screen.getByTestId("checkout-pay"));
    expect(await screen.findByText("Your payment is processing. Credits appear as soon as it is confirmed.")).toBeTruthy();
    expect(m.setWallet).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it("goes to the bank's page when the payment needs 3-D Secure", async () => {
    const u = userEvent.setup();
    const assign = vi.fn();
    vi.stubGlobal("location", { ...window.location, assign });
    m.purchase.mockResolvedValue({ ...bought(), status: "pending", next_action: { type: "redirect", url: "https://bank.example/3ds" } });
    view();
    await toPayment(u);
    await fillCard(u);
    await agree(u);
    await u.click(screen.getByTestId("checkout-pay"));
    await waitFor(() => expect(assign).toHaveBeenCalledWith("https://bank.example/3ds"));
    expect(onClose).not.toHaveBeenCalled();
    vi.unstubAllGlobals();
  });

  it("disables paying when purchases are switched off", async () => {
    const u = userEvent.setup();
    m.purchase.mockRejectedValue(err("feature_unavailable", 503));
    view();
    await toPayment(u);
    await fillCard(u);
    await agree(u);
    await u.click(screen.getByTestId("checkout-pay"));
    expect(await screen.findByText("Credit purchases are not available right now. Please try again later.")).toBeTruthy();
    expect((screen.getByTestId("checkout-pay") as HTMLButtonElement).disabled).toBe(true);
  });

  it("shows no message of its own for the age gate (the global handler answers)", async () => {
    const u = userEvent.setup();
    m.purchase.mockRejectedValue(err("age_restricted", 403));
    view();
    await toPayment(u);
    await fillCard(u);
    await agree(u);
    await u.click(screen.getByTestId("checkout-pay"));
    await waitFor(() => expect(m.purchase).toHaveBeenCalled());
    expect(screen.queryByRole("alert")).toBeNull();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("moves focus to the new step's title", async () => {
    const u = userEvent.setup();
    view();
    await toPayment(u);
    await waitFor(() => expect(document.activeElement?.textContent).toBe("Complete your purchase"));
  });
});
