"use client";

import * as React from "react";
import Link from "next/link";
import { Gift, X } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { REFERRAL_KEY, REFERRAL_STORED_EVENT, hasStoredReferral, referralProgramOf } from "@/lib/referral";
import { useSession } from "@/lib/auth/session-provider";
import { useSiteConfig } from "@/lib/site-config/site-config-provider";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const DISMISSED_KEY = "staryield.referral.welcome-dismissed";

function subscribe(onChange: () => void) {
  const onStorage = (e: StorageEvent) => {
    if (e.key === null || e.key === REFERRAL_KEY || e.key === DISMISSED_KEY) onChange();
  };
  window.addEventListener(REFERRAL_STORED_EVENT, onChange);
  window.addEventListener("storage", onStorage);
  return () => {
    window.removeEventListener(REFERRAL_STORED_EVENT, onChange);
    window.removeEventListener("storage", onStorage);
  };
}

function dismissed(): boolean {
  try {
    return sessionStorage.getItem(DISMISSED_KEY) === "1";
  } catch {
    return false;
  }
}

/**
 * Banner for a visitor who arrived with a friend's invite link (a code is
 * kept in this browser) while the program is on. Honest about the offer: the
 * friend gets the standard welcome offer, nothing more, and the inviter's
 * reward is disclosed. On the home page it links to sign-up and can be
 * dismissed for the session; on the sign-up steps it only informs.
 */
export function ReferralWelcome({ placement, className }: { placement: "home" | "signup"; className?: string }) {
  const { text, locale } = useI18n();
  const { status } = useSession();
  const { config } = useSiteConfig();
  // The standard sign-up offer (the same for invited and other visitors).
  const minutes = config?.signup_offer?.enabled ? config.signup_offer.free_minutes : 0;
  const [hidden, setHidden] = React.useState(false);
  // Client-only reads (localStorage / sessionStorage); the server renders nothing.
  const invited = React.useSyncExternalStore(subscribe, () => hasStoredReferral(), () => false);
  const wasDismissed = React.useSyncExternalStore(subscribe, dismissed, () => true);

  if (!invited || status !== "anonymous" || !referralProgramOf(config).enabled) return null;
  if (placement === "home" && (hidden || wasDismissed)) return null;

  const dismiss = () => {
    setHidden(true);
    try {
      sessionStorage.setItem(DISMISSED_KEY, "1");
    } catch {
      // Hidden for this page view only.
    }
  };

  return (
    <aside
      className={cn(
        "relative flex items-start gap-3 border border-[rgb(170_142_98/0.6)] bg-[var(--cream-light)] p-4 text-left text-[#17150f]",
        placement === "home" ? "mx-4 my-4 rounded-[12px] md:mx-auto md:max-w-[960px] md:rounded-[4px]" : "mb-4 w-[544px] max-w-full rounded-[8px]",
        className,
      )}
      aria-labelledby="referral-welcome-title"
      data-testid="referral-welcome"
    >
      <Gift aria-hidden className="mt-0.5 size-5 shrink-0 text-[#8a6a1f]" strokeWidth={1.6} />
      <div className={cn("flex min-w-0 flex-1 flex-col gap-1", placement === "home" && "pr-6")}>
        <h2 id="referral-welcome-title" className="font-display text-base">
          {text("A friend invited you to Staryield")}
        </h2>
        <p className="text-[13px] leading-[1.5]">
          {minutes > 0
            ? minutes === 1
              ? text("Create your free account: like every new client, your first minute of live chat is free.")
              : text("Create your free account: like every new client, your first {value0} minutes of live chat are free.", {
                  value0: new Intl.NumberFormat(locale).format(minutes),
                })
            : text("Create your free account to chat live with our psychic advisors.")}
        </p>
        <p className="text-xs text-[var(--muted-ink)]">{text("Your friend earns free minutes when you complete your first reading.")}</p>
        {placement === "home" ? (
          <Button asChild variant="cta" size="lg" className="mt-2 self-start px-4">
            <Link href={`/${locale}/signup`}>{text("Create my account")}</Link>
          </Button>
        ) : null}
      </div>
      {placement === "home" ? (
        <button
          type="button"
          onClick={dismiss}
          aria-label={text("Dismiss")}
          className="absolute top-2 right-2 rounded p-1 text-[var(--muted-ink)] hover:text-[#17150f] focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <X aria-hidden className="size-4" strokeWidth={1.6} />
        </button>
      ) : null}
    </aside>
  );
}
