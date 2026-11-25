import type { FaqItem } from "@/content/seo/types";

/** The written part of one tarot birth card page, per locale, keyed by card slug. */
export type TarotEntry = {
  /** Three keywords for the description. */
  keywords: string;
  /** The direct answer shown in the first paragraph. */
  intro: string;
  /** What the card means as a birth card. */
  meaning: string;
  gifts: [string, string, string];
  shadow: [string, string, string];
  /** Something particular to this card (numbering, naming), when there is one. */
  note?: string;
};

export type TarotContent = Record<string, TarotEntry>;

/** The role a card plays in a birth number's chain. */
export type ChainRole = "personality" | "middle" | "soul" | "both";

/** What the constellation sentences need: names are already localized. */
export type ConstellationInput = {
  card: string;
  birthNumber: number;
  /** The chain when this card is the first result ([The Sun, Wheel of Fortune, The Magician]). */
  chain: string[];
  /** Birth numbers (and their first cards) whose chain ends with this card, other than itself. */
  soulOf: { number: number; card: string }[];
  /** The card's family: every card sharing its soul card. */
  family: string[];
  /** True when no birth date from 1900 onward gives this card as the first result (The Magician). */
  soulOnly: boolean;
};

/**
 * The sentences around the entries, per locale: titles, headings, facts,
 * the constellation, the worked example, the FAQ and the hub.
 */
export type TarotFrame = {
  eyebrow: string;
  title: (card: string) => string;
  description: (card: string, keywords: string) => string;
  h1: (card: string) => string;
  headings: {
    meaning: (card: string) => string;
    facts: string;
    gifts: (card: string) => string;
    shadow: (card: string) => string;
    constellation: (card: string) => string;
    example: string;
  };
  labels: {
    arcana: string;
    numeral: string;
    birthNumber: string;
    letter: string;
    attribution: string;
    element: string;
    planet: string;
    sign: string;
    total: string;
    personality: string;
    middle: string;
    soul: string;
    both: string;
  };
  /** One sentence on the card's Golden Dawn correspondence. */
  attribution: (card: string, letter: string, value: string, kind: "element" | "planet" | "sign") => string;
  constellation: (input: ConstellationInput) => string[];
  /** The sentence before the worked example's lines. */
  example: (date: string) => string;
  faq: (input: {
    card: string;
    entry: { meaning: string };
    /** The constellation paragraph(s) joined, and the attribution sentence. */
    constellation: string;
    attribution: string;
    date: string;
    birthNumber: number;
    soulOnly: boolean;
    /** "10 and 19": the totals whose soul card this is (empty when none). */
    soulOf: string;
  }) => FaqItem[];
  related: string;
  hubLink: string;
  calculatorLink: string;
  hub: {
    title: string;
    description: string;
    h1: string;
    intro: string;
    gridHeading: string;
    sections: { id: string; heading: string; paragraphs: string[] }[];
    faq: FaqItem[];
    tile: (birthNumber: number, numeral: string) => string;
  };
  breadcrumb: { hub: string };
  /** Alt text of a card's artwork. */
  cardAlt: (card: string) => string;
};
