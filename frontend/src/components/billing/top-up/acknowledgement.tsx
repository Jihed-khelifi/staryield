"use client";

import { useI18n } from "@/i18n/i18n-provider";
import type { PricingPack } from "@/lib/api/pricing";
import { formatPackPrice } from "@/lib/pricing";
import { LinkedCopy } from "@/components/billing/linked-copy";
import { usePolicyLink } from "@/components/billing/policy-links";

/**
 * "By continuing, you agree to purchase …": states what is bought (§24.7). On
 * the pay step it sits next to "Pay" and the policy checkbox (§25.6) carries
 * the formal agreement. It always states the selected pack, and its two links
 * open in a new tab so a live chat is never left.
 */
export function PurchaseAcknowledgement({ pack, id }: { pack: PricingPack; id?: string }) {
  const { text, locale } = useI18n();
  const copy = text(
    "By continuing, you agree to purchase {value0} credits for {value1} USD as a one-time payment. No subscription or automatic top-up is included. Review our [Terms] and [Refund Policy].",
    { value0: pack.credits.toLocaleString(locale), value1: formatPackPrice(pack.price_cents, pack.currency, locale) },
  );
  const link = usePolicyLink();
  return (
    <p id={id} className="top-up-ack" data-testid="top-up-ack">
      <LinkedCopy copy={copy} links={[link("terms"), link("refund-policy")]} />
    </p>
  );
}
