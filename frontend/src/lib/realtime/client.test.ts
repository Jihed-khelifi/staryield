// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const ticket = vi.fn();
vi.mock("@/lib/api/endpoints", () => ({
  realtimeApi: { ticket: () => ticket(), presence: vi.fn() },
  chatApi: { send: vi.fn(), typing: vi.fn(), markRead: vi.fn() },
}));
vi.mock("@/lib/api/client", () => ({ refreshSession: vi.fn(async () => true) }));

class MockWebSocket {
  static OPEN = 1;
  static instances: MockWebSocket[] = [];
  readyState = 0;
  sent: string[] = [];
  onopen?: () => void;
  onmessage?: (e: { data: string }) => void;
  onclose?: (e: { code: number; reason: string }) => void;
  onerror?: () => void;
  constructor(public url: string, public protocols: string[]) {
    MockWebSocket.instances.push(this);
  }
  send(data: string) {
    this.sent.push(data);
  }
  close(code = 1000, reason = "") {
    this.readyState = 3;
    this.onclose?.({ code, reason });
  }
  // test helpers
  open() {
    this.readyState = 1;
    this.onopen?.();
  }
  push(env: object) {
    this.onmessage?.({ data: JSON.stringify(env) });
  }
  hello(extra: object = {}) {
    this.push({ type: "hello", ts: new Date().toISOString(), data: { connection_id: "c1", heartbeat_ms: 25000, ...extra } });
  }
  frames() {
    return this.sent.map((s) => JSON.parse(s) as { type: string; ref?: string; data?: Record<string, unknown> });
  }
}

const latest = () => MockWebSocket.instances.at(-1)!;
const flush = () => new Promise((r) => setTimeout(r, 0));

async function client() {
  const { RealtimeClient } = await import("@/lib/realtime/client");
  return new RealtimeClient("user-1");
}

beforeEach(() => {
  MockWebSocket.instances = [];
  ticket.mockReset();
  ticket.mockResolvedValue({ ticket: "t1", path: "/api/v1/realtime/ws", expires_in: 30 });
  vi.stubGlobal("WebSocket", MockWebSocket);
  sessionStorage.clear();
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("RealtimeClient", () => {
  it("connects with a ticket and the negotiated protocol, then opens on hello", async () => {
    const rt = await client();
    const states: string[] = [];
    rt.onState((s) => states.push(s));
    rt.start();
    await flush();
    const ws = latest();
    expect(ws.url).toContain("/api/v1/realtime/ws?ticket=t1");
    expect(ws.protocols).toEqual(["staryield.v1"]);
    ws.open();
    ws.hello();
    expect(rt.connection.state).toBe("open");
    expect(states).toEqual(["connecting", "open"]);
    rt.stop();
  });

  it("resolves requests on ack and rejects on error frames", async () => {
    const rt = await client();
    rt.start();
    await flush();
    latest().open();
    latest().hello();
    const ok = rt.request("message.send", { body: "hi" });
    const bad = rt.request("message.send", { body: "" });
    const [first, second] = latest().frames().filter((f) => f.type === "message.send");
    latest().push({ type: "ack", ref: first.ref, data: { message: { id: "m1" } } });
    latest().push({ type: "error", ref: second.ref, error: { code: "validation_failed", message: "Write something" } });
    await expect(ok).resolves.toEqual({ message: { id: "m1" } });
    await expect(bad).rejects.toMatchObject({ code: "validation_failed" });
    rt.stop();
  });

  it("queues intents while connecting and flushes them after hello", async () => {
    const rt = await client();
    rt.start();
    const pending = rt.request("session.join", { session_id: "s1" });
    await flush();
    latest().open();
    expect(latest().frames()).toHaveLength(0);
    latest().hello();
    const join = latest().frames().find((f) => f.type === "session.join")!;
    latest().push({ type: "ack", ref: join.ref, data: { status: "active" } });
    await expect(pending).resolves.toEqual({ status: "active" });
    rt.stop();
  });

  it("reconnects after a drop, resumes from the last event id and re-joins sessions", async () => {
    vi.useFakeTimers();
    const rt = await client();
    const events: string[] = [];
    rt.on("*", (e) => events.push(e.type));
    rt.start();
    await vi.advanceTimersByTimeAsync(0);
    latest().open();
    latest().hello();
    void rt.joinSession("s1").catch(() => undefined);
    latest().push({ id: "171-0", type: "message.created", ts: "", data: {} });
    expect(events).toContain("message.created");
    expect(sessionStorage.getItem("staryield.rt.last:user-1")).toBe("171-0");

    latest().close(1006, "abnormal");
    expect(rt.connection.state).toBe("reconnecting");
    await vi.advanceTimersByTimeAsync(1_000);
    const second = latest();
    expect(MockWebSocket.instances).toHaveLength(2);
    expect(second.url).toContain("resume=171-0");
    second.open();
    second.hello({ resumed: true });
    expect(second.frames().some((f) => f.type === "session.join" && f.data?.session_id === "s1")).toBe(true);
    rt.stop();
  });

  it("stops and reports revocation on close code 4001", async () => {
    const rt = await client();
    const revoked = vi.fn();
    rt.on("auth.revoked", revoked);
    rt.start();
    await flush();
    latest().open();
    latest().hello();
    latest().close(4001, "session revoked");
    expect(revoked).toHaveBeenCalledOnce();
    expect(rt.connection.state).toBe("closed");
  });

  it("detects dead connections with heartbeats", async () => {
    vi.useFakeTimers();
    const rt = await client();
    rt.start();
    await vi.advanceTimersByTimeAsync(0);
    const ws = latest();
    ws.open();
    ws.hello();
    await vi.advanceTimersByTimeAsync(20_000);
    expect(ws.frames().some((f) => f.type === "ping")).toBe(true);
    const close = vi.spyOn(ws, "close");
    await vi.advanceTimersByTimeAsync(40_000); // no traffic for > 50s
    expect(close).toHaveBeenCalledWith(4000, "heartbeat timeout");
    rt.stop();
  });

  it("falls back to SSE after repeated WebSocket handshake failures", async () => {
    vi.useFakeTimers();
    const sources: string[] = [];
    vi.stubGlobal(
      "EventSource",
      class {
        static CLOSED = 2;
        readyState = 0;
        onmessage?: (e: { data: string }) => void;
        onerror?: () => void;
        constructor(url: string) {
          sources.push(url);
        }
        close() {}
      },
    );
    const rt = await client();
    rt.start();
    for (let i = 0; i < 3; i++) {
      await vi.advanceTimersByTimeAsync(0);
      latest().close(1006, "handshake failed");
      await vi.advanceTimersByTimeAsync(31_000);
    }
    expect(sources).toHaveLength(1);
    expect(sources[0]).toContain("/api/v1/realtime/stream");
    expect(rt.connection.transport).toBe("sse");
    rt.stop();
  });

  it("goes offline with the browser and reconnects when back online", async () => {
    const rt = await client();
    rt.start();
    await flush();
    latest().open();
    latest().hello();
    latest().close(1006, "");
    window.dispatchEvent(new Event("offline"));
    expect(rt.connection.state).toBe("offline");
    window.dispatchEvent(new Event("online"));
    await flush();
    expect(MockWebSocket.instances).toHaveLength(2);
    rt.stop();
  });
});
