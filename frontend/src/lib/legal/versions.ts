/**
 * Versions of the documents a buyer agrees to at checkout. They are sent with
 * every purchase as consent evidence (POST /credits/purchases `consent`,
 * docs/platform-extensions.md §25.2). The legal documents themselves are
 * server-only modules, so the client keeps this small copy; `versions.test.ts`
 * fails as soon as a document's `meta.version` changes without it.
 */
export const LEGAL_VERSIONS = {
  // growth (2026-11-25): e-mail confirmation, free calculators and guides, booking, reminders and the
  // "Get started" checklist, shareable result images, the referral program. Before: revisions 6.0 (2026-11-19).
  // The API refuses consent dates in the future (UTC): ship on or after 2026-11-25 UTC.
  terms: "2026-11-25",
  privacy: "2026-11-25",
  refund: "2026-11-25",
} as const;

export type LegalConsent = { terms: string; privacy: string; refund: string };

/** The consent payload of a purchase: the versions on screen when the buyer ticked the box. */
export function purchaseConsent(): LegalConsent {
  return { ...LEGAL_VERSIONS };
}
