/**
 * Site-wide SEO constants and metadata builders: the public origin, absolute
 * canonical and hreflang URLs, and the Open Graph / Twitter card of a page.
 * Pure (no React, no I18n), so metadata, the sitemap, the proxy and the tests
 * share it.
 */
import type { Metadata } from "next";

import { defaultLocale, localePath, locales, type Locale } from "@/i18n/config";

/** The public origin: NEXT_PUBLIC_SITE_URL, default https://staryield.net (no trailing slash). */
export const SITE_URL = (process.env.NEXT_PUBLIC_SITE_URL || "https://staryield.net").replace(/\/+$/, "");
export const SITE_NAME = "Staryield";

/** Open Graph locales of the site's languages. */
export const OG_LOCALE: Record<Locale, string> = { en: "en_US", fr: "fr_FR", es: "es_ES" };

const ASSETS = "https://assets.staryield.net/assets";

/**
 * Public R2 artwork used for link previews (dimensions as stored on R2; an
 * advisor's photo comes from the API without them).
 */
export type SeoImage = { url: string; width?: number; height?: number };

/** Tarot cards on a reading table (landscape, used by the calculator articles): the default preview. */
export const DEFAULT_SEO_IMAGE: SeoImage = { url: `${ASSETS}/articles/natal/twelve-houses-intro-0cf68eb2.png`, width: 640, height: 427 };

/**
 * A sunrise sky through the clouds (square, 1024 px): the preview of the Life
 * Path and sign compatibility pages, where tarot cards would be off topic.
 * Dedicated 1200 × 630 previews per family can replace it here once on R2.
 */
export const SKY_SEO_IMAGE: SeoImage = { url: `${ASSETS}/figma/f7e76.png`, width: 1024, height: 1024 };

/**
 * X's large-image card needs a picture at least 300 px wide (2:1 crop); a
 * smaller one (the 210 px tarot card) is shown with the small "summary" card.
 */
export function twitterCardFor(image: SeoImage): "summary" | "summary_large_image" {
  return image.width !== undefined && image.width < 300 ? "summary" : "summary_large_image";
}

/** The sun mark (logo) used by the Organization structured data. */
export const LOGO_URL = `${ASSETS}/brand/sun-mark-192-c139a00d54.png`;

/** The card artwork of a Major Arcana slug (210 × 355 on R2). */
export function tarotSeoImage(slug: string): SeoImage {
  return { url: `${ASSETS}/tarot/${slug}.png`, width: 210, height: 355 };
}

/** Absolute URL of a site path ("/en/about"). */
export function absoluteUrl(path: string): string {
  const clean = path === "/" || path === "" ? "" : path.startsWith("/") ? path : `/${path}`;
  return `${SITE_URL}${clean}`;
}

/** Absolute URL of a locale-less route in a locale ("/about", "fr" → https://…/fr/about; "" is the home page). */
export function localeUrl(route: string, locale: Locale): string {
  return absoluteUrl(localePath(route === "" ? "/" : route, locale));
}

/** hreflang → absolute URL for every locale plus x-default (the English page). */
export function languageAlternates(route: string): Record<string, string> {
  return {
    ...Object.fromEntries(locales.map((locale) => [locale, localeUrl(route, locale)])),
    "x-default": localeUrl(route, defaultLocale),
  };
}

/** Canonical + hreflang alternates of a locale-less route. */
export function pageAlternates(route: string, locale: Locale): NonNullable<Metadata["alternates"]> {
  return { canonical: localeUrl(route, locale), languages: languageAlternates(route) };
}

export type PageMetadataInput = {
  locale: Locale;
  /** Locale-less route: "" (home), "/about", "/life-path-number/7". */
  route: string;
  /** The page title without the site suffix (the layout's template adds "— Staryield"). */
  title: string;
  description: string;
  /** Preview image; `imageAlt` describes it in the page's language. */
  image?: SeoImage;
  imageAlt: string;
  type?: "website" | "article";
  /** The title is used as is, without the layout's "— Staryield" template (the home page). */
  absoluteTitle?: boolean;
};

/** "Title — Staryield", the form used by the layout template and the link previews. */
export function fullTitle(title: string): string {
  return `${title} — ${SITE_NAME}`;
}

/**
 * Title, description, canonical, hreflang, Open Graph and Twitter card for an
 * indexable page (the large-image card unless the picture is too small for it).
 */
export function pageMetadata({
  locale,
  route,
  title,
  description,
  image = DEFAULT_SEO_IMAGE,
  imageAlt,
  type = "website",
  absoluteTitle = false,
}: PageMetadataInput): Metadata {
  const shown = absoluteTitle ? title : fullTitle(title);
  const images = [{ url: image.url, ...(image.width && image.height ? { width: image.width, height: image.height } : {}), alt: imageAlt }];
  return {
    title: absoluteTitle ? { absolute: title } : title,
    description,
    alternates: pageAlternates(route, locale),
    openGraph: {
      type,
      siteName: SITE_NAME,
      locale: OG_LOCALE[locale],
      alternateLocale: locales.filter((other) => other !== locale).map((other) => OG_LOCALE[other]),
      title: shown,
      description,
      url: localeUrl(route, locale),
      images,
    },
    twitter: { card: twitterCardFor(image), title: shown, description, images },
  };
}

/** Never indexed, links not followed (one-time links, private pages). */
export const NOINDEX: Metadata["robots"] = { index: false, follow: false };
/** Not indexed, but its links may be followed (sign-in steps, sample results). */
export const NOINDEX_FOLLOW: Metadata["robots"] = { index: false, follow: true };
