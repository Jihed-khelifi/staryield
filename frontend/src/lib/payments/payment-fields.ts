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
 *  - Real processors (Stripe Payment Element, NMI Collect.js…) plug in here
 *    with their hosted iframes once credentials exist.
 */
import type * as React from "react";

import type { ClientConfig } from "@/lib/api/billing";

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
};

export type PaymentFieldsProps = {
  config: ClientConfig;
  disabled?: boolean;
  /** Visual density: dialogs in Settings, or the top-up checkout card. */
  variant?: "dialog" | "checkout";
  /** Prefill for the cardholder name (never card data). */
  cardholder?: string;
  ref?: React.Ref<PaymentFieldsHandle>;
};

export type PaymentFieldsAdapter = {
  type: string;
  Fields: React.ComponentType<PaymentFieldsProps>;
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
