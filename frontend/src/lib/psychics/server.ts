/**
 * growth (seo): server-side reads of the public advisors.
 *
 * - `fetchAdvisor`: one profile (`GET /psychics/{slug}`) for the profile
 *   page's metadata and status code. Only an advisor the API confirms gets a
 *   title, a description and a link preview; an advisor the API does not know
 *   (or no longer shows) is a real 404. Any other failure (timeout, network,
 *   bad status, malformed body) yields null: the page then renders its
 *   client-side profile as before and publishes nothing about the slug.
 * - `fetchAdvisorSlugs`: every public advisor's slug, read from the
 *   directory (`GET /psychics`, 48 per page) for the advisors' sitemap.
 */
import { API_URL } from "@/lib/api/config";
import type { DirectoryPage, Psychic } from "@/lib/api/types";
import { createServerCache } from "@/lib/server-cache";

/** An advisor profile slug (`/psychics/<slug>`), as the profile route accepts it. */
export const ADVISOR_SLUG = /^(?=.{1,64}$)[a-z0-9]+(-[a-z0-9]+)*$/;

/** What the page's metadata needs from a profile. */
export type AdvisorSummary = Pick<Psychic, "slug" | "name" | "headline" | "image_url">;

/** The API's answer: the advisor, or a confirmed "no such public advisor" (404). */
export type AdvisorLookup = { found: true; advisor: AdvisorSummary } | { found: false };

/** The profile is the page's subject, so it gets a little more time than the layout's site config. */
export const ADVISOR_SERVER_TIMEOUT_MS = 1_500;
export const ADVISOR_SERVER_TTL_MS = 60_000;
export const ADVISOR_SERVER_FAILURE_TTL_MS = 5_000;
/** Slugs come from the URL: the per-isolate cache keeps at most this many. */
export const ADVISOR_SERVER_MAX_ENTRIES = 500;
/** Next.js revalidation of the fetch itself. */
export const ADVISOR_REVALIDATE_SECONDS = 60;

const memory = createServerCache<AdvisorLookup>({
  ttlMs: ADVISOR_SERVER_TTL_MS,
  failureTtlMs: ADVISOR_SERVER_FAILURE_TTL_MS,
  maxEntries: ADVISOR_SERVER_MAX_ENTRIES,
});

/** Test hook: forget every cached profile. */
export function resetAdvisorCache() {
  memory.clear();
}

/** Narrow an unknown JSON body to the fields the metadata uses (null when malformed). */
export function parseAdvisorSummary(body: unknown, slug: string): AdvisorSummary | null {
  const p = body as Partial<Psychic> | null;
  if (!p || typeof p !== "object" || p.slug !== slug || typeof p.name !== "string" || !p.name.trim()) return null;
  return {
    slug: p.slug,
    name: p.name.trim(),
    headline: typeof p.headline === "string" ? p.headline : "",
    image_url: typeof p.image_url === "string" ? p.image_url : "",
  };
}

type Options = { timeoutMs?: number; fetchImpl?: typeof fetch; baseUrl?: string; now?: () => number; cache?: boolean };

/** The public profile of `slug` (a slug the route already validated), or null when the API cannot tell. */
export async function fetchAdvisor(slug: string, opts: Options = {}): Promise<AdvisorLookup | null> {
  const { now = Date.now, cache = true } = opts;
  if (!cache) return fetchAdvisorOnce(slug, opts);
  return memory.get(`${opts.baseUrl ?? API_URL}|${slug}`, () => fetchAdvisorOnce(slug, opts), now);
}

async function fetchAdvisorOnce(slug: string, opts: Options): Promise<AdvisorLookup | null> {
  const { timeoutMs = ADVISOR_SERVER_TIMEOUT_MS, fetchImpl = fetch, baseUrl = API_URL } = opts;
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), timeoutMs);
  try {
    const res = await fetchImpl(`${baseUrl}/api/v1/psychics/${encodeURIComponent(slug)}`, {
      headers: { Accept: "application/json" },
      signal: ctrl.signal,
      next: { revalidate: ADVISOR_REVALIDATE_SECONDS },
    } as RequestInit);
    if (res.status === 404) return { found: false };
    if (!res.ok) return null;
    const advisor = parseAdvisorSummary(await res.json(), slug);
    return advisor ? { found: true, advisor } : null;
  } catch {
    return null;
  } finally {
    clearTimeout(timer);
  }
}

/** The directory's largest page (GET /psychics `page_size`). */
export const ADVISOR_DIRECTORY_PAGE_SIZE = 48;
/** A sitemap holds at most 50,000 URLs, and each profile is listed in 3 locales. */
export const ADVISOR_SITEMAP_MAX = Math.floor(50_000 / 3);
export const ADVISOR_SLUGS_TTL_MS = 60 * 60_000;
export const ADVISOR_SLUGS_FAILURE_TTL_MS = 60_000;
/** Directory pages requested at once while listing every advisor. */
const DIRECTORY_CONCURRENCY = 4;

const slugsMemory = createServerCache<string[]>({ ttlMs: ADVISOR_SLUGS_TTL_MS, failureTtlMs: ADVISOR_SLUGS_FAILURE_TTL_MS });

/** Test hook: forget the cached advisor list. */
export function resetAdvisorSlugsCache() {
  slugsMemory.clear();
}

/**
 * Every public advisor's slug, sorted (at most `ADVISOR_SITEMAP_MAX`), or
 * null when any directory page cannot be read: a partial list is never
 * returned, so a sitemap never drops profiles because of one failed request.
 */
export async function fetchAdvisorSlugs(opts: Options = {}): Promise<string[] | null> {
  const { now = Date.now, cache = true } = opts;
  if (!cache) return fetchAdvisorSlugsOnce(opts);
  return slugsMemory.get(`${opts.baseUrl ?? API_URL}|slugs`, () => fetchAdvisorSlugsOnce(opts), now);
}

async function fetchAdvisorSlugsOnce(opts: Options): Promise<string[] | null> {
  const first = await fetchDirectoryPage(1, opts);
  if (!first) return null;
  const lastPage = Math.min(first.totalPages, Math.ceil(ADVISOR_SITEMAP_MAX / ADVISOR_DIRECTORY_PAGE_SIZE));
  const pages = Array.from({ length: Math.max(0, lastPage - 1) }, (_, index) => index + 2);
  const slugs = new Set(first.slugs);
  for (let index = 0; index < pages.length; index += DIRECTORY_CONCURRENCY) {
    const batch = await Promise.all(pages.slice(index, index + DIRECTORY_CONCURRENCY).map((page) => fetchDirectoryPage(page, opts)));
    for (const result of batch) {
      if (!result) return null;
      for (const slug of result.slugs) slugs.add(slug);
    }
  }
  return [...slugs].sort().slice(0, ADVISOR_SITEMAP_MAX);
}

/** One directory page: its well-formed slugs and the page count, or null on any failure. */
async function fetchDirectoryPage(page: number, opts: Options): Promise<{ slugs: string[]; totalPages: number } | null> {
  const { timeoutMs = ADVISOR_SERVER_TIMEOUT_MS, fetchImpl = fetch, baseUrl = API_URL } = opts;
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), timeoutMs);
  try {
    const query = new URLSearchParams({ page: String(page), page_size: String(ADVISOR_DIRECTORY_PAGE_SIZE) });
    const res = await fetchImpl(`${baseUrl}/api/v1/psychics?${query}`, { headers: { Accept: "application/json" }, signal: ctrl.signal, cache: "no-store" });
    if (!res.ok) return null;
    const body = (await res.json()) as Partial<DirectoryPage> | null;
    if (!body || !Array.isArray(body.items) || typeof body.total_pages !== "number") return null;
    const slugs = body.items.map((item) => item?.slug).filter((slug): slug is string => typeof slug === "string" && ADVISOR_SLUG.test(slug));
    return { slugs, totalPages: body.total_pages };
  } catch {
    return null;
  } finally {
    clearTimeout(timer);
  }
}
