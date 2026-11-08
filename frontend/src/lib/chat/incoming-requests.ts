/**
 * The advisor's set of pending chat requests (docs §21.1). Pure functions over
 * an immutable map keyed by session id; every entry keeps the local receive
 * time so `expires_at` can be corrected with the server clock.
 */
import { secondsUntil, snapshot, type SessionSnapshot } from "@/components/chat/chat-model";
import type { ChatSession } from "@/lib/api/types";

export type PendingRequests = ReadonlyMap<string, SessionSnapshot>;

export const EMPTY_REQUESTS: PendingRequests = new Map();

/** Realtime events that change the pending set. */
export const REQUEST_EVENTS = [
  "session.requested",
  "session.accepted",
  "session.started",
  "session.declined",
  "session.expired",
  "session.cancelled",
  "session.ended",
] as const;

const LEAVING = new Set(["session.accepted", "session.started", "session.declined", "session.expired", "session.cancelled", "session.ended"]);

/** Seconds left before the request expires (0 when it has no expiry or is past it). */
export function requestSecondsLeft(s: SessionSnapshot, now = Date.now()): number {
  return secondsUntil(s.session.expires_at, s, now);
}

/** A request is live while it is `requested` and not past `expires_at`. */
export function isLiveRequest(s: SessionSnapshot, now = Date.now()): boolean {
  if (s.session.status !== "requested") return false;
  return !s.session.expires_at || requestSecondsLeft(s, now) > 0;
}

/** Apply one realtime session event. Returns the same map when nothing changes. */
export function applyRequestEvent(current: PendingRequests, type: string, session: ChatSession | undefined, now = Date.now()): PendingRequests {
  if (!session?.id) return current;
  if (type === "session.requested") {
    if (session.status !== "requested") return current;
    const snap = { session, receivedAt: now } satisfies SessionSnapshot;
    if (!isLiveRequest(snap, now)) return current;
    const next = new Map(current);
    next.set(session.id, snap);
    return next;
  }
  if (LEAVING.has(type) && current.has(session.id)) {
    const next = new Map(current);
    next.delete(session.id);
    return next;
  }
  return current;
}

/** Replace the set from a REST snapshot of `status=requested` sessions. */
export function replaceRequests(sessions: readonly ChatSession[], now = Date.now()): PendingRequests {
  const next = new Map<string, SessionSnapshot>();
  for (const s of sessions) {
    const snap = { ...snapshot(s), receivedAt: now };
    if (isLiveRequest(snap, now)) next.set(s.id, snap);
  }
  return next;
}

/** Drop entries past their expiry. Returns the same map when none expired. */
export function pruneExpired(current: PendingRequests, now = Date.now()): PendingRequests {
  let next: Map<string, SessionSnapshot> | null = null;
  for (const [id, s] of current) {
    if (!isLiveRequest(s, now)) {
      next ??= new Map(current);
      next.delete(id);
    }
  }
  return next ?? current;
}

/** The most recently requested entry (shown in the banner). */
export function newestRequest(current: PendingRequests): SessionSnapshot | null {
  let best: SessionSnapshot | null = null;
  for (const s of current.values()) {
    if (!best || new Date(s.session.requested_at).getTime() >= new Date(best.session.requested_at).getTime()) best = s;
  }
  return best;
}

/** Milliseconds until the earliest expiry (null when nothing expires). */
export function nextExpiryMs(current: PendingRequests, now = Date.now()): number | null {
  let min: number | null = null;
  for (const s of current.values()) {
    if (!s.session.expires_at) continue;
    const ms = requestSecondsLeft(s, now) * 1000;
    if (min === null || ms < min) min = ms;
  }
  return min;
}
