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

export type Placement = { body: string; sign: string; glyph: string; retrograde?: boolean };

/**
 * The client's placements: the full natal list when the API sends one
 * (with the rising sign after the Sun), otherwise the Sun, rising and Moon
 * signs from the profile summary.
 */
export function placementsFor(profile: Pick<ClientProfile, "astro"> | null | undefined): Placement[] {
  if (!profile) return [];
  const glyph = (body: string) => planetSymbol[body] ?? "✦";
  const astro = profile.astro;
  if (astro?.placements?.length) {
    const out: Placement[] = astro.placements
      .filter((p) => p.body && p.sign)
      .map((p) => ({ body: p.body, sign: p.sign, glyph: glyph(p.body), retrograde: p.retrograde }));
    if (astro.rising_sign) {
      const at = out.findIndex((p) => p.body === "Sun") + 1;
      out.splice(at, 0, { body: "Ascendant", sign: astro.rising_sign, glyph: glyph("Ascendant") });
    }
    return out;
  }
  if (!astro?.sun_sign) return [];
  const out: Placement[] = [{ body: "Sun", sign: astro.sun_sign, glyph: glyph("Sun") }];
  if (astro.rising_sign) out.push({ body: "Ascendant", sign: astro.rising_sign, glyph: glyph("Ascendant") });
  if (astro.moon_sign) out.push({ body: "Moon", sign: astro.moon_sign, glyph: glyph("Moon") });
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
