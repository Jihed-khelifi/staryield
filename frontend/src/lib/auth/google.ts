/**
 * Google sign-in, browser side (docs §23.1): start an authorization-code
 * flow and navigate to Google. No Google script, cookie or request exists
 * on a Staryield page until the visitor presses the button.
 */
import { authApi } from "@/lib/api/endpoints";
import { ApiError, toApiError } from "@/lib/api/errors";
import type { GoogleIntent, GoogleStartInput } from "@/lib/api/types";
import { attributionForSignup } from "@/lib/attribution";
import { safeNext } from "@/lib/navigation";

/** Wrapper around the one real navigation, so tests can observe it. */
export const browser = {
  assign: (url: string) => window.location.assign(url),
};

/**
 * The URL to send the browser to, or null when it is not safe: only `https:`
 * (and `http://localhost` / `http://127.0.0.1` outside production, for a local
 * test issuer), never with credentials in the URL.
 */
export function authorizationTarget(raw: unknown, development: boolean = process.env.NODE_ENV !== "production"): string | null {
  if (typeof raw !== "string" || !raw) return null;
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    return null;
  }
  if (url.username || url.password) return null;
  if (url.protocol === "https:") return url.toString();
  if (development && url.protocol === "http:" && (url.hostname === "localhost" || url.hostname === "127.0.0.1")) return url.toString();
  return null;
}

/** `next` for the API: a same-site path without the locale prefix (§23.1). */
export function nextForApi(search: string, locale: string): string | undefined {
  const next = safeNext(search, locale);
  if (!next) return undefined;
  return next === `/${locale}` ? "/" : next.slice(locale.length + 1);
}

/**
 * A `next` coming back from the API as a locale-prefixed path for the router
 * (accepts both locale-less and already-prefixed paths; null when unsafe).
 */
export function localizedNext(next: string | null | undefined, locale: string): string | null {
  if (!next) return null;
  const prefixed =
    next === `/${locale}` || next.startsWith(`/${locale}/`) ? next : next.startsWith("/") && !next.startsWith("//") ? `/${locale}${next === "/" ? "" : next}` : null;
  if (!prefixed) return null;
  return safeNext(`next=${encodeURIComponent(prefixed)}`, locale);
}

const INTENT_KEY = "staryield.google.intent";
const INTENT_TTL_MS = 15 * 60_000;

type RememberedIntent = { intent: GoogleIntent; next?: string; at: number };

/** Remember what the visitor was doing, so "Try again" on the return screen restarts it. */
export function rememberGoogleIntent(intent: GoogleIntent, next?: string) {
  try {
    sessionStorage.setItem(INTENT_KEY, JSON.stringify({ intent, next, at: Date.now() } satisfies RememberedIntent));
  } catch {
    // Storage blocked: "Try again" falls back to signing in.
  }
}

export function recallGoogleIntent(now = Date.now()): { intent: GoogleIntent; next?: string } | null {
  try {
    const raw = sessionStorage.getItem(INTENT_KEY);
    if (!raw) return null;
    const value = JSON.parse(raw) as RememberedIntent;
    if (!["signin", "signup", "link"].includes(value.intent) || now - value.at > INTENT_TTL_MS) return null;
    return { intent: value.intent, next: typeof value.next === "string" ? value.next : undefined };
  } catch {
    return null;
  }
}

export function forgetGoogleIntent() {
  try {
    sessionStorage.removeItem(INTENT_KEY);
  } catch {
    // Nothing to clean up.
  }
}

/** Attribution is best effort: an API that rejects it must never block the sign-in. */
function attributionRejected(error: ApiError): boolean {
  return error.code === "validation_failed" && Object.keys(error.fields).some((f) => f.startsWith("attribution"));
}

/** Run `call` with attribution, and once more without it when the API rejects the attribution. */
export async function withAttributionFallback<T>(call: (attribution: ReturnType<typeof attributionForSignup>) => Promise<T>): Promise<T> {
  const attribution = attributionForSignup();
  try {
    return await call(attribution);
  } catch (err) {
    if (!attribution || !attributionRejected(toApiError(err))) throw err;
    return call(undefined);
  }
}

/**
 * Ask the API for Google's authorization URL and navigate to it. Throws an
 * ApiError (e.g. `google_unavailable`) when anything goes wrong, including a
 * URL that is not safe to navigate to.
 */
export async function startGoogle(input: { intent: GoogleIntent; next?: string; locale: string }): Promise<void> {
  const { intent, next, locale } = input;
  const res = await withAttributionFallback((attribution) => {
    const body: GoogleStartInput = {
      intent,
      accept_terms: intent === "signup" ? true : undefined,
      next,
      locale,
      // Linking from settings has no campaign to attribute.
      attribution: intent === "link" ? undefined : attribution,
    };
    return authApi.googleStart(body);
  });
  const target = authorizationTarget(res?.authorization_url);
  if (!target) throw new ApiError({ code: "google_unavailable", message: "Refused an unsafe Google authorization URL", status: 0 });
  rememberGoogleIntent(intent, next);
  browser.assign(target);
}
