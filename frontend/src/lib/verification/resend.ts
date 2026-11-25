/**
 * Resending the verification link: the cooldown arithmetic and the mapping of
 * API outcomes to what the person sees. Pure, so it is unit tested without a
 * DOM; `useResendVerification` wires it to React.
 */
import { toApiError, type ApiError } from "@/lib/api/errors";
import type { VerificationSent } from "@/lib/api/email-verification";

/** The server's cooldown is 60 s: a longer wait is clock skew or the hourly budget. */
export const COOLDOWN_CAP_SECONDS = 120;

export type ResendState =
  | { phase: "idle" }
  | { phase: "sending" }
  /** A link was sent to `email` (masked); another can be requested at `availableAt` (epoch ms). */
  | { phase: "sent"; email: string; availableAt: number }
  /** Asked again inside the cooldown (another tab, double click). */
  | { phase: "cooldown"; availableAt: number }
  /** The hourly budget is spent. */
  | { phase: "limited"; availableAt: number }
  /** The address turned out to be verified already. */
  | { phase: "verified" }
  | { phase: "error"; error: ApiError };

/** Whole seconds left until `availableAt` (never negative). */
export function secondsUntil(availableAt: number, now: number): number {
  return Math.max(0, Math.ceil((availableAt - now) / 1000));
}

/**
 * When the next link can be requested, from the server's `resend_available_at`.
 * The server's clock may differ from this browser's, so the wait is capped to
 * the cooldown: a wrong clock never locks the button for long (the server
 * still enforces the real cooldown).
 */
export function availableAtFrom(sent: Pick<VerificationSent, "resend_available_at">, now: number): number {
  const at = Date.parse(sent.resend_available_at);
  if (!Number.isFinite(at)) return now + COOLDOWN_CAP_SECONDS * 1000 / 2;
  return Math.min(Math.max(at, now), now + COOLDOWN_CAP_SECONDS * 1000);
}

export function stateFromSent(sent: VerificationSent, now: number): ResendState {
  return { phase: "sent", email: sent.email, availableAt: availableAtFrom(sent, now) };
}

/** Map a failed resend to a state: 409 verified, 429 cooldown or hourly limit, anything else is retryable. */
export function stateFromError(raw: unknown, now: number): ResendState {
  const error = toApiError(raw);
  if (error.code === "already_verified") return { phase: "verified" };
  if (error.code === "rate_limited" || error.status === 429) {
    const seconds = error.retryAfter && error.retryAfter > 0 ? error.retryAfter : COOLDOWN_CAP_SECONDS / 2;
    const availableAt = now + seconds * 1000;
    return seconds > COOLDOWN_CAP_SECONDS ? { phase: "limited", availableAt } : { phase: "cooldown", availableAt };
  }
  return { phase: "error", error };
}

/** The time a state is waiting for, when it is. */
export function waitingUntil(state: ResendState): number | null {
  return state.phase === "sent" || state.phase === "cooldown" || state.phase === "limited" ? state.availableAt : null;
}
