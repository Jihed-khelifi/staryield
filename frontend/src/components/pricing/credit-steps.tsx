"use client";

import { useI18n } from "@/i18n/i18n-provider";
import { PricingSection } from "@/components/pricing/cta-link";
import { usePricingView } from "@/components/pricing/pricing-context";

/** Figma 1352:2367 — "A little clarity on credits": three steps. */
export function CreditSteps() {
  const { text } = useI18n();
  const { trialMinutes, rate } = usePricingView();
  const trial = trialMinutes !== null;

  const steps = [
    trial
      ? { title: text("Try {n} minutes free", { n: trialMinutes }), body: text("Create an account and start your free trial. No payment method required.") }
      : { title: text("Create a free account"), body: text("Sign up in a minute and browse our advisors. No payment method required.") },
    {
      title: text("Check your advisor’s rate"),
      body: rate
        ? text("Advisors charge {min}–{max} credits per minute. You’ll see the exact rate before your reading begins.", { min: rate.min, max: rate.max })
        : text("Advisors set their own rate per minute. You’ll see the exact rate before your reading begins."),
    },
    {
      title: text("Choose a pack when you’re ready"),
      body: trial
        ? text("After your free trial, choose a one-time credit pack from the top-up menu. There is no subscription or automatic recharge.")
        : text("Choose a one-time credit pack from the top-up menu whenever you need more credits. There is no subscription or automatic recharge."),
    },
  ];

  return (
    <PricingSection aria-labelledby="steps-title" innerClassName="flex flex-col gap-8">
      <h2 id="steps-title" className="font-display text-[clamp(1.5rem,3.5vw,2.25rem)] leading-tight text-ink">
        {text("A little clarity on credits")}
      </h2>
      <ol className="grid grid-cols-1 gap-6 min-[768px]:grid-cols-2 min-[1200px]:grid-cols-3" data-testid="credit-steps">
        {steps.map((step, index) => (
          <li key={index} className="flex flex-col gap-4 rounded-md bg-warm-gray px-7 py-6 font-serif leading-normal">
            <span aria-hidden className="text-3xl text-ink/70">
              {String(index + 1).padStart(2, "0")}
            </span>
            <h3 className="text-2xl text-ink">{step.title}</h3>
            <p className="text-lg text-[#3f4133]">{step.body}</p>
          </li>
        ))}
      </ol>
    </PricingSection>
  );
}
