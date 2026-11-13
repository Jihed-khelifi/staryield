"use client";

import type { PricingSummary } from "@/lib/api/pricing";
import { CreditSteps } from "@/components/pricing/credit-steps";
import { FinalCta } from "@/components/pricing/final-cta";
import { PackGrid } from "@/components/pricing/pack-grid";
import { PricingFaq } from "@/components/pricing/pricing-faq";
import { PricingHero } from "@/components/pricing/pricing-hero";
import { PricingProvider } from "@/components/pricing/pricing-context";

/**
 * Public pricing (Figma 1352:2360 desktop / 1352:2361 mobile). `initial` is
 * the catalog the server fetched (null when the API was unavailable: the page
 * then renders everything but the prices, with a Retry).
 */
export function PricingPage({ initial }: { initial: PricingSummary | null }) {
  return (
    <PricingProvider initial={initial}>
      <div className="flex w-full flex-col items-center bg-cream" data-testid="pricing-page">
        <PricingHero />
        <PackGrid />
        <CreditSteps />
        <PricingFaq />
        <FinalCta />
      </div>
    </PricingProvider>
  );
}
