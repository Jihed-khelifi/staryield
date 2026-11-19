// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import type { ClientConfig } from "@/lib/api/billing";
import type { PaymentFieldsHandle } from "@/lib/payments/payment-fields";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const m = vi.hoisted(() => ({
  stripe: {
    createConfirmationToken: vi.fn(),
    confirmSetup: vi.fn(),
    handleNextAction: vi.fn(),
  },
  elements: { submit: vi.fn() },
  getStripe: vi.fn(),
  elementsOptions: vi.fn(),
  elementOptions: vi.fn(),
  ready: { mode: "ready" as "ready" | "never" | "error" },
  /** Thrown by <Elements> while set, as Stripe.js throws an IntegrationError from `stripe.elements()`. */
  elementsError: { current: null as Error | null },
  clientConfig: vi.fn(),
}));

vi.mock("@stripe/react-stripe-js", async () => {
  const React = await import("react");
  return {
    Elements: ({ children, options }: { children: React.ReactNode; options: unknown }) => {
      m.elementsOptions(options);
      if (m.elementsError.current) throw m.elementsError.current;
      return <div data-testid="elements">{children}</div>;
    },
    PaymentElement: ({ options, onReady, onLoadError }: { options: unknown; onReady: () => void; onLoadError: () => void }) => {
      m.elementOptions(options);
      React.useEffect(() => {
        if (m.ready.mode === "ready") onReady();
        if (m.ready.mode === "error") onLoadError();
      }, [onReady, onLoadError]);
      return <div data-testid="payment-element" />;
    },
    useStripe: () => m.stripe,
    useElements: () => m.elements,
  };
});
vi.mock("@/lib/payments/stripe", async (importOriginal) => ({ ...(await importOriginal<typeof import("@/lib/payments/stripe")>()), getStripe: m.getStripe }));
vi.mock("@/lib/api/billing", () => ({ billingApi: { clientConfig: m.clientConfig } }));

import { paymentAdapters, PaymentFields } from "@/components/billing/payment-fields";
import { StripeCardFields } from "@/components/billing/stripe-card-fields";
import { POWERED_BY_STRIPE_HREF, POWERED_BY_STRIPE_SRC, STRIPE_FONTS } from "@/config/payments";
import { logger } from "@/lib/logger";

const payment: ClientConfig = { provider: "stripe", type: "stripe_elements", publishable_key: "pk_test_x", mode: "payment", amount_cents: 1000, currency: "USD", payment_method_types: ["card"], livemode: false };
const setup: ClientConfig = { provider: "stripe", type: "stripe_elements", publishable_key: "pk_test_x", mode: "setup", client_secret: "seti_1_secret_2", livemode: true };
const SECURE_COPY = "Payments are processed securely by Stripe. Your card details are encrypted and sent directly to Stripe — Staryield never sees or stores them.";

function mount(config: ClientConfig = payment, props: Partial<React.ComponentProps<typeof StripeCardFields>> = {}) {
  const ref = React.createRef<PaymentFieldsHandle>();
  const onState = vi.fn();
  renderWithI18n(<StripeCardFields config={config} onState={onState} ref={ref} {...props} />);
  return { ref, onState };
}

beforeAll(stubResizeObserver);
beforeEach(() => {
  m.ready.mode = "ready";
  m.elementsError.current = null;
  m.getStripe.mockResolvedValue(m.stripe);
  m.elements.submit.mockResolvedValue({});
  m.stripe.createConfirmationToken.mockResolvedValue({
    confirmationToken: { id: "ctoken_123", payment_method_preview: { card: { brand: "visa", last4: "4242" }, billing_details: { address: { country: "FR" } } } },
  });
  m.stripe.confirmSetup.mockResolvedValue({ setupIntent: { status: "succeeded", payment_method: "pm_123" } });
  m.stripe.handleNextAction.mockResolvedValue({ paymentIntent: { status: "succeeded" } });
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("registration", () => {
  it("is the stripe_elements adapter and states how Stripe secures the card itself", () => {
    expect(paymentAdapters.stripe_elements.Fields).toBe(StripeCardFields);
    expect(paymentAdapters.stripe_elements.securedBy).toBe(true);
    expect(paymentAdapters.stripe_elements.async).toBe(true);
    expect(paymentAdapters.sandbox.securedBy).toBeUndefined();
    expect(paymentAdapters.sandbox.async).toBeUndefined();
  });
});

describe("payment mode", () => {
  it("builds a deferred-intent Elements with the amount, card only, our appearance, fonts and locale", async () => {
    mount();
    await screen.findByTestId("payment-element");
    const options = m.elementsOptions.mock.calls.at(-1)?.[0];
    expect(options).toMatchObject({ mode: "payment", amount: 1000, currency: "usd", allowedPaymentMethodTypes: ["card"], locale: "en", fonts: STRIPE_FONTS });
    // The Stripe.js release @stripe/stripe-js pins ("endive") throws an IntegrationError for the old name.
    expect(options).not.toHaveProperty("paymentMethodTypes");
    expect(options.appearance.variables.borderRadius).toBe("8px");
    expect(options.appearance.variables.fontFamily).toContain("Source Serif 4");
    expect(options.appearance.rules[".Input:focus"]).toBeTruthy();
    expect(m.elementOptions.mock.calls.at(-1)?.[0]).toMatchObject({ layout: "tabs" });
    expect(m.getStripe).toHaveBeenCalledWith("pk_test_x", "en");
  });

  it("offers cards only, in both modes: Link stays off (it brings bank and Klarna payments the API's card-only intents refuse)", async () => {
    mount();
    await screen.findByTestId("payment-element");
    expect(m.elementOptions.mock.calls.at(-1)?.[0]).toMatchObject({ wallets: { link: "never" } });
    cleanup();
    mount(setup);
    await screen.findByTestId("payment-element");
    expect(m.elementOptions.mock.calls.at(-1)?.[0]).toMatchObject({ wallets: { link: "never" } });
  });

  it("allows exactly the payment method types the API created the intent with, card only when it names none", async () => {
    mount({ ...payment, payment_method_types: ["card", "link"] });
    await screen.findByTestId("payment-element");
    expect(m.elementsOptions.mock.calls.at(-1)?.[0].allowedPaymentMethodTypes).toEqual(["card", "link"]);
    for (const payment_method_types of [undefined, []]) {
      cleanup();
      mount({ ...payment, payment_method_types });
      await screen.findByTestId("payment-element");
      expect(m.elementsOptions.mock.calls.at(-1)?.[0].allowedPaymentMethodTypes).toEqual(["card"]);
    }
  });

  it("shows a skeleton until the element is ready, and reports loading then ready", async () => {
    m.ready.mode = "never";
    const { onState } = mount();
    expect(screen.getByTestId("stripe-skeleton").getAttribute("aria-busy")).toBe("true");
    expect(screen.getByRole("status").textContent).toBe("Loading secure card fields…");
    await screen.findByTestId("payment-element");
    expect(screen.getByTestId("stripe-skeleton")).toBeTruthy();
    expect(onState).toHaveBeenLastCalledWith("loading");
  });

  it("drops the skeleton and reports ready once the element is ready", async () => {
    const { onState } = mount();
    await waitFor(() => expect(onState).toHaveBeenLastCalledWith("ready"));
    expect(screen.queryByTestId("stripe-skeleton")).toBeNull();
  });

  it("tokenize: submits the element, then returns the ConfirmationToken with the card preview", async () => {
    const { ref, onState } = mount();
    await waitFor(() => expect(onState).toHaveBeenLastCalledWith("ready"));
    const card = await ref.current!.tokenize();
    expect(m.elements.submit).toHaveBeenCalledBefore(m.stripe.createConfirmationToken);
    expect(m.stripe.createConfirmationToken).toHaveBeenCalledWith({ elements: m.elements });
    expect(card).toEqual({ provider: "stripe", token: "ctoken_123", brand: "visa", last4: "4242", billingCountry: "FR" });
  });

  it("tokenize: Stripe's validation errors stay in the element (null, no message of ours)", async () => {
    const { ref, onState } = mount();
    await waitFor(() => expect(onState).toHaveBeenLastCalledWith("ready"));
    m.elements.submit.mockResolvedValue({ error: { type: "validation_error", message: "Your card number is incomplete." } });
    expect(await ref.current!.tokenize()).toBeNull();
    expect(m.stripe.createConfirmationToken).not.toHaveBeenCalled();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("tokenize: any other error is shown inline with role=alert, and cleared by the next try", async () => {
    const { ref, onState } = mount();
    await waitFor(() => expect(onState).toHaveBeenLastCalledWith("ready"));
    m.stripe.createConfirmationToken.mockResolvedValueOnce({ error: { type: "api_connection_error", message: "Could not reach Stripe." } });
    expect(await act(() => ref.current!.tokenize())).toBeNull();
    expect((await screen.findByTestId("stripe-error")).textContent).toBe("Could not reach Stripe.");
    expect(screen.getByRole("alert")).toBeTruthy();
    expect(await act(() => ref.current!.tokenize())).toMatchObject({ token: "ctoken_123" });
    await waitFor(() => expect(screen.queryByTestId("stripe-error")).toBeNull());
  });

  it("tokenize: an empty result is an inline error", async () => {
    const { ref, onState } = mount();
    await waitFor(() => expect(onState).toHaveBeenLastCalledWith("ready"));
    m.stripe.createConfirmationToken.mockResolvedValue({});
    expect(await act(() => ref.current!.tokenize())).toBeNull();
    expect((await screen.findByTestId("stripe-error")).textContent).toBe("We couldn't read your card details. Please check them and try again.");
  });

  it("tokenize does nothing before the element is ready", async () => {
    m.ready.mode = "never";
    const { ref } = mount();
    await screen.findByTestId("payment-element");
    expect(await ref.current!.tokenize()).toBeNull();
    expect(m.elements.submit).not.toHaveBeenCalled();
  });
});

describe("setup mode (Add a card)", () => {
  it("uses the SetupIntent's client secret", async () => {
    mount(setup);
    await screen.findByTestId("payment-element");
    const options = m.elementsOptions.mock.calls.at(-1)?.[0];
    expect(options).toMatchObject({ clientSecret: "seti_1_secret_2", locale: "en" });
    expect(options.mode).toBeUndefined();
  });

  it("tokenize: confirms the SetupIntent without redirecting and returns the payment method id", async () => {
    const { ref, onState } = mount(setup);
    await waitFor(() => expect(onState).toHaveBeenLastCalledWith("ready"));
    const card = await ref.current!.tokenize();
    expect(m.stripe.confirmSetup).toHaveBeenCalledWith({ elements: m.elements, redirect: "if_required", confirmParams: { return_url: `${window.location.origin}/en/profile/settings` } });
    expect(m.stripe.createConfirmationToken).not.toHaveBeenCalled();
    expect(card).toEqual({ provider: "stripe", token: "pm_123" });
  });

  it("accepts an expanded payment method object", async () => {
    m.stripe.confirmSetup.mockResolvedValue({ setupIntent: { status: "succeeded", payment_method: { id: "pm_obj" } } });
    const { ref, onState } = mount(setup);
    await waitFor(() => expect(onState).toHaveBeenLastCalledWith("ready"));
    expect(await ref.current!.tokenize()).toEqual({ provider: "stripe", token: "pm_obj" });
  });

  it("validation errors stay in the element; other errors and an unfinished SetupIntent are shown", async () => {
    const { ref, onState } = mount(setup);
    await waitFor(() => expect(onState).toHaveBeenLastCalledWith("ready"));
    m.stripe.confirmSetup.mockResolvedValueOnce({ error: { type: "validation_error", message: "Incomplete." } });
    expect(await act(() => ref.current!.tokenize())).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
    m.stripe.confirmSetup.mockResolvedValueOnce({ error: { type: "card_error", message: "Your card was declined." } });
    expect(await act(() => ref.current!.tokenize())).toBeNull();
    expect((await screen.findByTestId("stripe-error")).textContent).toBe("Your card was declined.");
    m.stripe.confirmSetup.mockResolvedValueOnce({ setupIntent: { status: "requires_action", payment_method: "pm_1" } });
    expect(await act(() => ref.current!.tokenize())).toBeNull();
    expect((await screen.findByTestId("stripe-error")).textContent).toContain("couldn't read your card details");
  });
});

describe("handleNextAction", () => {
  it("runs Stripe's next action with the client secret", async () => {
    const { ref, onState } = mount();
    await waitFor(() => expect(onState).toHaveBeenLastCalledWith("ready"));
    await expect(ref.current!.handleNextAction({ type: "use_stripe_sdk", client_secret: "pi_1_secret_2" })).resolves.toEqual({ ok: true });
    expect(m.stripe.handleNextAction).toHaveBeenCalledWith({ clientSecret: "pi_1_secret_2" });
  });

  it("is not ok when the payer could not be authenticated", async () => {
    m.stripe.handleNextAction.mockResolvedValue({ error: { type: "card_error", message: "Authentication failed." } });
    const { ref, onState } = mount();
    await waitFor(() => expect(onState).toHaveBeenLastCalledWith("ready"));
    await expect(ref.current!.handleNextAction({ type: "use_stripe_sdk", client_secret: "s" })).resolves.toEqual({ ok: false, message: "Authentication failed." });
  });

  it("settles the sandbox action without Stripe", async () => {
    const { ref, onState } = mount();
    await waitFor(() => expect(onState).toHaveBeenLastCalledWith("ready"));
    await expect(ref.current!.handleNextAction({ type: "sandbox_3ds" })).resolves.toEqual({ ok: true });
    expect(m.stripe.handleNextAction).not.toHaveBeenCalled();
  });
});

describe("secured by Stripe", () => {
  it("is always under the fields, with the exact copy and the Powered by Stripe mark from R2", async () => {
    m.ready.mode = "never";
    mount();
    expect(screen.getByTestId("secured-by-stripe-copy").textContent).toBe(SECURE_COPY);
    const img = screen.getByAltText("Powered by Stripe") as HTMLImageElement;
    expect(img.getAttribute("src")).toBe(POWERED_BY_STRIPE_SRC);
    expect(img.getAttribute("loading")).toBe("eager");
    const link = img.closest("a")!;
    expect(link.getAttribute("href")).toBe(POWERED_BY_STRIPE_HREF);
    expect(link.getAttribute("target")).toBe("_blank");
    expect(link.getAttribute("rel")).toBe("noopener noreferrer");
    expect(link.getAttribute("aria-label")).toBe("Powered by Stripe (opens in a new tab)");
  });

  it("is still shown when Stripe.js could not load, with a retry", async () => {
    m.getStripe.mockRejectedValueOnce(new Error("blocked"));
    const u = userEvent.setup();
    const { onState } = mount();
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("The secure card form could not be loaded");
    expect(alert.getAttribute("data-reason")).toBe("script");
    expect(screen.getByTestId("secured-by-stripe")).toBeTruthy();
    expect(onState).toHaveBeenLastCalledWith("error");
    await u.click(screen.getByRole("button", { name: "Try again" }));
    await screen.findByTestId("payment-element");
    expect(m.getStripe).toHaveBeenCalledTimes(2);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows a Test mode badge with test keys only", async () => {
    mount({ ...payment, livemode: false });
    expect(screen.getByTestId("stripe-test-mode").textContent).toBe("Test mode");
    cleanup();
    mount({ ...payment, livemode: true });
    expect(screen.queryByTestId("stripe-test-mode")).toBeNull();
    cleanup();
    mount({ ...payment, livemode: undefined });
    expect(screen.queryByTestId("stripe-test-mode")).toBeNull();
  });

  it("reports an element that fails to load, with a retry", async () => {
    m.ready.mode = "error";
    const { onState } = mount();
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("could not be loaded");
    expect(alert.getAttribute("data-reason")).toBe("element");
    expect(onState).toHaveBeenLastCalledWith("error");
    expect(screen.getByTestId("secured-by-stripe")).toBeTruthy();
  });

  it("keeps a failure inside Stripe's Elements in the card form: alert, error status, logged, and Try again mounts it afresh", async () => {
    // What production hit: Stripe.js threw an IntegrationError from `stripe.elements()` and the whole site fell to the root error page.
    const integration = Object.assign(new Error("Invalid value for elements(): `paymentMethodTypes` is no longer supported in this version of Stripe.js."), { name: "IntegrationError" });
    m.elementsError.current = integration;
    const logError = vi.spyOn(logger, "error").mockImplementation(() => {});
    vi.spyOn(console, "error").mockImplementation(() => {}); // React reports the caught error too.
    const u = userEvent.setup();
    const { ref, onState } = mount();

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("The secure card form could not be loaded");
    expect(alert.getAttribute("data-reason")).toBe("crash");
    expect(screen.getByTestId("secured-by-stripe")).toBeTruthy();
    await waitFor(() => expect(onState).toHaveBeenLastCalledWith("error"));
    expect(logError).toHaveBeenCalledWith(expect.stringContaining("paymentMethodTypes"), integration, expect.objectContaining({ code: "stripe_elements_failed" }));
    await expect(ref.current!.tokenize()).rejects.toMatchObject({ code: "feature_unavailable" });

    m.elementsError.current = null;
    await u.click(screen.getByRole("button", { name: "Try again" }));
    await screen.findByTestId("payment-element");
    await waitFor(() => expect(onState).toHaveBeenLastCalledWith("ready"));
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("is accessible", async () => {
    mount();
    await screen.findByTestId("payment-element");
    expect(await axeViolations()).toEqual([]);
  });
});

describe("through PaymentFields", () => {
  it("renders Stripe's fields for a stripe_elements config, without the generic note, and reports ready once the element is", async () => {
    m.clientConfig.mockResolvedValue(payment);
    const onStatus = vi.fn();
    renderWithI18n(<PaymentFields purpose="credits" onStatus={onStatus} genericNote={<p>generic provider note</p>} />);
    await screen.findByTestId("stripe-fields");
    await waitFor(() => expect(onStatus).toHaveBeenLastCalledWith("ready"));
    expect(screen.queryByText("generic provider note")).toBeNull();
    expect(m.clientConfig).toHaveBeenCalledWith("credits", {}, expect.anything());
  });

  it("asks for the configuration of the pack being paid, and again when the pack changes", async () => {
    m.clientConfig.mockResolvedValue(payment);
    const view = renderWithI18n(<PaymentFields purpose="credits" packId="pack-1" />);
    await screen.findByTestId("stripe-fields");
    expect(m.clientConfig).toHaveBeenLastCalledWith("credits", { packId: "pack-1" }, expect.anything());
    view.rerender(<PaymentFields purpose="credits" packId="pack-2" />);
    await waitFor(() => expect(m.clientConfig).toHaveBeenLastCalledWith("credits", { packId: "pack-2" }, expect.anything()));
  });

  it("keeps the generic note for the sandbox", async () => {
    m.clientConfig.mockResolvedValue({ provider: "sandbox_a", type: "sandbox", token_format: "sbx_a_<brand>_<last4>_<mm>_<yyyy>", test_cards: [] });
    renderWithI18n(<PaymentFields purpose="credits" genericNote={<p>generic provider note</p>} />);
    expect(await screen.findByText("generic provider note")).toBeTruthy();
    expect(screen.queryByTestId("secured-by-stripe")).toBeNull();
  });

  it("holds the status at loading while Stripe's element loads", async () => {
    m.ready.mode = "never";
    m.clientConfig.mockResolvedValue(payment);
    const onStatus = vi.fn();
    renderWithI18n(<PaymentFields purpose="credits" onStatus={onStatus} />);
    await screen.findByTestId("stripe-fields");
    expect(onStatus).not.toHaveBeenCalledWith("ready");
  });

  it("asks for the setup configuration when saving a card", async () => {
    m.clientConfig.mockResolvedValue(setup);
    renderWithI18n(<PaymentFields purpose="method" />);
    await screen.findByTestId("stripe-fields");
    expect(m.clientConfig).toHaveBeenCalledWith("method", {}, expect.anything());
  });
});
