import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  fetchSiteConfig,
  parseSiteConfig,
  resetSiteConfigCache,
  SITE_CONFIG_SERVER_FAILURE_TTL_MS,
  SITE_CONFIG_SERVER_TIMEOUT_MS,
  SITE_CONFIG_SERVER_TTL_MS,
} from "@/lib/site-config/server";

const good = {
  banners: {
    rotation_seconds: 6,
    items: [
      { id: "b1", text: "Hi", cta_label: "Go", action: { type: "link", href: "/signup" }, audience: "all" },
      { id: "bad", text: 5 },
    ],
  },
  signup_offer: { enabled: true, free_minutes: 5 },
  free_mode: { active: false },
  generated_at: "2026-11-07T00:00:00Z",
};
const ok = (body: unknown, status = 200) => Promise.resolve(new Response(JSON.stringify(body), { status }));

describe("fetchSiteConfig", () => {
  beforeEach(() => resetSiteConfigCache());

  it("returns the config, keeps only well-formed banners and asks for the locale", async () => {
    const fetchImpl = vi.fn(() => ok(good));
    const cfg = await fetchSiteConfig("fr", { fetchImpl: fetchImpl as unknown as typeof fetch, baseUrl: "http://api.test" });
    expect(cfg?.banners.items.map((b) => b.id)).toEqual(["b1"]);
    expect(cfg?.signup_offer.free_minutes).toBe(5);
    const [url, init] = fetchImpl.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("http://api.test/api/v1/site/config?locale=fr");
    expect((init.headers as Record<string, string>)["X-Locale"]).toBe("fr");
  });

  it("is null on a failed status, a network error and a malformed body", async () => {
    const opts = { baseUrl: "http://api.test", cache: false };
    expect(await fetchSiteConfig("en", { ...opts, fetchImpl: (() => ok({}, 500)) as unknown as typeof fetch })).toBeNull();
    expect(await fetchSiteConfig("en", { ...opts, fetchImpl: (() => Promise.reject(new Error("down"))) as unknown as typeof fetch })).toBeNull();
    expect(await fetchSiteConfig("en", { ...opts, fetchImpl: (() => ok({ banners: 1 })) as unknown as typeof fetch })).toBeNull();
    expect(await fetchSiteConfig("en", { ...opts, fetchImpl: (() => Promise.resolve(new Response("<html>", { status: 200 }))) as unknown as typeof fetch })).toBeNull();
  });

  it("gives up after the timeout (700 ms by default) and returns null", async () => {
    vi.useFakeTimers();
    const fetchImpl = vi.fn((_url: string, init: RequestInit) => new Promise<Response>((_, reject) => init.signal?.addEventListener("abort", () => reject(new Error("aborted")))));
    const pending = fetchSiteConfig("en", { fetchImpl: fetchImpl as unknown as typeof fetch, baseUrl: "http://api.test" });
    await vi.advanceTimersByTimeAsync(SITE_CONFIG_SERVER_TIMEOUT_MS - 1);
    let settled = false;
    void pending.then(() => (settled = true));
    await vi.advanceTimersByTimeAsync(0);
    expect(settled).toBe(false);
    await vi.advanceTimersByTimeAsync(2);
    expect(await pending).toBeNull();
    vi.useRealTimers();
    expect(SITE_CONFIG_SERVER_TIMEOUT_MS).toBeLessThanOrEqual(700);
  });
});

describe("fetchSiteConfig per-isolate cache", () => {
  beforeEach(() => resetSiteConfigCache());

  it("asks the API at most once per TTL and locale, and shares one request between concurrent renders", async () => {
    let t = 1_000;
    const fetchImpl = vi.fn(() => ok(good));
    const opts = { fetchImpl: fetchImpl as unknown as typeof fetch, baseUrl: "http://api.test", now: () => t };
    const [a, b] = await Promise.all([fetchSiteConfig("en", opts), fetchSiteConfig("en", opts)]);
    expect(a).toEqual(b);
    expect(fetchImpl).toHaveBeenCalledTimes(1);
    t += SITE_CONFIG_SERVER_TTL_MS - 1;
    await fetchSiteConfig("en", opts);
    expect(fetchImpl).toHaveBeenCalledTimes(1);
    await fetchSiteConfig("fr", opts);
    expect(fetchImpl).toHaveBeenCalledTimes(2);
  });

  it("serves the last good copy while refreshing and when a refresh fails, then retries soon", async () => {
    let t = 1_000;
    const fetchImpl = vi.fn(() => ok(good));
    const opts = { fetchImpl: fetchImpl as unknown as typeof fetch, baseUrl: "http://api.test", now: () => t };
    const first = await fetchSiteConfig("en", opts);
    t += SITE_CONFIG_SERVER_TTL_MS + 1;
    fetchImpl.mockImplementation(() => ok({}, 503));
    // Stale-while-revalidate: the expired copy is served at once.
    expect(await fetchSiteConfig("en", opts)).toEqual(first);
    await Promise.resolve();
    await new Promise((r) => setTimeout(r, 0));
    // The failed refresh kept the copy and backs off only briefly.
    expect(await fetchSiteConfig("en", opts)).toEqual(first);
    const calls = fetchImpl.mock.calls.length;
    t += SITE_CONFIG_SERVER_FAILURE_TTL_MS + 1;
    await fetchSiteConfig("en", opts);
    expect(fetchImpl.mock.calls.length).toBe(calls + 1);
  });

  it("does not cache a first failure for long and never returns a stale copy across locales", async () => {
    let t = 1_000;
    const fetchImpl = vi.fn(() => ok({}, 500));
    const opts = { fetchImpl: fetchImpl as unknown as typeof fetch, baseUrl: "http://api.test", now: () => t };
    expect(await fetchSiteConfig("es", opts)).toBeNull();
    fetchImpl.mockImplementation(() => ok(good));
    t += SITE_CONFIG_SERVER_FAILURE_TTL_MS + 1;
    expect((await fetchSiteConfig("es", opts))?.signup_offer.free_minutes).toBe(5);
  });
});

describe("parseSiteConfig", () => {
  it("rejects incomplete shapes", () => {
    expect(parseSiteConfig(null)).toBeNull();
    expect(parseSiteConfig({ ...good, signup_offer: {} })).toBeNull();
    expect(parseSiteConfig({ ...good, free_mode: undefined })).toBeNull();
  });

  it("reads which sign-in providers are configured (§23.1)", () => {
    expect(parseSiteConfig({ ...good, auth: { google: true } })?.auth).toEqual({ google: true });
    expect(parseSiteConfig({ ...good, auth: { google: false } })?.auth).toEqual({ google: false });
  });

  it("treats a missing or malformed `auth` (older APIs) as Google off", () => {
    expect(parseSiteConfig(good)?.auth).toEqual({ google: false });
    expect(parseSiteConfig({ ...good, auth: null })?.auth).toEqual({ google: false });
    expect(parseSiteConfig({ ...good, auth: {} })?.auth).toEqual({ google: false });
    expect(parseSiteConfig({ ...good, auth: { google: "yes" } })?.auth).toEqual({ google: false });
  });

  it("reads the memberships switch; absent or malformed (older APIs) means off (§25.1)", () => {
    expect(parseSiteConfig({ ...good, features: { memberships: true } })?.features).toEqual({ memberships: true });
    expect(parseSiteConfig({ ...good, features: { memberships: false } })?.features).toEqual({ memberships: false });
    expect(parseSiteConfig(good)?.features).toEqual({ memberships: false });
    expect(parseSiteConfig({ ...good, features: null })?.features).toEqual({ memberships: false });
    expect(parseSiteConfig({ ...good, features: {} })?.features).toEqual({ memberships: false });
    expect(parseSiteConfig({ ...good, features: { memberships: "true" } })?.features).toEqual({ memberships: false });
  });
});

