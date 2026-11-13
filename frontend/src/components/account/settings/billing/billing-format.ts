import type { TranslateCopy } from "@/i18n/translate-copy";
import type { PaymentMethod, Plan, Transaction } from "@/lib/api/billing";

import { formatMoney } from "../format";

const BRANDS: Record<string, string> = { visa: "Visa", mastercard: "Mastercard", amex: "American Express", discover: "Discover" };

/** "Visa", "Mastercard"… (unknown brands are capitalised). */
export function brandName(brand: string): string {
  return BRANDS[brand.toLowerCase()] ?? (brand ? brand.charAt(0).toUpperCase() + brand.slice(1) : "Card");
}

/** "Visa ending in 4242". */
export function cardLabel(method: Pick<PaymentMethod, "brand" | "last4">, text: TranslateCopy): string {
  return text("{value0} ending in {value1}", { value0: brandName(method.brand), value1: method.last4 });
}

/** "Expires 12/25". */
export function expiryLabel(method: Pick<PaymentMethod, "exp_month" | "exp_year">, text: TranslateCopy): string {
  return text("Expires {value0}", { value0: `${String(method.exp_month).padStart(2, "0")}/${String(method.exp_year).slice(-2)}` });
}

/** A saved card that can be charged. */
export function usable(method: PaymentMethod): boolean {
  return method.status === "active" && !method.expired;
}

/** "$9.99 USD / mo", "$99.00 USD / yr", "$25.00 USD every 3 months": the currency is always named (§25.1). */
export function planPrice(plan: Pick<Plan, "price_cents" | "currency" | "interval" | "interval_count">, locale: string, text: TranslateCopy): string {
  const price = `${formatMoney(plan.price_cents, plan.currency, locale)} ${(plan.currency || "USD").toUpperCase()}`;
  const count = Math.max(1, plan.interval_count || 1);
  if (plan.interval === "year") return count === 1 ? text("{value0} / yr", { value0: price }) : text("{value0} every {value1} years", { value0: price, value1: count });
  return count === 1 ? text("{value0} / mo", { value0: price }) : text("{value0} every {value1} months", { value0: price, value1: count });
}

type Cadence = Pick<Plan, "interval" | "interval_count">;

/** "every month", "every 3 months", "every year", "every 2 years": the plan's billing rhythm as a phrase. */
export function planCadence(plan: Cadence, text: TranslateCopy): string {
  const count = Math.max(1, plan.interval_count || 1);
  if (plan.interval === "year") return count === 1 ? text("every year") : text("every {value0} years", { value0: count });
  return count === 1 ? text("every month") : text("every {value0} months", { value0: count });
}

/**
 * The first renewal of a plan bought now: one interval from `from` (one month
 * for the standard plan). The day is clamped, so 31 January renews on the last
 * day of February rather than in March.
 */
export function nextRenewal(plan: Cadence, from: Date = new Date()): Date {
  const count = Math.max(1, plan.interval_count || 1);
  const months = plan.interval === "year" ? 12 * count : count;
  const target = new Date(from.getFullYear(), from.getMonth() + months, 1, from.getHours(), from.getMinutes(), from.getSeconds());
  const lastDay = new Date(target.getFullYear(), target.getMonth() + 1, 0).getDate();
  target.setDate(Math.min(from.getDate(), lastDay));
  return target;
}

/** The plan's benefits as short sentences (admin-set amounts first). */
export function planBenefits(plan: Plan, text: TranslateCopy): string[] {
  const out: string[] = [];
  if (plan.monthly_credit_grant > 0) out.push(text("{value0} credits every month", { value0: plan.monthly_credit_grant }));
  if (plan.monthly_free_minutes > 0) out.push(text("{value0} free chat minutes every month", { value0: plan.monthly_free_minutes }));
  if (plan.chat_discount_pct > 0) out.push(text("{value0}% off paid chat minutes", { value0: plan.chat_discount_pct }));
  if (plan.waitlist_priority) out.push(text("First to know when a busy advisor is free"));
  return [...out, ...(plan.benefits ?? [])];
}

const DECLINES: Record<string, string> = {
  card_declined: "Your card was declined. Try another card.",
  expired_card: "This card has expired. Use another card.",
  insufficient_funds: "Your card was declined for insufficient funds. Try another card.",
  processing_error: "The payment could not be processed. Please try again.",
  payment_method_required: "Add a card to continue.",
};

/** Copy for a `payment_failed` decline reason. */
export function declineMessage(reason: unknown): string {
  return (typeof reason === "string" && DECLINES[reason]) || "Your payment was declined. Try another card.";
}

/** Signed amount for the history: charges are negative, refunds positive. */
export function transactionAmount(t: Pick<Transaction, "kind" | "amount_cents" | "currency">, locale: string): string {
  const amount = formatMoney(t.amount_cents, t.currency, locale);
  return t.kind === "refund" ? `+${amount}` : `−${amount}`;
}

/** Status shown next to a transaction (succeeded ones show none). */
export const TRANSACTION_STATUS: Record<string, string> = {
  pending: "Pending",
  failed: "Failed",
  refunded: "Refunded",
  partially_refunded: "Partly refunded",
  disputed: "Disputed",
};

/** Fallback description per kind. */
export const TRANSACTION_KIND: Record<string, string> = {
  credit_purchase: "Credit purchase",
  subscription_initial: "Premium Membership",
  subscription_renewal: "Membership renewal",
  refund: "Refund",
};
