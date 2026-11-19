/**
 * Run a purchase's `next_action` without a mounted card form (revisions 6.0,
 * §26.5): a purchase paid with a *saved* card can still need 3-D Secure. For a
 * new card the mounted Payment Element does it (`PaymentFieldsHandle`); both
 * share `finishStripeAction`.
 *
 *  - `use_stripe_sdk`: load Stripe.js with the publishable key from
 *    `GET /billing/client-config` and `stripe.handleNextAction` (a modal, no
 *    redirect).
 *  - `sandbox_3ds`: the development sandbox settles by itself: nothing to do.
 *  - a provider page (`url`): https only, as before; the payer leaves the page.
 */
import type { Stripe } from "@stripe/stripe-js";

import { billingApi, type NextAction } from "@/lib/api/billing";
import { toApiError } from "@/lib/api/errors";
import { currentLocale } from "@/lib/api/config";
import type { NextActionResult } from "@/lib/payments/payment-fields";
import { getStripe } from "@/lib/payments/stripe";

/** The outcome plus whether the browser is leaving for the provider's page. */
export type RunNextActionResult = NextActionResult & { redirected?: boolean };

/** `stripe.handleNextAction` for a PaymentIntent's client secret. */
export async function finishStripeAction(stripe: Stripe, clientSecret: string): Promise<NextActionResult> {
  try {
    const { error } = await stripe.handleNextAction({ clientSecret });
    if (error) return { ok: false, message: error.message };
    return { ok: true };
  } catch (error) {
    return { ok: false, message: error instanceof Error ? error.message : undefined };
  }
}

/** True for an absolute https URL (a provider page the payer may be sent to). */
export function isHttpsUrl(url: string | undefined): url is string {
  return typeof url === "string" && /^https:\/\//.test(url);
}

export async function runNextAction(next: NextAction, signal?: AbortSignal): Promise<RunNextActionResult> {
  if (next.type === "sandbox_3ds") return { ok: true };
  if (next.type === "use_stripe_sdk" && next.client_secret) {
    let stripe: Stripe | null;
    try {
      // Only the publishable key is needed here (no Payment Element is built).
      const config = await billingApi.clientConfig("credits", {}, signal);
      if (!config.publishable_key) return { ok: false };
      stripe = await getStripe(config.publishable_key, currentLocale());
    } catch (error) {
      if (toApiError(error).code === "aborted") throw error;
      return { ok: false };
    }
    if (!stripe) return { ok: false };
    return finishStripeAction(stripe, next.client_secret);
  }
  if (next.url) {
    if (!isHttpsUrl(next.url)) return { ok: false };
    window.location.assign(next.url);
    return { ok: true, redirected: true };
  }
  // An action this build does not know: do not pretend it was completed.
  return { ok: false };
}
