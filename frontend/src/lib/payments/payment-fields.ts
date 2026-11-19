/**
 * Card entry is delegated to the routed payment provider. The API returns a
 * `ClientConfig` (`GET /billing/client-config`) naming the provider and the
 * kind of fields to load; a `PaymentFieldsAdapter` renders those fields and
 * turns what the payer typed into a provider token. Only that token is sent
 * to our API — the card number (PAN) and security code never are.
 *
 * Adapters:
 *  - `sandbox` (development): local fields that build the sandbox token
 *    `sbx_<a|b>_<brand>_<last4>_<mm>_<yyyy>` described by `token_format`.
 *  - `stripe_elements` (revisions 6.0, §26.5): Stripe's Payment Element in
 *    Stripe's iframe, styled with our design tokens; the token is a
 *    ConfirmationToken (payment) or a payment method id (setup).
 *  - Other processors (NMI Collect.js…) plug in here with their hosted iframes.
 */
import type * as React from "react";

import type { ClientConfig, NextAction } from "@/lib/api/billing";

/** A tokenised card, ready for `POST /billing/payment-methods` or a purchase. */
export type TokenizedCard = {
  provider: string;
  token: string;
  brand?: string;
  last4?: string;
  billingCountry?: string;
};

/** Imperative handle of the mounted fields. */
export type PaymentFieldsHandle = {
  /**
   * Validate and tokenise. Resolves null when the fields are invalid (the
   * fields show their own errors and take focus).
   */
  tokenize: () => Promise<TokenizedCard | null>;
  /**
   * Run what the payer still has to do after the charge was created (3-D
   * Secure in Stripe's modal). Never navigates; resolves `ok: false` with a
   * message when the payer could not be authenticated.
   */
  handleNextAction: (next: NextAction) => Promise<NextActionResult>;
};

/** How a next action ended (revisions 6.0, §26.5). */
export type NextActionResult = { ok: boolean; message?: string };

export type PaymentFieldsProps = {
  config: ClientConfig;
  disabled?: boolean;
  /** Visual density: dialogs in Settings, or the top-up checkout card. */
  variant?: "dialog" | "checkout";
  /** Prefill for the cardholder name (never card data). */
  cardholder?: string;
  /**
   * Adapters whose fields load asynchronously (Stripe.js, an iframe) report
   * when they can tokenise; the parent keeps its pay button off until "ready".
   */
  onState?: (state: PaymentFieldsState) => void;
  ref?: React.Ref<PaymentFieldsHandle>;
};

export type PaymentFieldsState = "loading" | "ready" | "error";

export type PaymentFieldsAdapter = {
  type: string;
  Fields: React.ComponentType<PaymentFieldsProps>;
  /** The fields state how the provider secures the card themselves, so the generic note is not shown. */
  securedBy?: boolean;
  /** The fields load asynchronously and report `onState`; until they say "ready" the form cannot pay. */
  async?: boolean;
};

/** The sandbox token prefix ("sbx_a") from the config's token format. */
export function sandboxTokenPrefix(config: Pick<ClientConfig, "provider" | "token_format">): string | null {
  const fromFormat = config.token_format?.match(/^(sbx_[ab])_/)?.[1];
  if (fromFormat) return fromFormat;
  const fromProvider = config.provider.match(/^sandbox_([ab])$/)?.[1];
  return fromProvider ? `sbx_${fromProvider}` : null;
}

/** Build the sandbox token from parsed card details. */
export function sandboxToken(prefix: string, card: { brand: string; last4: string; month: number; year: number; country?: string }): string {
  const country = card.country ? `_${card.country.toLowerCase()}` : "";
  return `${prefix}_${card.brand}_${card.last4}_${String(card.month).padStart(2, "0")}_${card.year}${country}`;
}
