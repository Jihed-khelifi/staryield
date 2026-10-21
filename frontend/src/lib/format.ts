import type { Locale } from "@/i18n/config";

/**
 * "07/30, 1:13 AM" in the design, in the viewer's time zone. Chat messages
 * are rendered client-side from server timestamps, so no SSR mismatch.
 */
export function formatMessageTime(iso: string, locale: Locale) {
  return new Intl.DateTimeFormat(locale, {
    month: "2-digit",
    day: "2-digit",
    hour: "numeric",
    minute: "2-digit",
  }).format(new Date(iso));
}

/** Session clock, "00:32:53". */
export function formatDuration(totalSeconds: number) {
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  return [hours, minutes, seconds]
    .map((part) => String(part).padStart(2, "0"))
    .join(":");
}

export function formatRating(rating: number, locale: Locale) {
  return new Intl.NumberFormat(locale, {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  }).format(rating);
}

export function formatCalendarDate(iso: string, locale: Locale) {
  return new Intl.DateTimeFormat(locale, {
    month: "long",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "UTC",
  }).format(new Date(iso));
}

/** Flag emoji for an ISO 3166-1 alpha-2 code ("FR" → 🇫🇷), "" when invalid. */
export function countryFlag(code: string | undefined | null): string {
  const c = (code ?? "").trim().toUpperCase();
  if (!/^[A-Z]{2}$/.test(c)) return "";
  return String.fromCodePoint(...[...c].map((ch) => 0x1f1e6 + ch.charCodeAt(0) - 65));
}

/** Localized country name ("FR" → "France" / "Francia"), falling back to the code. */
export function regionName(code: string | undefined | null, locale: string): string {
  const c = (code ?? "").trim().toUpperCase();
  if (!/^[A-Z]{2}$/.test(c)) return "";
  try {
    return new Intl.DisplayNames([locale], { type: "region" }).of(c) ?? c;
  } catch {
    return c;
  }
}

/** Localized language name ("es" → "Spanish" / "espagnol"). */
export function languageName(code: string | undefined | null, locale: string): string {
  const c = (code ?? "").trim();
  if (!c) return "";
  try {
    return new Intl.DisplayNames([locale], { type: "language" }).of(c) ?? c;
  } catch {
    return c;
  }
}

/** "Oct 20, 2026" in the viewer's language. */
export function formatDate(iso: string | undefined | null, locale: string, style: "medium" | "long" = "medium"): string {
  if (!iso) return "";
  const d = /^\d{4}-\d{2}-\d{2}$/.test(iso) ? new Date(`${iso}T12:00:00Z`) : new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return new Intl.DateTimeFormat(locale, { dateStyle: style, ...(/^\d{4}-\d{2}-\d{2}$/.test(iso) ? { timeZone: "UTC" } : {}) }).format(d);
}
