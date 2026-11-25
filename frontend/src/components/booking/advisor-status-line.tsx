"use client";

import { useI18n } from "@/i18n/i18n-provider";
import type { PresenceStatus } from "@/lib/api/types";
import { useFreeMode } from "@/lib/site-config/site-config-provider";
import { cn } from "@/lib/utils";

const statusName: Record<PresenceStatus, string> = { online: "Online", busy: "Busy", away: "Away", offline: "Offline" };

/**
 * "● Online" / "○ Offline · 30 credits/min": the status dot (filled when the
 * advisor can be reached, hollow when offline, so it is not told by colour
 * alone) and the status, plus the price when the book button next to it does
 * not already say it (its line under "Chat now" carries the rate or the free
 * minutes of an online advisor). Shared by the phone booking bar and
 * `AdvisorStrip`.
 */
export function AdvisorStatusLine({ status, ratePerMinute, className }: { status: PresenceStatus; ratePerMinute: number; className?: string }) {
  const { text, locale } = useI18n();
  const freeMode = useFreeMode();
  const price =
    status === "online"
      ? null
      : freeMode
        ? text("Free for a limited time")
        : text("{value0} credits/min", { value0: ratePerMinute.toLocaleString(locale) });
  return (
    <p className={cn("flex min-w-0 items-center gap-1.5 font-serif", className)} data-testid="advisor-status-line">
      <span className={`booking-dot ${status}`} aria-hidden />
      <span className="shrink-0">{text(statusName[status])}</span>
      {price ? (
        <>
          <span aria-hidden>·</span>
          <span className="truncate" data-testid="advisor-price">
            {price}
          </span>
        </>
      ) : null}
    </p>
  );
}
