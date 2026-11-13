/**
 * Pricing catalog, the welcome offer and refund requests
 * (docs/platform-extensions.md §24). Prices are integer cents; every
 * savings badge is computed by the API from the live prices, never here.
 * Internal economics (fees, advisor share) never reach the web app.
 */
import { api } from "@/lib/api/client";
import type { Page, Wallet } from "@/lib/api/types";

export type PackBadge = {
  type: "new_client_offer" | "most_popular" | "best_value" | "save";
  /** `save` only: whole percent, floored by the API so it is never overstated. */
  percent?: number;
};

export type PricingPack = {
  id: string;
  code: string;
  kind: "standard" | "intro";
  /** Localized by the API (falls back to English). */
  name: string;
  tagline: string;
  /** Short eyebrow used when the pack has no badge ("" when unset). */
  label: string;
  credits: number;
  price_cents: number;
  currency: string;
  featured: boolean;
  new_clients_only: boolean;
  sort_order: number;
  badges: PackBadge[];
};

/** Credits per minute advisors may charge. */
export type RateRange = { min: number; max: number };

/** GET /pricing (public, server-rendered pricing page). */
export type PricingSummary = {
  currency: string;
  free_trial: { enabled: boolean; minutes: number };
  rate_range: RateRange;
  reference?: { pack_id: string; credits: number; price_cents: number };
  packs: PricingPack[];
  generated_at: string;
};

/** GET /credits/catalog (signed-in top-up dialog: one round trip). */
export type TopUpCatalog = {
  /** What this account can buy now: the eligible intro pack first, then standard packs. */
  packs: PricingPack[];
  welcome_offer: { eligible: boolean; pack_id?: string };
  rate_range: RateRange;
  wallet: Wallet;
  /** What the buyer's card statement shows (§25.3); absent from older APIs. */
  statement_descriptor?: string;
};

/** `offer_unavailable` (409) `details.reason`. */
export type OfferUnavailableReason = "already_redeemed" | "not_new_client" | "payment_method_used" | "card_check_unavailable" | "no_offer";

export type RefundReason = "unused_credits" | "technical_problem" | "incorrect_charge" | "unauthorized_charge" | "reading_concern" | "other";
export type RefundRequestStatus = "open" | "approved" | "denied" | "credited" | "withdrawn";

export type RefundQuote = {
  transaction_id: string;
  pack_name: string;
  currency: string;
  amount_paid_cents: number;
  already_refunded_cents: number;
  credits_purchased: number;
  credits_used: number;
  credits_remaining: number;
  refundable_cents: number;
  purchased_at: string;
  window_ends_at: string;
  within_window: boolean;
};

export type RefundRequest = {
  id: string;
  /** "RR-" + 8 characters, shown to the client and support. */
  reference: string;
  transaction_id: string;
  status: RefundRequestStatus;
  reason: RefundReason;
  details: string;
  channel: "web" | "email" | "admin";
  /** Snapshot when the request was made. */
  quote: RefundQuote;
  resolution?: { note: string; refunded_cents?: number; credits_granted?: number; resolved_at: string };
  created_at: string;
  updated_at: string;
};

/** On credit-purchase rows of GET /billing/transactions. */
export type TransactionRefundInfo = {
  refundable_cents: number;
  credits_remaining: number;
  window_ends_at: string;
  can_request: boolean;
  request?: { id: string; reference: string; status: RefundRequestStatus };
};

export const pricingApi = {
  /** Public summary; the pricing page fetches it on the server (see `lib/pricing/server.ts`). */
  summary: (signal?: AbortSignal) => api<PricingSummary>("/api/v1/pricing", { signal }),
  /** Signed-in clients: packs this account can buy now, offer eligibility, rate range and wallet. */
  catalog: (signal?: AbortSignal) => api<TopUpCatalog>("/api/v1/credits/catalog", { signal }),
};

export const refundsApi = {
  quote: (transactionId: string, signal?: AbortSignal) =>
    api<RefundQuote>("/api/v1/billing/refund-quote", { query: { transaction_id: transactionId }, signal }),
  create: (input: { transaction_id: string; reason: RefundReason; details?: string }, idempotencyKey: string) =>
    api<RefundRequest>("/api/v1/billing/refund-requests", {
      method: "POST",
      body: { ...input, idempotency_key: idempotencyKey },
      idempotent: true,
    }),
  list: (cursor?: string, limit = 20, signal?: AbortSignal) =>
    api<Page<RefundRequest>>("/api/v1/billing/refund-requests", { query: { cursor, limit }, signal }),
  withdraw: (id: string) =>
    api<RefundRequest>(`/api/v1/billing/refund-requests/${encodeURIComponent(id)}/withdraw`, { method: "POST", idempotent: true }),
};
