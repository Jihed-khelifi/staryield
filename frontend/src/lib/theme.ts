/**
 * App-wide colour theme. The API stores the display name ("Warm Beige",
 * "Light Cream"); the browser keeps a slug in the `sy_theme` cookie so the
 * server renders `<html data-theme>` in the first response — no flash of the
 * wrong colours. The cookie is a display preference only (listed as
 * essential in the Privacy Notice), readable by scripts on purpose.
 */

export const THEME_COOKIE = "sy_theme";
export const THEMES = ["Warm Beige", "Light Cream"] as const;
export type ThemeLabel = (typeof THEMES)[number];
export type ThemeSlug = "warm-beige" | "light-cream";

export const DEFAULT_THEME: ThemeSlug = "warm-beige";

const SLUGS: Record<ThemeLabel, ThemeSlug> = { "Warm Beige": "warm-beige", "Light Cream": "light-cream" };

/** Slug for an API theme name (unknown names fall back to the default). */
export function themeSlug(label: string | null | undefined): ThemeSlug {
  return label && label in SLUGS ? SLUGS[label as ThemeLabel] : DEFAULT_THEME;
}

/** A cookie value we trust (anything else renders the default theme). */
export function parseThemeCookie(value: string | null | undefined): ThemeSlug {
  return value === "light-cream" ? "light-cream" : DEFAULT_THEME;
}

/** Read the theme cookie from a `document.cookie` style string. */
export function themeFromCookieHeader(cookie: string): ThemeSlug {
  const match = cookie.match(/(?:^|;\s*)sy_theme=([^;]*)/);
  return parseThemeCookie(match ? decodeURIComponent(match[1]) : null);
}

/** The theme on screen. */
export function currentTheme(): ThemeSlug {
  if (typeof document === "undefined") return DEFAULT_THEME;
  return parseThemeCookie(document.documentElement.dataset.theme);
}

/**
 * Apply a theme now and remember it for the next server render (1 year,
 * Lax, Secure on HTTPS).
 */
export function applyTheme(slug: ThemeSlug) {
  if (typeof document === "undefined") return;
  document.documentElement.dataset.theme = slug;
  const secure = window.location.protocol === "https:" ? "; secure" : "";
  document.cookie = `${THEME_COOKIE}=${slug}; path=/; max-age=31536000; samesite=lax${secure}`;
}
