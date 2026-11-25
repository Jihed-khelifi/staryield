/**
 * The programmatic search pages (Life Path numbers, tarot birth cards, sign
 * pairings and their hubs) as plain, fully localized data. Each family's
 * `index.ts` composes a page from its language-independent facts
 * (`data.ts`) and its per-locale copy (`en.ts`, `fr.ts`, `es.ts`); the
 * server component <SeoArticle> renders it, and the same object feeds the
 * page metadata and the JSON-LD, so what search engines read is exactly
 * what the page shows.
 *
 * Inline text may use `**bold**` and `[label](href)` (internal paths only),
 * rendered by <InlineText>. Routes are locale-less ("/life-path-number/7").
 */
import type { Crumb, FaqItem } from "@/lib/seo/jsonld";
import type { SeoImage } from "@/lib/seo/site";

export type { Crumb, FaqItem };

/** A link; `short` is the visible text when the context already names the rest (the full `label` stays for assistive tech). */
export type SeoLink = { label: string; href: string; note?: string; short?: string };

export type SeoBlock =
  | { type: "p"; text: string }
  | { type: "list"; items: string[]; ordered?: boolean }
  /** Label / value pairs (number, numeral, attribution, element…). */
  | { type: "facts"; items: { label: string; value: string }[]; columns?: 2 | 3 }
  /** Links shown as chips (compatible numbers, the cards of a constellation). */
  | { type: "links"; items: SeoLink[] }
  /** Love / friendship / work on a 1–5 scale, with the heuristic's explanation. */
  | { type: "indicators"; items: { key: string; label: string; value: number; valueText: string }[]; note: string };

export type SeoSection = { id: string; heading: string; blocks: SeoBlock[] };

/** One tile of a hub grid: artwork (a card, a zodiac glyph) or a big numeral. */
export type HubItem = {
  href: string;
  title: string;
  subtitle: string;
  image?: { src: string; width: number; height: number };
  numeral?: string;
};

/** The compatibility hub: one group per sign with its twelve pairings. */
export type HubGroup = {
  title: string;
  image: { src: string; width: number; height: number };
  links: SeoLink[];
};

export type SeoPage = {
  /** Locale-less route of the page. */
  route: string;
  /** <title> without the site suffix, and the meta description. */
  title: string;
  description: string;
  /** Small label above the H1 ("Numerology · Life Path"). */
  eyebrow: string;
  h1: string;
  /** The direct answer: the first paragraph under the H1. */
  intro: string;
  /** Visible artwork next to the intro (tarot card, zodiac glyphs). */
  artwork?: { src: string; alt: string; width: number; height: number }[];
  /** A large numeral shown instead of artwork (Life Path pages). */
  numeral?: string;
  sections: SeoSection[];
  /** Hub grid (hub pages only). */
  grid?: { heading: string; intro?: string; items?: HubItem[]; groups?: HubGroup[] };
  faq: FaqItem[];
  related: { heading: string; links: SeoLink[] };
  breadcrumbs: Crumb[];
  /** Link preview image and its alt text. */
  image: SeoImage;
  imageAlt: string;
  /** Last content change (ISO date), for the sitemap. */
  updated: string;
};
