/**
 * Growth: e-mail verification. Copy of the `account.email_verified`
 * notification, and the kinds whose arrival is already announced somewhere
 * else (the realtime event shows its own toast in every tab), so the
 * notifications provider records them without a second toast.
 */
export const emailVerifiedCopy = {
  title: "Email verified",
  body: "Your email address is confirmed. Thank you for securing your account.",
} as const;

const ANNOUNCED_ELSEWHERE: ReadonlySet<string> = new Set(["account.email_verified"]);

export function isAnnouncedElsewhere(kind: string): boolean {
  return ANNOUNCED_ELSEWHERE.has(kind);
}
