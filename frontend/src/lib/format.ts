import type { Locale } from "@/i18n/config";

/**
 * "07/30, 1:13 AM" in the design — rendered per locale but pinned to UTC so
 * the server and the client always agree.
 */
export function formatMessageTime(iso: string, locale: Locale) {
  return new Intl.DateTimeFormat(locale, {
    month: "2-digit",
    day: "2-digit",
    hour: "numeric",
    minute: "2-digit",
    timeZone: "UTC",
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
