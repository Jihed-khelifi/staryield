/**
 * The 18+ gate answers client actions with 403 `birth_date_required` or
 * `age_restricted` (revisions 5.0, §24.6). Any API call or realtime frame
 * that gets one reports it here, and the session provider's age effects turn
 * it into the redirect to the birth step or the "account restricted" screen.
 * A separate module keeps the HTTP client and the realtime client free of
 * React and of each other.
 */
import type { ApiError } from "@/lib/api/errors";

export type AgeBlock = "birth_date_required" | "restricted";

type Listener = (block: AgeBlock, error?: ApiError) => void;

const listeners = new Set<Listener>();

/** The age block an API error stands for (null for every other error). */
export function ageBlockOf(error: { code: string | number }): AgeBlock | null {
  if (error.code === "birth_date_required") return "birth_date_required";
  if (error.code === "age_restricted") return "restricted";
  return null;
}

/** Report an API error; only age-gate errors reach the listeners. */
export function notifyAgeBlocked(error: ApiError): void {
  const block = ageBlockOf(error);
  if (block) listeners.forEach((fn) => fn(block, error));
}

/**
 * A guard that stopped an action before any request (the session already says
 * the account is blocked): same handling as the API's answer.
 */
export function reportAgeBlock(block: AgeBlock): void {
  listeners.forEach((fn) => fn(block));
}

/** Subscribe to age-gate answers from any API call or realtime frame. */
export function onAgeBlocked(fn: Listener): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}
