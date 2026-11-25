/**
 * A visitor's "Notify me when online" (or "Join the waiting list") before
 * they have an account. The book button sends them to sign up (or sign in)
 * and back to the advisor's profile; the remembered intent lets the first
 * book button for that advisor finish the join once they are a client, so
 * the click they made as a visitor is not lost.
 *
 * Kept in sessionStorage: same tab and same origin only (Google's redirect
 * comes back to the same tab), so no other site or link can make a client
 * join a list, and it expires with the tab or after {@link BOOKING_INTENT_TTL_MS}.
 */

const KEY = "staryield.booking.intent";

/** Sign-up, the birth step and a Google round trip fit comfortably in this. */
export const BOOKING_INTENT_TTL_MS = 60 * 60_000;

/** Same shape as an advisor slug (lowercase words and hyphens). */
const SLUG = /^[a-z0-9][a-z0-9-]{0,79}$/;

type Stored = { slug: string; action: "join_waitlist"; at: number };

function storage(): Storage | null {
  try {
    return typeof window === "undefined" ? null : window.sessionStorage;
  } catch {
    return null;
  }
}

/** Remembers that the visitor asked to wait for this advisor. */
export function rememberBookingIntent(slug: string, now: number = Date.now()): void {
  if (!SLUG.test(slug)) return;
  try {
    storage()?.setItem(KEY, JSON.stringify({ slug, action: "join_waitlist", at: now } satisfies Stored));
  } catch {
    // Storage full or blocked: the visitor simply clicks again after sign-up.
  }
}

/**
 * Takes the remembered intent for this advisor: true once (it is removed),
 * when it is fresh. An intent for another advisor is left alone; a stale or
 * malformed one is dropped.
 */
export function takeBookingIntent(slug: string, now: number = Date.now()): boolean {
  const store = storage();
  if (!store) return false;
  let value: Partial<Stored> | null = null;
  try {
    const raw = store.getItem(KEY);
    if (!raw) return false;
    value = JSON.parse(raw) as Partial<Stored>;
  } catch {
    value = null;
  }
  const valid =
    value !== null &&
    typeof value === "object" &&
    value.action === "join_waitlist" &&
    typeof value.slug === "string" &&
    typeof value.at === "number" &&
    value.at <= now &&
    now - value.at <= BOOKING_INTENT_TTL_MS;
  if (valid && value!.slug !== slug) return false;
  try {
    store.removeItem(KEY);
  } catch {
    // Blocked storage: nothing more to do.
  }
  return valid;
}
