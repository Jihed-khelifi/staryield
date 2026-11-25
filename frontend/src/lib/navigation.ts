/**
 * Validate a post-login redirect target from `?next=`. Only same-origin,
 * locale-prefixed paths are allowed, which prevents open redirects
 * (e.g. `next=//evil.example` or `next=https://evil.example`).
 */
export function safeNext(search: string, locale: string): string | null {
  const next = new URLSearchParams(search).get("next");
  if (!next) return null;
  if (!next.startsWith(`/${locale}/`) && next !== `/${locale}`) return null;
  if (next.startsWith("//") || next.includes("\\") || /[\u0000-\u001f]/.test(next)) return null;
  try {
    const url = new URL(next, "https://staryield.invalid");
    if (url.origin !== "https://staryield.invalid") return null;
    return url.pathname + url.search + url.hash;
  } catch {
    return null;
  }
}

/** Login URL that returns to the current page afterwards. */
export function loginUrl(locale: string): string {
  const here = typeof window !== "undefined" ? window.location.pathname + window.location.search : `/${locale}`;
  return `/${locale}/login?next=${encodeURIComponent(here)}`;
}

/** `path` with `?next=<next>` when there is somewhere to continue to afterwards. */
export function withNext(path: string, next?: string | null): string {
  return next ? `${path}?next=${encodeURIComponent(next)}` : path;
}

/**
 * Sign-up URL that continues to `next` (a locale-prefixed path) once the
 * account exists. The sign-up steps keep `?next=` and the birth step, which
 * ends sign-up, honours it (see `safeNext`).
 */
export function signupUrl(locale: string, next?: string | null): string {
  return withNext(`/${locale}/signup`, next);
}

/** A sign-up step path that keeps the `?next=` of the current address (only when it is safe). */
export function keepNext(path: string, search: string, locale: string): string {
  return withNext(path, safeNext(search, locale));
}
