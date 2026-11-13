/**
 * revisions 5.1: processors and customers look for the refund policy under a
 * handful of obvious URLs. They all redirect permanently (308) to
 * `/{locale}/refund-policy` (docs/platform-extensions.md §25.6), with or
 * without a locale prefix. Used by the proxy (so a bare path goes straight to
 * the final URL in one hop) and by the screen router (fallback).
 */
export const REFUND_POLICY_ALIASES = ["refunds", "refund", "reimburse", "refunds-policy"] as const;

/** True for `refunds`, `refund`, `reimburse` or `refunds-policy` (any case, optional trailing slash). */
export function isRefundAlias(segment: string): boolean {
  const normalized = segment.replace(/\/+$/, "").toLowerCase();
  return (REFUND_POLICY_ALIASES as readonly string[]).includes(normalized);
}
