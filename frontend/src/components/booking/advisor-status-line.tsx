"use client";

import { useI18n } from "@/i18n/i18n-provider";
import type { PresenceStatus } from "@/lib/api/types";
import { chatLine } from "@/lib/booking/booking-state";
import { useBookingViewer } from "@/lib/booking/booking-viewer";
import { STATUS_NAME } from "@/lib/psychics-live";
import { cn } from "@/lib/utils";

/**
 * "● Online · 30 credits/min" / "○ Offline · 30 credits/min": the status dot
 * (filled when the advisor can be reached, hollow when offline, so it is not
 * told by colour alone), the status, and what a reading costs. The price is
 * left out only where the book button next to it already says it: an online
 * advisor's button line is the rate itself or free mode. When that line is a
 * free-minutes offer, the rate stays here, so the price is always in view at
 * the moment of choice. Shared by the phone booking bar and `AdvisorStrip`.
 */
export function AdvisorStatusLine({ status, ratePerMinute, className }: { status: PresenceStatus; ratePerMinute: number; className?: string }) {
  const { text, locale } = useI18n();
  const viewer = useBookingViewer();
  const rate = text("{value0} credits/min", { value0: ratePerMinute.toLocaleString(locale) });
  let price: string | null;
  if (status !== "online") price = viewer.freeMode ? text("Free for a limited time") : rate;
  else {
    const line = chatLine(viewer);
    price = line === "own_minutes" || line === "signup_offer" ? rate : null;
  }
  return (
    <p className={cn("flex min-w-0 items-center gap-1.5 font-serif", className)} data-testid="advisor-status-line">
      <span className={`status-dot ${status}`} aria-hidden />
      <span className="shrink-0">{text(STATUS_NAME[status])}</span>
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
