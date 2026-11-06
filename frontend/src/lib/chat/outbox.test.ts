import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api/errors";
import type { ChatMessage } from "@/lib/api/types";
import {
  afterFailure,
  backoffMs,
  classifyFailure,
  enqueue,
  failClosedSessions,
  MAX_AUTO_ATTEMPTS,
  newEntry,
  nextRetryAt,
  nextToSend,
  OUTBOX_LIMIT,
  OUTBOX_MAX_AGE_MS,
  outboxKey,
  outboxMessage,
  readOutbox,
  retryEntry,
  writeOutbox,
  type OutboxEntry,
} from "@/lib/chat/outbox";
import { OutboxController } from "@/lib/chat/outbox-controller";

const USER = "user-1";
const CONV = "11111111-1111-4111-8111-111111111111";
const S1 = "22222222-2222-4222-8222-222222222222";
const S2 = "33333333-3333-4333-8333-333333333333";

/** A Storage double (sessionStorage is per tab; tests get their own). */
class MemoryStorage implements Storage {
  private map = new Map<string, string>();
  get length() {
    return this.map.size;
  }
  clear() {
    this.map.clear();
  }
  getItem(key: string) {
    return this.map.has(key) ? this.map.get(key)! : null;
  }
  key(i: number) {
    return [...this.map.keys()][i] ?? null;
  }
  removeItem(key: string) {
    this.map.delete(key);
  }
  setItem(key: string, value: string) {
    this.map.set(key, value);
  }
}

const entry = (n: number, over: Partial<OutboxEntry> = {}): OutboxEntry => ({
  ...newEntry({ client_msg_id: `cmid-${String(n).padStart(4, "0")}`, conversation_id: CONV, session_id: S1, sender_id: USER, body: `message ${n}` }, Date.parse("2026-11-05T10:00:00Z") + n),
  ...over,
});

const NOW = Date.parse("2026-11-05T10:01:00Z");

describe("outbox persistence", () => {
  it("round-trips entries through storage, per user", () => {
    const store = new MemoryStorage();
    writeOutbox(USER, [entry(1), entry(2)], store);
    expect(readOutbox(USER, NOW, store).map((e) => e.body)).toEqual(["message 1", "message 2"]);
    expect(readOutbox("someone-else", NOW, store)).toEqual([]);
    writeOutbox(USER, [], store);
    expect(store.getItem(outboxKey(USER))).toBeNull();
  });

  it("queues again a message that was in flight when the page went away", () => {
    const store = new MemoryStorage();
    writeOutbox(USER, [entry(1, { state: "sending", attempts: 1, retry_at: NOW + 9_000 })], store);
    const [e] = readOutbox(USER, NOW, store);
    expect(e.state).toBe("queued");
    expect(e.retry_at).toBe(0);
    expect(e.attempts).toBe(1);
  });

  it("drops invalid, foreign and expired entries (storage is not trusted)", () => {
    const store = new MemoryStorage();
    const good = entry(1);
    const old = entry(2, { created_at: new Date(NOW - OUTBOX_MAX_AGE_MS - 1).toISOString() });
    store.setItem(
      outboxKey(USER),
      JSON.stringify([
        good,
        old,
        { ...entry(3), sender_id: "intruder" },
        { ...entry(4), client_msg_id: "<script>" },
        { ...entry(5), conversation_id: "not-a-uuid" },
        { ...entry(6), body: "   " },
        { ...entry(7), body: "x".repeat(5000) },
        { ...entry(8), state: "delivered" },
        { ...entry(9), attempts: -1 },
        "garbage",
        null,
      ]),
    );
    expect(readOutbox(USER, NOW, store).map((e) => e.client_msg_id)).toEqual([good.client_msg_id]);
    store.setItem(outboxKey(USER), "{not json");
    expect(readOutbox(USER, NOW, store)).toEqual([]);
    store.setItem(outboxKey(USER), JSON.stringify({ a: 1 }));
    expect(readOutbox(USER, NOW, store)).toEqual([]);
  });

  it("survives blocked or full storage", () => {
    const broken = {
      getItem: () => {
        throw new Error("SecurityError");
      },
      setItem: () => {
        throw new Error("QuotaExceededError");
      },
      removeItem: () => undefined,
    } as unknown as Storage;
    expect(readOutbox(USER, NOW, broken)).toEqual([]);
    expect(writeOutbox(USER, [entry(1)], broken)).toBe(false);
    expect(readOutbox(USER, NOW, null)).toEqual([]);
  });

  it("caps the number of entries", () => {
    let list: OutboxEntry[] = [];
    for (let i = 0; i < OUTBOX_LIMIT + 5; i++) list = enqueue(list, entry(i));
    expect(list).toHaveLength(OUTBOX_LIMIT);
    expect(list[0].client_msg_id).toBe(entry(5).client_msg_id);
    // Enqueueing the same client id twice is a no-op.
    expect(enqueue(list, list[3])).toBe(list);
  });
});

describe("outbox ordering and retries", () => {
  it("sends in order: nothing newer goes while the oldest is in flight or backing off", () => {
    const list = [entry(1), entry(2)];
    expect(nextToSend(list, S1, NOW)?.client_msg_id).toBe(entry(1).client_msg_id);
    expect(nextToSend([{ ...list[0], state: "sending" }, list[1]], S1, NOW)).toBeNull();
    expect(nextToSend([{ ...list[0], retry_at: NOW + 1000 }, list[1]], S1, NOW)).toBeNull();
    expect(nextRetryAt([{ ...list[0], retry_at: NOW + 1000 }, list[1]], S1)).toBe(NOW + 1000);
    // Failed entries wait for the user and do not block the rest.
    expect(nextToSend([{ ...list[0], state: "failed" }, list[1]], S1, NOW)?.client_msg_id).toBe(entry(2).client_msg_id);
    // Other sessions are not sent into this one.
    expect(nextToSend([entry(3, { session_id: S2 })], S1, NOW)).toBeNull();
  });

  it("backs off exponentially with jitter, capped at 30 s", () => {
    expect(backoffMs(1, () => 1)).toBe(1000);
    expect(backoffMs(2, () => 1)).toBe(2000);
    expect(backoffMs(4, () => 1)).toBe(8000);
    expect(backoffMs(20, () => 1)).toBe(30_000);
    expect(backoffMs(3, () => 0)).toBe(250);
  });

  it("classifies failures", () => {
    expect(classifyFailure(new ApiError({ code: "network_error", message: "" }))).toBe("retry");
    expect(classifyFailure(new ApiError({ code: "timeout", message: "" }))).toBe("retry");
    expect(classifyFailure(new ApiError({ code: "realtime_unavailable", message: "" }))).toBe("retry");
    expect(classifyFailure(new ApiError({ code: "whatever", message: "", status: 503 }))).toBe("retry");
    expect(classifyFailure(new ApiError({ code: "invalid_state", message: "", details: { status: "disconnected" } }))).toBe("wait");
    expect(classifyFailure(new ApiError({ code: "invalid_state", message: "", details: { status: "settled" } }))).toBe("fatal");
    expect(classifyFailure(new ApiError({ code: "validation_failed", message: "", status: 422 }))).toBe("fatal");
    expect(classifyFailure(new ApiError({ code: "not_found", message: "", status: 404 }))).toBe("fatal");
  });

  it("schedules a retry, honours Retry-After, and fails after the automatic attempts", () => {
    const e = { ...entry(1), attempts: 1 };
    const retry = afterFailure(e, new ApiError({ code: "network_error", message: "" }), NOW, () => 1);
    expect(retry).toMatchObject({ state: "queued", error: "network_error", retry_at: NOW + 1000 });
    const limited = afterFailure(e, new ApiError({ code: "rate_limited", message: "", status: 429, retryAfter: 7 }), NOW);
    expect(limited.retry_at).toBe(NOW + 7000);
    const exhausted = afterFailure({ ...e, attempts: MAX_AUTO_ATTEMPTS }, new ApiError({ code: "timeout", message: "" }), NOW);
    expect(exhausted.state).toBe("failed");
    const fatal = afterFailure(e, new ApiError({ code: "invalid_state", message: "", details: { status: "ended" } }), NOW);
    expect(fatal).toMatchObject({ state: "failed", error: "invalid_state" });
  });

  it("retries a failed message, optionally in the conversation's new session", () => {
    const [again] = retryEntry([{ ...entry(1), state: "failed", attempts: 5, error: "invalid_state" }], entry(1).client_msg_id, S2);
    expect(again).toMatchObject({ state: "queued", attempts: 0, retry_at: 0, session_id: S2, error: undefined });
  });

  it("fails what a closed session still holds", () => {
    const list = [entry(1), entry(2, { session_id: S2 })];
    const out = failClosedSessions(list, (id) => id === S1);
    expect(out.map((e) => e.state)).toEqual(["failed", "queued"]);
    expect(out[0].error).toBe("session_ended");
    expect(failClosedSessions(list, () => false)).toBe(list);
  });

  it("shows an entry as a pending message of the viewer", () => {
    expect(outboxMessage({ ...entry(1), state: "failed", error: "timeout" })).toMatchObject({
      id: entry(1).client_msg_id,
      client_msg_id: entry(1).client_msg_id,
      sender_id: USER,
      kind: "text",
      pending: "failed",
      failure: "timeout",
    });
  });
});

describe("OutboxController (delivery pump)", () => {
  let store: MemoryStorage;
  let clock: number;
  beforeEach(() => {
    vi.useFakeTimers();
    store = new MemoryStorage();
    clock = NOW;
  });
  afterEach(() => vi.useRealTimers());

  const delivered = (cmid: string, seq: number): ChatMessage => ({
    id: `srv-${seq}`,
    seq,
    client_msg_id: cmid,
    conversation_id: CONV,
    session_id: S1,
    sender_id: USER,
    kind: "text",
    body: cmid,
    created_at: new Date(clock).toISOString(),
  });

  function setup(transport: (e: OutboxEntry) => Promise<ChatMessage>) {
    const controller = new OutboxController(USER, { now: () => clock, random: () => 1, storage: store });
    const onDelivered = vi.fn();
    const onFailed = vi.fn();
    controller.configure(transport, { onDelivered, onFailed });
    controller.start();
    return { controller, onDelivered, onFailed };
  }

  const queue = (c: OutboxController, n: number, session = S1) =>
    c.send({ client_msg_id: `cmid-${String(n).padStart(4, "0")}`, conversation_id: CONV, session_id: session, body: `message ${n}` });

  it("persists queued messages until the connection is ready, then sends them in order", async () => {
    const order: string[] = [];
    let seq = 0;
    const { controller, onDelivered } = setup(async (e) => {
      order.push(e.client_msg_id);
      return delivered(e.client_msg_id, ++seq);
    });
    controller.setTarget(S1, false); // offline
    queue(controller, 1);
    queue(controller, 2);
    queue(controller, 3);
    await vi.runAllTimersAsync();
    expect(order).toEqual([]);
    expect(readOutbox(USER, clock, store)).toHaveLength(3); // survives a refresh

    controller.setTarget(S1, true);
    await vi.runAllTimersAsync();
    expect(order).toEqual(["cmid-0001", "cmid-0002", "cmid-0003"]);
    expect(onDelivered).toHaveBeenCalledTimes(3);
    expect(controller.getSnapshot()).toEqual([]);
    expect(store.getItem(outboxKey(USER))).toBeNull();
  });

  it("a refresh loses nothing: a new controller picks the queue up and flushes it", async () => {
    const first = setup(async () => new Promise<ChatMessage>(() => undefined)); // never acknowledged
    first.controller.setTarget(S1, true);
    queue(first.controller, 1);
    queue(first.controller, 2);
    await vi.advanceTimersByTimeAsync(10);
    expect(first.controller.getSnapshot()[0].state).toBe("sending");
    first.controller.stop(); // the page goes away mid-send

    const sent: string[] = [];
    const second = setup(async (e) => {
      sent.push(e.client_msg_id);
      return delivered(e.client_msg_id, sent.length);
    });
    expect(second.controller.getSnapshot().map((e) => e.state)).toEqual(["queued", "queued"]);
    second.controller.setTarget(S1, true);
    await vi.runAllTimersAsync();
    // The in-flight message is resent (idempotent on client_msg_id), still first.
    expect(sent).toEqual(["cmid-0001", "cmid-0002"]);
  });

  it("retries transient failures with backoff, and right away when the connection returns", async () => {
    let attempts = 0;
    const { controller, onDelivered } = setup(async (e) => {
      attempts++;
      if (attempts < 3) throw new ApiError({ code: "network_error", message: "offline" });
      return delivered(e.client_msg_id, 1);
    });
    controller.setTarget(S1, true);
    queue(controller, 1);
    await vi.advanceTimersByTimeAsync(0);
    expect(attempts).toBe(1);
    expect(controller.getSnapshot()[0]).toMatchObject({ state: "queued", error: "network_error", retry_at: clock + 1000 });
    // Not before the backoff…
    await vi.advanceTimersByTimeAsync(500);
    expect(attempts).toBe(1);
    // …unless the connection drops and comes back: then at once.
    controller.setTarget(S1, false);
    controller.setTarget(S1, true);
    await vi.advanceTimersByTimeAsync(0);
    expect(attempts).toBe(2);
    clock += 2000;
    await vi.advanceTimersByTimeAsync(2000);
    expect(attempts).toBe(3);
    expect(onDelivered).toHaveBeenCalledOnce();
    expect(controller.getSnapshot()).toEqual([]);
  });

  it("keeps messages queued while the chat is paused, and fails them when it ends", async () => {
    const { controller, onFailed } = setup(async () => {
      throw new ApiError({ code: "invalid_state", message: "paused", status: 409, details: { status: "disconnected" } });
    });
    controller.setTarget(S1, true);
    queue(controller, 1);
    queue(controller, 2);
    await vi.advanceTimersByTimeAsync(0);
    expect(controller.getSnapshot().map((e) => e.state)).toEqual(["queued", "queued"]);
    expect(onFailed).not.toHaveBeenCalled();
    controller.sessionClosed(S1);
    expect(controller.getSnapshot().map((e) => e.state)).toEqual(["failed", "failed"]);
    expect(controller.getSnapshot()[0].error).toBe("session_ended");
  });

  it("marks a message failed on a permanent error and lets the user retry or remove it", async () => {
    let fail = true;
    const { controller, onFailed, onDelivered } = setup(async (e) => {
      if (fail) throw new ApiError({ code: "validation_failed", message: "bad", status: 422 });
      return delivered(e.client_msg_id, 9);
    });
    controller.setTarget(S1, true);
    queue(controller, 1);
    queue(controller, 2);
    await vi.runAllTimersAsync();
    expect(controller.getSnapshot().map((e) => e.state)).toEqual(["failed", "failed"]);
    expect(onFailed).toHaveBeenCalledTimes(2);
    controller.discard("cmid-0002");
    expect(controller.getSnapshot()).toHaveLength(1);
    fail = false;
    controller.retry("cmid-0001", S1);
    await vi.runAllTimersAsync();
    expect(onDelivered).toHaveBeenCalledOnce();
    expect(controller.getSnapshot()).toEqual([]);
  });

  it("drops entries the server already confirmed (seen in history or events)", async () => {
    const { controller } = setup(async () => new Promise<ChatMessage>(() => undefined));
    controller.setTarget(S1, false);
    queue(controller, 1);
    queue(controller, 2);
    controller.confirm(["cmid-0001"]);
    expect(controller.getSnapshot().map((e) => e.client_msg_id)).toEqual(["cmid-0002"]);
  });

  it("notifies subscribers on every change", () => {
    const { controller } = setup(async (e) => delivered(e.client_msg_id, 1));
    const listener = vi.fn();
    const off = controller.subscribe(listener);
    controller.setTarget(S1, false);
    queue(controller, 1);
    expect(listener).toHaveBeenCalled();
    off();
  });
});
