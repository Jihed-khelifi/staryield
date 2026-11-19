"use client";

import { FiClock } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import { formatCountdown, holdReasonCopy, isHoldOpen } from "@/lib/chat/hold";
import { useHoldSecondsLeft } from "@/lib/chat/use-hold-countdown";
import { useChatHold } from "@/components/chat/chat-hold-context";
import { Button } from "@/components/ui/button";

/**
 * revisions 6.0 (§26.1): the banner above the composer while the reading is
 * held. Clients get the reason, what continuing costs, the countdown to the
 * server's deadline and "Top up" (the dialog opened by itself once; this is the
 * way back to it) and, when a top-up did not convert the hold, "Continue
 * reading". Advisors get the same countdown and are told their replies are
 * hidden. The countdown is not a live region: the chatroom announces the hold
 * and its end once, through the feed's log.
 */
export function HoldBanner() {
  const { role, hold, counterpartName, clockOffsetMs, openTopUp, continueOffered, continuing, continueReading } = useChatHold();
  const open = isHoldOpen(hold) ? hold : null;
  const seconds = useHoldSecondsLeft(open?.expires_at, clockOffsetMs);
  const { text } = useI18n();
  if (!open) return null;

  const timer = (
    <span className="hold-banner-timer" role="timer" aria-live="off" aria-label={text("Time left")} data-testid="hold-banner-timer">
      {formatCountdown(seconds)}
    </span>
  );

  if (role === "psychic") {
    const sentence =
      open.reason === "credits_exhausted"
        ? text("{value0}'s credits ran out. Your replies are hidden from them until they top up.", { value0: counterpartName })
        : text("{value0}'s free minutes are over. Your replies are hidden from them until they top up.", { value0: counterpartName });
    return (
      <div className="hold-banner" data-role="psychic" data-reason={open.reason} data-testid="hold-banner">
        <FiClock aria-hidden className="hold-banner-icon" />
        <p className="hold-banner-copy">
          {sentence} {"—"} {timer}
        </p>
      </div>
    );
  }

  return (
    <div className="hold-banner" data-role="client" data-reason={open.reason} data-testid="hold-banner">
      <FiClock aria-hidden className="hold-banner-icon" />
      <div className="hold-banner-copy">
        <p className="hold-banner-title">
          {text(holdReasonCopy(open.reason))} {timer}
        </p>
        <p className="hold-banner-note" data-testid="hold-banner-note">
          {text("Top up now to keep chatting with {value0} and read their replies. Continuing costs {value1} credits per minute.", {
            value0: counterpartName,
            value1: open.price_credits,
          })}
        </p>
      </div>
      <div className="hold-banner-actions">
        {continueOffered ? (
          <Button type="button" variant="cta" size="sm" onClick={continueReading} disabled={continuing} aria-busy={continuing || undefined} data-testid="hold-banner-continue">
            {text("Continue reading ({value0} credits)", { value0: open.price_credits })}
          </Button>
        ) : null}
        <Button type="button" variant={continueOffered ? "outline" : "cta"} size="sm" onClick={openTopUp} data-testid="hold-banner-topup">
          {text("Top up")}
        </Button>
      </div>
    </div>
  );
}
