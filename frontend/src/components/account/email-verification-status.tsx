"use client";

import * as React from "react";
import { BadgeCheck, MailWarning } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { useSession } from "@/lib/auth/session-provider";
import { ResendVerification } from "@/components/account/resend-verification";

/**
 * The e-mail row of the account settings: a "Verified" badge, or "Not
 * verified" with the resend control. The session is the source of truth (the
 * realtime event updates it in every tab); the settings document only
 * decides until the session user is known.
 */
export function EmailVerificationStatus({ verified }: { verified: boolean }) {
  const { text } = useI18n();
  const { user } = useSession();
  const isVerified = user?.email_verified ?? verified;
  if (isVerified) {
    return (
      <p className="flex items-center gap-1.5 text-xs font-medium text-success" data-testid="email-verified-badge">
        <BadgeCheck aria-hidden className="size-4 shrink-0" />
        <span>{text("Verified")}</span>
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-2" data-testid="email-unverified">
      <p className="flex items-center gap-1.5 text-xs font-medium text-[var(--settings-text)]">
        {/* The gold marks the state (icon contrast 3:1); the label keeps the text colour (4.5:1 on every card background). */}
        <MailWarning aria-hidden className="size-4 shrink-0 text-[var(--settings-gold-text)]" />
        <span>{text("Not verified")}</span>
      </p>
      <ResendVerification />
    </div>
  );
}
