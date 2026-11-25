import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  ADVISOR_DIRECTORY_PAGE_SIZE,
  ADVISOR_REVALIDATE_SECONDS,
  ADVISOR_SERVER_MAX_ENTRIES,
  ADVISOR_SERVER_TIMEOUT_MS,
  ADVISOR_SLUG,
  ADVISOR_SLUGS_TTL_MS,
  fetchAdvisor,
  fetchAdvisorSlugs,
  parseAdvisorSummary,
  resetAdvisorCache,
  resetAdvisorSlugsCache,
} from "@/lib/psychics/server";
import { createServerCache } from "@/lib/server-cache";

const ok = (body: unknown, status = 200) => Promise.resolve(new Response(JSON.stringify(body), { status }));
const asFetch = (fn: unknown) => fn as typeof fetch;
const profile = (slug: string, extra: object = {}) => ({ slug, name: " María Luz ", headline: "Tarot", image_url: "https://assets.staryield.net/a.webp", ...extra });

describe("parseAdvisorSummary", () => {
  it("keeps only the fields the metadata uses, for the slug that was asked", () => {
    expect(parseAdvisorSummary(profile("maria-luz", { rate_per_minute: 50 }), "maria-luz")).toEqual({
      slug: "maria-luz",
      name: "María Luz",
      headline: "Tarot",
      image_url: "https://assets.staryield.net/a.webp",
    });
  });

  it("is null for another advisor, a blank name or an unusable body", () => {
    expect(parseAdvisorSummary(profile("theo"), "maria-luz")).toBeNull();
    expect(parseAdvisorSummary(profile("maria-luz", { name: "  " }), "maria-luz")).toBeNull();
    expect(parseAdvisorSummary(null, "maria-luz")).toBeNull();
    expect(parseAdvisorSummary("maria-luz", "maria-luz")).toBeNull();
  });
});

describe("fetchAdvisor", () => {
  beforeEach(() => resetAdvisorCache());

  it("reads the public profile with Next revalidation", async () => {
    const fetchImpl = vi.fn(() => ok(profile("maria-luz")));
    expect(await fetchAdvisor("maria-luz", { fetchImpl: asFetch(fetchImpl), baseUrl: "http://api.test" })).toMatchObject({
      found: true,
      advisor: { slug: "maria-luz", name: "María Luz" },
    });
    const [url, init] = fetchImpl.mock.calls[0] as unknown as [string, RequestInit & { next?: { revalidate: number } }];
    expect(url).toBe("http://api.test/api/v1/psychics/maria-luz");
    expect(init.next?.revalidate).toBe(ADVISOR_REVALIDATE_SECONDS);
  });

  it("confirms an unknown advisor only on a 404; any other failure is null (nothing published)", async () => {
    const opts = { baseUrl: "http://api.test", cache: false };
    expect(await fetchAdvisor("ghost", { ...opts, fetchImpl: asFetch(() => ok({ code: "not_found" }, 404)) })).toEqual({ found: false });
    expect(await fetchAdvisor("theo", { ...opts, fetchImpl: asFetch(() => ok({}, 503)) })).toBeNull();
    expect(await fetchAdvisor("theo", { ...opts, fetchImpl: asFetch(() => ok({}, 429)) })).toBeNull();
    expect(await fetchAdvisor("theo", { ...opts, fetchImpl: asFetch(() => Promise.reject(new Error("down"))) })).toBeNull();
    expect(await fetchAdvisor("theo", { ...opts, fetchImpl: asFetch(() => ok(profile("ramone"))) })).toBeNull();
  });

  it("gives up after the timeout", async () => {
    vi.useFakeTimers();
    const fetchImpl = vi.fn((_url: string, init: RequestInit) => new Promise<Response>((_, reject) => init.signal?.addEventListener("abort", () => reject(new Error("aborted")))));
    const pending = fetchAdvisor("theo", { fetchImpl: asFetch(fetchImpl), baseUrl: "http://api.test" });
    await vi.advanceTimersByTimeAsync(ADVISOR_SERVER_TIMEOUT_MS + 1);
    expect(await pending).toBeNull();
    vi.useRealTimers();
  });

  it("shares one request between the metadata and the page of the same render", async () => {
    const fetchImpl = vi.fn(() => ok(profile("theo")));
    const opts = { fetchImpl: asFetch(fetchImpl), baseUrl: "http://api.test" };
    await Promise.all([fetchAdvisor("theo", opts), fetchAdvisor("theo", opts)]);
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  it("accepts only well-formed slugs", () => {
    for (const slug of ["theo", "maria-luz", "a1-b2"]) expect(ADVISOR_SLUG.test(slug), slug).toBe(true);
    for (const slug of ["", "Theo", "maria--luz", "-theo", "theo-", "a".repeat(65), "the_o", "../x"]) expect(ADVISOR_SLUG.test(slug), slug).toBe(false);
  });
});

describe("fetchAdvisorSlugs", () => {
  beforeEach(() => resetAdvisorSlugsCache());

  /** A directory of `count` advisors served 48 per page. */
  function directory(count: number, fail?: (page: number) => boolean) {
    const slugs = Array.from({ length: count }, (_, index) => `advisor-${String(index).padStart(4, "0")}`);
    return vi.fn((url: string) => {
      const query = new URL(url).searchParams;
      const page = Number(query.get("page"));
      const size = Number(query.get("page_size"));
      if (fail?.(page)) return ok({ code: "unavailable" }, 503);
      return ok({ items: slugs.slice((page - 1) * size, page * size).map((slug) => ({ slug })), page, page_size: size, total: count, total_pages: Math.ceil(count / size) });
    });
  }

  it("reads every directory page and returns each slug once, sorted", async () => {
    const fetchImpl = directory(100);
    const slugs = await fetchAdvisorSlugs({ fetchImpl: asFetch(fetchImpl), baseUrl: "http://api.test", cache: false });
    expect(slugs).toHaveLength(100);
    expect(slugs).toEqual([...(slugs ?? [])].sort());
    expect(fetchImpl).toHaveBeenCalledTimes(Math.ceil(100 / ADVISOR_DIRECTORY_PAGE_SIZE));
    expect(fetchImpl.mock.calls[0][0]).toBe(`http://api.test/api/v1/psychics?page=1&page_size=${ADVISOR_DIRECTORY_PAGE_SIZE}`);
  });

  it("drops malformed slugs and never returns a partial list", async () => {
    const malformed = vi.fn(() => ok({ items: [{ slug: "theo" }, { slug: "Not A Slug" }, { name: "no slug" }, null], total_pages: 1 }));
    expect(await fetchAdvisorSlugs({ fetchImpl: asFetch(malformed), baseUrl: "http://api.test", cache: false })).toEqual(["theo"]);
    const partial = directory(200, (page) => page === 3);
    expect(await fetchAdvisorSlugs({ fetchImpl: asFetch(partial), baseUrl: "http://api.test", cache: false })).toBeNull();
    expect(await fetchAdvisorSlugs({ fetchImpl: asFetch(() => ok({ items: "x" })), baseUrl: "http://api.test", cache: false })).toBeNull();
  });

  it("is read at most once per hour, keeping the last good list when a refresh fails", async () => {
    let t = 1_000;
    let failing = false;
    const fetchImpl = directory(10, () => failing);
    const opts = { fetchImpl: asFetch(fetchImpl), baseUrl: "http://api.test", now: () => t };
    expect(await fetchAdvisorSlugs(opts)).toHaveLength(10);
    expect(await fetchAdvisorSlugs(opts)).toHaveLength(10);
    expect(fetchImpl).toHaveBeenCalledTimes(1);
    t += ADVISOR_SLUGS_TTL_MS + 1;
    failing = true;
    expect(await fetchAdvisorSlugs(opts)).toHaveLength(10);
  });
});

describe("server cache bound", () => {
  it("keeps at most maxEntries keys, dropping the least recently stored", async () => {
    const cache = createServerCache<string>({ ttlMs: 60_000, failureTtlMs: 1_000, maxEntries: 2 });
    const load = vi.fn(async () => "value");
    await cache.get("a", load);
    await cache.get("b", load);
    await cache.get("c", load);
    expect(load).toHaveBeenCalledTimes(3);
    await cache.get("c", load);
    await cache.get("b", load);
    expect(load).toHaveBeenCalledTimes(3);
    await cache.get("a", load);
    expect(load).toHaveBeenCalledTimes(4);
    expect(ADVISOR_SERVER_MAX_ENTRIES).toBeGreaterThan(0);
  });
});
