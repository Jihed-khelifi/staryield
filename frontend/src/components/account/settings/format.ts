/** Locale-aware formatting for the settings page (dates, times, money, sizes). */

/** A valid IANA zone or undefined (falls back to the device zone). */
function zoneOrDevice(timeZone?: string): string | undefined {
  if (!timeZone) return undefined;
  try {
    new Intl.DateTimeFormat("en", { timeZone });
    return timeZone;
  } catch {
    return undefined;
  }
}

/** "Nov 5, 2026" in the page language and the chosen time zone. */
export function formatDay(iso: string | undefined, locale: string, timeZone?: string): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeZone: zoneOrDevice(timeZone) }).format(date);
}

/** "Nov 5, 2026, 14:03" (local wall time in the chosen zone). */
export function formatDateTime(iso: string | undefined, locale: string, timeZone?: string): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short", timeZone: zoneOrDevice(timeZone) }).format(date);
}

/** "2 hours ago", "yesterday"… (null within the last two minutes: "active now"). */
export function relativeTime(iso: string, locale: string, now = Date.now()): string | null {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return null;
  const seconds = Math.round((then - now) / 1000);
  if (Math.abs(seconds) < 120) return null;
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
  const steps: [Intl.RelativeTimeFormatUnit, number][] = [
    ["minute", 60],
    ["hour", 3600],
    ["day", 86_400],
    ["week", 604_800],
    ["month", 2_592_000],
    ["year", 31_536_000],
  ];
  let unit: Intl.RelativeTimeFormatUnit = "minute";
  let size = 60;
  for (const [u, s] of steps) {
    if (Math.abs(seconds) >= s) {
      unit = u;
      size = s;
    }
  }
  return rtf.format(Math.round(seconds / size), unit);
}

/** "$9.99" in the page language. */
export function formatMoney(cents: number, currency: string, locale: string): string {
  try {
    return new Intl.NumberFormat(locale, { style: "currency", currency }).format(cents / 100);
  } catch {
    return `${(cents / 100).toFixed(2)} ${currency}`;
  }
}

/** "1.2 MB". */
export function formatBytes(bytes: number, locale: string): string {
  const units = ["byte", "kilobyte", "megabyte", "gigabyte"] as const;
  let value = bytes;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return new Intl.NumberFormat(locale, { style: "unit", unit: units[i], unitDisplay: "short", maximumFractionDigits: i === 0 ? 0 : 1 }).format(value);
}
