/**
 * The merchant of record as customers, card schemes and payment processors
 * must see it everywhere: website footer, contact page, legal documents,
 * checkout and e-mails (docs/platform-extensions.md §25.4). One source for the
 * web app; the API's e-mails use the same values (backend `internal/mail`),
 * and tests keep both in line with the legal documents.
 *
 * Only facts live here. Sentences around them are copy (`text("…")`) so they
 * are translated: e.g. "Support is available 24/7. We reply within 1–2 days."
 */
export const COMPANY = {
  legalName: "Staryield Media LLC",
  address: {
    street: "30 N Gould St, Ste N",
    city: "Sheridan",
    region: "WY",
    postalCode: "82801",
    country: "United States",
  },
  /** The postal address on one line, as printed in the footer and the legal documents. */
  addressLine: "30 N Gould St, Ste N, Sheridan, WY 82801, United States",
  supportEmail: "support@staryield.net",
  /** What customers see on their card statement (Refund Policy §11; STARYIELD_PAYMENTS_STATEMENT_DESCRIPTOR). */
  statementDescriptor: "STARYIELD MEDIA LLC",
} as const;

/** `mailto:` link to support, optionally with a subject (e.g. "Refund Request"). */
export function supportMailto(subject?: string): string {
  return `mailto:${COMPANY.supportEmail}${subject ? `?subject=${encodeURIComponent(subject)}` : ""}`;
}
