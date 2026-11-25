"use client";

import * as React from "react";

import { emailVerificationApi } from "@/lib/api/email-verification";
import { withVerificationSent } from "@/lib/auth/email-verified";
import { useSession } from "@/lib/auth/session-provider";
import { logger } from "@/lib/logger";
import { availableAtFrom, secondsUntil, stateFromError, stateFromSent, waitingUntil, type ResendState } from "@/lib/verification/resend";

export type ResendVerification = {
  state: ResendState;
  /** Seconds until another link can be requested (0 when it can be now). */
  secondsLeft: number;
  busy: boolean;
  canResend: boolean;
  /** A link is waiting in the inbox (so the action is "resend", not "send"). */
  pending: boolean;
  resend: () => Promise<void>;
};

/**
 * Resend the verification link with its cooldown, limits and error states.
 * Shared by the account notice, the settings row and the verify page. Mount
 * it once the session user is known: the link the account already has (the
 * welcome e-mail, another tab, a reminder) starts the countdown at once, so
 * nobody is invited to click into the server's cooldown. A sent link is
 * written into the session (every surface of the tab then says a link is
 * waiting); a 409 (the address is verified) refreshes the session so every
 * surface drops its "not verified" state.
 */
export function useResendVerification(): ResendVerification {
  const { user, reload, updateUser } = useSession();
  const [state, setState] = React.useState<ResendState>({ phase: "idle" });
  const inFlight = React.useRef(false);
  const [now, setNow] = React.useState(() => Date.now());
  // The cooldown of the link the account had when this control appeared (capped against a wrong clock, like a sent one).
  const [initialWait] = React.useState(() => {
    const pending = user?.email_verification;
    return pending ? availableAtFrom(pending, Date.now()) : null;
  });
  const until = waitingUntil(state) ?? (state.phase === "idle" ? initialWait : null);

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
      updateUser((u) => withVerificationSent(u, new Date(t).toISOString(), sent.resend_available_at));
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
  }, [reload, updateUser]);

  const secondsLeft = until === null ? 0 : secondsUntil(until, now);
  const busy = state.phase === "sending";
  const done = state.phase === "verified" || state.phase === "undeliverable";
  const pending = Boolean(user?.email_verification) || state.phase === "sent";
  return { state, secondsLeft, busy, canResend: !busy && secondsLeft === 0 && !done, pending, resend };
}
