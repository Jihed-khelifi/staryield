"use client";

import * as React from "react";
import { Check, Copy, Share2 } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { useReferrals } from "@/lib/referrals/referral-provider";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/**
 * The invite link with "Copy link" and, where the browser has a share sheet,
 * "Share". When copying is refused the link is selected so it can be copied
 * by hand. Reads the nearest ReferralProvider.
 */
export function ReferralLinkField({ className, tone = "light", labelVisible = false }: { className?: string; tone?: "light" | "dark"; labelVisible?: boolean }) {
  const { text } = useI18n();
  const { data, copied, canShare, copyLink, shareLink } = useReferrals();
  const input = React.useRef<HTMLInputElement>(null);
  const [sharing, setSharing] = React.useState(false);
  const id = React.useId();
  if (!data) return null;

  const copy = async () => {
    if (!(await copyLink())) {
      input.current?.focus();
      input.current?.select();
    }
  };
  const share = async () => {
    setSharing(true);
    try {
      await shareLink();
    } finally {
      setSharing(false);
    }
  };

  return (
    <div className={cn("flex flex-col gap-1.5", className)} data-testid="referral-link-field">
      <label htmlFor={id} className={labelVisible ? "text-xs font-medium tracking-wide uppercase opacity-80" : "sr-only"}>
        {text("Your invite link")}
      </label>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-stretch">
        <input
          ref={input}
          id={id}
          readOnly
          value={data.link}
          onFocus={(e) => e.currentTarget.select()}
          data-testid="referral-link"
          className={cn(
            "h-10 w-full min-w-0 rounded-[6px] border px-3 font-mono sm:flex-1 text-[13px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
            tone === "dark" ? "border-white/40 bg-white/10 text-white" : "border-[rgb(170_142_98/0.6)] bg-[var(--cream-light)] text-[#17150f]",
          )}
        />
        <div className="flex gap-2">
          <Button type="button" className="gold-button h-10 flex-1 px-4 sm:flex-none" onClick={() => void copy()} data-testid="referral-copy" aria-live="polite">
            {copied ? <Check aria-hidden strokeWidth={1.8} /> : <Copy aria-hidden strokeWidth={1.6} />}
            {copied ? text("Copied") : text("Copy link")}
          </Button>
          {canShare ? (
            <Button
              type="button"
              variant="outline"
              className="h-10 flex-1 px-4 sm:flex-none"
              onClick={() => void share()}
              disabled={sharing}
              aria-busy={sharing || undefined}
              data-testid="referral-share"
            >
              <Share2 aria-hidden strokeWidth={1.6} />
              {text("Share")}
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  );
}
