/**
 * Server-side read of the public site configuration (§15.1) for the first
 * paint: the promo bar then renders without a layout shift. Any failure
 * (timeout, network, bad status, malformed body) yields null and the client
 * provider loads the configuration itself.
 */
import { API_URL } from "@/lib/api/config";
import type { SiteBanner, SiteConfig } from "@/lib/api/types";

/** Tight budget: a slow API must never hold up the page. */
export const SITE_CONFIG_SERVER_TIMEOUT_MS = 700;

const AUDIENCES = new Set(["all", "anonymous", "signed_in"]);

function isBanner(value: unknown): value is SiteBanner {
  const b = value as SiteBanner | null;
  return (
    !!b &&
    typeof b.id === "string" &&
    typeof b.text === "string" &&
    typeof b.cta_label === "string" &&
    !!b.action &&
    (b.action.type === "link" || b.action.type === "none") &&
    typeof b.action.href === "string" &&
    AUDIENCES.has(b.audience)
  );
}

/** Narrow an unknown JSON body to a SiteConfig (null when malformed). */
export function parseSiteConfig(body: unknown): SiteConfig | null {
  const c = body as Partial<SiteConfig> | null;
  if (!c || typeof c !== "object") return null;
  const banners = c.banners;
  if (!banners || !Array.isArray(banners.items) || typeof banners.rotation_seconds !== "number") return null;
  if (!c.signup_offer || typeof c.signup_offer.enabled !== "boolean" || typeof c.signup_offer.free_minutes !== "number") return null;
  if (!c.free_mode || typeof c.free_mode.active !== "boolean") return null;
  // Older APIs send no `auth`: no sign-in provider is available then (§23.1).
  const auth = { google: (c.auth as { google?: unknown } | undefined)?.google === true };
  return { ...(c as SiteConfig), auth, banners: { ...banners, items: banners.items.filter(isBanner) } };
}

/**
 * Per-isolate memory cache. Every server render needs the configuration, and
 * on the edge all renders reach the API from a few shared egress addresses
 * (one rate-limit bucket), so each isolate asks at most once per TTL and
 * locale, shares one request between concurrent renders, and backs off
 * briefly after a failure. The client provider still revalidates on its own.
 */
export const SITE_CONFIG_SERVER_TTL_MS = 30_000;
export const SITE_CONFIG_SERVER_FAILURE_TTL_MS = 5_000;
type CacheEntry = { value: SiteConfig | null; expires: number; pending?: Promise<SiteConfig | null> };
const memory = new Map<string, CacheEntry>();

/** Test hook: forget every cached configuration. */
export function resetSiteConfigCache() {
  memory.clear();
}

export async function fetchSiteConfig(
  locale: string,
  opts: { timeoutMs?: number; fetchImpl?: typeof fetch; baseUrl?: string; now?: () => number; cache?: boolean } = {},
): Promise<SiteConfig | null> {
  const { now = Date.now, cache = true } = opts;
  if (!cache) return fetchSiteConfigOnce(locale, opts);
  const key = `${opts.baseUrl ?? API_URL}|${locale}`;
  const hit = memory.get(key);
  if (hit && hit.expires > now()) return hit.value;
  const stale = hit?.value ?? null;
  // A refresh is in flight: serve the last good copy at once, else wait for it.
  if (hit?.pending) return stale ?? hit.pending;
  const pending = fetchSiteConfigOnce(locale, opts).then((fresh) => {
    // A failed refresh keeps the last good copy and is retried soon.
    const value = fresh ?? stale;
    memory.set(key, { value, expires: now() + (fresh ? SITE_CONFIG_SERVER_TTL_MS : SITE_CONFIG_SERVER_FAILURE_TTL_MS) });
    return value;
  });
  memory.set(key, { value: stale, expires: 0, pending });
  return stale ?? pending;
}

async function fetchSiteConfigOnce(
  locale: string,
  opts: { timeoutMs?: number; fetchImpl?: typeof fetch; baseUrl?: string },
): Promise<SiteConfig | null> {
  const { timeoutMs = SITE_CONFIG_SERVER_TIMEOUT_MS, fetchImpl = fetch, baseUrl = API_URL } = opts;
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), timeoutMs);
  try {
    const res = await fetchImpl(`${baseUrl}/api/v1/site/config?locale=${encodeURIComponent(locale)}`, {
      headers: { Accept: "application/json", "X-Locale": locale },
      signal: ctrl.signal,
      next: { revalidate: 30 },
    } as RequestInit);
    if (!res.ok) return null;
    return parseSiteConfig(await res.json());
  } catch {
    return null;
  } finally {
    clearTimeout(timer);
  }
}
