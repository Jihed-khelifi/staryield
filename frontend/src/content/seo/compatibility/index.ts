/**
 * The sign compatibility pages (`/compatibility` and
 * `/compatibility/<sign>-and-<sign>`), composed from the language-independent
 * factors in `data.ts` and the per-locale building blocks. Server-only.
 */
import type { Locale } from "@/i18n/config";
import type { ZodiacSign } from "@/lib/api/astrology";
import { SIGNS } from "@/lib/astro/zodiac";
import { zodiacIconUrl } from "@/lib/astro-icons";
import { DEFAULT_SEO_IMAGE } from "@/lib/seo/site";
import {
  ALL_PAIRS,
  ASPECTS,
  MODALITIES,
  aspectBetween,
  elementOf,
  elementPairKey,
  indicatorsFor,
  modalityPairKey,
  overallLevel,
  pairSlug,
  parsePairSlug,
  relatedPairs,
  rulerOf,
} from "@/content/seo/compatibility/data";
import type { CompatibilityCopy, PairFacts } from "@/content/seo/compatibility/types";
import en from "@/content/seo/compatibility/en";
import fr from "@/content/seo/compatibility/fr";
import es from "@/content/seo/compatibility/es";
import { ELEMENT_NAMES, MODALITY_NAMES, PLANET_NAMES, SEO_COMMON, SIGN_NAMES, signOf } from "@/content/seo/names";
import type { SeoPage } from "@/content/seo/types";
import { HUB_ROUTES, hubRelated } from "@/content/seo/hubs";

export const compatibilityCopy: Record<Locale, CompatibilityCopy> = { en, fr, es };

/** When the compatibility pages last changed (sitemap `lastmod`). */
export const COMPATIBILITY_UPDATED = "2026-11-25";

export const COMPATIBILITY_HUB = HUB_ROUTES.compatibility;
/** The zodiac sign calculator, where a cusp birthday can be settled with a birth time. */
export const SIGN_CALCULATOR = "/calculators/astrology";

export function pairRoute(a: ZodiacSign, b: ZodiacSign): string {
  return `${COMPATIBILITY_HUB}/${pairSlug(a, b)}`;
}

/** The zodiac glyph artwork of a sign (transparent SVG on R2, 48 × 48 box). */
function signArtwork(sign: ZodiacSign) {
  return { src: zodiacIconUrl(sign) ?? "", width: 48, height: 48 };
}

/** Every localized fact of a pair, as the frame's sentence builders receive them. */
export function pairFacts(locale: Locale, a: ZodiacSign, b: ZodiacSign): PairFacts {
  const copy = compatibilityCopy[locale];
  const aspect = aspectBetween(a, b);
  const indicators = indicatorsFor(a, b);
  const level = overallLevel(indicators);
  return {
    a: SIGN_NAMES[locale][a],
    b: SIGN_NAMES[locale][b],
    ofA: signOf(locale, a),
    ofB: signOf(locale, b),
    same: a === b,
    aspect,
    aspectName: copy.aspects[aspect].name,
    aspectWithArticle: copy.aspects[aspect].withArticle,
    angle: copy.aspects[aspect].angle,
    level,
    levelText: copy.frame.levels[level],
    elementA: ELEMENT_NAMES[locale][elementOf(a)],
    elementB: ELEMENT_NAMES[locale][elementOf(b)],
    modalityA: MODALITY_NAMES[locale][MODALITIES[a]],
    modalityB: MODALITY_NAMES[locale][MODALITIES[b]],
    rulerA: PLANET_NAMES[locale][rulerOf(a)],
    rulerB: PLANET_NAMES[locale][rulerOf(b)],
    sharedRuler: rulerOf(a) === rulerOf(b),
    indicators,
  };
}

export function compatibilityPage(locale: Locale, slug: string): SeoPage | null {
  const parsed = parsePairSlug(slug);
  if (!parsed || !parsed.canonical) return null;
  const { a, b } = parsed;
  const copy = compatibilityCopy[locale];
  const frame = copy.frame;
  const facts = pairFacts(locale, a, b);
  const aspect = copy.aspects[facts.aspect];
  const element = copy.elements[elementPairKey(elementOf(a), elementOf(b))];
  const modality = copy.modalities[modalityPairKey(MODALITIES[a], MODALITIES[b])];
  const [profileA, profileB] = [copy.signs[a], copy.signs[b]];
  const pair = frame.pairName(facts);
  const route = pairRoute(a, b);
  const signFacts = (of: string, elementName: string, modalityName: string, ruler: string) => [
    { label: frame.labels.element(of), value: elementName },
    { label: frame.labels.modality(of), value: modalityName },
    { label: frame.labels.ruler(of), value: ruler.charAt(0).toLocaleUpperCase() + ruler.slice(1) },
  ];
  const indicatorItems = (["love", "friendship", "work"] as const).map((key) => ({
    key,
    label: frame.labels[key],
    value: facts.indicators[key],
    valueText: frame.labels.outOf(facts.indicators[key]),
  }));
  return {
    route,
    title: frame.title(facts.a, facts.b, facts.same),
    description: frame.description(facts),
    eyebrow: frame.eyebrow,
    h1: frame.h1(facts.a, facts.b, facts.same),
    intro: frame.intro(facts),
    artwork: (facts.same ? [a] : [a, b]).map((sign) => ({ ...signArtwork(sign), alt: frame.signAlt(SIGN_NAMES[locale][sign]) })),
    sections: [
      {
        id: "overview",
        heading: frame.headings.overview(pair),
        blocks: [
          { type: "p", text: aspect.essence },
          { type: "p", text: frame.portrait(facts, profileA.traits, profileB.traits) },
        ],
      },
      {
        id: "elements",
        heading: frame.headings.elements,
        blocks: [
          {
            type: "facts",
            columns: 3,
            items: [
              ...signFacts(facts.ofA, facts.elementA, facts.modalityA, facts.rulerA),
              ...(facts.same ? [] : signFacts(facts.ofB, facts.elementB, facts.modalityB, facts.rulerB)),
            ],
          },
          { type: "p", text: element.summary },
          { type: "p", text: modality.summary },
        ],
      },
      { id: "rulers", heading: frame.headings.rulers, blocks: [{ type: "p", text: frame.rulers(facts, copy.planets[rulerOf(a)], copy.planets[rulerOf(b)]) }] },
      {
        id: "strengths",
        heading: frame.headings.strengths(pair),
        blocks: [
          { type: "list", items: [aspect.strength, element.strength] },
          { type: "p", text: frame.brings(facts, profileA.brings, profileB.brings) },
        ],
      },
      {
        id: "friction",
        heading: frame.headings.friction(pair),
        blocks: [
          { type: "list", items: [aspect.friction, element.friction, modality.friction] },
          { type: "p", text: frame.needs(facts, profileA.needs, profileB.needs) },
        ],
      },
      { id: "tips", heading: frame.headings.tips, blocks: [{ type: "list", items: [aspect.tip, element.tip, modality.tip] }] },
      {
        id: "indicators",
        heading: frame.headings.indicators,
        blocks: [
          { type: "p", text: frame.reading(facts) },
          { type: "indicators", items: indicatorItems, note: frame.indicatorNote },
        ],
      },
    ],
    faq: frame.faq(facts, {
      essence: aspect.essence,
      loveA: profileA.love,
      loveB: profileB.love,
      elementSummary: `${element.summary} ${modality.summary}`,
    }),
    related: {
      heading: frame.related,
      links: [
        ...relatedPairs(a, b).map((other) => ({
          label: frame.h1(SIGN_NAMES[locale][other.a], SIGN_NAMES[locale][other.b], other.a === other.b),
          href: pairRoute(other.a, other.b),
        })),
        { label: frame.hubLink, href: COMPATIBILITY_HUB },
        { label: frame.calculatorLink, href: SIGN_CALCULATOR },
      ],
    },
    breadcrumbs: [
      { name: SEO_COMMON[locale].home, route: "" },
      { name: frame.breadcrumb.hub, route: COMPATIBILITY_HUB },
      { name: frame.h1(facts.a, facts.b, facts.same), route },
    ],
    image: DEFAULT_SEO_IMAGE,
    imageAlt: frame.imageAlt,
    updated: COMPATIBILITY_UPDATED,
  };
}

export function compatibilityHubPage(locale: Locale): SeoPage {
  const copy = compatibilityCopy[locale];
  const frame = copy.frame;
  const hub = frame.hub;
  return {
    route: COMPATIBILITY_HUB,
    title: hub.title,
    description: hub.description,
    eyebrow: frame.eyebrow,
    h1: hub.h1,
    intro: hub.intro,
    sections: [
      ...hub.sections.map((section) => ({
        id: section.id,
        heading: section.heading,
        blocks: section.paragraphs.map((text) => ({ type: "p" as const, text })),
      })),
      {
        id: "aspects",
        heading: hub.aspectsHeading,
        blocks: [{ type: "list", items: ASPECTS.map((key) => hub.aspectLine(copy.aspects[key].name, copy.aspects[key].angle, copy.aspects[key].essence)) }],
      },
    ],
    grid: {
      heading: hub.gridHeading,
      intro: hub.gridIntro,
      groups: SIGNS.map((sign) => ({
        title: SIGN_NAMES[locale][sign],
        image: signArtwork(sign),
        links: SIGNS.map((other) => {
          const [first, second] = SIGNS.indexOf(sign) <= SIGNS.indexOf(other) ? [sign, other] : [other, sign];
          return {
            label: frame.h1(SIGN_NAMES[locale][first], SIGN_NAMES[locale][second], first === second),
            short: SIGN_NAMES[locale][other],
            href: pairRoute(sign, other),
            note: frame.levelShort[overallLevel(indicatorsFor(sign, other))],
          };
        }),
      })),
    },
    faq: hub.faq,
    related: hubRelated(locale, "compatibility", { label: frame.calculatorLink, href: SIGN_CALCULATOR }),
    breadcrumbs: [
      { name: SEO_COMMON[locale].home, route: "" },
      { name: frame.breadcrumb.hub, route: COMPATIBILITY_HUB },
    ],
    image: DEFAULT_SEO_IMAGE,
    imageAlt: frame.imageAlt,
    updated: COMPATIBILITY_UPDATED,
  };
}

/** The canonical pair slugs, for `generateStaticParams` and the sitemap. */
export const COMPATIBILITY_SLUGS: readonly string[] = ALL_PAIRS.map((pair) => pair.slug);
