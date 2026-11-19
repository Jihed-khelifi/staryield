/**
 * Stripe.js and the Payment Element's look (revisions 6.0, §26.5).
 *
 * Stripe.js always loads from js.stripe.com (`loadStripe`): it is never
 * bundled or self-hosted. One instance per publishable key, so opening the
 * top-up twice never loads the script twice. The Payment Element renders in
 * Stripe's iframe, which cannot read our CSS, so the design tokens of
 * `src/app/globals.css` are resolved here into an Appearance object.
 */
import { loadStripe, type Appearance, type Stripe, type StripeElementLocale } from "@stripe/stripe-js";

import { STRIPE_FONT_FAMILY } from "@/config/payments";

const instances = new Map<string, Promise<Stripe | null>>();

/** Stripe's UI languages we ship; anything else lets Stripe detect the browser's. */
const LOCALES: Record<string, StripeElementLocale> = { en: "en", fr: "fr", es: "es" };

export function stripeLocale(locale: string): StripeElementLocale {
  return LOCALES[locale] ?? "auto";
}

/**
 * The Stripe instance for a publishable key (memoised). The locale only
 * changes the language of Stripe's own modals (3-D Secure); the Elements get
 * theirs through their options. A failed load is forgotten so a retry works.
 */
export function getStripe(publishableKey: string, locale = "en"): Promise<Stripe | null> {
  let instance = instances.get(publishableKey);
  if (!instance) {
    instance = loadStripe(publishableKey, { locale: stripeLocale(locale) }).catch((error: unknown) => {
      instances.delete(publishableKey);
      throw error;
    });
    instances.set(publishableKey, instance);
  }
  return instance;
}

/** Forget every instance (tests). */
export function resetStripeForTests() {
  instances.clear();
}

/** The design tokens the Payment Element needs, as colours (CSS variables do not cross the iframe). */
export type PaymentTokens = {
  ink: string;
  cream: string;
  creamLight: string;
  gold: string;
  amber: string;
  goldCta: string;
  warmGray: string;
  danger: string;
};

/** The values of `:root` in `src/app/globals.css`; used on the server and when a variable cannot be read. */
export const DEFAULT_PAYMENT_TOKENS: PaymentTokens = {
  ink: "#15100b",
  cream: "#e0d5c3",
  creamLight: "#f4f0ea",
  gold: "#aa8e62",
  amber: "#c29a3d",
  goldCta: "#d9a036",
  warmGray: "#c1bba9",
  danger: "#991b1b",
};

const TOKEN_VARIABLES: Record<keyof PaymentTokens, string> = {
  ink: "--ink",
  cream: "--cream",
  creamLight: "--cream-light",
  gold: "--gold",
  amber: "--amber",
  goldCta: "--gold-cta",
  warmGray: "--warm-gray",
  danger: "--danger",
};

const COLOUR = /^(#[0-9a-f]{3,8}|rgba?\([^)]*\)|hsla?\([^)]*\))$/i;

/** Read the live tokens (a theme may change them), falling back to the defaults. */
export function readPaymentTokens(root?: Element): PaymentTokens {
  const tokens = { ...DEFAULT_PAYMENT_TOKENS };
  const element = root ?? (typeof document !== "undefined" ? document.documentElement : undefined);
  if (!element || typeof getComputedStyle === "undefined") return tokens;
  const style = getComputedStyle(element);
  for (const key of Object.keys(TOKEN_VARIABLES) as (keyof PaymentTokens)[]) {
    const value = style.getPropertyValue(TOKEN_VARIABLES[key]).trim();
    if (COLOUR.test(value)) tokens[key] = value;
  }
  return tokens;
}

/** `rgba` of a hex colour (focus rings are translucent). */
function withAlpha(colour: string, alpha: number): string {
  const hex = colour.match(/^#([0-9a-f]{6})$/i)?.[1];
  if (!hex) return colour;
  const n = parseInt(hex, 16);
  return `rgba(${n >> 16}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`;
}

export const STRIPE_FONT_STACK = `"${STRIPE_FONT_FAMILY}", Georgia, serif`;

/**
 * The Payment Element's Appearance: ink on cream-light fields with a gold
 * border, 8 px radius, a gold focus ring, Source Serif 4, uppercase labels
 * like our own forms and the danger colour for errors.
 */
export function stripeAppearance(tokens: PaymentTokens = readPaymentTokens()): Appearance {
  const ring = `0 0 0 2px ${withAlpha(tokens.amber, 0.55)}`;
  return {
    theme: "flat",
    labels: "above",
    variables: {
      colorPrimary: tokens.amber,
      colorBackground: tokens.creamLight,
      colorText: tokens.ink,
      colorTextSecondary: tokens.ink,
      colorTextPlaceholder: tokens.warmGray,
      colorDanger: tokens.danger,
      colorIcon: tokens.gold,
      colorIconHover: tokens.ink,
      fontFamily: STRIPE_FONT_STACK,
      fontSizeBase: "15px",
      fontSizeSm: "12px",
      fontWeightNormal: "400",
      fontWeightMedium: "600",
      spacingUnit: "4px",
      gridRowSpacing: "14px",
      gridColumnSpacing: "10px",
      borderRadius: "8px",
      focusBoxShadow: ring,
      focusOutline: "none",
    },
    rules: {
      ".Label": {
        color: tokens.ink,
        fontSize: "11px",
        fontWeight: "700",
        letterSpacing: "0.77px",
        textTransform: "uppercase",
        marginBottom: "6px",
      },
      ".Input": {
        backgroundColor: tokens.creamLight,
        border: `1px solid ${tokens.gold}`,
        boxShadow: "none",
        color: tokens.ink,
        padding: "12px 14px",
      },
      ".Input:focus": {
        borderColor: tokens.amber,
        boxShadow: ring,
        outline: "none",
      },
      ".Input--invalid": {
        borderColor: tokens.danger,
        boxShadow: "none",
        color: tokens.ink,
      },
      ".Input::placeholder": { color: tokens.warmGray },
      ".Error": { color: tokens.danger, fontSize: "13px" },
      ".Tab": {
        backgroundColor: tokens.creamLight,
        border: `1px solid ${tokens.gold}`,
        boxShadow: "none",
        color: tokens.ink,
      },
      ".Tab:hover": { backgroundColor: tokens.cream, color: tokens.ink },
      ".Tab--selected": {
        backgroundColor: tokens.cream,
        borderColor: tokens.amber,
        boxShadow: ring,
        color: tokens.ink,
      },
      ".TabLabel": { color: tokens.ink, fontWeight: "600" },
      ".TabIcon--selected": { fill: tokens.ink },
      ".Block": { backgroundColor: tokens.creamLight, border: `1px solid ${tokens.gold}`, boxShadow: "none" },
    },
  };
}
