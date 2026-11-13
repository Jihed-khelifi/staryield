/**
 * Premium Membership, saved cards and payment history
 * (docs/platform-extensions.md §12). Card data never reaches these routes:
 * the browser loads the routed provider's card fields (`clientConfig`), the
 * provider returns a token, and only that token is posted. Prices always
 * come from the server-side plan version.
 */
import { api, newId } from "@/lib/api/client";
import type { TransactionRefundInfo } from "@/lib/api/pricing";
import type { Page, Purchase } from "@/lib/api/types";
import type { LegalConsent } from "@/lib/legal/versions";

export type Plan = {
  id: string;
  code: string;
  name: string;
  description: string;
  version_id: string;
  version: number;
  price_cents: number;
  currency: string;
  interval: "month" | "year";
  interval_count: number;
  monthly_credit_grant: number;
  chat_discount_bps: number;
  chat_discount_pct: number;
  monthly_free_minutes: number;
  waitlist_priority: boolean;
  benefits: string[];
};

export type ClientConfigType = "sandbox" | "stripe_elements" | "nmi_collectjs" | "authnet_accept_hosted";

export type ClientConfig = {
  provider: string;
  type: ClientConfigType | (string & {});
  publishable_key?: string;
  tokenization_key?: string;
  hosted_form_url?: string;
  client_secret?: string;
  token_format?: string;
  test_cards?: { brand: string; last4: string; behaviour: string }[];
  expires_at?: string;
};

export type PaymentMethod = {
  id: string;
  provider: string;
  brand: string;
  last4: string;
  exp_month: number;
  exp_year: number;
  is_default: boolean;
  status: "active" | "expired" | "failed";
  expired: boolean;
  created_at: string;
};

export type SubscriptionStatus = "incomplete" | "active" | "past_due" | "canceled" | "expired";

export type Subscription = {
  id: string;
  status: SubscriptionStatus;
  plan: Plan;
  cancel_at_period_end: boolean;
  current_period_start?: string;
  current_period_end?: string;
  renews_at?: string;
  next_retry_at?: string;
  grace_until?: string;
  canceled_at?: string;
  ended_at?: string;
  payment_method?: PaymentMethod;
  created_at: string;
};

export type TransactionKind = "credit_purchase" | "subscription_initial" | "subscription_renewal" | "refund";
export type TransactionStatus = "pending" | "succeeded" | "failed" | "refunded" | "partially_refunded" | "disputed";

export type Transaction = {
  id: string;
  kind: TransactionKind | (string & {});
  status: TransactionStatus | (string & {});
  amount_cents: number;
  refunded_amount_cents: number;
  currency: string;
  description: string;
  credits?: number;
  payment_method?: { brand: string; last4: string };
  failure_code?: string;
  /** revisions 5.0 (§24.4): credit purchases only. */
  refund?: TransactionRefundInfo;
  created_at: string;
};

/** E.g. 3-D Secure: the provider needs the payer on its page. */
export type NextAction = { type: string; url?: string; client_secret?: string };

export type SubscriptionResult = { subscription: Subscription; transaction?: Transaction; next_action?: NextAction };

export type PaymentPurpose = "credits" | "subscription";

export const billingApi = {
  plans: (signal?: AbortSignal) => api<{ items: Plan[] }>("/api/v1/billing/plans", { signal }).then((r) => r.items),
  clientConfig: (purpose: PaymentPurpose, planVersionId?: string, signal?: AbortSignal) =>
    api<ClientConfig>("/api/v1/billing/client-config", { query: { purpose, plan_version_id: planVersionId }, signal }),
  paymentMethods: (signal?: AbortSignal) =>
    api<{ items: PaymentMethod[] }>("/api/v1/billing/payment-methods", { signal }).then((r) => r.items),
  /** Saves a provider token (201, or 200 when the same card is already saved). */
  attachMethod: (input: { provider: string; token: string; billing_country?: string; make_default?: boolean }, idempotencyKey = newId()) =>
    api<PaymentMethod>("/api/v1/billing/payment-methods", {
      method: "POST",
      body: { ...input, idempotency_key: idempotencyKey },
      idempotent: true,
      timeoutMs: 30_000,
    }),
  /** 409 payment_method_required when it is the only card of a renewing membership. */
  removeMethod: (id: string) => api<void>(`/api/v1/billing/payment-methods/${encodeURIComponent(id)}`, { method: "DELETE", idempotent: true }),
  setDefault: (id: string) =>
    api<PaymentMethod>(`/api/v1/billing/payment-methods/${encodeURIComponent(id)}/default`, { method: "POST", idempotent: true }),
  subscription: (signal?: AbortSignal) =>
    api<{ subscription: Subscription | null }>("/api/v1/billing/subscription", { signal }).then((r) => r.subscription),
  /** Charges the first period now; 402 payment_failed (details.reason) on a decline. */
  subscribe: (input: { plan_version_id: string; payment_method_id: string }, idempotencyKey: string) =>
    api<SubscriptionResult>("/api/v1/billing/subscriptions", {
      method: "POST",
      body: { ...input, idempotency_key: idempotencyKey },
      idempotent: true,
      timeoutMs: 30_000,
    }),
  /** Changes the membership's card; a past-due membership is charged at once. */
  changeMethod: (paymentMethodId: string) =>
    api<SubscriptionResult>("/api/v1/billing/subscription", { method: "PATCH", body: { payment_method_id: paymentMethodId }, timeoutMs: 30_000 }),
  /** Stops auto-renewal; benefits last until the end of the paid period. */
  cancel: (reason?: string) =>
    api<{ subscription: Subscription }>("/api/v1/billing/subscription/cancel", {
      method: "POST",
      body: reason ? { reason } : undefined,
      idempotent: true,
    }).then((r) => r.subscription),
  resume: () => api<{ subscription: Subscription }>("/api/v1/billing/subscription/resume", { method: "POST", idempotent: true }).then((r) => r.subscription),
  transactions: (cursor?: string, limit = 5, signal?: AbortSignal) =>
    api<Page<Transaction>>("/api/v1/billing/transactions", { query: { cursor, limit }, signal }),
};

/** POST /credits/purchases with a saved card or a provider token (§12). */
export function purchaseCredits(input: {
  pack_id: string;
  idempotency_key: string;
  /** The Terms, Privacy and Refund Policy versions the buyer agreed to (§25.2); 422 `fields.consent` when missing. */
  consent: LegalConsent;
  receipt_email?: string;
  payment_method_id?: string;
  payment_token?: string;
  provider?: string;
  save_method?: boolean;
}) {
  return api<Purchase & { next_action?: NextAction }>("/api/v1/credits/purchases", {
    method: "POST",
    body: input,
    idempotent: true,
    timeoutMs: 30_000,
  });
}
