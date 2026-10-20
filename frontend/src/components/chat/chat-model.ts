/**
 * Chat view-model helpers. The server owns session state, billing and
 * message identity; these helpers only merge what the server sends and derive
 * display values (timers are computed from server timestamps, never from a
 * client-side counter).
 */
import type { ChatMessage, ChatSession } from "@/lib/api/types";

export type UiMessage = ChatMessage & {
  /** Local delivery state for messages typed in this tab. */
  pending?: "sending" | "failed";
};

/** Insert or replace messages (by id, or by client id for optimistic ones), keeping chronological order. */
export function mergeMessages(current: UiMessage[], incoming: UiMessage[]): UiMessage[] {
  const out = [...current];
  for (const m of incoming) {
    const byId = out.findIndex((x) => x.id === m.id);
    const byClient = m.client_msg_id ? out.findIndex((x) => x.client_msg_id === m.client_msg_id && x.sender_id === m.sender_id) : -1;
    const idx = byId >= 0 ? byId : byClient;
    if (idx >= 0) out[idx] = { ...out[idx], ...m, pending: m.pending };
    else out.push(m);
  }
  return out.sort((a, b) => (a.created_at === b.created_at ? a.id.localeCompare(b.id) : a.created_at.localeCompare(b.created_at)));
}

/** A session snapshot with the local time it was received (clock sync). */
export type SessionSnapshot = { session: ChatSession; receivedAt: number };

export function snapshot(session: ChatSession): SessionSnapshot {
  return { session, receivedAt: Date.now() };
}

/** Server clock offset in ms (server − local) at the time of the snapshot. */
export function clockOffset(s: SessionSnapshot): number {
  return new Date(s.session.server_time).getTime() - s.receivedAt;
}

/** Elapsed active seconds, extrapolated from the server value while active. */
export function elapsedSeconds(s: SessionSnapshot | null, now = Date.now()): number {
  if (!s) return 0;
  const base = s.session.active_seconds;
  if (s.session.status !== "active") return base;
  return base + Math.max(0, Math.floor((now - s.receivedAt) / 1000));
}

/** Seconds until an ISO deadline, using the server clock. */
export function secondsUntil(iso: string | undefined, s: SessionSnapshot | null, now = Date.now()): number {
  if (!iso || !s) return 0;
  return Math.max(0, Math.ceil((new Date(iso).getTime() - (now + clockOffset(s))) / 1000));
}

export const OPEN_STATUSES = new Set(["requested", "accepted", "active", "disconnected"]);

export function isOpen(session: ChatSession | null | undefined): boolean {
  return Boolean(session && OPEN_STATUSES.has(session.status));
}

/** English copy for end reasons (translated with text()). */
export const endReasonCopy: Record<string, string> = {
  client_ended: "The client ended the chat.",
  psychic_ended: "The advisor ended the chat.",
  credits_exhausted: "The chat ended because the credit balance ran out.",
  grace_expired: "The chat ended because the connection could not be restored.",
  admin_terminated: "The chat was ended by Staryield support.",
  account_disabled: "The chat ended because an account was deactivated.",
  declined: "The advisor declined this request.",
  request_timeout: "The advisor did not answer in time. No credits were used.",
  not_started: "The chat could not start in time. No credits were used.",
  cancelled: "The request was cancelled.",
};

/** English copy for a system message body ("session.started", "session.ended:<reason>"). */
export function systemCopy(body: string): string {
  if (body === "session.started") return "Live chat started";
  if (body.startsWith("session.ended")) return "Live chat ended";
  return body;
}

export function newClientMsgId(): string {
  const rand = typeof crypto !== "undefined" && "randomUUID" in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random()}`;
  return `m-${rand}`.replace(/[^A-Za-z0-9_\-:.]/g, "").slice(0, 64);
}
