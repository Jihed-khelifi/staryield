"use client";
import { useId } from "react";

import { useI18n } from "@/i18n/i18n-provider";
import type { Psychic } from "@/lib/api/types";
import { useSession } from "@/lib/auth/session-provider";
import { AdvisorStatusLine } from "@/components/booking/advisor-status-line";
import { BookButton } from "@/components/booking/book-button";

/** Advisor and admin accounts cannot book, so their profile view has no bar (nothing to act on). */
export function useBookingBarVisible(): boolean {
  const { status, user } = useSession();
  return status !== "authenticated" || user?.role === "client";
}

/**
 * The sticky booking bar of an advisor's profile on phones: who, whether they
 * are available, what it costs (the bar's status line, or the button's own
 * line when the advisor is online), and the standard book button, always
 * within thumb reach. It floats above the mobile tab bar (`.booking-bar` in
 * globals.css keeps the safe-area insets) and is hidden from the tablet width
 * up, where the hero button is always near. The page adds `has-booking-bar`
 * to its main element so the end of the page is never covered.
 */
export function BookingBar({ advisor, onAvailable }: { advisor: Pick<Psychic, "slug" | "name" | "status" | "rate_per_minute">; onAvailable?: () => void }) {
  const { text } = useI18n();
  const nameId = useId();
  const visible = useBookingBarVisible();
  if (!visible) return null;
  return (
    <aside className="booking-bar" aria-label={text("Book a reading")} data-testid="booking-bar">
      <div className="min-w-0 flex-1">
        <p id={nameId} className="truncate font-display text-[15px] leading-tight text-ink">
          {text(advisor.name)}
        </p>
        <AdvisorStatusLine status={advisor.status} ratePerMinute={advisor.rate_per_minute} className="mt-0.5 text-[12px] text-[var(--muted-ink)]" />
      </div>
      <BookButton advisor={advisor} placement="profile_bar" describedBy={nameId} className="max-w-[62%] shrink-0" onAvailable={onAvailable} />
    </aside>
  );
}
