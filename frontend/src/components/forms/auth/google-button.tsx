"use client";

import * as React from "react";
import { Loader2 } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { toApiError, type ApiError } from "@/lib/api/errors";
import type { GoogleIntent } from "@/lib/api/types";
import { startGoogle } from "@/lib/auth/google";
import { useSiteConfig } from "@/lib/site-config/site-config-provider";
import { cn } from "@/lib/utils";
import { GoogleG } from "@/components/brand/google-g";

/** Google sign-in is configured on the API (older APIs send no `auth`: off). */
export function useGoogleAvailable(): boolean {
  const { config } = useSiteConfig();
  return config?.auth?.google === true;
}

/**
 * "Continue with Google" / "Sign up with Google", drawn to Google's branding
 * rules (white fill, #747775 1 px border, #1F1F1F label, 40 px high, 18 px
 * four-colour G, 12/10/12 px padding, 14/20 medium label). Starts the
 * server-side flow and navigates to Google; renders nothing unless the API
 * has Google configured.
 */
export function GoogleButton({
  intent,
  next,
  beforeStart,
  onError,
  className,
}: {
  intent: Exclude<GoogleIntent, "link">;
  /** Locale-less path to return to afterwards (read when pressed). */
  next?: () => string | undefined;
  /** Return false to stop (e.g. the sign-up terms box is not ticked yet). */
  beforeStart?: () => boolean | Promise<boolean>;
  onError: (error: ApiError) => void;
  className?: string;
}) {
  const { text, locale } = useI18n();
  const available = useGoogleAvailable();
  const [busy, setBusy] = React.useState(false);

  // Coming back with the browser's Back button restores this page from the
  // cache, still "busy": make the button usable again.
  React.useEffect(() => {
    const onShow = (event: PageTransitionEvent) => {
      if (event.persisted) setBusy(false);
    };
    window.addEventListener("pageshow", onShow);
    return () => window.removeEventListener("pageshow", onShow);
  }, []);

  if (!available) return null;

  async function press() {
    if (busy) return;
    if (beforeStart && !(await beforeStart())) return;
    setBusy(true);
    try {
      await startGoogle({ intent, next: next?.(), locale });
      // The page is leaving for Google: stay busy.
    } catch (err) {
      setBusy(false);
      onError(toApiError(err));
    }
  }

  return (
    <button
      type="button"
      onClick={() => void press()}
      disabled={busy}
      aria-busy={busy}
      data-testid="google-button"
      data-intent={intent}
      className={cn(
        "inline-flex h-10 w-full select-none items-center justify-center gap-2.5 rounded-[4px] border border-[#747775] bg-[#FFFFFF] px-3 text-sm font-medium leading-5 text-[#1F1F1F]",
        "transition-colors outline-none hover:bg-[#F1F1F1] focus-visible:ring-3 focus-visible:ring-ring/60 disabled:cursor-wait disabled:opacity-70",
        className,
      )}
    >
      {busy ? <Loader2 aria-hidden className="size-[18px] shrink-0 animate-spin motion-reduce:animate-none" /> : <GoogleG size={18} className="shrink-0" />}
      <span>{busy ? text("Opening Google…") : text(intent === "signup" ? "Sign up with Google" : "Continue with Google")}</span>
    </button>
  );
}
