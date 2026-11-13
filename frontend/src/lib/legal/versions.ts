/**
 * Versions of the documents a buyer agrees to at checkout. They are sent with
 * every purchase as consent evidence (POST /credits/purchases `consent`,
 * docs/platform-extensions.md §25.2). The legal documents themselves are
 * server-only modules, so the client keeps this small copy; `versions.test.ts`
 * fails as soon as a document's `meta.version` changes without it.
 */
export const LEGAL_VERSIONS = {
  // revisions 5.1: memberships removed from the Terms and Privacy Policy, consent record, withdrawal paragraph (2026-11-13).
  terms: "2026-11-13",
  privacy: "2026-11-13",
  refund: "2026-11-13",
} as const;

export type LegalConsent = { terms: string; privacy: string; refund: string };

/** The consent payload of a purchase: the versions on screen when the buyer ticked the box. */
export function purchaseConsent(): LegalConsent {
  return { ...LEGAL_VERSIONS };
}
