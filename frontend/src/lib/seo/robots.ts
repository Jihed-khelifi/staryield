import type { MetadataRoute } from "next";

import { SITE_URL } from "@/lib/seo/site";
import { ADVISOR_SITEMAP_PATH } from "@/lib/seo/sitemap";

/**
 * Private and utility paths crawlers should not fetch (every locale: `/*`).
 * Their pages are also `noindex`; this keeps crawl budget on public content
 * and one-time links (with tokens) out of crawlers' queues.
 */
export const DISALLOWED_PATHS = [
  "/*/profile",
  "/*/chatroom",
  "/*/credits",
  // Sign-up and sign-in steps after the first screen, password reset and the Google return page.
  "/*/signup/",
  "/*/forgot-password",
  "/*/reset-password",
  "/*/auth/",
  // One-time e-mail links and the age gate.
  "/*/confirm-email",
  "/*/cancel-email-change",
  "/*/keep-account",
  "/*/verify-email",
  "/*/unsubscribe",
  "/*/age-restricted",
  // Support requests (signed-in list and private links).
  "/*/support/",
  // Short links (redirects to the API).
  "/go/",
] as const;

/** /robots.txt: public pages are open, private and utility paths are not; points to the sitemaps (pages, advisors). */
export function buildRobots(): MetadataRoute.Robots {
  return {
    rules: { userAgent: "*", allow: "/", disallow: [...DISALLOWED_PATHS] },
    sitemap: [`${SITE_URL}/sitemap.xml`, `${SITE_URL}${ADVISOR_SITEMAP_PATH}`],
  };
}
