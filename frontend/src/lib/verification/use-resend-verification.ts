"use client";

import * as React from "react";

import { emailVerificationApi } from "@/lib/api/email-verification";
import { useSession } from "@/lib/auth/session-provider";
import { logger } from "@/lib/logger";
import { secondsUntil, stateFromError, stateFromSent, waitingUntil, type ResendState } from "@/lib/verification/resend";

export type ResendVerification = {
  state: ResendState;
  /** Seconds until another link can be requested (0 when it can be now). */
  secondsLeft: number;
  busy: boolean;
  canResend: boolean;
  resend: () => Promise<void>;
};

/**
 * Resend the verification link with its cooldown, hourly limit and error
 * states. Shared by the account notice, the settings row and the verify page.
 * A 409 (the address is verified) refreshes the session so every surface
 * drops its "not verified" state.
 */
export function useResendVerification(): ResendVerification {
  const { reload } = useSession();
  const [state, setState] = React.useState<ResendState>({ phase: "idle" });
  const inFlight = React.useRef(false);
  const until = waitingUntil(state);
  const [now, setNow] = React.useState(() => Date.now());

  // Tick once a second while a wait is running.
  React.useEffect(() => {
    if (until === null) return;
    const id = window.setInterval(() => {
      const t = Date.now();
      setNow(t);
      if (t >= until) window.clearInterval(id);
    }, 1000);
    return () => window.clearInterval(id);
  }, [until]);

  const resend = React.useCallback(async () => {
    if (inFlight.current) return;
    inFlight.current = true;
    setState({ phase: "sending" });
    try {
      const sent = await emailVerificationApi.resend();
      const t = Date.now();
      setNow(t);
      setState(stateFromSent(sent, t));
    } catch (err) {
      const t = Date.now();
      const next = stateFromError(err, t);
      setNow(t);
      if (next.phase === "verified") void reload();
      else if (next.phase === "error") logger.debug("verification: resend failed", { code: String(next.error.code), requestId: next.error.requestId });
      setState(next);
    } finally {
      inFlight.current = false;
    }
  }, [reload]);

  const secondsLeft = until === null ? 0 : secondsUntil(until, now);
  const busy = state.phase === "sending";
  return { state, secondsLeft, busy, canResend: !busy && secondsLeft === 0 && state.phase !== "verified", resend };
}
