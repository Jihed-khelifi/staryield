/** API location. Set NEXT_PUBLIC_API_URL at build time (e.g. https://api.staryield.net). DONE */
export const API_URL = (
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
).replace(/\/+$/, "");

/** WebSocket origin derived from the API URL. */
export const WS_URL = API_URL.replace(/^http/, "ws");

/** True when the bundle runs in a browser. */
export const isBrowser = typeof window !== "undefined";

/** The page language (sent as X-Locale on REST and `locale` on realtime connects). */
export function currentLocale(): string {
  return typeof document !== "undefined" ? document.documentElement.lang || "en" : "en";
}
