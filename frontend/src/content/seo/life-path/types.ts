import type { LifePathNumber } from "@/content/seo/life-path/data";
import type { FaqItem } from "@/content/seo/types";

/** The written part of one Life Path page, per locale. Sentences are complete and standalone. */
export type LifePathEntry = {
  /** The archetype: "The Seeker". */
  title: string;
  /** Three keywords for the description: "independence, initiative, courage". */
  keywords: string;
  /** The direct answer shown in the first paragraph. */
  intro: string;
  archetype: string;
  strengths: [string, string, string];
  challenges: [string, string, string];
  love: string;
  work: string;
  purpose: string;
  /** One sentence on why the traditionally compatible numbers fit. */
  compatibleNote: string;
};

export type LifePathContent = Record<LifePathNumber, LifePathEntry>;

/** "June 18, 1981" in the page's language, and the reduction lines of that date. */
export type WorkedExample = {
  date: string;
  /** "Month: 6", "Day: 18 → 1 + 8 = 9", … and the total line. */
  lines: string[];
};

/**
 * The sentences around the entries, per locale: titles, headings, the
 * worked example, the FAQ and the hub. Functions receive facts that are
 * already computed (numbers, formatted dates, names) and only phrase them.
 */
export type LifePathFrame = {
  eyebrow: string;
  title: (n: number, archetype: string) => string;
  description: (n: number, keywords: string) => string;
  h1: (n: number, archetype: string) => string;
  headings: {
    meaning: (n: number) => string;
    strengths: (n: number) => string;
    challenges: (n: number) => string;
    love: (n: number) => string;
    work: (n: number) => string;
    calculation: (n: number) => string;
    compatible: (n: number) => string;
  };
  labels: { number: string; kind: string; single: string; master: string; root: string; month: string; day: string; year: string; total: string };
  /** How the reduction works (the method paragraph above the worked example). */
  method: string;
  /** "Take someone born on {date}." */
  example: (date: string, n: number) => string;
  /** Why a master number is kept (11 → 2, 22 → 4, 33 → 6). */
  masterKept: (n: number, root: number) => string;
  /** The master number a single digit can hide (2 → 11, 4 → 22, 6 → 33). */
  masterHidden: (n: number, master: number) => string;
  /** Link label of another number ("Life Path 3"). */
  link: (n: number) => string;
  faq: (input: { n: number; master: boolean; root?: number; date: string; entry: { intro: string; work: string; compatibleNote: string } }) => FaqItem[];
  related: string;
  /** Link label to the hub and to the calculator. */
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
    tile: (n: number) => string;
  };
  breadcrumb: { hub: string; item: (n: number) => string };
  imageAlt: string;
};
