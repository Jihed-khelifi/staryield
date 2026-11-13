"use client";

import { useI18n } from "@/i18n/i18n-provider";
import type { PricingPack } from "@/lib/api/pricing";
import { formatPackPrice } from "@/lib/pricing";
import { cn } from "@/lib/utils";
import { CtaLink } from "@/components/pricing/cta-link";
import { formatCredits, packEyebrow, packMinutesLine } from "@/components/pricing/pack-copy";
import { usePricingView } from "@/components/pricing/pricing-context";

/**
 * One credit pack (Figma "Pricing card"): badge or label, name, one-time USD
 * price, credits, the approximate minutes at the cheapest and the dearest
 * advisor, the tagline, and the call to action. The eyebrow is computed from
 * the API's badges, so every claim matches the live prices.
 */
export function PackCard({ pack }: { pack: PricingPack }) {
  const { text, locale } = useI18n();
  const { rate, trialMinutes, signedIn, links } = usePricingView();
  const intro = pack.kind === "intro";
  const eyebrow = packEyebrow(pack, text);
  const titleId = `pack-${pack.code || pack.id}`;
  const label = intro ? text("Start With the Welcome Offer") : text("Get Started");
  const fine = intro
    ? text("Available once per eligible new customer after sign-up. Limit one per customer and payment method.")
    : signedIn
      ? null
      : trialMinutes !== null
        ? text("Start with {n} free minutes. No card required.", { n: trialMinutes })
        : text("Create a free account.");

  return (
    <article
      aria-labelledby={titleId}
      data-testid="pack-card"
      data-pack={pack.code}
      data-featured={pack.featured || undefined}
      className={cn(
        "paper-overlay flex h-full flex-col gap-3 rounded-md border bg-cream p-6 text-ink",
        pack.featured ? "border-2 border-gold bg-sage" : "border-warm-gray",
      )}
    >
      {/* The paper overlay paints above the card, so content sits in its own stacking layer. */}
      <div className="relative z-10 flex h-full flex-col gap-3">
        {eyebrow && (
          <p className={cn("w-fit max-w-full rounded-sm px-2 py-1 font-serif text-xs leading-snug uppercase", pack.featured ? "bg-amber text-ink" : "bg-[#e5e3d5] text-[#3a3d2d]")}>
            {eyebrow}
          </p>
        )}
        <h3 id={titleId} className="font-serif text-2xl font-semibold leading-snug">
          {pack.name}
        </h3>
        <p className="flex flex-wrap items-baseline gap-x-2 font-serif leading-snug">
          <span className="text-5xl">{formatPackPrice(pack.price_cents, pack.currency, locale)}</span>
          <span className={cn("text-sm", pack.featured ? "text-ink" : "text-ink/75")}>{text("{currency} · one time", { currency: pack.currency })}</span>
        </p>
        <p className="font-serif text-3xl leading-snug">{text("{n} credits", { n: formatCredits(pack.credits, locale) })}</p>
        {rate && <p className={cn("min-h-[3em] font-serif text-base leading-snug", pack.featured ? "text-ink" : "text-[#414536]")}>{packMinutesLine(pack.credits, rate, text, locale)}</p>}
        {pack.tagline && <p className={cn("font-serif text-lg leading-snug", pack.featured ? "text-ink" : "text-ink/80")}>{pack.tagline}</p>}
        <div className="mt-auto flex flex-col gap-3 pt-3">
          <CtaLink href={links.pack} className="w-full" aria-label={`${label} — ${pack.name}`} data-testid="pack-cta">
            {label}
          </CtaLink>
          {fine && <p className={cn("font-serif text-xs leading-snug", pack.featured ? "text-ink" : "text-ink/75")}>{fine}</p>}
        </div>
      </div>
    </article>
  );
}
