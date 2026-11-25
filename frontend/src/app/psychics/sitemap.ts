import type { MetadataRoute } from "next";

import { fetchAdvisorSlugs } from "@/lib/psychics/server";
import { buildAdvisorSitemap } from "@/lib/seo/sitemap";

/**
 * Rendered on request (the build may have no API to read), from the
 * per-isolate copy of the directory that `fetchAdvisorSlugs` keeps for an hour.
 */
export const dynamic = "force-dynamic";

/** /psychics/sitemap.xml: every public advisor profile in every locale (listed in robots.txt). */
export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const slugs = await fetchAdvisorSlugs();
  // Without the directory, answer with an error crawlers retry, never an empty list that reads as "no profiles".
  if (!slugs) throw new Error("The advisor directory could not be read for the sitemap.");
  return buildAdvisorSitemap(slugs);
}
