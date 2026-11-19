import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api/errors";

const m = vi.hoisted(() => ({ clientConfig: vi.fn(), getStripe: vi.fn(), handleNextAction: vi.fn() }));
vi.mock("@/lib/api/billing", () => ({ billingApi: { clientConfig: m.clientConfig } }));
vi.mock("@/lib/payments/stripe", () => ({ getStripe: m.getStripe }));

import { finishStripeAction, isHttpsUrl, runNextAction } from "@/lib/payments/next-action";

beforeEach(() => {
  m.clientConfig.mockResolvedValue({ provider: "stripe", type: "stripe_elements", publishable_key: "pk_test_x" });
  m.getStripe.mockResolvedValue({ handleNextAction: m.handleNextAction });
  m.handleNextAction.mockResolvedValue({ paymentIntent: { status: "succeeded" } });
});
afterEach(() => {
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe("runNextAction", () => {
  it("settles the sandbox 3-D Secure at once", async () => {
    await expect(runNextAction({ type: "sandbox_3ds" })).resolves.toEqual({ ok: true });
    expect(m.clientConfig).not.toHaveBeenCalled();
  });

  it("runs Stripe's next action with the publishable key from client-config (saved card, nothing mounted)", async () => {
    await expect(runNextAction({ type: "use_stripe_sdk", client_secret: "pi_1_secret_2" })).resolves.toEqual({ ok: true });
    expect(m.clientConfig).toHaveBeenCalledWith("credits", {}, undefined);
    expect(m.getStripe).toHaveBeenCalledWith("pk_test_x", expect.any(String));
    expect(m.handleNextAction).toHaveBeenCalledWith({ clientSecret: "pi_1_secret_2" });
  });

  it("is not ok when the bank could not authenticate, with Stripe's message", async () => {
    m.handleNextAction.mockResolvedValue({ error: { type: "card_error", message: "We are unable to authenticate your payment method." } });
    await expect(runNextAction({ type: "use_stripe_sdk", client_secret: "pi_1_secret_2" })).resolves.toEqual({ ok: false, message: "We are unable to authenticate your payment method." });
  });

  it("is not ok when Stripe.js throws, cannot load or there is no key", async () => {
    m.handleNextAction.mockRejectedValue(new Error("boom"));
    await expect(runNextAction({ type: "use_stripe_sdk", client_secret: "s" })).resolves.toEqual({ ok: false, message: "boom" });
    m.getStripe.mockRejectedValue(new Error("blocked"));
    await expect(runNextAction({ type: "use_stripe_sdk", client_secret: "s" })).resolves.toEqual({ ok: false });
    m.getStripe.mockResolvedValue(null);
    await expect(runNextAction({ type: "use_stripe_sdk", client_secret: "s" })).resolves.toEqual({ ok: false });
    m.clientConfig.mockResolvedValue({ provider: "stripe", type: "stripe_elements" });
    await expect(runNextAction({ type: "use_stripe_sdk", client_secret: "s" })).resolves.toEqual({ ok: false });
    m.clientConfig.mockRejectedValue(new ApiError({ code: "network_error", message: "x" }));
    await expect(runNextAction({ type: "use_stripe_sdk", client_secret: "s" })).resolves.toEqual({ ok: false });
  });

  it("rethrows an abort instead of calling it a failure", async () => {
    m.clientConfig.mockRejectedValue(new ApiError({ code: "aborted", message: "aborted" }));
    await expect(runNextAction({ type: "use_stripe_sdk", client_secret: "s" })).rejects.toMatchObject({ code: "aborted" });
  });

  it("sends the payer to an https provider page and refuses anything else", async () => {
    const assign = vi.fn();
    vi.stubGlobal("window", { location: { assign } });
    await expect(runNextAction({ type: "redirect", url: "https://bank.example/3ds" })).resolves.toEqual({ ok: true, redirected: true });
    expect(assign).toHaveBeenCalledWith("https://bank.example/3ds");
    await expect(runNextAction({ type: "redirect", url: "http://bank.example/3ds" })).resolves.toEqual({ ok: false });
    await expect(runNextAction({ type: "redirect", url: "javascript:alert(1)" })).resolves.toEqual({ ok: false });
    expect(assign).toHaveBeenCalledOnce();
  });

  it("does not pretend an unknown action was completed", async () => {
    await expect(runNextAction({ type: "something_new" })).resolves.toEqual({ ok: false });
    await expect(runNextAction({ type: "use_stripe_sdk" })).resolves.toEqual({ ok: false });
  });
});

describe("finishStripeAction and isHttpsUrl", () => {
  it("calls handleNextAction with the client secret", async () => {
    const stripe = { handleNextAction: m.handleNextAction } as never;
    await expect(finishStripeAction(stripe, "secret")).resolves.toEqual({ ok: true });
    expect(m.handleNextAction).toHaveBeenCalledWith({ clientSecret: "secret" });
  });
  it("accepts only https", () => {
    expect(isHttpsUrl("https://a.example")).toBe(true);
    expect(isHttpsUrl("http://a.example")).toBe(false);
    expect(isHttpsUrl(undefined)).toBe(false);
  });
});
