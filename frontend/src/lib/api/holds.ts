/**
 * Free-trial hold (revisions 6.0, docs/platform-extensions.md §26.1): the
 * reading is held for `hold_seconds` when its next minute cannot be paid. The
 * advisor keeps writing, the client sees those replies locked until a top-up
 * continues the reading (`converted`) or, after the hold expired, until one
 * minute is paid to reveal them (`revealed`). The server owns every outcome;
 * these are the wire types and the three endpoints the client uses.
 */
import { api, newId } from "@/lib/api/client";
import type { ChatMessage, ChatSession, Wallet } from "@/lib/api/types";

/** Why the reading was held: this reading only used free minutes, or the credits ran out. */
export type HoldReason = "trial_ended" | "credits_exhausted";
export type HoldStatus = "open" | "converted" | "expired" | "revealed";

export type ChatHold = {
  id: string;
  session_id: string;
  conversation_id: string;
  reason: HoldReason;
  status: HoldStatus;
  started_at: string;
  /** The server's deadline: the countdown always runs to this, never to a local "60". */
  expires_at: string;
  resolved_at?: string;
  revealed_at?: string;
  /** What continuing or revealing costs (the reading's per-minute rate). */
  price_credits: number;
  held_messages: number;
};

/** Realtime `session.held`: the session already carries the hold. */
export type SessionHeldEvent = { session: ChatSession; hold: ChatHold };
/** Realtime `session.hold_extended`: a credit purchase is pending, the deadline moved. */
export type HoldExtendedEvent = { session_id: string; hold_id: string; expires_at: string };
/** Realtime `session.hold_ended`: the hold is `converted` or `expired`. */
export type HoldEndedEvent = { session_id: string; hold: ChatHold };
/** Realtime `messages.revealed`: the client gets the full messages, the advisor only their ids. */
export type MessagesRevealedEvent = {
  conversation_id: string;
  session_id: string;
  hold_id: string;
  messages?: ChatMessage[];
  message_ids?: string[];
};

export type HoldContinueResult = { session: ChatSession; hold: ChatHold; wallet: Wallet };
export type HoldRevealResult = { hold: ChatHold; messages: ChatMessage[]; wallet: Wallet };

export const holdsApi = {
  /** A participant's view of one hold (the client's reveal UI after a reload). */
  get: (holdId: string, signal?: AbortSignal) => api<ChatHold>(`/api/v1/chat/holds/${encodeURIComponent(holdId)}`, { signal }),
  /** Pay one minute and continue the held reading. 402 `insufficient_credits`, 409 `invalid_state` unless held. */
  continueReading: (sessionId: string, idempotencyKey: string = newId()) =>
    api<HoldContinueResult>(`/api/v1/chat/sessions/${encodeURIComponent(sessionId)}/hold/continue`, {
      method: "POST",
      body: { idempotency_key: idempotencyKey },
      idempotent: true,
    }),
  /** Pay `price_credits` once to read the messages of an expired hold. Replays with the same key are safe. */
  reveal: (holdId: string, idempotencyKey: string = newId()) =>
    api<HoldRevealResult>(`/api/v1/chat/holds/${encodeURIComponent(holdId)}/reveal`, {
      method: "POST",
      body: { idempotency_key: idempotencyKey },
      idempotent: true,
    }),
};
