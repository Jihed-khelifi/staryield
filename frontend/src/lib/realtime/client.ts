/**
 * Realtime client for the Staryield API.
 *
 * Transport: WebSocket (protocol "staryield.v1") authenticated by a
 * single-use ticket. When WebSockets keep failing (corporate proxies, broken
 * networks) the client falls back to Server-Sent Events for delivery and
 * REST for intents, transparently for callers.
 *
 * Reliability:
 * - Every durable server event carries a stream ID. The client remembers the
 *   last one and resumes from it after any reconnect, so no event is lost;
 *   if the server cannot replay ("resync_required"), consumers refetch state.
 * - Reconnects use exponential backoff with full jitter, and react to the
 *   browser's online/offline and visibility changes.
 * - Application-level heartbeats detect dead connections that TCP misses.
 * - request() returns a promise settled by the server's ack/error frame; frames
 *   issued while reconnecting are queued and sent once connected.
 *
 * The server is authoritative: the client only sends intents and renders the
 * state the server publishes.
 */
import { API_URL, WS_URL, currentLocale } from "@/lib/api/config";
import { refreshSession } from "@/lib/api/client";
import { chatApi, realtimeApi } from "@/lib/api/endpoints";
import { ApiError, toApiError } from "@/lib/api/errors";
import { logger } from "@/lib/logger";

export type ConnectionState = "idle" | "connecting" | "open" | "reconnecting" | "offline" | "closed";
export type TransportKind = "ws" | "sse";

export type RealtimeEvent<T = unknown> = {
  id?: string;
  type: string;
  ts: string;
  ref?: string;
  data?: T;
  error?: { code: string; message: string; details?: Record<string, unknown>; retry_after?: number };
};

type Handler = (event: RealtimeEvent) => void;

type Pending = {
  resolve: (value: unknown) => void;
  reject: (error: ApiError) => void;
  timer: ReturnType<typeof setTimeout>;
  frame: string;
};

const PROTOCOL = "staryield.v1";
const HEARTBEAT_MS = 20_000;
const DEAD_AFTER_MS = 50_000;
const MAX_BACKOFF_MS = 30_000;
const WS_FAILURES_BEFORE_SSE = 3;
const SSE_RETRY_WS_AFTER_MS = 5 * 60_000;

export class RealtimeClient {
  private ws: WebSocket | null = null;
  private es: EventSource | null = null;
  private transport: TransportKind = "ws";
  private state: ConnectionState = "idle";
  private connectionId: string | null = null;
  private lastEventId: string | null = null;
  private handlers = new Map<string, Set<Handler>>();
  private stateListeners = new Set<(state: ConnectionState, transport: TransportKind) => void>();
  private pending = new Map<string, Pending>();
  private queue: { ref: string; frame: string }[] = [];
  private joined = new Set<string>();
  private attempts = 0;
  private wsFailures = 0;
  private openedOnce = false;
  private refN = 0;
  private stopped = false;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private sseSince = 0;
  private lastMessageAt = 0;
  private readonly storageKey: string;

  constructor(private readonly userId: string) {
    this.storageKey = `staryield.rt.last:${userId}`;
    try {
      this.lastEventId = sessionStorage.getItem(this.storageKey);
    } catch {
      this.lastEventId = null;
    }
  }

  /** Begin connecting and watching network/visibility changes. */
  start() {
    this.stopped = false;
    window.addEventListener("online", this.onOnline);
    window.addEventListener("offline", this.onOffline);
    document.addEventListener("visibilitychange", this.onVisible);
    void this.connect();
  }

  /** Close for good (sign-out, revoked session). */
  stop() {
    this.stopped = true;
    window.removeEventListener("online", this.onOnline);
    window.removeEventListener("offline", this.onOffline);
    document.removeEventListener("visibilitychange", this.onVisible);
    this.clearTimers();
    this.ws?.close(1000, "client closed");
    this.es?.close();
    this.ws = null;
    this.es = null;
    this.failPending("realtime_unavailable");
    this.setState("closed");
  }

  get connection(): { state: ConnectionState; transport: TransportKind } {
    return { state: this.state, transport: this.transport };
  }

  /** Subscribe to an event type ("*" for all). Returns an unsubscribe. */
  on(type: string, handler: Handler): () => void {
    let set = this.handlers.get(type);
    if (!set) {
      set = new Set();
      this.handlers.set(type, set);
    }
    set.add(handler);
    return () => set!.delete(handler);
  }

  onState(listener: (state: ConnectionState, transport: TransportKind) => void): () => void {
    this.stateListeners.add(listener);
    return () => this.stateListeners.delete(listener);
  }

  /** Mark a chat session as joined (re-joined automatically after reconnects). */
  async joinSession<T>(sessionId: string): Promise<T> {
    this.joined.add(sessionId);
    return this.request<T>("session.join", { session_id: sessionId });
  }

  async leaveSession(sessionId: string): Promise<void> {
    this.joined.delete(sessionId);
    if (this.state !== "open") return;
    await this.request("session.leave", { session_id: sessionId }).catch((error) =>
      logger.debug("leave failed", { code: toApiError(error).code }),
    );
  }

  /** Send an intent; resolves with the ack payload or rejects with ApiError. */
  request<T = unknown>(type: string, data: unknown, timeoutMs = 10_000): Promise<T> {
    if (this.stopped) return Promise.reject(new ApiError({ code: "realtime_unavailable", message: "Realtime client stopped." }));
    if (this.transport === "sse") return this.viaRest<T>(type, data);
    const ref = `r${++this.refN}`;
    const frame = JSON.stringify({ type, ref, data });
    return new Promise<T>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(ref);
        this.queue = this.queue.filter((q) => q.ref !== ref);
        reject(new ApiError({ code: "timeout", message: `No acknowledgement for ${type}.` }));
      }, timeoutMs);
      this.pending.set(ref, { resolve: resolve as (v: unknown) => void, reject, timer, frame });
      if (this.state === "open" && this.ws?.readyState === WebSocket.OPEN) this.ws.send(frame);
      else this.queue.push({ ref, frame });
    });
  }

  /** Fire-and-forget frame (typing indicators). */
  send(type: string, data: unknown) {
    if (this.transport === "sse") {
      void this.viaRest(type, data).catch(() => undefined);
      return;
    }
    if (this.state === "open" && this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify({ type, data }));
  }

  // --- transport -----------------------------------------------------------

  private async connect() {
    if (this.stopped) return;
    if (typeof navigator !== "undefined" && navigator.onLine === false) {
      this.setState("offline");
      return;
    }
    this.setState(this.openedOnce ? "reconnecting" : "connecting");
    if (this.transport === "ws" && this.wsFailures >= WS_FAILURES_BEFORE_SSE) {
      logger.warn("realtime: WebSocket unavailable, falling back to SSE", undefined, { code: "realtime_fallback" });
      this.transport = "sse";
      this.sseSince = Date.now();
    }
    if (this.transport === "sse") {
      this.connectSSE();
      return;
    }
    let ticket: { ticket: string; path: string };
    try {
      ticket = await realtimeApi.ticket();
    } catch (raw) {
      const error = toApiError(raw);
      if (["unauthenticated", "session_expired", "account_disabled"].includes(String(error.code))) {
        this.emit({ type: "auth.revoked", ts: new Date().toISOString(), data: { reason: error.code } });
        this.stop();
        return;
      }
      logger.debug("realtime: ticket request failed", { code: String(error.code) });
      this.scheduleReconnect();
      return;
    }
    if (this.stopped) return;
    const url = new URL(`${WS_URL}${ticket.path}`);
    url.searchParams.set("ticket", ticket.ticket);
    // Realtime error frames are localized server-side (§10).
    url.searchParams.set("locale", currentLocale());
    if (this.lastEventId) url.searchParams.set("resume", this.lastEventId);

    let opened = false;
    let ws: WebSocket;
    try {
      ws = new WebSocket(url.toString(), [PROTOCOL]);
    } catch (error) {
      logger.warn("realtime: WebSocket construction failed", error);
      this.wsFailures++;
      this.scheduleReconnect();
      return;
    }
    this.ws = ws;
    ws.onopen = () => {
      opened = true;
      this.lastMessageAt = Date.now();
    };
    ws.onmessage = (event) => this.handleRaw(String(event.data));
    ws.onerror = () => logger.debug("realtime: websocket error");
    ws.onclose = (event) => {
      if (this.ws !== ws) return;
      this.ws = null;
      this.stopHeartbeat();
      this.failPending("realtime_unavailable");
      if (this.stopped) return;
      if (!opened) this.wsFailures++;
      logger.debug("realtime: closed", { code: String(event.code), reason: event.reason });
      switch (event.code) {
        case 4001: // session revoked (deactivated, password changed, signed out elsewhere)
          this.emit({ type: "auth.revoked", ts: new Date().toISOString(), data: { reason: "revoked" } });
          this.stop();
          return;
        case 1001: // server restarting
          this.scheduleReconnect(400);
          return;
        case 4009: // too many connections for this user
          this.scheduleReconnect(15_000);
          return;
      }
      this.scheduleReconnect();
    };
  }

  private connectSSE() {
    const url = new URL(`${API_URL}/api/v1/realtime/stream`);
    url.searchParams.set("locale", currentLocale());
    if (this.lastEventId) url.searchParams.set("resume", this.lastEventId);
    const es = new EventSource(url.toString(), { withCredentials: true });
    this.es = es;
    es.onmessage = (event) => this.handleRaw(event.data);
    es.onerror = () => {
      if (this.es !== es) return;
      if (es.readyState === EventSource.CLOSED) {
        // The server refused the stream (likely an expired session).
        this.es = null;
        void refreshSession().then((ok) => {
          if (!ok) {
            this.emit({ type: "auth.revoked", ts: new Date().toISOString(), data: { reason: "session_expired" } });
            this.stop();
          } else this.scheduleReconnect();
        });
        return;
      }
      this.setState("reconnecting"); // EventSource retries with Last-Event-ID itself
    };
  }

  private handleRaw(raw: string) {
    let env: RealtimeEvent;
    try {
      env = JSON.parse(raw) as RealtimeEvent;
    } catch (error) {
      logger.warn("realtime: malformed frame", error);
      return;
    }
    this.lastMessageAt = Date.now();
    if (env.id) this.rememberEventId(env.id);

    switch (env.type) {
      case "hello":
        this.onHello(env as RealtimeEvent<{ connection_id: string; heartbeat_ms?: number; resumed?: boolean }>);
        return;
      case "ack":
      case "error":
        if (env.ref && this.pending.has(env.ref)) {
          const p = this.pending.get(env.ref)!;
          clearTimeout(p.timer);
          this.pending.delete(env.ref);
          if (env.type === "ack") p.resolve(env.data);
          else
            p.reject(
              new ApiError({
                code: env.error?.code ?? "internal_error",
                message: env.error?.message ?? "Request failed.",
                details: env.error?.details,
                retryAfter: env.error?.retry_after,
              }),
            );
          return;
        }
        if (env.type === "error") logger.debug("realtime: unsolicited error", { code: env.error?.code });
        return;
      case "pong":
        return;
      case "server.shutdown":
        this.setState("reconnecting");
        break;
    }
    this.emit(env);
  }

  private onHello(env: RealtimeEvent<{ connection_id: string; heartbeat_ms?: number; resumed?: boolean }>) {
    this.connectionId = env.data?.connection_id ?? null;
    this.attempts = 0;
    if (this.transport === "ws") this.wsFailures = 0;
    const reconnected = this.openedOnce;
    this.openedOnce = true;
    this.setState("open");
    if (this.transport === "ws") {
      const frames = this.queue;
      this.queue = [];
      for (const q of frames) if (this.pending.has(q.ref)) this.ws?.send(q.frame);
      this.startHeartbeat(env.data?.heartbeat_ms);
    } else {
      this.flushQueueViaRest();
    }
    // Presence is per connection: re-join every chat session.
    for (const id of this.joined) {
      void this.request("session.join", { session_id: id }).catch((error) =>
        logger.debug("realtime: rejoin failed", { code: toApiError(error).code }),
      );
    }
    this.emit({ ...env, data: { ...env.data, reconnected } });
    if (this.transport === "sse" && Date.now() - this.sseSince > SSE_RETRY_WS_AFTER_MS) {
      // Periodically give WebSockets another chance.
      this.wsFailures = 0;
    }
  }

  private flushQueueViaRest() {
    const frames = this.queue;
    this.queue = [];
    for (const q of frames) {
      const p = this.pending.get(q.ref);
      if (!p) continue;
      const parsed = JSON.parse(q.frame) as { type: string; data: unknown };
      clearTimeout(p.timer);
      this.pending.delete(q.ref);
      this.viaRest(parsed.type, parsed.data).then(p.resolve, (e) => p.reject(toApiError(e)));
    }
  }

  /** Map intents to REST calls when streaming over SSE. */
  private async viaRest<T>(type: string, data: unknown): Promise<T> {
    const d = (data ?? {}) as Record<string, string>;
    if (!this.connectionId && (type === "session.join" || type === "session.leave"))
      throw new ApiError({ code: "realtime_unavailable", message: "Not connected yet." });
    switch (type) {
      case "message.send":
        return (await chatApi.send(d.session_id, d.body, d.client_msg_id)) as T;
      case "session.join":
        return (await realtimeApi.presence(this.connectionId!, d.session_id, "join")) as T;
      case "session.leave":
        return (await realtimeApi.presence(this.connectionId!, d.session_id, "leave")) as T;
      case "typing":
        await chatApi.typing(d.session_id, Boolean((data as { typing?: boolean }).typing));
        return undefined as T;
      case "message.read":
        await chatApi.markRead(d.conversation_id, d.message_id);
        return { read: true } as T;
    }
    throw new ApiError({ code: "bad_request", message: `Unsupported intent ${type}.` });
  }

  private rememberEventId(id: string) {
    this.lastEventId = id;
    try {
      sessionStorage.setItem(this.storageKey, id);
    } catch {
      // Resume still works within this page lifetime.
    }
  }

  private startHeartbeat(serverMs?: number) {
    this.stopHeartbeat();
    const every = Math.min(serverMs ?? HEARTBEAT_MS, HEARTBEAT_MS);
    this.heartbeatTimer = setInterval(() => {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
      if (Date.now() - this.lastMessageAt > DEAD_AFTER_MS) {
        logger.debug("realtime: heartbeat timeout, reconnecting");
        this.ws.close(4000, "heartbeat timeout");
        return;
      }
      this.ws.send(JSON.stringify({ type: "ping", ref: `hb${++this.refN}` }));
    }, every);
  }

  private stopHeartbeat() {
    if (this.heartbeatTimer) clearInterval(this.heartbeatTimer);
    this.heartbeatTimer = null;
  }

  private scheduleReconnect(delay?: number) {
    if (this.stopped) return;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    const exp = Math.min(MAX_BACKOFF_MS, 500 * 2 ** this.attempts);
    const wait = delay ?? Math.round(Math.random() * exp); // full jitter
    this.attempts++;
    this.setState("reconnecting");
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      void this.connect();
    }, wait);
  }

  private clearTimers() {
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
    this.stopHeartbeat();
  }

  private failPending(code: string) {
    for (const [ref, p] of this.pending) {
      // Queued (unsent) frames survive a reconnect; sent ones are failed so
      // callers can retry them (message sends are idempotent).
      if (this.queue.some((q) => q.ref === ref)) continue;
      clearTimeout(p.timer);
      p.reject(new ApiError({ code, message: "Connection lost before acknowledgement." }));
      this.pending.delete(ref);
    }
  }

  private emit(env: RealtimeEvent) {
    for (const key of [env.type, "*"]) {
      this.handlers.get(key)?.forEach((h) => {
        try {
          h(env);
        } catch (error) {
          logger.error(`realtime handler for ${env.type} threw`, error);
        }
      });
    }
  }

  private setState(state: ConnectionState) {
    if (this.state === state) return;
    this.state = state;
    this.stateListeners.forEach((l) => l(state, this.transport));
  }

  private onOnline = () => {
    if (this.state === "open" || this.stopped) return;
    this.attempts = 0;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
    void this.connect();
  };

  private onOffline = () => {
    if (this.stopped) return;
    this.setState("offline");
  };

  private onVisible = () => {
    if (document.visibilityState !== "visible" || this.stopped) return;
    if (this.state === "reconnecting" || this.state === "offline") this.onOnline();
  };
}
