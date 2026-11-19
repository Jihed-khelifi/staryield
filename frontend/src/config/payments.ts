/**
 * Payment-provider assets (revisions 6.0, docs/platform-extensions.md §26.5).
 * Everything that is not code loads from R2 (https://assets.staryield.net);
 * Stripe.js itself is the one script that always loads from js.stripe.com
 * (a card-network rule). Keep the hashed URLs in this one file.
 */
import type { StripeElementsOptionsMode } from "@stripe/stripe-js";

/** The official "Powered by Stripe" badge (dark variant, viewBox 150×34). */
export const POWERED_BY_STRIPE_SRC = "https://assets.staryield.net/assets/payments/powered-by-stripe-e434f84af2.svg";
/** Stripe asks the badge to link to its site. */
export const POWERED_BY_STRIPE_HREF = "https://stripe.com";
/** Intrinsic size of the badge (reserved up front so the page never shifts). */
export const POWERED_BY_STRIPE_SIZE = { width: 150, height: 34 } as const;

/** Source Serif 4, Latin subset: one variable file covering weights 300–600. */
export const SOURCE_SERIF_SRC = "https://assets.staryield.net/assets/fonts/source-serif-4-latin-variable-8e38e3fbd7.woff2";
export const STRIPE_FONT_FAMILY = "Source Serif 4";

/** The font declarations handed to `<Elements fonts>` (the same variable file, three weights). */
export const STRIPE_FONTS: NonNullable<StripeElementsOptionsMode["fonts"]> = (["300", "400", "600"] as const).map((weight) => ({
  family: STRIPE_FONT_FAMILY,
  src: `url(${SOURCE_SERIF_SRC}) format("woff2")`,
  weight,
  style: "normal" as const,
  display: "swap",
}));
