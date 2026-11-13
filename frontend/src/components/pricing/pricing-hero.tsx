"use client";

import { useI18n } from "@/i18n/i18n-provider";
import { CtaLink } from "@/components/pricing/cta-link";
import { usePricingView } from "@/components/pricing/pricing-context";

/** Figma 1352:2365 — eyebrow, promise, trial invitation and the trust commitments. */
export function PricingHero() {
  const { text } = useI18n();
  const { trialMinutes, signedIn, links } = usePricingView();
  const trial = trialMinutes !== null;

  const cta = signedIn ? text("Browse Psychics") : trial ? text("Start {n} Free Minutes", { n: trialMinutes }) : text("Sign up");
  const chips = [
    ...(trial ? [text("{n} minutes free", { n: trialMinutes })] : []),
    text("One-time payments"),
    text("No subscriptions"),
    text("No automatic top-ups"),
    text("Advisor rates shown upfront"),
  ];

  return (
    <section aria-labelledby="pricing-title" className="flex w-full flex-col items-center gap-6 px-6 py-12 text-center min-[768px]:px-12 min-[768px]:py-16 min-[1200px]:px-24">
      <p className="font-serif text-xs tracking-[1px] text-ink/80 uppercase">
        <span aria-hidden>✦ </span>
        {text("SIMPLE, TRANSPARENT PRICING")}
        <span aria-hidden> ✦</span>
      </p>
      <h1 id="pricing-title" className="max-w-[1060px] font-display text-[clamp(1.875rem,5vw,3rem)] leading-tight text-ink">
        {text("Choose your credits. Choose your advisor. Stay in control.")}
      </h1>
      <p className="max-w-[720px] font-serif text-[18px] leading-normal text-ink">
        {trial
          ? text("Start with {n} minutes free—no payment method required. When you’re ready, choose a one-time credit pack and connect with an advisor whose rate fits your budget.", { n: trialMinutes })
          : text("Create a free account, then choose a one-time credit pack and connect with an advisor whose rate fits your budget.")}
      </p>
      <div className="flex w-full max-w-[380px] flex-col items-center gap-3">
        <CtaLink href={links.trial} className="w-full sm:w-[280px]" data-testid="pricing-hero-cta">
          {cta}
        </CtaLink>
        {trial && !signedIn && <p className="font-serif text-sm text-ink/75">{text("No card required for your free trial.")}</p>}
      </div>
      <ul className="flex w-full flex-wrap justify-center gap-x-4 gap-y-2" aria-label={text("Our commitments")}>
        {chips.map((chip) => (
          <li key={chip} className="rounded-md bg-warm-gray px-3 py-2 font-serif text-sm leading-normal text-[#2f3226]">
            <span aria-hidden>✓ </span>
            {chip}
          </li>
        ))}
      </ul>
    </section>
  );
}
