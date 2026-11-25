import type { ZodiacSign } from "@/lib/api/astrology";
import type { Planet } from "@/content/seo/names";
import type { AspectKey, ElementPairKey, Indicators, ModalityPairKey, OverallLevel } from "@/content/seo/compatibility/data";
import type { FaqItem } from "@/content/seo/types";

/**
 * The typed building blocks every pairing page is composed from, per
 * locale: the angular relationship (7), the element pair (10), the
 * modality pair (6) and each sign's profile (12). A page combines one of
 * each with the two signs' profiles and rulers, so all 78 pages differ
 * while every sentence stays true to its factor.
 */
export type AspectCopy = {
  /** "trine". */
  name: string;
  /** "a trine" (with the article the language needs). */
  withArticle: string;
  /** "120°, four signs apart". */
  angle: string;
  /** What the relationship means in traditional astrology (one or two sentences). */
  essence: string;
  strength: string;
  friction: string;
  tip: string;
};

export type ElementPairCopy = { summary: string; strength: string; friction: string; tip: string };

export type ModalityPairCopy = { summary: string; friction: string; tip: string };

export type SignProfile = {
  /** "bold, direct and quick to act" (completes "Aries is …"). */
  traits: string;
  /** "initiative and courage" (completes "Aries brings …"). */
  brings: string;
  /** "room to lead and quick decisions" (completes "Aries needs …"). */
  needs: string;
  /** How the sign loves (a full sentence). */
  love: string;
};

/** Facts handed to the frame's sentence builders; names are localized. */
export type PairFacts = {
  a: string;
  b: string;
  /** The sign after "of" in the language ("du Bélier", "de Aries", "of Aries"). */
  ofA: string;
  ofB: string;
  same: boolean;
  aspect: AspectKey;
  aspectName: string;
  aspectWithArticle: string;
  angle: string;
  level: OverallLevel;
  levelText: string;
  elementA: string;
  elementB: string;
  modalityA: string;
  modalityB: string;
  rulerA: string;
  rulerB: string;
  sharedRuler: boolean;
  indicators: Indicators;
};

export type CompatibilityFrame = {
  eyebrow: string;
  title: (a: string, b: string, same: boolean) => string;
  description: (facts: PairFacts) => string;
  h1: (a: string, b: string, same: boolean) => string;
  /** The direct answer under the H1. */
  intro: (facts: PairFacts) => string;
  /** "Aries and Leo" / "two Aries signs", for the headings. */
  pairName: (facts: PairFacts) => string;
  headings: {
    overview: (pair: string) => string;
    elements: string;
    rulers: string;
    strengths: (pair: string) => string;
    friction: (pair: string) => string;
    tips: string;
    indicators: string;
  };
  labels: {
    /** Fact labels; they receive the "of" form of the sign ("of Aries", "du Bélier"). */
    element: (of: string) => string;
    modality: (of: string) => string;
    ruler: (of: string) => string;
    love: string;
    friendship: string;
    work: string;
    outOf: (value: number) => string;
  };
  /** Sentences about the two signs, built from their profiles. */
  portrait: (facts: PairFacts, traitsA: string, traitsB: string) => string;
  brings: (facts: PairFacts, bringsA: string, bringsB: string) => string;
  needs: (facts: PairFacts, needsA: string, needsB: string) => string;
  rulers: (facts: PairFacts, planetA: string, planetB: string) => string;
  /** The overall reading of the indicators. */
  reading: (facts: PairFacts) => string;
  indicatorNote: string;
  levels: Record<OverallLevel, string>;
  /** Short level labels for the hub grid ("Harmonious"). */
  levelShort: Record<OverallLevel, string>;
  faq: (facts: PairFacts, input: { essence: string; loveA: string; loveB: string; elementSummary: string }) => FaqItem[];
  related: string;
  hubLink: string;
  calculatorLink: string;
  hub: {
    title: string;
    description: string;
    h1: string;
    intro: string;
    gridHeading: string;
    gridIntro: string;
    sections: { id: string; heading: string; paragraphs: string[] }[];
    /** One line per relationship for the hub's list of aspects. */
    aspectLine: (name: string, angle: string, essence: string) => string;
    aspectsHeading: string;
    faq: FaqItem[];
  };
  breadcrumb: { hub: string };
  imageAlt: string;
  signAlt: (sign: string) => string;
};

export type CompatibilityCopy = {
  aspects: Record<AspectKey, AspectCopy>;
  elements: Record<ElementPairKey, ElementPairCopy>;
  modalities: Record<ModalityPairKey, ModalityPairCopy>;
  signs: Record<ZodiacSign, SignProfile>;
  /** What each ruling planet stands for ("Mars, the planet of drive and courage"). */
  planets: Record<Planet, string>;
  frame: CompatibilityFrame;
};
