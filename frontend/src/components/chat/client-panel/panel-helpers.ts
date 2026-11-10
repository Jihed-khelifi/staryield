/**
 * Pure helpers for the advisor's client panel (unit-tested).
 */
import { errorMessage, toApiError } from "@/lib/api/errors";
import type { ClientProfile } from "@/lib/api/types";
import { relativeTime } from "@/lib/notifications/notification-copy";
import { planetSymbol } from "@/components/account/chart-copy";

/** Placements shown before "+ expand" (Figma 1172:1191). */
export const PLACEMENTS_PREVIEW = 3;
/** Saved messages shown before "+ expand" (Figma 1174:1858). */
export const SAVED_PREVIEW = 2;
/** Notes per page, in the panel and in "Notes from other psychics". */
export const NOTES_PAGE_SIZE = 5;
/** Same limit as the API (domain.NoteMaxChars, counted in characters). */
export const NOTE_MAX_CHARS = 2000;

export type Placement = {
  body: string;
  sign: string;
  glyph: string;
  retrograde?: boolean;
  /** The slot exists but the sign is not known (no birth time): shown as "Unknown", `sign` is empty. */
  unknown?: boolean;
};

/**
 * The slots of the overview, three per row (§23.5). Bodies the API sends that
 * are not listed here (Chiron, the Midheaven…) are not shown.
 */
export const PLACEMENT_SLOTS = ["Sun", "Moon", "Ascendant", "Mercury", "Venus", "Mars", "Jupiter", "Saturn", "Uranus", "Neptune", "Pluto"] as const;

/** Slots that stay in place as "Unknown" when the chart exists but the sign cannot be told. */
const KEEP_AS_UNKNOWN = new Set<string>(["Moon", "Ascendant"]);

/**
 * The client's placements in fixed order: Sun, Moon, Ascendant / Mercury,
 * Venus, Mars / Jupiter, Saturn, Uranus / Neptune, Pluto. The Ascendant
 * appears once (the API's own entry and `rising_sign` are one slot) and the
 * Midheaven never does. When a chart exists but the Moon or the Ascendant is
 * unknown (no birth time), its slot stays and reads "Unknown", so the rows
 * never shift. Without a natal list the summary signs fill the first slots.
 */
export function placementsFor(profile: Pick<ClientProfile, "astro"> | null | undefined): Placement[] {
  const astro = profile?.astro;
  if (!astro) return [];
  const glyph = (body: string) => planetSymbol[body] ?? "✦";
  const listed = new Map<string, { sign: string; retrograde?: boolean }>();
  for (const p of astro.placements ?? []) {
    if (p.body && p.sign && !listed.has(p.body)) listed.set(p.body, { sign: p.sign, retrograde: p.retrograde });
  }
  const summary: Record<string, string | undefined> = { Sun: astro.sun_sign, Moon: astro.moon_sign, Ascendant: astro.rising_sign };
  const chart = listed.size > 0 || Boolean(astro.sun_sign);
  if (!chart) return [];
  const out: Placement[] = [];
  for (const body of PLACEMENT_SLOTS) {
    const entry = listed.get(body);
    const sign = entry?.sign ?? summary[body];
    if (sign) {
      out.push(entry?.retrograde !== undefined ? { body, sign, glyph: glyph(body), retrograde: entry.retrograde } : { body, sign, glyph: glyph(body) });
    } else if (KEEP_AS_UNKNOWN.has(body)) {
      out.push({ body, sign: "", glyph: glyph(body), unknown: true });
    }
  }
  return out;
}

/** Note-specific copy for API failures (English; translate with `text()`). */
export const noteErrorCopy = {
  limit_reached: "You have reached the note limit for this client. Delete older notes to add new ones.",
  forbidden: "You can keep notes about a client after your first session together.",
} as const;

/** The message to show for a failed note action. */
export function noteErrorMessage(error: unknown): string {
  const e = toApiError(error);
  if (e.code === "limit_reached") return noteErrorCopy.limit_reached;
  if (e.code === "forbidden") return noteErrorCopy.forbidden;
  return errorMessage(e);
}

/**
 * "just now" for the first 45 seconds, else a short relative time
 * ("2 min. ago"), in the viewer's locale.
 */
export function sinceLabel(iso: string, locale: string, now: number = Date.now()): { justNow: boolean; relative: string } {
  const at = Date.parse(iso);
  if (!Number.isFinite(at) || Math.abs(now - at) < 45_000) return { justNow: true, relative: "" };
  return { justNow: false, relative: relativeTime(iso, locale, now) };
}

/** Optimistic notes get a temporary id until the API answers. */
export function isTempNote(id: string): boolean {
  return id.startsWith("temp-");
}
