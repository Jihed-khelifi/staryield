"use client";

import { Hourglass } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import type { TopUpHold } from "@/lib/billing/top-up";
import { formatCountdown, holdReasonCopy } from "@/lib/chat/hold";
import { useHoldSecondsLeft } from "@/lib/chat/use-hold-countdown";

/**
 * revisions 6.0 (§26.1): shown in the top-up dialog while the reading is held.
 * Why it is held, what continuing costs, and the time left (the server's
 * deadline). The countdown is not a live region (it would be read every
 * second); the chatroom announces the hold itself.
 */
export function HoldNotice({ hold, advisorName }: { hold: TopUpHold; advisorName?: string }) {
  const { text } = useI18n();
  const seconds = useHoldSecondsLeft(hold.expiresAt, hold.clockOffsetMs);
  const name = advisorName?.trim() || text("your advisor");
  return (
    <section className="top-up-hold" aria-label={text("Reading on hold")} data-testid="top-up-hold">
      <Hourglass aria-hidden size={20} className="top-up-hold-icon" />
      <div className="top-up-hold-copy">
        <p className="top-up-hold-reason">
          {text(holdReasonCopy(hold.reason))}{" "}
          <span className="top-up-hold-timer" role="timer" aria-live="off" aria-label={text("Time left")} data-testid="top-up-hold-timer">
            {formatCountdown(seconds)}
          </span>
        </p>
        <p data-testid="top-up-hold-price">
          {text("Top up now to keep chatting with {value0} and read their replies. Continuing costs {value1} credits per minute.", {
            value0: name,
            value1: hold.priceCredits,
          })}
        </p>
      </div>
    </section>
  );
}
