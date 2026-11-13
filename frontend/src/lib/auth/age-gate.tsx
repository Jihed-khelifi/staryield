"use client";

/**
 * The 18+ age gate on the web app (revisions 5.0, docs §24.6). There is no age
 * pop-up: the age comes from the date of birth given at sign-up (or the birth
 * step after Google sign-up) and the API says where the account stands in
 * `user.age_status`:
 *
 * - `adult`: nothing changes.
 * - `birth_date_required`: gated pages and actions send the person to
 *   `/signup/birth?next=<where they were>`.
 * - `restricted`: gated pages and actions show the calm "Account restricted"
 *   screen (`/age-restricted`).
 *
 * Three pieces, one rule: `AgeGuard` wraps a gated page, `useRequireAdult()`
 * guards a gated action before it calls the API, and `AgeGateEffects` (mounted
 * once in the locale layout) reacts to a 403 `birth_date_required` /
 * `age_restricted` from any API call or realtime frame. Public pages never
 * use them, so they stay open to everyone.
 */
import * as React from "react";
import { usePathname, useRouter } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { onAgeBlocked, reportAgeBlock, type AgeBlock } from "@/lib/api/age-events";
import type { AgeStatus } from "@/lib/api/types";
import { ageStatusOf, birthStepHref, isBirthStepPath, restrictedHref } from "@/lib/auth/age-status";
import { useSession } from "@/lib/auth/session-provider";

function currentPath(fallback: string): string {
  return typeof window === "undefined" ? fallback : window.location.pathname + window.location.search;
}

export type AgeGuardState = "loading" | "anonymous" | AgeStatus;

/**
 * The age state of the session. `allowed` is true for anonymous visitors (the
 * page asks them to sign in itself) and for adults; for the other states the
 * hook has already started the redirect.
 */
export function useAgeGuard(): { state: AgeGuardState; allowed: boolean } {
  const { locale } = useI18n();
  const router = useRouter();
  const pathname = usePathname();
  const { status, user } = useSession();
  const state: AgeGuardState = status === "authenticated" ? ageStatusOf(user) : status;
  React.useEffect(() => {
    if (state === "birth_date_required") router.replace(birthStepHref(locale, currentPath(pathname)));
    else if (state === "restricted") router.replace(restrictedHref(locale));
  }, [state, locale, router, pathname]);
  return { state, allowed: state === "adult" || state === "anonymous" };
}

/**
 * Wrap the content of a gated page. While the session loads, and while a
 * redirect is on its way, nothing of the page renders (so it makes no gated
 * call that would be refused).
 */
export function AgeGuard({ children }: { children: React.ReactNode }) {
  const { allowed } = useAgeGuard();
  return allowed ? <>{children}</> : null;
}

/**
 * Guard a gated action (start a chat, join a waiting list, pay, add a card...)
 * before it calls the API. Returns a check: true when the account may go on;
 * otherwise it has reported the block (the redirect is `AgeGateEffects`') and
 * the action must stop. The API enforces the same rule, so this only saves a
 * refused request.
 */
export function useRequireAdult(): () => boolean {
  const { status, user } = useSession();
  return React.useCallback(() => {
    if (status !== "authenticated") return true; // signing in comes first; the page handles it
    const age = ageStatusOf(user);
    if (age === "adult") return true;
    reportAgeBlock(age);
    return false;
  }, [status, user]);
}

/** Ignore a second block this soon after the first (parallel calls fail together). */
const BLOCK_DEBOUNCE_MS = 1500;

/**
 * Mounted once under the session provider: a 403 `birth_date_required` or
 * `age_restricted` from any call becomes the redirect to the birth step or to
 * the restricted screen, and the session user is re-read so every guard agrees.
 * It never redirects from the page it would send the person to (no loops).
 */
export function AgeGateEffects(): null {
  const { locale } = useI18n();
  const router = useRouter();
  const pathname = usePathname();
  const { reload } = useSession();
  const last = React.useRef(0);
  const where = React.useRef(pathname);
  React.useEffect(() => {
    where.current = pathname;
  }, [pathname]);
  React.useEffect(
    () =>
      onAgeBlocked((block: AgeBlock) => {
        const now = Date.now();
        if (now - last.current < BLOCK_DEBOUNCE_MS) return;
        const here = where.current;
        if (block === "birth_date_required") {
          if (isBirthStepPath(here, locale)) return;
          last.current = now;
          void reload();
          router.replace(birthStepHref(locale, currentPath(here)));
        } else {
          if (here === restrictedHref(locale)) return;
          last.current = now;
          void reload();
          router.replace(restrictedHref(locale));
        }
      }),
    [locale, router, reload],
  );
  return null;
}
