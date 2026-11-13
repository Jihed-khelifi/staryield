import { beforeEach, describe, expect, it, vi } from "vitest";

import { parsePricingSummary } from "@/lib/pricing/parse";
import { canonicalPricing } from "@/test/pricing-fixtures";
import {
  fetchPricing,
  PRICING_REVALIDATE_SECONDS,
  PRICING_SERVER_FAILURE_TTL_MS,
  PRICING_SERVER_TIMEOUT_MS,
  PRICING_SERVER_TTL_MS,
  resetPricingCache,
} from "@/lib/pricing/server";

const ok = (body: unknown, status = 200) => Promise.resolve(new Response(JSON.stringify(body), { status }));
const asFetch = (fn: unknown) => fn as typeof fetch;

describe("parsePricingSummary", () => {
  it("keeps the catalog in the order the API sent", () => {
    const summary = parsePricingSummary(canonicalPricing());
    expect(summary?.packs.map((p) => p.code)).toEqual(["welcome_600", "quick_150", "starter_315", "popular_675", "deep_1350", "premium_2400"]);
    expect(summary?.free_trial).toEqual({ enabled: true, minutes: 4 });
    expect(summary?.rate_range).toEqual({ min: 30, max: 60 });
    expect(summary?.reference?.pack_id).toBe("p-quick");
  });

  it("drops malformed packs and badges but never repairs a price", () => {
    const body = canonicalPricing() as unknown as { packs: Record<string, unknown>[] };
    body.packs[1] = { ...body.packs[1], price_cents: "20" };
    body.packs[2] = { ...body.packs[2], badges: [{ type: "save", percent: 0 }, { type: "mystery" }, { type: "save", percent: 4.9 }] };
    const summary = parsePricingSummary(body);
    expect(summary?.packs.map((p) => p.code)).not.toContain("quick_150");
    expect(summary?.packs.find((p) => p.code === "starter_315")?.badges).toEqual([{ type: "save", percent: 4 }]);
  });

  it("is null for an unusable body, an empty catalog and an impossible rate range", () => {
    expect(parsePricingSummary(null)).toBeNull();
    expect(parsePricingSummary({})).toBeNull();
    expect(parsePricingSummary(canonicalPricing({ packs: [] }))).toBeNull();
    expect(parsePricingSummary(canonicalPricing({ rate_range: { min: 60, max: 30 } }))).toBeNull();
    expect(parsePricingSummary({ ...canonicalPricing(), free_trial: undefined })).toBeNull();
  });
});

describe("fetchPricing", () => {
  beforeEach(() => resetPricingCache());

  it("reads /api/v1/pricing in the reader's locale with Next revalidation", async () => {
    const fetchImpl = vi.fn(() => ok(canonicalPricing()));
    const summary = await fetchPricing("fr", { fetchImpl: asFetch(fetchImpl), baseUrl: "http://api.test" });
    expect(summary?.packs).toHaveLength(6);
    const [url, init] = fetchImpl.mock.calls[0] as unknown as [string, RequestInit & { next?: { revalidate: number } }];
    expect(url).toBe("http://api.test/api/v1/pricing");
    expect((init.headers as Record<string, string>)["X-Locale"]).toBe("fr");
    expect((init.headers as Record<string, string>)["Accept-Language"]).toBe("fr");
    expect(init.next?.revalidate).toBe(PRICING_REVALIDATE_SECONDS);
    expect(PRICING_REVALIDATE_SECONDS).toBe(60);
  });

  it("is null on a failed status, a network error and a malformed body (no invented prices)", async () => {
    const opts = { baseUrl: "http://api.test", cache: false };
    expect(await fetchPricing("en", { ...opts, fetchImpl: asFetch(() => ok({}, 503)) })).toBeNull();
    expect(await fetchPricing("en", { ...opts, fetchImpl: asFetch(() => Promise.reject(new Error("down"))) })).toBeNull();
    expect(await fetchPricing("en", { ...opts, fetchImpl: asFetch(() => ok({ packs: "no" })) })).toBeNull();
    expect(await fetchPricing("en", { ...opts, fetchImpl: asFetch(() => Promise.resolve(new Response("<html>", { status: 200 }))) })).toBeNull();
  });

  it("gives up after the timeout and returns null", async () => {
    vi.useFakeTimers();
    const fetchImpl = vi.fn((_url: string, init: RequestInit) => new Promise<Response>((_, reject) => init.signal?.addEventListener("abort", () => reject(new Error("aborted")))));
    const pending = fetchPricing("en", { fetchImpl: asFetch(fetchImpl), baseUrl: "http://api.test" });
    await vi.advanceTimersByTimeAsync(PRICING_SERVER_TIMEOUT_MS + 1);
    expect(await pending).toBeNull();
    vi.useRealTimers();
  });

  it("asks the API at most once per TTL and locale and shares one request between concurrent renders", async () => {
    let t = 1_000;
    const fetchImpl = vi.fn(() => ok(canonicalPricing()));
    const opts = { fetchImpl: asFetch(fetchImpl), baseUrl: "http://api.test", now: () => t };
    await Promise.all([fetchPricing("en", opts), fetchPricing("en", opts)]);
    expect(fetchImpl).toHaveBeenCalledTimes(1);
    t += PRICING_SERVER_TTL_MS - 1;
    await fetchPricing("en", opts);
    expect(fetchImpl).toHaveBeenCalledTimes(1);
    await fetchPricing("es", opts);
    expect(fetchImpl).toHaveBeenCalledTimes(2);
  });

  it("serves the last good copy when a refresh fails, then retries soon", async () => {
    let t = 1_000;
    const fetchImpl = vi.fn().mockImplementationOnce(() => ok(canonicalPricing())).mockImplementationOnce(() => ok({}, 500)).mockImplementation(() => ok(canonicalPricing({ rate_range: { min: 20, max: 50 } })));
    const opts = { fetchImpl: asFetch(fetchImpl), baseUrl: "http://api.test", now: () => t };
    expect((await fetchPricing("en", opts))?.rate_range.min).toBe(30);
    t += PRICING_SERVER_TTL_MS + 1;
    // Expired: the stale copy is served at once while the (failing) refresh runs.
    expect((await fetchPricing("en", opts))?.rate_range.min).toBe(30);
    await Promise.resolve();
    t += PRICING_SERVER_FAILURE_TTL_MS + 1;
    await fetchPricing("en", opts);
    await vi.waitFor(async () => expect((await fetchPricing("en", opts))?.rate_range.min).toBe(20));
  });
});
