"use client";

import { useI18n } from "@/i18n/i18n-provider";
import { CtaLink } from "@/components/pricing/cta-link";
import { usePricingView } from "@/components/pricing/pricing-context";

/** Figma 1352:2369 — the closing invitation on the sage band. */
export function FinalCta() {
  const { text } = useI18n();
  const { trialMinutes, signedIn, links } = usePricingView();
  const trial = trialMinutes !== null;
  const cta = signedIn ? text("Browse Psychics") : trial ? text("Start My Free Reading") : text("Sign up");

  return (
    <section aria-labelledby="final-title" className="paper-overlay flex w-full flex-col items-center gap-4 bg-sage px-6 py-12 text-center min-[768px]:px-12 min-[768px]:py-16">
      <div className="relative z-10 flex w-full flex-col items-center gap-4">
        <h2 id="final-title" className="font-display text-[clamp(1.5rem,3.5vw,2.25rem)] leading-tight text-white">
          {trial ? text("Start with {n} minutes free", { n: trialMinutes }) : text("Start your first reading")}
        </h2>
        <CtaLink href={links.trial} className="w-full sm:w-[300px]" data-testid="pricing-final-cta">
          {cta}
        </CtaLink>
        <p className="font-serif text-base text-ink">{text("No card required. No automatic charges.")}</p>
      </div>
    </section>
  );
}
