/**
 * Structured legal documents (Privacy Policy, Terms of Use).
 *
 * Inline text may contain links written as `[label](href)`. Only internal
 * paths (`/contact`), in-page anchors (`#refunds`) and `mailto:` links are
 * allowed; `npm run test:content` rejects anything else. Internal paths are
 * prefixed with the reader's locale when rendered unless they already start
 * with a locale segment (e.g. `/en/privacy`).
 *
 * Section and heading ids must be identical in every locale so anchors and
 * cross-references keep working when a reader switches language.
 */
export type InlineText = string;

export type LegalHeadingBlock = {
  type: "h3" | "h4";
  id: string;
  /** Clause number, e.g. "2.1" or "6.1.3"; omitted for unnumbered headings. */
  n?: string;
  title: string;
};

export type LegalBlock =
  | { type: "p"; text: InlineText }
  | { type: "ul"; items: InlineText[] }
  | { type: "ol"; items: InlineText[] }
  | LegalHeadingBlock
  | { type: "table"; head: InlineText[]; rows: InlineText[][] };

export type LegalSection = {
  id: string;
  /** Top-level clause number, e.g. "4". */
  n: string;
  title: string;
  blocks: LegalBlock[];
};

export type LegalMeta = {
  /** Bump on every substantive change; must match across locales. */
  version: string;
  /** ISO dates (YYYY-MM-DD). */
  effective: string;
  lastUpdated: string;
  badge: string;
  title: string;
};

export type LegalLabels = {
  toc: string;
  effective: string;
  lastUpdated: string;
  backToTop: string;
};

export type LegalDocumentContent = {
  meta: LegalMeta;
  labels: LegalLabels;
  /** Notices shown before the first numbered section. */
  intro: LegalBlock[];
  sections: LegalSection[];
};

export type LegalDocumentId = "privacy" | "terms";
