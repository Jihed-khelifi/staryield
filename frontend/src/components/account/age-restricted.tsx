"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ShieldAlert } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { ageStatusOf, birthStepHref } from "@/lib/auth/age-status";
import { useSession } from "@/lib/auth/session-provider";
import { AuthLegal, AuthShell } from "@/components/forms/auth/auth-card";
import { Button } from "@/components/ui/button";

/**
 * `/{locale}/age-restricted` (revisions 5.0, §24.6): the calm full-page screen
 * of an account restricted to adults. No retry with another date: the way
 * forward is support (a wrong date is reviewed by a person), closing the
 * account, or reading the privacy policy. Anyone who is not restricted is sent
 * on, so the page never contradicts the account.
 */
export function AgeRestricted() {
  const { text, locale } = useI18n();
  const router = useRouter();
  const { status, user } = useSession();
  const age = status === "authenticated" ? ageStatusOf(user) : null;

  React.useEffect(() => {
    if (status === "anonymous") router.replace(`/${locale}/login?next=${encodeURIComponent(`/${locale}/age-restricted`)}`);
    else if (age === "adult") router.replace(`/${locale}/profile`);
    else if (age === "birth_date_required") router.replace(birthStepHref(locale));
  }, [status, age, locale, router]);

  if (age !== "restricted") return null;

  return (
    <AuthShell promo={false}>
      <section className="auth-card paper-page email-link-card" aria-labelledby="age-restricted-title" data-testid="age-restricted">
        <div className="auth-card-body">
          <ShieldAlert aria-hidden className="email-link-icon text-amber" />
          <h1 id="age-restricted-title" className="text-center text-[28px] font-medium leading-tight" tabIndex={-1}>
            {text("Account restricted")}
          </h1>
          <div className="grid gap-3 text-center text-sm font-light">
            <p>{text("Staryield is for adults 18 and over. Chats, readings and purchases are not available for this account.")}</p>
            <p>{text("If you entered the wrong date of birth, contact our support team and we will review it.")}</p>
          </div>
        </div>
        <div className="email-link-actions">
          <Button asChild className="auth-continue gold-button w-full">
            <Link href={`/${locale}/contact`} data-testid="age-restricted-support">
              {text("Contact support")}
            </Link>
          </Button>
          <Button asChild variant="outline" size="lg" className="w-full border-ink/40">
            <Link href={`/${locale}/profile/settings#account-actions`} data-testid="age-restricted-close">
              {text("Close my account")}
            </Link>
          </Button>
          <Button asChild variant="outline" size="lg" className="w-full border-ink/40">
            <Link href={`/${locale}/privacy`} data-testid="age-restricted-privacy">
              {text("Privacy Policy")}
            </Link>
          </Button>
        </div>
        <AuthLegal />
      </section>
    </AuthShell>
  );
}
