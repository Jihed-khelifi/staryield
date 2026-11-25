/**
 * Search metadata of the site's existing indexable pages (the `[...screen]`
 * routes and the home page): keyword-aware titles and descriptions, written
 * as English source copy and translated with `text()` (every string is in
 * `copy-{en,fr,es}.json`; `pages.test.ts` checks it), plus the sitemap
 * fields. The programmatic pages describe themselves (src/content/seo).
 */
import { legalDocuments } from "@/content/legal";
import type { MetadataRoute } from "next";

type ChangeFrequency = NonNullable<MetadataRoute.Sitemap[number]["changeFrequency"]>;

export type PageSeo = {
  title: string;
  description: string;
  /** Last content change (ISO date): a build-time constant, never the request time. */
  updated: string;
  changeFrequency: ChangeFrequency;
  priority: number;
};

/** When the marketing pages and calculator articles last changed. */
export const SITE_CONTENT_UPDATED = "2026-11-25";

export const HOME_SEO = {
  title: "Staryield — Free Astrology Calculators & Private Psychic Readings",
  description:
    "Free natal chart, Life Path and tarot birth card calculators, plus private live chats with psychic advisors. For entertainment purposes only.",
  updated: SITE_CONTENT_UPDATED,
} as const;

/** The image alt text of the default link preview (tarot cards on a reading table). */
export const DEFAULT_IMAGE_ALT = "Tarot cards spread on a reading table";

/** Calculator routes ("calculators/<kind>"), in the order of the site navigation. */
export const CALCULATOR_ROUTES = [
  "calculators/natal-chart",
  "calculators/path-of-life",
  "calculators/astrology",
  // Added by the tarot birth card calculator (another growth stream); indexed with the others.
  "calculators/tarot-birth-card",
] as const;

/** Locale-less route (no leading slash) → its search metadata. */
export const PAGE_SEO: Record<string, PageSeo> = {
  "calculators/natal-chart": {
    title: "Free Natal Chart Calculator",
    description:
      "Enter your birth date, time and place to calculate your natal chart: planets, houses and a twelve-card tarot spread. Free, with no sign-up.",
    updated: SITE_CONTENT_UPDATED,
    changeFrequency: "monthly",
    priority: 0.9,
  },
  "calculators/path-of-life": {
    title: "Life Path Number Calculator",
    description:
      "Calculate your Life Path number from your birth date, with every step shown, plus your personal year and month. Free and private: it runs in your browser.",
    updated: SITE_CONTENT_UPDATED,
    changeFrequency: "monthly",
    priority: 0.9,
  },
  "calculators/astrology": {
    title: "Zodiac Sign Calculator",
    description:
      "Find your sun sign from your birth date. Born on a cusp? Add your birth time and place for an exact answer. Free, with no sign-up.",
    updated: SITE_CONTENT_UPDATED,
    changeFrequency: "monthly",
    priority: 0.9,
  },
  "calculators/tarot-birth-card": {
    title: "Tarot Birth Card Calculator",
    description:
      "Find your tarot birth cards from your birth date: your personality and soul cards in the Major Arcana, with every step of the arithmetic. Free.",
    updated: SITE_CONTENT_UPDATED,
    changeFrequency: "monthly",
    priority: 0.9,
  },
  about: {
    title: "About Staryield",
    description: "Who we are, how Staryield works and the values behind our psychic readings and free astrology tools.",
    updated: SITE_CONTENT_UPDATED,
    changeFrequency: "yearly",
    priority: 0.5,
  },
  reviews: {
    title: "Staryield Reviews",
    description: "What customers say about their readings and advisors on Staryield.",
    updated: SITE_CONTENT_UPDATED,
    changeFrequency: "monthly",
    priority: 0.5,
  },
  standards: {
    title: "Advisor Integrity & Interaction Standards",
    description:
      "The standards every Staryield advisor follows: honesty, privacy, respectful conduct and readings offered for entertainment only.",
    updated: SITE_CONTENT_UPDATED,
    changeFrequency: "yearly",
    priority: 0.4,
  },
  faq: {
    title: "Frequently Asked Questions",
    description: "Answers about readings, credits, privacy and how Staryield works.",
    updated: SITE_CONTENT_UPDATED,
    changeFrequency: "monthly",
    priority: 0.5,
  },
  pricing: {
    title: "Pricing",
    description:
      "Simple, transparent pricing: start with free minutes, then choose a one-time credit pack. No subscriptions and no automatic top-ups.",
    updated: SITE_CONTENT_UPDATED,
    changeFrequency: "monthly",
    priority: 0.6,
  },
  contact: {
    title: "Contact Us",
    description: "Reach the Staryield support team by e-mail or with the contact form. Support is available 24/7.",
    updated: SITE_CONTENT_UPDATED,
    changeFrequency: "yearly",
    priority: 0.4,
  },
  disclaimer: {
    title: "Disclaimer",
    description: "Readings and calculators on Staryield are offered for entertainment purposes only. Read the full disclaimer.",
    updated: SITE_CONTENT_UPDATED,
    changeFrequency: "yearly",
    priority: 0.3,
  },
  terms: {
    title: "Terms of Use",
    description: "The terms that apply when you use Staryield, its calculators and its live readings.",
    updated: legalDocuments.terms.en.meta.lastUpdated,
    changeFrequency: "yearly",
    priority: 0.3,
  },
  privacy: {
    title: "Privacy Policy",
    description: "How Staryield collects, uses and protects your personal data, and the rights you have over it.",
    updated: legalDocuments.privacy.en.meta.lastUpdated,
    changeFrequency: "yearly",
    priority: 0.3,
  },
  "refund-policy": {
    title: "Refund Policy",
    description: "How refunds work for Staryield credit packs, promotional credits and readings, and how to request one.",
    updated: legalDocuments.refund.en.meta.lastUpdated,
    changeFrequency: "yearly",
    priority: 0.3,
  },
};

/** An advisor profile's description ({value0} is the advisor's name). */
export const ADVISOR_DESCRIPTION =
  "Read {value0}’s profile and reviews, and chat privately with this advisor on Staryield. Readings are for entertainment purposes only.";

/**
 * Sign-in and sign-up steps: no search value and personal by nature, so they
 * are not indexed (their links may still be followed).
 */
export const AUTH_ROUTES = new Set([
  "signup",
  "signup/password",
  "signup/birth",
  "signup/offer",
  "login",
  "forgot-password",
  "reset-password",
]);
