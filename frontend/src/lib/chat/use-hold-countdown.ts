"use client";

import * as React from "react";

import { holdSecondsLeft } from "@/lib/chat/hold";

/**
 * Seconds left until the server's deadline, redrawn every second. The deadline
 * (`expires_at`) comes from the server and moves with `session.hold_extended`;
 * the offset is the server's clock minus this device's, so a wrong device clock
 * cannot shorten or lengthen the hold. Without a deadline it is 0 and idle.
 */
export function useHoldSecondsLeft(expiresAt: string | undefined, clockOffsetMs = 0): number {
  const [now, setNow] = React.useState(() => Date.now());
  // The deadline moving (an extension) does not restart the ticker: the next tick redraws against the new deadline.
  const counting = Boolean(expiresAt);
  React.useEffect(() => {
    if (!counting) return;
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [counting]);
  return holdSecondsLeft(expiresAt, now, clockOffsetMs);
}
