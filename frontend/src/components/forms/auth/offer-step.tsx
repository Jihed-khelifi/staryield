"use client";

import * as React from "react";
import { useRouter } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { useSession } from "@/lib/auth/session-provider";
import { useSignupFreeMinutes } from "@/lib/site-config/site-config-provider";

import { AuthCard, AuthShell } from "./auth-card";

/** Last sign-up step: the introductory offer, then on to the advisors. */
export function OfferStep() {
  const { text, locale } = useI18n();
  const router = useRouter();
  const { status } = useSession();
  const freeMinutes = useSignupFreeMinutes();

  React.useEffect(() => {
    track("signup_step_viewed", { step: "offer" });
  }, []);

  React.useEffect(() => {
    if (status === "anonymous") router.replace(`/${locale}/login?next=${encodeURIComponent(`/${locale}/signup/offer`)}`);
  }, [status, locale, router]);

  return (
    <AuthShell>
      <AuthCard
        title="Find love and build strong relationships with Staryield"
        className="offer-card"
        legal={false}
        testId="offer-step"
        onSubmit={(event) => {
          event.preventDefault();
          track("signup_step_completed", { step: "offer" });
          router.push(`/${locale}/psychics`);
        }}
      >
        {freeMinutes > 0 ? (
          <p className="mt-2 text-center font-light" data-testid="offer-free-minutes">
            {text("First {value0} minutes are on us!", { value0: freeMinutes })}
          </p>
        ) : null}
      </AuthCard>
    </AuthShell>
  );
}
