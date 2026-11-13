"use client";

/**
 * Everything the sections of the pricing page share, so none of them takes
 * the catalog, the session or the live trial as props: the catalog (server
 * state), the free-trial minutes, the advisor rate range, and where each
 * call to action leads for this visitor.
 *
 * Signed-out hrefs are what the server renders; the session upgrades them on
 * the client once it is known (no markup differs between server and first
 * client render, so there is no hydration mismatch).
 */
import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import type { PricingSummary, RateRange } from "@/lib/api/pricing";
import { useSession } from "@/lib/auth/session-provider";
import { psychicsHref } from "@/lib/psychics-link";
import { useSignupFreeMinutes } from "@/lib/site-config/site-config-provider";
import { usePricing, type PricingStatus } from "@/components/pricing/use-pricing";

type PricingView = {
  summary: PricingSummary | null;
  status: PricingStatus;
  retry: () => void;
  /** Live free-trial minutes; null when the offer is off (or unknown: plain "sign up" copy). */
  trialMinutes: number | null;
  /** Credits per minute advisors charge; null while the catalog is unavailable. */
  rate: RateRange | null;
  signedIn: boolean;
  links: {
    /** "Start N free minutes" / "Start my free reading": sign-up, or the advisor directory when signed in. */
    trial: string;
    /** Pack buttons: sign-up, or the top-up page when signed in. */
    pack: string;
  };
};

const PricingContext = React.createContext<PricingView | null>(null);

export function PricingProvider({ initial, children }: { initial: PricingSummary | null; children: React.ReactNode }) {
  const { locale } = useI18n();
  const { status: session } = useSession();
  const { summary, status, retry } = usePricing(initial);
  // Without a catalog the layout's site configuration still knows the live trial.
  const siteTrial = useSignupFreeMinutes();

  const value = React.useMemo<PricingView>(() => {
    const signedIn = session === "authenticated";
    const trial = summary ? (summary.free_trial.enabled ? summary.free_trial.minutes : 0) : siteTrial;
    return {
      summary,
      status,
      retry,
      trialMinutes: trial > 0 ? trial : null,
      rate: summary?.rate_range ?? null,
      signedIn,
      links: {
        trial: signedIn ? psychicsHref(locale, true) : `/${locale}/signup`,
        pack: signedIn ? `/${locale}/credits` : `/${locale}/signup`,
      },
    };
  }, [summary, status, retry, session, siteTrial, locale]);

  return <PricingContext.Provider value={value}>{children}</PricingContext.Provider>;
}

export function usePricingView(): PricingView {
  const value = React.useContext(PricingContext);
  if (!value) throw new Error("usePricingView must be used inside <PricingProvider>");
  return value;
}
