/**
 * Server-side read of the public pricing catalog (docs/platform-extensions.md
 * §24.1, `GET /pricing`) so the pricing page is server-rendered with the live
 * prices. Any failure (timeout, network, bad status, malformed body) yields
 * null and the page renders without prices and a Retry button: prices are
 * never invented.
 */
import { API_URL } from "@/lib/api/config";
import type { PricingSummary } from "@/lib/api/pricing";
import { parsePricingSummary } from "@/lib/pricing/parse";
import { createServerCache } from "@/lib/server-cache";

/** The page is the main content, so it gets a little more time than the layout's site config. */
export const PRICING_SERVER_TIMEOUT_MS = 2_000;
export const PRICING_SERVER_TTL_MS = 30_000;
export const PRICING_SERVER_FAILURE_TTL_MS = 5_000;
/** Next.js revalidation of the fetch itself (§24.8: revalidated every 60 s). */
export const PRICING_REVALIDATE_SECONDS = 60;

const memory = createServerCache<PricingSummary>({ ttlMs: PRICING_SERVER_TTL_MS, failureTtlMs: PRICING_SERVER_FAILURE_TTL_MS });

/** Test hook: forget every cached catalog. */
export function resetPricingCache() {
  memory.clear();
}

type Options = { timeoutMs?: number; fetchImpl?: typeof fetch; baseUrl?: string; now?: () => number; cache?: boolean };

export async function fetchPricing(locale: string, opts: Options = {}): Promise<PricingSummary | null> {
  const { now = Date.now, cache = true } = opts;
  if (!cache) return fetchPricingOnce(locale, opts);
  return memory.get(`${opts.baseUrl ?? API_URL}|${locale}`, () => fetchPricingOnce(locale, opts), now);
}

async function fetchPricingOnce(locale: string, opts: Options): Promise<PricingSummary | null> {
  const { timeoutMs = PRICING_SERVER_TIMEOUT_MS, fetchImpl = fetch, baseUrl = API_URL } = opts;
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), timeoutMs);
  try {
    const res = await fetchImpl(`${baseUrl}/api/v1/pricing`, {
      headers: { Accept: "application/json", "X-Locale": locale, "Accept-Language": locale },
      signal: ctrl.signal,
      next: { revalidate: PRICING_REVALIDATE_SECONDS },
    } as RequestInit);
    if (!res.ok) return null;
    return parsePricingSummary(await res.json());
  } catch {
    return null;
  } finally {
    clearTimeout(timer);
  }
}
