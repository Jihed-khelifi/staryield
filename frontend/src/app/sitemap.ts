import type { MetadataRoute } from "next";

import { buildSitemap } from "@/lib/seo/sitemap";

/** /sitemap.xml (see `lib/seo/sitemap.ts`). */
export default function sitemap(): MetadataRoute.Sitemap {
  return buildSitemap();
}
