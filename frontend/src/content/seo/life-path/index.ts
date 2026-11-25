/**
 * The Life Path pages (`/life-path-number` and `/life-path-number/<n>`),
 * composed from the facts in `data.ts` and the per-locale copy. Server-only:
 * import from server components, never from client code.
 */
import type { Locale } from "@/i18n/config";
import { lifePathReduction, romanNumeral } from "@/lib/calculators";
import { SKY_SEO_IMAGE } from "@/lib/seo/site";
import {
  LIFE_PATH_COMPATIBLE,
  LIFE_PATH_EXAMPLE_DATE,
  LIFE_PATH_NUMBERS,
  MASTER_ROOT,
  lifePathNeighbours,
  lifePathSlug,
  type LifePathNumber,
} from "@/content/seo/life-path/data";
import type { LifePathContent, LifePathFrame, WorkedExample } from "@/content/seo/life-path/types";
import en, { frame as frameEn } from "@/content/seo/life-path/en";
import fr, { frame as frameFr } from "@/content/seo/life-path/fr";
import es, { frame as frameEs } from "@/content/seo/life-path/es";
import { SEO_COMMON, SEO_TEXT, labelled, longDate, spacedSum } from "@/content/seo/names";
import type { SeoPage } from "@/content/seo/types";
import { HUB_ROUTES, hubRelated } from "@/content/seo/hubs";

export const lifePathContent: Record<Locale, LifePathContent> = { en, fr, es };
export const lifePathFrames: Record<Locale, LifePathFrame> = { en: frameEn, fr: frameFr, es: frameEs };

/** When the Life Path pages last changed (sitemap `lastmod`). */
export const LIFE_PATH_UPDATED = "2026-11-25";

export const LIFE_PATH_HUB = HUB_ROUTES.lifePath;
export const LIFE_PATH_CALCULATOR = "/calculators/path-of-life";

export function lifePathRoute(n: LifePathNumber): string {
  return `${LIFE_PATH_HUB}/${lifePathSlug(n)}`;
}

/** The single digits a master number can hide (2 → 11, 4 → 22, 6 → 33). */
const HIDDEN_MASTER: Partial<Record<LifePathNumber, LifePathNumber>> = { 2: 11, 4: 22, 6: 33 };

function isMaster(n: LifePathNumber): n is 11 | 22 | 33 {
  return n === 11 || n === 22 || n === 33;
}

/** The worked example of a number: its example birth date reduced step by step. */
export function lifePathWorkedExample(locale: Locale, n: LifePathNumber): WorkedExample {
  const frame = lifePathFrames[locale];
  const reduction = lifePathReduction(LIFE_PATH_EXAMPLE_DATE[n]);
  const label = { month: frame.labels.month, day: frame.labels.day, year: frame.labels.year } as Record<string, string>;
  const chain = (value: number, steps: string[]) => [String(value), ...steps.map(spacedSum)].join(" → ");
  return {
    date: longDate(locale, LIFE_PATH_EXAMPLE_DATE[n]),
    lines: [
      ...reduction.components.map((c) => labelled(locale, label[c.label], chain(c.value, c.steps))),
      labelled(locale, frame.labels.total, `${reduction.components.map((c) => c.reduced).join(" + ")} = ${chain(reduction.total, reduction.final)}`),
    ],
  };
}

export function lifePathPage(locale: Locale, n: LifePathNumber): SeoPage {
  const frame = lifePathFrames[locale];
  const entry = lifePathContent[locale][n];
  const example = lifePathWorkedExample(locale, n);
  const compatible = LIFE_PATH_COMPATIBLE[n];
  const master = isMaster(n);
  const root = master ? MASTER_ROOT[n] : undefined;
  const hidden = HIDDEN_MASTER[n];
  const route = lifePathRoute(n);
  return {
    route,
    title: frame.title(n, entry.title),
    description: frame.description(n, entry.keywords),
    eyebrow: frame.eyebrow,
    h1: frame.h1(n, entry.title),
    intro: entry.intro,
    numeral: romanNumeral(n),
    sections: [
      { id: "meaning", heading: frame.headings.meaning(n), blocks: [{ type: "p", text: entry.archetype }] },
      { id: "strengths", heading: frame.headings.strengths(n), blocks: [{ type: "list", items: [...entry.strengths] }] },
      { id: "challenges", heading: frame.headings.challenges(n), blocks: [{ type: "list", items: [...entry.challenges] }] },
      { id: "love", heading: frame.headings.love(n), blocks: [{ type: "p", text: entry.love }] },
      {
        id: "work",
        heading: frame.headings.work(n),
        blocks: [
          { type: "p", text: entry.work },
          { type: "p", text: entry.purpose },
        ],
      },
      {
        id: "calculation",
        heading: frame.headings.calculation(n),
        blocks: [
          {
            type: "facts",
            columns: 3,
            items: [
              { label: frame.labels.number, value: String(n) },
              { label: frame.labels.kind, value: master ? frame.labels.master : frame.labels.single },
              ...(root ? [{ label: frame.labels.root, value: String(root) }] : []),
            ],
          },
          { type: "p", text: frame.method },
          { type: "p", text: frame.example(example.date, n) },
          { type: "list", ordered: true, items: example.lines },
          ...(root ? [{ type: "p" as const, text: frame.masterKept(n, root) }] : []),
          ...(hidden ? [{ type: "p" as const, text: frame.masterHidden(n, hidden) }] : []),
        ],
      },
      {
        id: "compatible",
        heading: frame.headings.compatible(n),
        blocks: [
          { type: "p", text: entry.compatibleNote },
          {
            type: "links",
            items: compatible.map((other) => ({ label: frame.link(other), href: lifePathRoute(other), note: lifePathContent[locale][other].title })),
          },
        ],
      },
    ],
    faq: frame.faq({
      n,
      master,
      root,
      date: example.date,
      entry,
    }),
    related: {
      heading: frame.related,
      links: [
        ...lifePathNeighbours(n).map((other) => ({ label: frame.link(other), href: lifePathRoute(other), note: lifePathContent[locale][other].title })),
        { label: frame.hubLink, href: LIFE_PATH_HUB },
        { label: frame.calculatorLink, href: LIFE_PATH_CALCULATOR },
      ],
    },
    breadcrumbs: [
      { name: SEO_COMMON[locale].home, route: "" },
      { name: frame.breadcrumb.hub, route: LIFE_PATH_HUB },
      { name: frame.breadcrumb.item(n), route },
    ],
    image: SKY_SEO_IMAGE,
    imageAlt: frame.imageAlt,
    updated: LIFE_PATH_UPDATED,
  };
}

export function lifePathHubPage(locale: Locale): SeoPage {
  const frame = lifePathFrames[locale];
  const hub = frame.hub;
  return {
    route: LIFE_PATH_HUB,
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
      items: LIFE_PATH_NUMBERS.map((n) => ({
        href: lifePathRoute(n),
        title: hub.tile(n),
        subtitle: lifePathContent[locale][n].title,
        numeral: romanNumeral(n),
      })),
    },
    faq: hub.faq,
    related: hubRelated(SEO_TEXT[locale], "lifePath", { label: frame.calculatorLink, href: LIFE_PATH_CALCULATOR }),
    breadcrumbs: [
      { name: SEO_COMMON[locale].home, route: "" },
      { name: frame.breadcrumb.hub, route: LIFE_PATH_HUB },
    ],
    image: SKY_SEO_IMAGE,
    imageAlt: frame.imageAlt,
    updated: LIFE_PATH_UPDATED,
  };
}
