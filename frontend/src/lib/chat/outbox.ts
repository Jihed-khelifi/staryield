/**
 * Outbox for chat text messages. Every text message goes here first and
 * stays until the server confirms it, so a refresh, a crash or an offline
 * spell loses nothing. Entries persist in sessionStorage (this tab only,
 * cleared with the tab and on sign-out), per user.
 *
 * Delivery is in order per conversation (§22.3): only the oldest unsent
 * entry may be in flight. Messages are routed by the API (into the live chat
 * when there is one, else outside a live chat), so entries are scoped to the
 * conversation; `session_id` is only kept for entries stored by older builds. Sends are idempotent on `client_msg_id`, so resending after an
 * unknown outcome (timeout, lost connection, reload) never duplicates.
 */
import type { ApiError } from "@/lib/api/errors";
import { MESSAGE_MAX_LENGTH } from "@/lib/schemas/message-schema";
import type { UiMessage } from "@/components/chat/chat-model";

export type OutboxState = "queued" | "sending" | "failed";

export type OutboxEntry = {
  client_msg_id: string;
  conversation_id: string;
  /** Legacy: entries stored before §22.3 name their session; ignored for routing. */
  session_id?: string;
  sender_id: string;
  body: string;
  created_at: string;
  /** Delivery attempts so far. */
  attempts: number;
  state: OutboxState;
  /** Epoch ms before which the entry is not retried (backoff). */
  retry_at: number;
  /** Error code of the last failure. */
  error?: string;
};

/** Entries kept at most (oldest dropped first). */
export const OUTBOX_LIMIT = 50;
/** Entries older than this are dropped when the outbox is read. */
export const OUTBOX_MAX_AGE_MS = 24 * 60 * 60 * 1000;
/** Automatic retries before an entry is marked failed (the user can retry). */
export const MAX_AUTO_ATTEMPTS = 5;
const BASE_BACKOFF_MS = 1_000;
const MAX_BACKOFF_MS = 30_000;

const KEY_PREFIX = "staryield.chat.outbox:";
const ID = /^[A-Za-z0-9_\-:.]{6,64}$/;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export const outboxKey = (userId: string) => `${KEY_PREFIX}${userId}`;

function storage(): Storage | null {
  try {
    return typeof sessionStorage === "undefined" ? null : sessionStorage;
  } catch {
    return null; // blocked storage (privacy settings, sandboxed frames)
  }
}

/** Shape-check one stored entry; storage is not trusted input. */
function valid(raw: unknown): raw is OutboxEntry {
  if (!raw || typeof raw !== "object") return false;
  const e = raw as Record<string, unknown>;
  return (
    typeof e.client_msg_id === "string" &&
    ID.test(e.client_msg_id) &&
    typeof e.conversation_id === "string" &&
    UUID.test(e.conversation_id) &&
    (e.session_id === undefined || (typeof e.session_id === "string" && UUID.test(e.session_id))) &&
    typeof e.sender_id === "string" &&
    e.sender_id.length > 0 &&
    e.sender_id.length <= 64 &&
    typeof e.body === "string" &&
    e.body.trim().length > 0 &&
    e.body.length <= MESSAGE_MAX_LENGTH &&
    typeof e.created_at === "string" &&
    !Number.isNaN(Date.parse(e.created_at)) &&
    typeof e.attempts === "number" &&
    Number.isInteger(e.attempts) &&
    e.attempts >= 0 &&
    (e.state === "queued" || e.state === "sending" || e.state === "failed") &&
    typeof e.retry_at === "number" &&
    Number.isFinite(e.retry_at) &&
    (e.error === undefined || (typeof e.error === "string" && e.error.length <= 64))
  );
}

/**
 * The user's outbox. Invalid, foreign and expired entries are dropped; an
 * entry that was in flight when the page went away is queued again (the
 * resend is idempotent).
 */
export function readOutbox(userId: string, now = Date.now(), store: Storage | null = storage()): OutboxEntry[] {
  if (!store || !userId) return [];
  let parsed: unknown;
  try {
    parsed = JSON.parse(store.getItem(outboxKey(userId)) ?? "[]");
  } catch {
    return [];
  }
  if (!Array.isArray(parsed)) return [];
  return parsed
    .filter(valid)
    .filter((e) => e.sender_id === userId && now - Date.parse(e.created_at) < OUTBOX_MAX_AGE_MS)
    .map((e) => (e.state === "sending" ? { ...e, state: "queued" as const, retry_at: 0 } : e))
    .slice(-OUTBOX_LIMIT);
}

/** Persist the outbox (best effort: a full or blocked storage keeps it in memory only). */
export function writeOutbox(userId: string, entries: OutboxEntry[], store: Storage | null = storage()): boolean {
  if (!store || !userId) return false;
  try {
    if (entries.length === 0) store.removeItem(outboxKey(userId));
    else store.setItem(outboxKey(userId), JSON.stringify(entries.slice(-OUTBOX_LIMIT)));
    return true;
  } catch {
    return false;
  }
}

/** Forget the user's outbox (sign-out). */
export function clearOutbox(userId: string, store: Storage | null = storage()) {
  try {
    store?.removeItem(outboxKey(userId));
  } catch {
    // Nothing persisted.
  }
}

export function newEntry(init: Pick<OutboxEntry, "client_msg_id" | "conversation_id" | "sender_id" | "body"> & { session_id?: string }, now = Date.now()): OutboxEntry {
  return { ...init, created_at: new Date(now).toISOString(), attempts: 0, state: "queued", retry_at: 0 };
}

export function enqueue(entries: OutboxEntry[], entry: OutboxEntry): OutboxEntry[] {
  if (entries.some((e) => e.client_msg_id === entry.client_msg_id)) return entries;
  return [...entries, entry].slice(-OUTBOX_LIMIT);
}

export function patchEntry(entries: OutboxEntry[], clientMsgId: string, patch: Partial<OutboxEntry>): OutboxEntry[] {
  return entries.map((e) => (e.client_msg_id === clientMsgId ? { ...e, ...patch } : e));
}

export function removeEntry(entries: OutboxEntry[], clientMsgId: string): OutboxEntry[] {
  return entries.filter((e) => e.client_msg_id !== clientMsgId);
}

/**
 * The entry to send next for a conversation, or null. Order is preserved: when
 * the oldest unsent entry is in flight or waiting for its backoff, nothing
 * newer is sent. Failed entries wait for the user and do not block others.
 */
export function nextToSend(entries: OutboxEntry[], conversationId: string, now = Date.now()): OutboxEntry | null {
  for (const e of entries) {
    if (e.conversation_id !== conversationId || e.state === "failed") continue;
    if (e.state === "sending") return null;
    return e.retry_at <= now ? e : null;
  }
  return null;
}

/** Earliest time a queued entry of the conversation becomes due (for a timer). */
export function nextRetryAt(entries: OutboxEntry[], conversationId: string): number | null {
  const first = entries.find((e) => e.conversation_id === conversationId && e.state !== "failed");
  return first && first.state === "queued" ? first.retry_at : null;
}

/** Exponential backoff with full jitter: up to 1 s, 2 s, 4 s … capped at 30 s. */
export function backoffMs(attempt: number, random: () => number = Math.random): number {
  const ceiling = Math.min(MAX_BACKOFF_MS, BASE_BACKOFF_MS * 2 ** Math.max(0, attempt - 1));
  return Math.max(250, Math.round(random() * ceiling));
}

const TRANSIENT = new Set(["network_error", "timeout", "realtime_unavailable", "service_unavailable", "internal_error", "rate_limited", "csrf_failed", "refresh_conflict"]);
/** The session cannot take messages right now, but may again (paused, starting): old servers only. */
const NOT_YET = new Set(["requested", "accepted", "disconnected"]);
/** Refused by the server's off-session rules: retrying the same message cannot help (§22.3). */
const FINAL = new Set(["off_session_disabled", "off_session_not_eligible", "off_session_limit_reached", "recipient_unavailable"]);

/**
 * What to do after a failed send:
 * - "retry": transient (offline, timeout, server busy) — retry with backoff;
 * - "wait": the chat is paused or starting — keep it queued for the resume;
 * - "fatal": the message cannot be delivered as is (invalid, or refused by the
 *   off-session rules such as the daily cap: the user sees why and may retry).
 */
export function classifyFailure(error: Pick<ApiError, "code" | "status" | "details">): "retry" | "wait" | "fatal" {
  if (FINAL.has(String(error.code))) return "fatal";
  if (error.code === "invalid_state") {
    const status = typeof error.details?.status === "string" ? error.details.status : "";
    return NOT_YET.has(status) ? "wait" : "fatal";
  }
  if (TRANSIENT.has(String(error.code)) || error.status >= 500) return "retry";
  return "fatal";
}

/** The entry after a failed attempt (attempts already counted). */
export function afterFailure(
  entry: OutboxEntry,
  error: Pick<ApiError, "code" | "status" | "details" | "retryAfter">,
  now = Date.now(),
  random: () => number = Math.random,
): OutboxEntry {
  const kind = classifyFailure(error);
  const code = String(error.code).slice(0, 64);
  if (kind === "fatal" || (kind === "retry" && entry.attempts >= MAX_AUTO_ATTEMPTS)) {
    return { ...entry, state: "failed", error: code, retry_at: 0 };
  }
  const delay = error.retryAfter ? error.retryAfter * 1000 : backoffMs(entry.attempts, random);
  return { ...entry, state: "queued", error: code, retry_at: now + delay };
}

/** Queue a failed entry again (user retry). */
export function retryEntry(entries: OutboxEntry[], clientMsgId: string): OutboxEntry[] {
  return patchEntry(entries, clientMsgId, { state: "queued", attempts: 0, retry_at: 0, error: undefined });
}

/** How an outbox entry is shown in the feed. */
export function outboxMessage(entry: OutboxEntry): UiMessage {
  return {
    id: entry.client_msg_id,
    client_msg_id: entry.client_msg_id,
    conversation_id: entry.conversation_id,
    session_id: entry.session_id,
    sender_id: entry.sender_id,
    kind: "text",
    body: entry.body,
    created_at: entry.created_at,
    pending: entry.state,
    failure: entry.error,
  };
}
