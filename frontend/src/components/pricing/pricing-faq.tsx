"use client";

import Link from "next/link";

import { useI18n } from "@/i18n/i18n-provider";
import { PricingSection } from "@/components/pricing/cta-link";
import { usePricingView } from "@/components/pricing/pricing-context";

/** Marks where the Refund Policy link goes, so each language keeps its own word order. */
const LINK_SLOT = "\u0000";

/** Figma 1352:2368 — "Before your first session": the design's static question and answer list. */
export function PricingFaq() {
  const { text, locale } = useI18n();
  const { trialMinutes, rate } = usePricingView();
  const trial = trialMinutes !== null;
  const [before, after] = text("Review our {link}.", { link: LINK_SLOT }).split(LINK_SLOT);

  const items = [
    trial
      ? { q: text("Do I need a card for the free trial?"), a: text("No. Your first {n} minutes require no payment method.", { n: trialMinutes }) }
      : { q: text("Do I need a card to sign up?"), a: text("No. Creating an account requires no payment method.") },
    { q: text("Are these subscriptions?"), a: text("No. Each credit pack is a one-time purchase.") },
    {
      q: text("Why do minute estimates vary?"),
      a: rate
        ? text("Advisors set rates between {min} and {max} credits per minute. Fewer credits per minute means more reading time from the same pack.", { min: rate.min, max: rate.max })
        : text("Advisors set their own rates. Fewer credits per minute means more reading time from the same pack."),
    },
    {
      q: text("Where can I buy a pack?"),
      a: trial
        ? text("After signing up, available packs appear in your top-up menu after the free trial or whenever you need more credits.")
        : text("After signing up, available packs appear in your top-up menu whenever you need more credits."),
    },
  ];

  return (
    <PricingSection aria-labelledby="faq-title" className="bg-cream-light" innerClassName="flex max-w-[800px] flex-col gap-6">
      <h2 id="faq-title" className="font-display text-[clamp(1.5rem,3.5vw,2.25rem)] leading-tight text-ink">
        {text("Before your first session")}
      </h2>
      <dl className="flex flex-col font-serif leading-normal" data-testid="pricing-faq">
        {items.map((item) => (
          <div key={item.q} className="flex flex-col gap-2 border-b border-warm-gray py-4">
            <dt className="text-xl text-ink">{item.q}</dt>
            <dd className="text-lg text-[#3f4133]">{item.a}</dd>
          </div>
        ))}
        <div className="flex flex-col gap-2 border-b border-warm-gray py-4">
          <dt className="text-xl text-ink">{text("What is your refund policy?")}</dt>
          <dd className="text-lg text-[#3f4133]">
            {before}
            <Link
              href={`/${locale}/refund-policy`}
              className="inline-block min-h-11 py-2 font-semibold text-[#6b4f1d] underline decoration-[#c29a3d] underline-offset-2 hover:text-ink focus-visible:rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#c29a3d]"
              data-testid="faq-refund-link"
            >
              {text("Refund Policy")}
            </Link>
            {after}
          </dd>
        </div>
      </dl>
    </PricingSection>
  );
}
