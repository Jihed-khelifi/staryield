// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// Refresh outcomes, cross-tab coordination and the sign-out rules of
// docs/platform-extensions.md §22.1.

type Handler = (url: string, init: RequestInit) => Response | Promise<Response>;

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json", ...headers } });
const expiredBody = { error: { code: "session_expired", message: "expired" } };

let handler: Handler;
let calls: { url: string; init: RequestInit }[];
let csrfCalls: number;

const refreshCalls = () => calls.filter((c) => c.url.endsWith("/auth/refresh"));
const header = (init: RequestInit, name: string) => new Headers(init.headers).get(name);

async function load() {
  vi.resetModules();
  return import("@/lib/api/client");
}

class FakeChannel {
  static instances: FakeChannel[] = [];
  sent: unknown[] = [];
  onmessage: ((e: MessageEvent) => void) | null = null;
  constructor(public name: string) {
    FakeChannel.instances.push(this);
  }
  postMessage(msg: unknown) {
    this.sent.push(msg);
  }
  close() {}
  receive(data: unknown) {
    this.onmessage?.({ data } as MessageEvent);
  }
}

beforeEach(() => {
  calls = [];
  csrfCalls = 0;
  FakeChannel.instances = [];
  localStorage.setItem("staryield.auth-hint", "1");
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit = {}) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/api/v1/auth/csrf")) return json(200, { csrf_token: `tok-${++csrfCalls}` });
      if (String(url).endsWith("/api/v1/client-logs")) return new Response(null, { status: 202 });
      return handler(String(url), init);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
  localStorage.clear();
});

describe("refresh outcomes", () => {
  it("answers 'refreshed' on 200 and keeps the hint", async () => {
    handler = () => json(200, { csrf_token: "tok-1", access_expires_at: "x" });
    const { refreshSessionOutcome, refreshSession, readAuthHint } = await load();
    await expect(refreshSessionOutcome()).resolves.toEqual({ outcome: "refreshed" });
    await expect(refreshSession()).resolves.toBe(true);
    expect(readAuthHint()).toBe(true);
  });

  it("answers 'expired' only on 401: clears the hint and notifies once", async () => {
    handler = () => json(401, expiredBody);
    const { refreshSessionOutcome, onSessionExpired, readAuthHint } = await load();
    const expired = vi.fn();
    onSessionExpired(expired);
    const result = await refreshSessionOutcome();
    expect(result.outcome).toBe("expired");
    expect(readAuthHint()).toBe(false);
    expect(expired).toHaveBeenCalledTimes(1);
  });

  it("retries once with a fresh CSRF token after 403 csrf_failed", async () => {
    let n = 0;
    handler = () => (++n === 1 ? json(403, { error: { code: "csrf_failed", message: "stale" } }) : json(200, {}));
    const { refreshSessionOutcome, onSessionExpired, readAuthHint } = await load();
    const expired = vi.fn();
    onSessionExpired(expired);
    await expect(refreshSessionOutcome()).resolves.toEqual({ outcome: "refreshed" });
    expect(refreshCalls()).toHaveLength(2);
    expect(header(refreshCalls()[0].init, "X-CSRF-Token")).toBe("tok-1");
    expect(header(refreshCalls()[1].init, "X-CSRF-Token")).toBe("tok-2");
    expect(csrfCalls).toBe(2);
    expect(expired).not.toHaveBeenCalled();
    expect(readAuthHint()).toBe(true);
  });

  it("does not sign out when csrf_failed persists", async () => {
    handler = () => json(403, { error: { code: "csrf_failed", message: "stale" } });
    const { refreshSessionOutcome, onSessionExpired, readAuthHint } = await load();
    const expired = vi.fn();
    onSessionExpired(expired);
    const result = await refreshSessionOutcome();
    expect(result.outcome).toBe("unavailable");
    expect(refreshCalls()).toHaveLength(2);
    expect(expired).not.toHaveBeenCalled();
    expect(readAuthHint()).toBe(true);
  });

  it.each([
    ["429", () => json(429, { error: { code: "rate_limited", message: "slow" } }, { "Retry-After": "3" }), "rate_limited"],
    ["503", () => json(503, { error: { code: "service_unavailable", message: "down" } }), "service_unavailable"],
    ["502 gateway page", () => new Response("<html>bad gateway</html>", { status: 502 }), "service_unavailable"],
    ["500", () => json(500, { error: { code: "internal_error", message: "oops" } }), "internal_error"],
    [
      "network failure",
      () => {
        throw new TypeError("Failed to fetch");
      },
      "network_error",
    ],
  ])("answers 'unavailable' for %s without clearing anything", async (_name, respond, code) => {
    handler = respond as Handler;
    const { refreshSessionOutcome, onSessionExpired, readAuthHint } = await load();
    const expired = vi.fn();
    onSessionExpired(expired);
    const result = await refreshSessionOutcome();
    expect(result.outcome).toBe("unavailable");
    expect(result.error?.code).toBe(code);
    expect(readAuthHint()).toBe(true);
    expect(expired).not.toHaveBeenCalled();
  });

  it("times out after 15 s as 'unavailable' (timeout)", async () => {
    vi.useFakeTimers();
    handler = (_url, init) =>
      new Promise((_, reject) => init.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError"))));
    const { refreshSessionOutcome, readAuthHint } = await load();
    const p = refreshSessionOutcome();
    await vi.advanceTimersByTimeAsync(15_100);
    const result = await p;
    expect(result.outcome).toBe("unavailable");
    expect(result.error?.code).toBe("timeout");
    expect(readAuthHint()).toBe(true);
  });

  it("treats refresh_conflict (old API) as refreshed after a short wait", async () => {
    vi.useFakeTimers();
    handler = () => json(409, { error: { code: "refresh_conflict", message: "again" } });
    const { refreshSessionOutcome } = await load();
    const p = refreshSessionOutcome();
    await vi.advanceTimersByTimeAsync(600);
    await expect(p).resolves.toEqual({ outcome: "refreshed" });
  });

  it("wraps a failing CSRF fetch as 'unavailable' (network_error)", async () => {
    handler = () => json(200, {});
    (fetch as unknown as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
      if (String(url).endsWith("/auth/csrf")) throw new TypeError("Load failed");
      return json(200, {});
    });
    const { refreshSessionOutcome } = await load();
    const result = await refreshSessionOutcome();
    expect(result.outcome).toBe("unavailable");
    expect(result.error?.code).toBe("network_error");
  });

  it("shares one refresh between concurrent callers", async () => {
    handler = () => json(200, {});
    const { refreshSessionOutcome } = await load();
    await Promise.all([refreshSessionOutcome(), refreshSessionOutcome(), refreshSessionOutcome()]);
    expect(refreshCalls()).toHaveLength(1);
  });
});

describe("api() with refresh outcomes", () => {
  it("retries the request after a refresh", async () => {
    let n = 0;
    handler = (url) => (url.endsWith("/auth/refresh") ? json(200, {}) : ++n === 1 ? json(401, expiredBody) : json(200, { id: "me" }));
    const { api } = await load();
    await expect(api("/api/v1/me")).resolves.toEqual({ id: "me" });
  });

  it("throws the retryable refresh error, not the 401, when refresh is unavailable", async () => {
    handler = (url) => (url.endsWith("/auth/refresh") ? json(503, { error: { code: "service_unavailable", message: "down" } }) : json(401, expiredBody));
    const { api, readAuthHint, onSessionExpired } = await load();
    const expired = vi.fn();
    onSessionExpired(expired);
    await expect(api("/api/v1/me", { retries: 0 })).rejects.toMatchObject({ code: "service_unavailable", retryable: true });
    expect(readAuthHint()).toBe(true);
    expect(expired).not.toHaveBeenCalled();
  });

  it("surfaces a network error when the refresh cannot connect", async () => {
    handler = (url) => {
      if (url.endsWith("/auth/refresh")) throw new TypeError("Failed to fetch");
      return json(401, expiredBody);
    };
    const { api } = await load();
    await expect(api("/api/v1/me", { retries: 0 })).rejects.toMatchObject({ code: "network_error" });
  });

  it("forces a second refresh when a shared/skipped one left the request unauthorized", async () => {
    let n = 0;
    handler = (url) => (url.endsWith("/auth/refresh") ? json(200, {}) : ++n < 3 ? json(401, expiredBody) : json(200, { ok: 1 }));
    const { api } = await load();
    await expect(api("/api/v1/me")).resolves.toEqual({ ok: 1 });
    expect(refreshCalls()).toHaveLength(2);
  });

  it("rejects with the 401 and signs out when the session really expired", async () => {
    handler = () => json(401, expiredBody);
    const { api, readAuthHint } = await load();
    await expect(api("/api/v1/me")).rejects.toMatchObject({ code: "session_expired" });
    expect(readAuthHint()).toBe(false);
  });

  it("wraps a failing CSRF fetch of an unsafe request as network_error", async () => {
    handler = () => json(200, {});
    (fetch as unknown as ReturnType<typeof vi.fn>).mockImplementation(async () => {
      throw new TypeError("Failed to fetch");
    });
    const { api } = await load();
    await expect(api("/api/v1/x", { method: "POST" })).rejects.toMatchObject({ code: "network_error" });
  });

  it("sends no CSRF token for csrf:false requests (stateless calculators)", async () => {
    handler = () => json(200, { ok: 1 });
    (fetch as unknown as ReturnType<typeof vi.fn>).mockImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/auth/csrf")) throw new TypeError("Failed to fetch"); // blocked
      return json(200, { ok: 1 });
    });
    const { api } = await load();
    await expect(api("/api/v1/astrology/natal", { method: "POST", body: {}, csrf: false })).resolves.toEqual({ ok: 1 });
    expect(calls.some((c) => c.url.endsWith("/auth/csrf"))).toBe(false);
  });
});

describe("cross-tab coordination", () => {
  it("runs the refresh under the Web Lock", async () => {
    const request = vi.fn(async (_name: string, cb: () => Promise<unknown>) => cb());
    vi.stubGlobal("navigator", { ...navigator, locks: { request }, onLine: true });
    handler = () => json(200, {});
    const { refreshSessionOutcome } = await load();
    await refreshSessionOutcome();
    expect(request).toHaveBeenCalledWith("staryield:auth-refresh", expect.any(Function));
  });

  it("works without Web Locks and BroadcastChannel", async () => {
    vi.stubGlobal("navigator", { ...navigator, locks: undefined, onLine: true });
    vi.stubGlobal("BroadcastChannel", undefined);
    handler = () => json(200, {});
    const { refreshSessionOutcome } = await load();
    await expect(refreshSessionOutcome()).resolves.toEqual({ outcome: "refreshed" });
  });

  it("announces a refresh and skips one another tab just did (10 s)", async () => {
    vi.stubGlobal("BroadcastChannel", FakeChannel);
    handler = () => json(200, {});
    const { refreshSessionOutcome, onSignedOutElsewhere } = await load();
    onSignedOutElsewhere(() => undefined); // starts listening
    const channel = FakeChannel.instances[0];
    expect(channel.name).toBe("staryield:auth");

    await refreshSessionOutcome();
    expect(channel.sent).toEqual([{ type: "refreshed", at: expect.any(Number) }]);
    expect(refreshCalls()).toHaveLength(1);

    // Skipped inside the window (our own refresh counts as well)…
    await refreshSessionOutcome();
    expect(refreshCalls()).toHaveLength(1);
    // …unless forced.
    await refreshSessionOutcome({ force: true });
    expect(refreshCalls()).toHaveLength(2);
  });

  it("skips a refresh another tab announced, and refreshes again once it is older than 10 s", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-11-09T10:00:00Z"));
    vi.stubGlobal("BroadcastChannel", FakeChannel);
    handler = () => json(200, {});
    const { refreshSessionOutcome, onSignedOutElsewhere } = await load();
    onSignedOutElsewhere(() => undefined);
    FakeChannel.instances[0].receive({ type: "refreshed", at: Date.now() });
    await expect(refreshSessionOutcome()).resolves.toEqual({ outcome: "refreshed" });
    expect(refreshCalls()).toHaveLength(0);
    vi.setSystemTime(new Date("2026-11-09T10:00:11Z"));
    await refreshSessionOutcome();
    expect(refreshCalls()).toHaveLength(1);
  });

  it("propagates sign-out: an expiry or explicit sign-out broadcasts, a received one notifies listeners", async () => {
    vi.stubGlobal("BroadcastChannel", FakeChannel);
    handler = () => json(401, expiredBody);
    const { refreshSessionOutcome, endSession, onSignedOutElsewhere, readAuthHint } = await load();
    const other = vi.fn();
    onSignedOutElsewhere(other);
    const channel = FakeChannel.instances[0];

    await refreshSessionOutcome();
    expect(channel.sent).toContainEqual({ type: "signed_out" });

    channel.sent.length = 0;
    localStorage.setItem("staryield.auth-hint", "1");
    endSession();
    expect(readAuthHint()).toBe(false);
    expect(channel.sent).toEqual([{ type: "signed_out" }]);

    channel.receive({ type: "signed_out" });
    expect(other).toHaveBeenCalledTimes(1);
    expect(channel.sent).toHaveLength(1); // receiving does not echo
  });
});
