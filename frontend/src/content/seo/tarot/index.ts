/**
 * The tarot birth card pages (`/tarot-birth-card` and
 * `/tarot-birth-card/<card>`), composed from the facts in `data.ts` and the
 * per-locale copy. Server-only.
 */
import type { Locale } from "@/i18n/config";
import { tarotBirthCards } from "@/lib/calculators";
import { birthCardsOf, constellationOf } from "@/lib/astro/tarot-birth-card";
import { tarotSeoImage } from "@/lib/seo/site";
import { TAROT_SLUGS, slugOfBirthNumber, tarotCardFacts, tarotNeighbours, type TarotCardFacts } from "@/content/seo/tarot/data";
import type { ChainRole, TarotContent, TarotFrame } from "@/content/seo/tarot/types";
import en, { frame as frameEn } from "@/content/seo/tarot/en";
import fr, { frame as frameFr } from "@/content/seo/tarot/fr";
import es, { frame as frameEs } from "@/content/seo/tarot/es";
import { CARD_NAMES, ELEMENT_NAMES, PLANET_NAMES, SEO_COMMON, SIGN_NAMES, labelled, listOf, longDate, spacedSum } from "@/content/seo/names";
import type { SeoPage } from "@/content/seo/types";
import { HUB_ROUTES, hubRelated } from "@/content/seo/hubs";

export const tarotContent: Record<Locale, TarotContent> = { en, fr, es };
export const tarotFrames: Record<Locale, TarotFrame> = { en: frameEn, fr: frameFr, es: frameEs };

/** When the tarot birth card pages last changed (sitemap `lastmod`). */
export const TAROT_UPDATED = "2026-11-25";

export const TAROT_HUB = HUB_ROUTES.tarot;
/** The tarot birth card calculator (added by the calculators stream). */
export const TAROT_CALCULATOR = "/calculators/tarot-birth-card";

export function tarotRoute(slug: string): string {
  return `${TAROT_HUB}/${slug}`;
}

function cardName(locale: Locale, birthNumber: number): string {
  return CARD_NAMES[locale][slugOfBirthNumber(birthNumber)];
}

function capitalize(value: string): string {
  return value.charAt(0).toLocaleUpperCase() + value.slice(1);
}

/** The localized value of a Golden Dawn attribution ("Aquarius", "the Sun", "Air"). */
function attributionValue(locale: Locale, facts: TarotCardFacts): string {
  const { attribution } = facts;
  if (attribution.kind === "element") return ELEMENT_NAMES[locale][attribution.value];
  if (attribution.kind === "planet") return PLANET_NAMES[locale][attribution.value];
  return SIGN_NAMES[locale][attribution.value];
}

/** True when no birth date from 1900 onward gives this number as the first total (only 1, The Magician). */
export function isSoulOnly(birthNumber: number): boolean {
  return birthNumber === 1;
}

/** The worked example: the example date's sum and each reduction, with the card it lands on. */
export function tarotWorkedExample(locale: Locale, facts: TarotCardFacts): { date: string; lines: string[] } {
  const frame = tarotFrames[locale];
  const result = tarotBirthCards(facts.exampleDate);
  if (!result) throw new RangeError(`Invalid example date for ${facts.slug}`);
  const [year, month, day] = facts.exampleDate.split("-").map(Number);
  const role = (index: number): ChainRole =>
    result.cards.length === 1 ? "both" : index === 0 ? "personality" : index === result.cards.length - 1 ? "soul" : "middle";
  const landing = (index: number) => `${result.cards[index]} → ${cardName(locale, result.cards[index])} (${frame.labels[role(index)]})`;
  const lines = [labelled(locale, frame.labels.total, `${month} + ${day} + ${year} = ${result.total}`)];
  result.steps.forEach((step, index) => {
    const spaced = spacedSum(step);
    lines.push(index === result.steps.length - 1 ? `${spaced.replace(/ = \d+$/, "")} = ${landing(0)}` : spaced);
  });
  for (let index = 1; index < result.cards.length; index++) {
    const previous = result.cards[index - 1];
    lines.push(`${String(previous).split("").join(" + ")} = ${landing(index)}`);
  }
  return { date: longDate(locale, facts.exampleDate), lines };
}

export function tarotPage(locale: Locale, slug: string): SeoPage | null {
  const facts = tarotCardFacts(slug);
  if (!facts) return null;
  const frame = tarotFrames[locale];
  const entry = tarotContent[locale][slug];
  const card = CARD_NAMES[locale][slug];
  const value = attributionValue(locale, facts);
  const attribution = frame.attribution(card, facts.letter, value, facts.attribution.kind);
  const soulCard = facts.chain[facts.chain.length - 1];
  const soulOf = constellationOf(facts.birthNumber)
    .filter((n) => n !== facts.birthNumber && birthCardsOf(n).at(-1) === facts.birthNumber)
    .map((n) => ({ number: n, card: cardName(locale, n) }));
  const soulOnly = isSoulOnly(facts.birthNumber);
  const constellation = frame.constellation({
    card,
    birthNumber: facts.birthNumber,
    chain: facts.chain.map((n) => cardName(locale, n)),
    soulOf,
    family: constellationOf(soulCard).map((n) => cardName(locale, n)),
    soulOnly,
  });
  const example = tarotWorkedExample(locale, facts);
  const route = tarotRoute(slug);
  const family = constellationOf(soulCard).filter((n) => n !== facts.birthNumber);
  return {
    route,
    title: frame.title(card),
    description: frame.description(card, entry.keywords),
    eyebrow: frame.eyebrow,
    h1: frame.h1(card),
    intro: entry.intro,
    artwork: [{ src: facts.image, alt: frame.cardAlt(card), width: 210, height: 355 }],
    sections: [
      {
        id: "meaning",
        heading: frame.headings.meaning(card),
        blocks: [{ type: "p", text: entry.meaning }, ...(entry.note ? [{ type: "p" as const, text: entry.note }] : [])],
      },
      {
        id: "facts",
        heading: frame.headings.facts,
        blocks: [
          {
            type: "facts",
            items: [
              { label: frame.labels.arcana, value: String(facts.arcana) },
              { label: frame.labels.numeral, value: facts.numeral },
              { label: frame.labels.birthNumber, value: String(facts.birthNumber) },
              { label: frame.labels.letter, value: facts.letter },
              { label: frame.labels.attribution, value: `${capitalize(value)} (${frame.labels[facts.attribution.kind]})` },
            ],
          },
          { type: "p", text: attribution },
        ],
      },
      { id: "gifts", heading: frame.headings.gifts(card), blocks: [{ type: "list", items: [...entry.gifts] }] },
      { id: "shadow", heading: frame.headings.shadow(card), blocks: [{ type: "list", items: [...entry.shadow] }] },
      {
        id: "constellation",
        heading: frame.headings.constellation(card),
        blocks: [
          ...constellation.map((text) => ({ type: "p" as const, text })),
          ...(family.length
            ? [{ type: "links" as const, items: family.map((n) => ({ label: cardName(locale, n), href: tarotRoute(slugOfBirthNumber(n)), note: String(n) })) }]
            : []),
        ],
      },
      {
        id: "example",
        heading: frame.headings.example,
        blocks: [
          { type: "p", text: frame.example(example.date) },
          { type: "list", ordered: true, items: example.lines },
        ],
      },
    ],
    faq: frame.faq({
      card,
      entry,
      constellation: constellation.join(" "),
      attribution,
      date: example.date,
      birthNumber: facts.birthNumber,
      soulOnly,
      soulOf: listOf(locale, soulOf.map((item) => String(item.number))),
    }),
    related: {
      heading: frame.related,
      links: [
        ...tarotNeighbours(slug).map((other) => ({ label: CARD_NAMES[locale][other], href: tarotRoute(other) })),
        { label: frame.hubLink, href: TAROT_HUB },
        { label: frame.calculatorLink, href: TAROT_CALCULATOR },
      ],
    },
    breadcrumbs: [
      { name: SEO_COMMON[locale].home, route: "" },
      { name: frame.breadcrumb.hub, route: TAROT_HUB },
      { name: card, route },
    ],
    image: tarotSeoImage(slug),
    imageAlt: frame.cardAlt(card),
    updated: TAROT_UPDATED,
  };
}

export function tarotHubPage(locale: Locale): SeoPage {
  const frame = tarotFrames[locale];
  const hub = frame.hub;
  const cards = TAROT_SLUGS.map((slug) => tarotCardFacts(slug)).filter((facts): facts is TarotCardFacts => facts !== null);
  return {
    route: TAROT_HUB,
    title: hub.title,
    description: hub.description,
    eyebrow: frame.eyebrow,
    h1: hub.h1,
    intro: hub.intro,
    sections: hub.sections.map((section) => ({
      id: section.id,
      heading: section.heading,
      blocks: section.paragraphs.map((text) => ({ type: "p" as const, text })),
    })),
    grid: {
      heading: hub.gridHeading,
      items: cards.map((facts) => ({
        href: tarotRoute(facts.slug),
        title: CARD_NAMES[locale][facts.slug],
        subtitle: hub.tile(facts.birthNumber, facts.numeral),
        image: { src: facts.image, width: 210, height: 355 },
      })),
    },
    faq: hub.faq,
    related: hubRelated(locale, "tarot", { label: frame.calculatorLink, href: TAROT_CALCULATOR }),
    breadcrumbs: [
      { name: SEO_COMMON[locale].home, route: "" },
      { name: frame.breadcrumb.hub, route: TAROT_HUB },
    ],
    image: tarotSeoImage("the-star"),
    imageAlt: frame.cardAlt(CARD_NAMES[locale]["the-star"]),
    updated: TAROT_UPDATED,
  };
}
