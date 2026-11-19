import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const m = vi.hoisted(() => ({ loadStripe: vi.fn() }));
vi.mock("@stripe/stripe-js", () => ({ loadStripe: m.loadStripe }));

import { POWERED_BY_STRIPE_SRC, SOURCE_SERIF_SRC, STRIPE_FONTS } from "@/config/payments";
import { DEFAULT_PAYMENT_TOKENS, getStripe, readPaymentTokens, resetStripeForTests, stripeAppearance, stripeLocale, STRIPE_FONT_STACK } from "@/lib/payments/stripe";

beforeEach(() => {
  resetStripeForTests();
  m.loadStripe.mockImplementation(async (key: string) => ({ key }));
});
afterEach(() => vi.clearAllMocks());

describe("getStripe", () => {
  it("loads Stripe.js once per publishable key", async () => {
    const a = await getStripe("pk_test_a", "en");
    const again = await getStripe("pk_test_a", "fr");
    const b = await getStripe("pk_test_b", "en");
    expect(a).toBe(again);
    expect(b).not.toBe(a);
    expect(m.loadStripe).toHaveBeenCalledTimes(2);
    expect(m.loadStripe).toHaveBeenCalledWith("pk_test_a", { locale: "en" });
  });

  it("shares an in-flight load between callers", async () => {
    const [a, b] = await Promise.all([getStripe("pk_test_a"), getStripe("pk_test_a")]);
    expect(a).toBe(b);
    expect(m.loadStripe).toHaveBeenCalledOnce();
  });

  it("forgets a failed load so a retry loads again", async () => {
    m.loadStripe.mockRejectedValueOnce(new Error("blocked"));
    await expect(getStripe("pk_test_a")).rejects.toThrow("blocked");
    await expect(getStripe("pk_test_a")).resolves.toEqual({ key: "pk_test_a" });
    expect(m.loadStripe).toHaveBeenCalledTimes(2);
  });

  it("maps the site language to Stripe's", () => {
    expect(stripeLocale("fr")).toBe("fr");
    expect(stripeLocale("es")).toBe("es");
    expect(stripeLocale("xx")).toBe("auto");
  });
});

describe("stripeAppearance", () => {
  const a = stripeAppearance(DEFAULT_PAYMENT_TOKENS);

  it("maps the design tokens to Stripe's variables", () => {
    expect(a.variables).toMatchObject({
      colorPrimary: DEFAULT_PAYMENT_TOKENS.amber,
      colorBackground: DEFAULT_PAYMENT_TOKENS.creamLight,
      colorText: DEFAULT_PAYMENT_TOKENS.ink,
      colorDanger: DEFAULT_PAYMENT_TOKENS.danger,
      colorTextPlaceholder: DEFAULT_PAYMENT_TOKENS.warmGray,
      borderRadius: "8px",
    });
  });

  it("uses Source Serif 4 with a Georgia/serif fallback", () => {
    expect(a.variables?.fontFamily).toBe(STRIPE_FONT_STACK);
    expect(STRIPE_FONT_STACK).toBe('"Source Serif 4", Georgia, serif');
  });

  it("has a gold focus ring and gold borders on the inputs and tabs", () => {
    expect(a.variables?.focusBoxShadow).toContain("rgba(194, 154, 61");
    expect(a.rules?.[".Input"]).toMatchObject({ border: `1px solid ${DEFAULT_PAYMENT_TOKENS.gold}`, color: DEFAULT_PAYMENT_TOKENS.ink });
    expect(a.rules?.[".Input:focus"]).toMatchObject({ borderColor: DEFAULT_PAYMENT_TOKENS.amber });
    expect(a.rules?.[".Input--invalid"]).toMatchObject({ borderColor: DEFAULT_PAYMENT_TOKENS.danger });
    expect(a.rules?.[".Tab--selected"]).toMatchObject({ borderColor: DEFAULT_PAYMENT_TOKENS.amber });
    expect(a.rules?.[".Label"]).toMatchObject({ textTransform: "uppercase" });
    expect(a.rules?.[".Error"]).toMatchObject({ color: DEFAULT_PAYMENT_TOKENS.danger });
  });

  it("follows custom tokens", () => {
    const custom = stripeAppearance({ ...DEFAULT_PAYMENT_TOKENS, ink: "#000000", danger: "#ff0000" });
    expect(custom.variables).toMatchObject({ colorText: "#000000", colorDanger: "#ff0000" });
  });
});

describe("readPaymentTokens", () => {
  it("falls back to the defaults without a document", () => {
    expect(readPaymentTokens()).toEqual(DEFAULT_PAYMENT_TOKENS);
  });

  it("reads colours from the element's CSS variables and ignores anything that is not a colour", () => {
    const style = { getPropertyValue: (name: string) => ({ "--ink": " #111111 ", "--gold": "url(x)", "--danger": "rgb(1, 2, 3)" })[name] ?? "" };
    vi.stubGlobal("getComputedStyle", () => style);
    const tokens = readPaymentTokens({} as Element);
    vi.unstubAllGlobals();
    expect(tokens.ink).toBe("#111111");
    expect(tokens.gold).toBe(DEFAULT_PAYMENT_TOKENS.gold);
    expect(tokens.danger).toBe("rgb(1, 2, 3)");
    expect(tokens.cream).toBe(DEFAULT_PAYMENT_TOKENS.cream);
  });
});

describe("payment assets", () => {
  it("loads the badge and the font from R2, three weights of one variable file", () => {
    expect(POWERED_BY_STRIPE_SRC).toMatch(/^https:\/\/assets\.staryield\.net\/assets\/payments\/powered-by-stripe-[0-9a-f]+\.svg$/);
    expect(SOURCE_SERIF_SRC).toMatch(/^https:\/\/assets\.staryield\.net\/assets\/fonts\/source-serif-4-latin-variable-[0-9a-f]+\.woff2$/);
    const fonts = STRIPE_FONTS as { family: string; src: string; weight: string }[];
    expect(fonts.map((f) => f.weight)).toEqual(["300", "400", "600"]);
    for (const font of fonts) {
      expect(font.family).toBe("Source Serif 4");
      expect(font.src).toContain(SOURCE_SERIF_SRC);
    }
  });
});
