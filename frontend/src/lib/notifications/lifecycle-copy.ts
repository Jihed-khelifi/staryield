/**
 * Lifecycle nudges (growth: lifecycle and onboarding): the in-app form of the
 * messages the lifecycle job sends (e-mail and push carry the same sentences,
 * rendered by the API in `backend/internal/mail/lifecycle.go`). They only state
 * what was true when they were sent: the wallet's free minutes and the
 * advisors online at that moment (`free_minutes`, `online_advisors`), never an
 * invented deadline.
 */
import type { TranslationValues } from "@/i18n/types";

type Translate = <T>(value: T, values?: TranslationValues) => T;

export type LifecycleKind = "lifecycle.verify_reminder" | "lifecycle.first_reading" | "lifecycle.invite_friends" | "lifecycle.win_back";

/** English source copy (keys of copy-*.json). */
export const lifecycleTemplates = {
  verifyTitle: "Confirm your email address",
  verifyBody: "We sent you a new confirmation link. Confirm your email address to keep your account secure.",
  freeMinuteTitle: "Your free minute is waiting",
  freeMinuteBody: "You have 1 free minute to use on your first reading.",
  freeMinutesTitle: "Your free minutes are waiting",
  freeMinutesBody: "You have {count} free minutes to use on your first reading.",
  advisorsOnlineTitle: "Advisors are online now",
  firstReadingOnlineBody: "Choose an advisor and start your first reading.",
  firstReadingTitle: "Your first reading is waiting",
  firstReadingBody: "Choose an advisor and start your first reading whenever you are ready.",
  inviteTitle: "Enjoyed your reading?",
  inviteBody: "Invite a friend to Staryield and share the experience.",
  advisorOnlineTitle: "An advisor is online now",
  winBackOneBody: "1 advisor is online now. Pick up where you left off.",
  winBackManyBody: "{count} advisors are online now. Pick up where you left off.",
  horoscopeTitle: "Your daily horoscope is ready",
  horoscopeBody: "Read today's horoscope for your sign.",
} as const;

/** A count from notification data: a non-negative integer, else 0. */
function count(value: unknown): number {
  const n = typeof value === "number" ? value : typeof value === "string" && value.trim() ? Number(value) : NaN;
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : 0;
}

/** Title and body of a lifecycle notification in the viewer's language. */
export function lifecycleText(
  kind: LifecycleKind,
  data: Record<string, unknown>,
  text: Translate,
  fmt: (value: number) => string,
): { title: string; body: string } {
  const L = lifecycleTemplates;
  switch (kind) {
    case "lifecycle.verify_reminder":
      return { title: text(L.verifyTitle), body: text(L.verifyBody) };
    case "lifecycle.first_reading": {
      const free = count(data.free_minutes);
      if (free === 1) return { title: text(L.freeMinuteTitle), body: text(L.freeMinuteBody) };
      if (free > 1) return { title: text(L.freeMinutesTitle), body: text(L.freeMinutesBody, { count: fmt(free) }) };
      if (count(data.online_advisors) > 0) return { title: text(L.advisorsOnlineTitle), body: text(L.firstReadingOnlineBody) };
      return { title: text(L.firstReadingTitle), body: text(L.firstReadingBody) };
    }
    case "lifecycle.invite_friends":
      return { title: text(L.inviteTitle), body: text(L.inviteBody) };
    case "lifecycle.win_back": {
      const online = count(data.online_advisors);
      if (online === 1) return { title: text(L.advisorOnlineTitle), body: text(L.winBackOneBody) };
      if (online > 1) return { title: text(L.advisorsOnlineTitle), body: text(L.winBackManyBody, { count: fmt(online) }) };
      return { title: text(L.horoscopeTitle), body: text(L.horoscopeBody) };
    }
  }
}
