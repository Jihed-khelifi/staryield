// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

type Handler = (url: string, init: RequestInit) => Response | Promise<Response>;

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json", ...headers } });

let handler: Handler;
let calls: { url: string; init: RequestInit }[];

async function load() {
  vi.resetModules();
  return import("@/lib/api/client");
}

beforeEach(() => {
  calls = [];
  localStorage.setItem("staryield.auth-hint", "1");
  vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit = {}) => {
    calls.push({ url: String(url), init });
    if (String(url).endsWith("/api/v1/auth/csrf")) return json(200, { csrf_token: "tok-1" });
    if (String(url).endsWith("/api/v1/client-logs")) return new Response(null, { status: 202 });
    return handler(String(url), init);
  }));
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
  localStorage.clear();
});

const header = (init: RequestInit, name: string) => new Headers(init.headers).get(name);

describe("api client", () => {
  it("sends CSRF tokens, request ids and cookies on unsafe requests", async () => {
    handler = () => json(200, { ok: true });
    const { api } = await load();
    await api("/api/v1/me/profile", { method: "PATCH", body: { nickname: "x" } });
    const call = calls.find((c) => c.url.endsWith("/me/profile"))!;
    expect(header(call.init, "X-CSRF-Token")).toBe("tok-1");
    expect(header(call.init, "X-Request-ID")).toMatch(/[0-9a-f-]{36}/);
    expect(call.init.credentials).toBe("include");
  });

  it("refreshes an expired session once and retries the request", async () => {
    let attempt = 0;
    handler = (url) => {
      if (url.endsWith("/auth/refresh")) return json(200, { csrf_token: "tok-2" });
      attempt++;
      return attempt === 1 ? json(401, { error: { code: "session_expired", message: "expired" } }) : json(200, { id: "me" });
    };
    const { api } = await load();
    await expect(api("/api/v1/me")).resolves.toEqual({ id: "me" });
    expect(calls.filter((c) => c.url.endsWith("/auth/refresh"))).toHaveLength(1);
  });

  it("deduplicates concurrent refreshes and notifies when the session is gone", async () => {
    handler = (url) =>
      url.endsWith("/auth/refresh")
        ? json(401, { error: { code: "session_expired", message: "gone" } })
        : json(401, { error: { code: "session_expired", message: "expired" } });
    const { api, onSessionExpired } = await load();
    const expired = vi.fn();
    onSessionExpired(expired);
    const results = await Promise.allSettled([api("/api/v1/a"), api("/api/v1/b"), api("/api/v1/c")]);
    expect(results.every((r) => r.status === "rejected")).toBe(true);
    expect(calls.filter((c) => c.url.endsWith("/auth/refresh"))).toHaveLength(1);
    expect(expired).toHaveBeenCalledTimes(1);
  });

  it("re-fetches a stale CSRF token once", async () => {
    let attempt = 0;
    handler = () => (++attempt === 1 ? json(403, { error: { code: "csrf_failed", message: "stale" } }) : json(200, { ok: 1 }));
    const { api } = await load();
    await expect(api("/api/v1/x", { method: "POST" })).resolves.toEqual({ ok: 1 });
    expect(calls.filter((c) => c.url.endsWith("/auth/csrf"))).toHaveLength(2);
  });

  it("retries idempotent requests on transient failures, honouring Retry-After", async () => {
    vi.useFakeTimers();
    let attempt = 0;
    handler = () => (++attempt < 3 ? json(503, { error: { code: "service_unavailable", message: "down" } }, { "Retry-After": "1" }) : json(200, { ok: true }));
    const { api } = await load();
    const p = api("/api/v1/psychics");
    await vi.advanceTimersByTimeAsync(2_500);
    await expect(p).resolves.toEqual({ ok: true });
    expect(attempt).toBe(3);
  });

  it("never retries non-idempotent writes", async () => {
    handler = () => json(503, { error: { code: "service_unavailable", message: "down" } });
    const { api } = await load();
    await expect(api("/api/v1/chat/sessions", { method: "POST" })).rejects.toMatchObject({ code: "service_unavailable" });
    expect(calls.filter((c) => c.url.endsWith("/chat/sessions"))).toHaveLength(1);
  });

  it("maps network failures, timeouts and malformed responses to typed errors", async () => {
    handler = () => {
      throw new TypeError("Failed to fetch");
    };
    const { api } = await load();
    await expect(api("/api/v1/x", { retries: 0 })).rejects.toMatchObject({ code: "network_error" });

    handler = () => new Response("<html>gateway</html>", { status: 502 });
    await expect(api("/api/v1/x", { retries: 0 })).rejects.toMatchObject({ code: "service_unavailable", status: 502 });

    handler = () => new Response("not json", { status: 200, headers: { "Content-Type": "application/json" } });
    await expect(api("/api/v1/x")).rejects.toMatchObject({ code: "invalid_response" });

    handler = (_url, init) =>
      new Promise((_, reject) => init.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError"))));
    await expect(api("/api/v1/slow", { timeoutMs: 20, retries: 0 })).rejects.toMatchObject({ code: "timeout" });
  });

  it("surfaces server error codes, details and request ids", async () => {
    handler = () =>
      json(402, { error: { code: "insufficient_credits", message: "no", details: { required: 40, available: 0 }, request_id: "req-9" } });
    const { api } = await load();
    await expect(api("/api/v1/chat/sessions", { method: "POST" })).rejects.toMatchObject({
      code: "insufficient_credits",
      status: 402,
      requestId: "req-9",
      details: { required: 40, available: 0 },
    });
  });
});
