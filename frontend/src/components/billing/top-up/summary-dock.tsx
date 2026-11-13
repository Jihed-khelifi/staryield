"use client";

import { useI18n } from "@/i18n/i18n-provider";
import type { PricingPack } from "@/lib/api/pricing";
import { advisorEstimate, type TopUpAdvisor } from "@/lib/billing/top-up";
import { formatPackPrice } from "@/lib/pricing";
import { Button } from "@/components/ui/button";

import { PurchaseAcknowledgement } from "./acknowledgement";

/**
 * The reserved summary under the packs (Figma 1352:3058): the selected pack,
 * the total, the live-reading estimate, "Continue to Checkout", the
 * acknowledgement and "Not now". A sibling of the scrolling list, never an
 * overlay, so it cannot cover the last pack. Everything comes from `pack`.
 */
export function SummaryDock({
  pack,
  context,
  onContinue,
  onDismiss,
}: {
  pack: PricingPack;
  context?: TopUpAdvisor;
  onContinue: () => void;
  onDismiss: () => void;
}) {
  const { text, locale } = useI18n();
  const price = formatPackPrice(pack.price_cents, pack.currency, locale);
  const estimate = advisorEstimate(pack.credits, context, locale, text);
  return (
    <section className="top-up-dock" aria-label={text("Order summary")} data-testid="top-up-dock">
      <div aria-live="polite" aria-atomic="true" className="top-up-dock-summary">
        <p className="top-up-dock-pack" data-testid="top-up-dock-pack">
          {text("{value0} · {value1} credits", { value0: pack.name, value1: pack.credits.toLocaleString(locale) })}
        </p>
        <div className="top-up-dock-total">
          <p data-testid="top-up-total">{text("Total: {value0} USD", { value0: price })}</p>
          <p>{text("One-time payment")}</p>
        </div>
        {estimate ? (
          <p className="top-up-dock-estimate" data-testid="top-up-advisor-estimate">
            {estimate}
          </p>
        ) : null}
      </div>
      <Button type="button" variant="cta" className="top-up-cta" onClick={onContinue} data-testid="top-up-continue">
        {text("Continue to Checkout — {value0}", { value0: price })}
      </Button>
      <PurchaseAcknowledgement pack={pack} />
      <button type="button" className="top-up-dismiss" onClick={onDismiss} data-testid="top-up-dismiss">
        {text("Not now")}
      </button>
    </section>
  );
}
