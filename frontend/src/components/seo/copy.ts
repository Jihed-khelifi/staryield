/**
 * Interface copy of the search pages per family (English source, translated
 * with `text()`; `lib/seo/seo.test.ts` checks every string is in copy-{en,fr,es}).
 * The long-form content lives in src/content/seo.
 */
export type SeoFamily = "life-path" | "tarot" | "compatibility";

export type FamilyCopy = {
  /** The advisor call to action. */
  cta: { heading: string; body: string };
  /** The "for entertainment" note, in the site's wording. */
  note: string;
  /** The calculator section. */
  calculator: { heading: string; intro: string };
};

export const FAMILY_COPY: Record<SeoFamily, FamilyCopy> = {
  "life-path": {
    cta: {
      heading: "Want to explore what your Life Path means for you?",
      body: "Chat privately with one of our psychic advisors about your numbers and the questions on your mind. Readings are offered for entertainment purposes only.",
    },
    note: "Numerology on Staryield is offered for entertainment purposes only. It describes a tradition, not a prediction, and is no substitute for professional advice.",
    calculator: {
      heading: "Calculate your Life Path number",
      intro: "Enter your birth date: the calculator opens with your Life Path number, every step of the sum, and your personal year and month.",
    },
  },
  tarot: {
    cta: {
      heading: "Curious what your birth cards are telling you?",
      body: "Chat privately with one of our psychic advisors about your cards and the themes they raise for you. Readings are offered for entertainment purposes only.",
    },
    note: "Tarot birth cards on Staryield are offered for entertainment purposes only. They describe a tradition, not a prediction, and are no substitute for professional advice.",
    calculator: {
      heading: "Find your tarot birth cards",
      intro: "Enter your birth date: the calculator opens with your personality and soul cards and every step of the arithmetic.",
    },
  },
  compatibility: {
    cta: {
      heading: "Wondering what the stars say about the two of you?",
      body: "Chat privately with one of our psychic advisors about your relationship and the questions on your mind. Readings are offered for entertainment purposes only.",
    },
    note: "Sign compatibility on Staryield is offered for entertainment purposes only. It describes astrological tradition, not a prediction, and is no substitute for professional or relationship advice.",
    calculator: {
      heading: "Find your compatibility",
      intro: "Enter both birth dates: we find each sun sign and open your pairing. Born on a cusp? You choose the sign.",
    },
  },
};
