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
  | { type: "table"; head: InlineText[]; rows: InlineText[][] }
  // revisions 5.0: used by the reading-card policy layout (Refund Policy).
  /** A highlighted rule or formula (the refund calculation). */
  | { type: "callout"; text: InlineText }
  /** A call-to-action link rendered as a gold button (`href` follows the same link rules as inline links). */
  | { type: "button"; label: string; href: string };

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

export type LegalLink = { label: string; href: string };

/**
 * revisions 5.0: the "policy" presentation (Figma 1360:2766 / 1360:2771): a
 * left-aligned header with actions, a sticky "On this page" list beside a
 * narrow reading card, related links, and a collapsible list on phones.
 * Documents without it keep the original centred layout.
 */
export type LegalPolicyLayout = {
  /** Header buttons: the first is the primary (gold) action. */
  actions: LegalLink[];
  related: { title: string; links: LegalLink[] };
  breadcrumb: { home: string; current: string };
};

export type LegalDocumentContent = {
  meta: LegalMeta;
  labels: LegalLabels;
  /** Notices shown before the first numbered section. */
  intro: LegalBlock[];
  sections: LegalSection[];
  /** revisions 5.0: set for documents shown with the policy layout. */
  policy?: LegalPolicyLayout;
};

// revisions 5.0: "refund" added.
export type LegalDocumentId = "privacy" | "terms" | "refund";
