"use client";

/**
 * Where "Psychics" leads (revisions 3.0): the directory is for signed-in
 * customers only. Visitors who are not signed in go to the home section
 * "Psychic readings built around your journey"; advisor profiles
 * (`/psychics/:slug`) stay public.
 */
import { useI18n } from "@/i18n/i18n-provider";
import { useSession } from "@/lib/auth/session-provider";

/** The id of the home section that replaces the directory for visitors. */
export const PSYCHICS_SECTION_ID = "psychics";

/** Locale-less path of the directory entry point. */
export function psychicsPath(signedIn: boolean): string {
  return signedIn ? "/psychics" : `#${PSYCHICS_SECTION_ID}`;
}

/** Full href (with the locale) of the directory entry point. */
export function psychicsHref(locale: string, signedIn: boolean): string {
  return `/${locale}${psychicsPath(signedIn)}`;
}

/** The href for the current session: the directory when signed in, the home section otherwise. */
export function usePsychicsHref(): string {
  const { locale } = useI18n();
  const { status } = useSession();
  return psychicsHref(locale, status === "authenticated");
}

/** Locale-less variant for components that prefix the locale themselves. */
export function usePsychicsPath(): string {
  const { status } = useSession();
  return psychicsPath(status === "authenticated");
}
