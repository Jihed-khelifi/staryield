/**
 * First-touch campaign attribution (docs/platform-extensions.md §5.4).
 *
 * A landing URL may carry `utm_*` parameters and a short-link click id
 * (`sy_click`, appended by the API's `/l/:code` redirect). The first touch is
 * kept for 30 days in localStorage and sent with the sign-up request, so the
 * server can link the new account to its campaign and short link.
 *
 * Privacy: only campaign parameters, the referrer origin (never its path or
 * query) and the landing path are stored. Storage failures (private mode,
 * blocked site data) are ignored — attribution is best effort and must never
 * break the page. Only `sy_click` is removed from the address bar; UTM tags
 * stay so shared links keep working.
 */
import type { AttributionInput } from "@/lib/api/types";
import { track } from "@/lib/analytics";

export const ATTRIBUTION_KEY = "staryield.attribution";
export const ATTRIBUTION_TTL_MS = 30 * 24 * 60 * 60 * 1000;
export const UTM_KEYS = ["utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term"] as const;
const CLICK_PARAM = "sy_click";
const CLICK_ID = /^[A-Za-z0-9_-]{1,128}$/;

type UtmKey = (typeof UTM_KEYS)[number];

export type AttributionEnv = {
  /** The full landing URL. */
  href: string;
  referrer: string;
  storage: Pick<Storage, "getItem" | "setItem" | "removeItem"> | null;
  now: number;
  /** Replace the address bar URL without navigating. */
  replaceUrl: (url: string) => void;
  track: typeof track;
};

function clean(value: string | null, max = 100): string | undefined {
  if (!value) return undefined;
  // Drop control characters; the server validates the rest.
  const v = value.replace(/[\u0000-\u001f\u007f]/g, "").trim().slice(0, max);
  return v || undefined;
}

/** Campaign parameters present in a query string (null when there are none). */
export function readCampaignParams(search: string): Pick<AttributionInput, UtmKey | "click_id"> | null {
  const params = new URLSearchParams(search);
  const out: Pick<AttributionInput, UtmKey | "click_id"> = {};
  for (const key of UTM_KEYS) {
    const v = clean(params.get(key));
    if (v) out[key] = v;
  }
  const click = clean(params.get(CLICK_PARAM), 128);
  if (click && CLICK_ID.test(click)) out.click_id = click;
  return Object.keys(out).length > 0 ? out : null;
}

/** The referrer's origin, or undefined for same-site or unusable referrers. */
export function referrerOrigin(referrer: string, ownOrigin: string): string | undefined {
  if (!referrer) return undefined;
  try {
    const url = new URL(referrer);
    if (url.protocol !== "http:" && url.protocol !== "https:") return undefined;
    if (url.origin === ownOrigin) return undefined;
    return url.origin;
  } catch {
    return undefined;
  }
}

/** The stored first touch, or null when missing, malformed or expired (expired entries are removed). */
export function loadAttribution(storage: AttributionEnv["storage"], now = Date.now()): AttributionInput | null {
  if (!storage) return null;
  let raw: string | null = null;
  try {
    raw = storage.getItem(ATTRIBUTION_KEY);
  } catch {
    return null;
  }
  if (!raw) return null;
  try {
    const value = JSON.parse(raw) as AttributionInput;
    const seen = value && typeof value === "object" && value.first_seen_at ? Date.parse(value.first_seen_at) : NaN;
    if (!Number.isFinite(seen) || now - seen > ATTRIBUTION_TTL_MS || seen - now > 60_000) throw new Error("expired");
    return value;
  } catch {
    try {
      storage.removeItem(ATTRIBUTION_KEY);
    } catch {
      // Nothing more to do.
    }
    return null;
  }
}

/** URL without the click id (and only it). Returns null when there is nothing to strip. */
export function stripClickId(href: string): string | null {
  try {
    const url = new URL(href);
    if (!url.searchParams.has(CLICK_PARAM)) return null;
    url.searchParams.delete(CLICK_PARAM);
    return url.pathname + url.search + url.hash;
  } catch {
    return null;
  }
}

function defaultEnv(): AttributionEnv | null {
  if (typeof window === "undefined") return null;
  let storage: AttributionEnv["storage"] = null;
  try {
    storage = window.localStorage;
  } catch {
    storage = null;
  }
  return {
    href: window.location.href,
    referrer: document.referrer,
    storage,
    now: Date.now(),
    replaceUrl: (url) => {
      try {
        // Keep the router's history state so client navigation still works.
        window.history.replaceState(window.history.state, "", url);
      } catch {
        // Some embedded browsers forbid replaceState; the id is harmless in the URL.
      }
    },
    track,
  };
}

/**
 * Read campaign parameters from the landing URL, keep the first touch, strip
 * `sy_click` from the address bar and record a `campaign_landing` event.
 */
export function captureAttribution(env: AttributionEnv | null = defaultEnv()): { landed: boolean; stored: boolean } {
  if (!env) return { landed: false, stored: false };
  let url: URL;
  try {
    url = new URL(env.href);
  } catch {
    return { landed: false, stored: false };
  }
  const params = readCampaignParams(url.search);
  const stripped = stripClickId(env.href);
  if (stripped !== null) env.replaceUrl(stripped);
  if (!params) return { landed: false, stored: false };

  let stored = false;
  if (!loadAttribution(env.storage, env.now) && env.storage) {
    const touch: AttributionInput = {
      ...params,
      referrer: referrerOrigin(env.referrer, url.origin),
      landing_path: url.pathname.slice(0, 512),
      first_seen_at: new Date(env.now).toISOString(),
    };
    try {
      env.storage.setItem(ATTRIBUTION_KEY, JSON.stringify(touch));
      stored = true;
    } catch {
      stored = false;
    }
  }

  const properties: Record<string, string | boolean> = { has_click: Boolean(params.click_id) };
  for (const key of ["utm_source", "utm_medium", "utm_campaign", "utm_content"] as const) {
    if (params[key]) properties[key] = params[key]!;
  }
  env.track("campaign_landing", properties);
  return { landed: true, stored };
}

/** The attribution to send with POST /auth/register (undefined when none). */
export function attributionForSignup(now = Date.now()): AttributionInput | undefined {
  let storage: AttributionEnv["storage"] = null;
  try {
    storage = typeof window !== "undefined" ? window.localStorage : null;
  } catch {
    storage = null;
  }
  const value = loadAttribution(storage, now);
  if (!value) return undefined;
  const out: AttributionInput = {};
  for (const key of [...UTM_KEYS, "click_id", "referrer", "landing_path", "first_seen_at"] as const) {
    const v = value[key];
    if (typeof v === "string" && v) out[key] = v;
  }
  return Object.keys(out).length > 0 ? out : undefined;
}
