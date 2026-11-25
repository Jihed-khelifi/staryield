"use client";

import { HeartHandshake } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { referralProgramOf } from "@/lib/referral";
import { ReferralProvider, useReferrals } from "@/lib/referrals/referral-provider";
import { useSiteConfig } from "@/lib/site-config/site-config-provider";
import { ErrorState } from "@/components/feedback/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { ReferralLinkField } from "@/components/referrals/referral-link-field";
import { rewardSentence } from "@/components/referrals/referral-copy";
import { cn } from "@/lib/utils";

/**
 * "Enjoyed your reading? Invite a friend" in the client's finished-reading
 * summary. Rendered only while the program is on (public configuration, so
 * nothing is requested otherwise); it loads the invite link itself, since the
 * chatroom is outside the account shell.
 */
export function ReferralPrompt({ className }: { className?: string }) {
  const { config } = useSiteConfig();
  if (!referralProgramOf(config).enabled) return null;
  return (
    <ReferralProvider>
      <PromptBody className={className} />
    </ReferralProvider>
  );
}

function PromptBody({ className }: { className?: string }) {
  const { text, locale } = useI18n();
  const { data, error, reload } = useReferrals();
  if (data && !data.program.enabled) return null;
  return (
    <section
      className={cn("rounded-[8px] border border-[rgb(170_142_98/0.45)] bg-[rgb(251_246_233/0.4)] p-3 text-left", className)}
      aria-labelledby="referral-prompt-title"
      data-testid="referral-prompt"
    >
      <h3 id="referral-prompt-title" className="flex items-center gap-2 text-sm font-semibold">
        <HeartHandshake aria-hidden className="size-4 text-[#8a6a1f]" strokeWidth={1.6} />
        {text("Enjoyed your reading? Invite a friend")}
      </h3>
      {data ? (
        <>
          <p className="mt-1 text-xs leading-[1.5] opacity-80">{rewardSentence(data.program, locale, text)}</p>
          <ReferralLinkField className="mt-2" />
        </>
      ) : error ? (
        <ErrorState error={error} onRetry={reload} compact title={text("Your invite link could not be loaded")} className="mt-2" />
      ) : (
        <div aria-busy="true" className="mt-2 flex flex-col gap-2">
          <p role="status" className="sr-only">
            {text("Loading your invite link…")}
          </p>
          <Skeleton className="h-3 w-2/3" />
          <Skeleton className="h-10 w-full" />
        </div>
      )}
    </section>
  );
}
