"use client";

import Link from "next/link";
import { Gift } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { referralProgramOf } from "@/lib/referral";
import { useReferrals } from "@/lib/referrals/referral-provider";
import { useSiteConfig } from "@/lib/site-config/site-config-provider";
import { ErrorState } from "@/components/feedback/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { ReferralLinkField } from "@/components/referrals/referral-link-field";
import { progressLine, rewardSentence } from "@/components/referrals/referral-copy";

const card =
  "rounded-[12px] border border-[rgb(170_142_98/0.35)] bg-[rgb(251_246_233/0.35)] p-[18px] shadow-[0_10px_28px_rgb(23_21_15/0.09)] md:rounded-[4px] md:p-5 md:shadow-none";

/**
 * The account overview's compact invite card: the reward, the link with
 * copy/share, the progress and a link to the invite page. Hidden while the
 * program is off.
 */
export function ReferralCard() {
  const { text, locale } = useI18n();
  const { data, error, reload } = useReferrals();
  const { config } = useSiteConfig();
  const announced = referralProgramOf(config).enabled;

  if (data && !data.program.enabled) return null;
  if (!data) {
    // Before the answer, the public configuration says whether to hold a place.
    if (!announced) return null;
    return (
      <section className={card} aria-labelledby="referral-card-title" data-testid="referral-card">
        <h2 id="referral-card-title" className="text-xl font-semibold text-[#17150f]">
          {text("Invite friends, get free minutes")}
        </h2>
        {error ? (
          <ErrorState error={error} onRetry={reload} compact title={text("Your invite link could not be loaded")} className="mt-3" />
        ) : (
          <div aria-busy="true" className="mt-3 flex flex-col gap-2">
            <p role="status" className="sr-only">
              {text("Loading your invite link…")}
            </p>
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-10 w-full" />
          </div>
        )}
      </section>
    );
  }

  return (
    <section className={card} aria-labelledby="referral-card-title" data-testid="referral-card">
      <div className="flex items-start gap-3">
        <Gift aria-hidden className="mt-0.5 size-6 shrink-0 text-[#8a6a1f]" strokeWidth={1.6} />
        <div className="flex min-w-0 flex-col gap-1">
          <h2 id="referral-card-title" className="text-xl font-semibold text-[#17150f]">
            {text("Invite friends, get free minutes")}
          </h2>
          <p className="text-[13px] leading-[1.5] text-[var(--muted-ink)] md:text-sm">{rewardSentence(data.program, locale, text)}</p>
        </div>
      </div>
      <ReferralLinkField className="mt-4" />
      <div className="mt-3 flex flex-col gap-1 text-[13px] md:flex-row md:items-center md:justify-between md:gap-4">
        <p className="text-[#17150f]" data-testid="referral-progress">
          {progressLine(data.stats, locale, text)}
        </p>
        <Link href={`/${locale}/profile/invite`} className="shrink-0 font-medium text-[#74571a] underline underline-offset-2">
          {text("How it works")}
        </Link>
      </div>
    </section>
  );
}
