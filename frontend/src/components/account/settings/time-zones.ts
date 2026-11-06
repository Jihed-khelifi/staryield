/**
 * IANA time zones for the Appearance → Time Zone picker, labelled like
 * "Los Angeles — Pacific Time (UTC−08:00)" in the page language.
 */

export type ZoneOption = { id: string; label: string };

const FALLBACK = ["UTC", "Europe/London", "Europe/Paris", "Europe/Madrid", "America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles", "America/Mexico_City", "America/Sao_Paulo", "Asia/Dubai", "Asia/Kolkata", "Asia/Tokyo", "Australia/Sydney"];

let zones: string[] | null = null;

/** Every zone the browser knows (UTC included). */
export function allTimeZones(): string[] {
  if (zones) return zones;
  let list: string[];
  try {
    list = Intl.supportedValuesOf("timeZone");
  } catch {
    list = FALLBACK;
  }
  zones = list.includes("UTC") ? list : ["UTC", ...list];
  return zones;
}

function part(zone: string, locale: string, timeZoneName: Intl.DateTimeFormatOptions["timeZoneName"], now: Date): string {
  try {
    return new Intl.DateTimeFormat(locale, { timeZone: zone, timeZoneName }).formatToParts(now).find((p) => p.type === "timeZoneName")?.value ?? "";
  } catch {
    return "";
  }
}

const labels = new Map<string, string>();

/** "Los Angeles — Pacific Time (UTC−08:00)"; unknown zones return the id. */
export function zoneLabel(zone: string, locale: string, now = new Date()): string {
  if (!zone) return "";
  const key = `${locale}|${zone}`;
  const cached = labels.get(key);
  if (cached) return cached;
  const city = (zone.split("/").pop() ?? zone).replace(/_/g, " ");
  const offset = part(zone, "en", "longOffset", now);
  if (!offset) return zone;
  const utc = offset === "GMT" ? "UTC" : offset.replace("GMT", "UTC").replace("-", "−");
  const generic = part(zone, locale, "longGeneric", now);
  const label = zone === "UTC" ? `UTC (${utc})` : generic && !/^(GMT|UTC)/.test(generic) ? `${city} — ${generic} (${utc})` : `${city} (${utc})`;
  labels.set(key, label);
  return label;
}

function fold(value: string): string {
  return value
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase();
}

/** Zones whose id or label contains every word of the query (max 40). */
export function searchTimeZones(query: string, locale: string, now = new Date()): ZoneOption[] {
  const words = fold(query).split(/[\s/,_-]+/).filter(Boolean);
  if (words.length === 0) return [];
  const out: ZoneOption[] = [];
  for (const id of allTimeZones()) {
    const label = zoneLabel(id, locale, now);
    const haystack = fold(`${id} ${label}`);
    if (words.every((word) => haystack.includes(word))) out.push({ id, label });
    if (out.length >= 40) break;
  }
  return out;
}
