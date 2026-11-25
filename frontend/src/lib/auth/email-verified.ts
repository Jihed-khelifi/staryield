import type { User } from "@/lib/api/types";

/** Realtime event: the address was verified (the link was opened in any browser). Data: `{ email_verified: true, email_verified_at }`. */
export const EMAIL_VERIFIED_EVENT = "account.email_verified";

export type EmailVerifiedEvent = { email_verified?: boolean; email_verified_at?: string };

/** The user with the address marked verified (the same object when it already is, so no re-render). */
export function withEmailVerified(user: User | null): User | null {
  if (!user || user.email_verified === true) return user;
  return { ...user, email_verified: true };
}

/**
 * Whether the event should show a toast: only the first time it changes this
 * user, and not on the verify page itself (which shows the result).
 */
export function shouldAnnounceVerified(data: EmailVerifiedEvent | undefined, user: User | null, pathname: string | null): boolean {
  if (data?.email_verified !== true || !user || user.email_verified === true) return false;
  return !(pathname ?? "").includes("/verify-email");
}
