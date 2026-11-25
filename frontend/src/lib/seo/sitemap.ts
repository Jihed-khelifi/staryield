/**
 * The sitemap's content (served by src/app/sitemap.ts): every indexable
 * page of the site, including the programmatic search pages.
 */
import type { MetadataRoute } from "next";

import { locales } from "@/i18n/config";
import { LIFE_PATH_NUMBERS } from "@/content/seo/life-path/data";
import { LIFE_PATH_HUB, LIFE_PATH_UPDATED, lifePathRoute } from "@/content/seo/life-path";
import { TAROT_SLUGS } from "@/content/seo/tarot/data";
import { TAROT_HUB, TAROT_UPDATED, tarotRoute } from "@/content/seo/tarot";
import { ALL_PAIRS } from "@/content/seo/compatibility/data";
import { COMPATIBILITY_HUB, COMPATIBILITY_UPDATED, pairRoute } from "@/content/seo/compatibility";
import { HOME_SEO, PAGE_SEO } from "@/lib/seo/pages";
import { languageAlternates, localeUrl } from "@/lib/seo/site";

type Entry = { route: string; updated: string; changeFrequency: NonNullable<MetadataRoute.Sitemap[number]["changeFrequency"]>; priority: number };

/** Every indexable page, once per locale (locale-less routes). */
export function sitemapEntries(): Entry[] {
  return [
    { route: "", updated: HOME_SEO.updated, changeFrequency: "weekly", priority: 1 },
    ...Object.entries(PAGE_SEO).map(([route, seo]) => ({
      route: `/${route}`,
      updated: seo.updated,
      changeFrequency: seo.changeFrequency,
      priority: seo.priority,
    })),
    { route: LIFE_PATH_HUB, updated: LIFE_PATH_UPDATED, changeFrequency: "monthly", priority: 0.8 },
    ...LIFE_PATH_NUMBERS.map((n) => ({ route: lifePathRoute(n), updated: LIFE_PATH_UPDATED, changeFrequency: "monthly" as const, priority: 0.7 })),
    { route: TAROT_HUB, updated: TAROT_UPDATED, changeFrequency: "monthly", priority: 0.8 },
    ...TAROT_SLUGS.map((slug) => ({ route: tarotRoute(slug), updated: TAROT_UPDATED, changeFrequency: "monthly" as const, priority: 0.7 })),
    { route: COMPATIBILITY_HUB, updated: COMPATIBILITY_UPDATED, changeFrequency: "monthly", priority: 0.8 },
    ...ALL_PAIRS.map((pair) => ({ route: pairRoute(pair.a, pair.b), updated: COMPATIBILITY_UPDATED, changeFrequency: "monthly" as const, priority: 0.6 })),
  ];
}

/** One entry per locale of each page, with its hreflang alternates (en, fr, es and x-default). */
function inEveryLocale(entries: (Omit<Entry, "updated"> & { updated?: string })[]): MetadataRoute.Sitemap {
  return entries.flatMap((entry) =>
    locales.map((locale) => ({
      url: localeUrl(entry.route, locale),
      ...(entry.updated ? { lastModified: entry.updated } : {}),
      changeFrequency: entry.changeFrequency,
      priority: entry.priority,
      alternates: { languages: languageAlternates(entry.route) },
    })),
  );
}

/**
 * The sitemap: every indexable URL in every locale, each with its hreflang
 * alternates. `lastModified` is the content's own date (a build-time
 * constant), never the request time. Advisor profiles have their own
 * sitemap (`buildAdvisorSitemap`), listed in robots.txt next to this one.
 */
export function buildSitemap(): MetadataRoute.Sitemap {
  return inEveryLocale(sitemapEntries());
}

/** Where the advisors' sitemap is served (src/app/psychics/sitemap.ts). */
export const ADVISOR_SITEMAP_PATH = "/psychics/sitemap.xml";

/**
 * The advisors' sitemap: every public profile (`/psychics/<slug>`) in every
 * locale. The directory gives no modification date, so none is claimed.
 */
export function buildAdvisorSitemap(slugs: readonly string[]): MetadataRoute.Sitemap {
  return inEveryLocale(slugs.map((slug) => ({ route: `/psychics/${slug}`, changeFrequency: "daily", priority: 0.7 })));
}
