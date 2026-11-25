import type { Metadata } from "next";

import type { Locale } from "@/i18n/config";
import { pageMetadata } from "@/lib/seo/site";
import type { SeoPage } from "@/content/seo/types";

/** Title, description, canonical, hreflang and link preview of a programmatic page. */
export function seoPageMetadata(page: SeoPage, locale: Locale): Metadata {
  return pageMetadata({
    locale,
    route: page.route,
    title: page.title,
    description: page.description,
    image: page.image,
    imageAlt: page.imageAlt,
    type: page.grid ? "website" : "article",
  });
}
