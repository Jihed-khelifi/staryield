import { PublicFrame } from "@/components/layout/public-frame";
import { PricingPage } from "@/components/pricing/pricing-page";
import type { Locale } from "@/i18n/config";
import { fetchPricing } from "@/lib/pricing/server";

/** Server entry of `/pricing`: reads the live catalog (60 s revalidation, 2 s budget) and renders the page inside the public frame. */
export async function PricingScreen({ locale }: { locale: Locale }) {
  const summary = await fetchPricing(locale);
  return (
    <PublicFrame>
      <PricingPage initial={summary} />
    </PublicFrame>
  );
}
