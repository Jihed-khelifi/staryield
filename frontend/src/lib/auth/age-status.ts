/**
 * Pure helpers of the 18+ age gate (revisions 5.0, docs §24.6): where each
 * status sends the person. The React side (guards, effects) is age-gate.tsx.
 */
import type { AgeStatus, User } from "@/lib/api/types";

/** The status of a user; accounts of an API without the gate count as adults. */
export function ageStatusOf(user: Pick<User, "age_status"> | null | undefined): AgeStatus {
  return user?.age_status ?? "adult";
}

/** The birth step, returning to `next` (a locale-prefixed path) afterwards. */
export function birthStepHref(locale: string, next?: string | null): string {
  const base = `/${locale}/signup/birth`;
  return next ? `${base}?next=${encodeURIComponent(next)}` : base;
}

/** The "Account restricted" screen. */
export function restrictedHref(locale: string): string {
  return `/${locale}/age-restricted`;
}

/**
 * Where a person who has just signed in should go: `target` for an adult, the
 * birth step (then `target`) without a date of birth, the restricted screen
 * for a restricted account.
 */
export function ageLanding(user: Pick<User, "age_status">, locale: string, target: string): string {
  switch (ageStatusOf(user)) {
    case "birth_date_required":
      return birthStepHref(locale, target);
    case "restricted":
      return restrictedHref(locale);
    default:
      return target;
  }
}

/** True on the birth step (any query), the one page that must not redirect to itself. */
export function isBirthStepPath(pathname: string, locale: string): boolean {
  return pathname === `/${locale}/signup/birth`;
}
