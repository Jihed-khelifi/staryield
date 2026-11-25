"use client";

import * as React from "react";
import { MailCheck } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { useSession } from "@/lib/auth/session-provider";
import { ResendVerification } from "@/components/account/resend-verification";

/**
 * Account overview card for an address that is not verified yet: why it
 * matters and the resend control. It disappears as soon as the address is
 * verified, in this tab or any other (the realtime event updates the session).
 */
export function EmailVerificationNotice() {
  const { text } = useI18n();
  const { status, user } = useSession();
  if (status !== "authenticated" || !user || user.email_verified !== false) return null;
  return (
    <section
      className="flex flex-col gap-3 rounded-[12px] border border-[rgb(170_142_98/0.45)] bg-[rgb(251_246_233/0.55)] p-[18px] shadow-[0_10px_28px_rgb(23_21_15/0.07)] md:flex-row md:items-center md:justify-between md:gap-6"
      aria-labelledby="verify-email-title"
      data-testid="email-verification-notice"
    >
      <div className="flex items-start gap-3">
        <MailCheck aria-hidden className="mt-0.5 size-6 shrink-0 text-[#8a6a1f]" strokeWidth={1.6} />
        <div className="flex flex-col gap-1">
          <h2 id="verify-email-title" className="text-lg font-semibold text-[#17150f]">
            {text("Confirm your email address")}
          </h2>
          <p className="text-[13px] leading-[1.55] font-light text-[var(--muted-ink)] md:text-sm">
            {text("We sent a link to {value0}. Confirming it secures your account and lets us send you reading receipts and account notices.", { value0: user.email })}
          </p>
        </div>
      </div>
      <ResendVerification />
    </section>
  );
}
