/**
 * The outbox delivery pump: holds the user's queued text messages (mirrored
 * to sessionStorage on every change) and sends them in order — one in
 * flight at a time, only while the connection is open and the target
 * session is active. Transient failures retry with exponential backoff, and
 * right away when the connection comes back; a paused session keeps
 * messages queued; anything else marks the message failed for the user.
 *
 * Framework-free so it can be unit tested; React reads it through
 * `useSyncExternalStore` (see use-outbox.ts).
 */
import { toApiError, type ApiError } from "@/lib/api/errors";
import type { ChatMessage } from "@/lib/api/types";
import {
  afterFailure,
  enqueue,
  failClosedSessions,
  newEntry,
  nextRetryAt,
  nextToSend,
  patchEntry,
  readOutbox,
  removeEntry,
  retryEntry,
  writeOutbox,
  type OutboxEntry,
} from "@/lib/chat/outbox";

export type OutboxTransport = (entry: OutboxEntry) => Promise<ChatMessage>;

export type OutboxHooks = {
  /** The server confirmed a message (merge it into the timeline). */
  onDelivered?: (message: ChatMessage, entry: OutboxEntry) => void;
  /** A message stopped retrying automatically. */
  onFailed?: (entry: OutboxEntry, error: ApiError) => void;
};

type Target = { sessionId: string | null; ready: boolean };

export class OutboxController {
  private entries: OutboxEntry[];
  private listeners = new Set<() => void>();
  private inFlight: string | null = null;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private target: Target = { sessionId: null, ready: false };
  private stopped = false;

  private transport: OutboxTransport | null = null;
  private hooks: OutboxHooks = {};

  constructor(
    readonly userId: string,
    private readonly opts: { now?: () => number; random?: () => number; storage?: Storage | null } = {},
  ) {
    this.entries = readOutbox(userId, this.now(), opts.storage);
  }

  /** How to send (WebSocket or REST) and whom to tell; refreshed on every render. */
  configure(transport: OutboxTransport, hooks: OutboxHooks = {}) {
    this.transport = transport;
    this.hooks = hooks;
    this.pump();
  }

  private now() {
    return (this.opts.now ?? Date.now)();
  }

  // --- store ---------------------------------------------------------------------

  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };

  getSnapshot = (): OutboxEntry[] => this.entries;

  private commit(next: OutboxEntry[]) {
    if (next === this.entries) return;
    this.entries = next;
    writeOutbox(this.userId, next, this.opts.storage);
    this.listeners.forEach((l) => l());
  }

  // --- intents ---------------------------------------------------------------------

  /** Where messages go and whether they can go now (connection open, session active). */
  setTarget(sessionId: string | null, ready: boolean) {
    const regained = ready && sessionId !== null && (!this.target.ready || this.target.sessionId !== sessionId);
    this.target = { sessionId, ready };
    if (regained && sessionId) {
      // The connection came back: skip the remaining backoff.
      let changed = false;
      const next = this.entries.map((e) => {
        if (e.session_id !== sessionId || e.state !== "queued" || e.retry_at === 0) return e;
        changed = true;
        return { ...e, retry_at: 0 };
      });
      if (changed) this.commit(next);
    }
    this.pump();
  }

  /** Queue a text message; returns the entry (its client id is the idempotency key). */
  send(init: { client_msg_id: string; conversation_id: string; session_id: string; body: string }): OutboxEntry {
    const entry = newEntry({ ...init, sender_id: this.userId }, this.now());
    this.commit(enqueue(this.entries, entry));
    this.pump();
    return entry;
  }

  /** Queue a failed message again, in `sessionId` (the conversation's open session). */
  retry(clientMsgId: string, sessionId: string) {
    if (!this.entries.some((e) => e.client_msg_id === clientMsgId && e.state === "failed")) return;
    this.commit(retryEntry(this.entries, clientMsgId, sessionId));
    this.pump();
  }

  discard(clientMsgId: string) {
    if (clientMsgId === this.inFlight) return;
    this.commit(removeEntry(this.entries, clientMsgId));
    this.pump();
  }

  /** Drop entries the server already has (seen in history or events). */
  confirm(clientMsgIds: Iterable<string>) {
    const ids = new Set(clientMsgIds);
    ids.delete(this.inFlight ?? "");
    if (!this.entries.some((e) => ids.has(e.client_msg_id))) return;
    this.commit(this.entries.filter((e) => !ids.has(e.client_msg_id)));
    this.pump();
  }

  /** The session ended: what it still holds cannot be delivered into it. */
  sessionClosed(sessionId: string) {
    const inFlight = this.inFlight;
    const rest = this.entries.filter((e) => e.client_msg_id !== inFlight);
    const failed = failClosedSessions(rest, (id) => id === sessionId);
    if (failed !== rest) this.commit(this.entries.map((e) => failed.find((f) => f.client_msg_id === e.client_msg_id) ?? e));
  }

  /** Resume delivery (after `stop`). */
  start() {
    this.stopped = false;
    this.pump();
  }

  /** Pause delivery (unmount). A send in flight still settles its entry. */
  stop() {
    this.stopped = true;
    if (this.timer) clearTimeout(this.timer);
    this.timer = null;
  }

  // --- delivery ----------------------------------------------------------------------

  private pump() {
    if (this.stopped || this.inFlight || !this.transport) return;
    if (this.timer) clearTimeout(this.timer);
    this.timer = null;
    const { sessionId, ready } = this.target;
    if (!ready || !sessionId) return;
    const now = this.now();
    const entry = nextToSend(this.entries, sessionId, now);
    if (!entry) {
      const at = nextRetryAt(this.entries, sessionId);
      if (at !== null && at > now) this.timer = setTimeout(() => this.pump(), at - now);
      return;
    }
    const cmid = entry.client_msg_id;
    this.inFlight = cmid;
    const sending = { ...entry, state: "sending" as const, attempts: entry.attempts + 1 };
    this.commit(patchEntry(this.entries, cmid, { state: "sending", attempts: sending.attempts }));
    this.transport(sending).then(
      (message) => {
        this.inFlight = null;
        this.commit(removeEntry(this.entries, cmid));
        this.hooks.onDelivered?.({ ...message, client_msg_id: message.client_msg_id || cmid }, sending);
        this.pump();
      },
      (err) => {
        this.inFlight = null;
        const error = toApiError(err);
        const current = this.entries.find((e) => e.client_msg_id === cmid);
        if (current) {
          const next = afterFailure(current, error, this.now(), this.opts.random);
          this.commit(this.entries.map((e) => (e.client_msg_id === cmid ? next : e)));
          if (next.state === "failed") this.hooks.onFailed?.(next, error);
        }
        this.pump();
      },
    );
  }
}
