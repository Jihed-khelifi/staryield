"use client";

import { FlaskConical, LockKeyhole } from "lucide-react";

import { POWERED_BY_STRIPE_HREF, POWERED_BY_STRIPE_SIZE, POWERED_BY_STRIPE_SRC } from "@/config/payments";
import { useI18n } from "@/i18n/i18n-provider";

/**
 * Under Stripe's card fields, always (revisions 6.0, §26.5): who processes the
 * payment, what Staryield never sees, the "Powered by Stripe" mark (from R2,
 * linking to stripe.com as Stripe asks) and, with test keys, a "Test mode"
 * badge.
 */
export function SecuredByStripe({ livemode }: { livemode?: boolean }) {
  const { text } = useI18n();
  return (
    <div className="payment-secured" data-testid="secured-by-stripe">
      <p className="payment-secured-copy">
        <LockKeyhole aria-hidden className="size-3.5 shrink-0" />
        <span data-testid="secured-by-stripe-copy">
          {text("Payments are processed securely by Stripe. Your card details are encrypted and sent directly to Stripe — Staryield never sees or stores them.")}
        </span>
      </p>
      <div className="payment-secured-marks">
        <a href={POWERED_BY_STRIPE_HREF} target="_blank" rel="noopener noreferrer" className="payment-stripe-mark" aria-label={text("Powered by Stripe (opens in a new tab)")}>
          {/* eslint-disable-next-line @next/next/no-img-element -- a fixed SVG from R2; no optimisation needed */}
          <img src={POWERED_BY_STRIPE_SRC} alt={text("Powered by Stripe")} width={POWERED_BY_STRIPE_SIZE.width} height={POWERED_BY_STRIPE_SIZE.height} loading="eager" decoding="async" />
        </a>
        {livemode === false ? (
          <span className="payment-test-badge" data-testid="stripe-test-mode">
            <FlaskConical aria-hidden className="size-3.5 shrink-0" />
            {text("Test mode")}
          </span>
        ) : null}
      </div>
    </div>
  );
}
