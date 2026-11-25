/**
 * Schema.org JSON-LD builders. Each returns a plain object describing one
 * thing the page visibly shows; <JsonLd> (json-ld.tsx) serialises it escaped.
 * Nothing here may describe content the page does not show (no ratings, no
 * prices, no reviews).
 */
import { COMPANY } from "@/config/company";
import { visibleSocialLinks } from "@/config/social";
import type { Locale } from "@/i18n/config";
import { parseInline } from "@/components/content/legal/inline-text";
import { LOGO_URL, SITE_NAME, SITE_URL, localeUrl } from "@/lib/seo/site";

export type JsonLdObject = Record<string, unknown>;

const CONTEXT = "https://schema.org";
const ORGANIZATION_ID = `${SITE_URL}/#organization`;

/** The company behind the site: name, logo, address, support e-mail and social profiles (all in the footer). */
export function organizationJsonLd(): JsonLdObject {
  const sameAs = visibleSocialLinks().map((link) => link.href).filter((href): href is string => Boolean(href));
  return {
    "@context": CONTEXT,
    "@type": "Organization",
    "@id": ORGANIZATION_ID,
    name: SITE_NAME,
    legalName: COMPANY.legalName,
    url: SITE_URL,
    logo: LOGO_URL,
    address: {
      "@type": "PostalAddress",
      streetAddress: COMPANY.address.street,
      addressLocality: COMPANY.address.city,
      addressRegion: COMPANY.address.region,
      postalCode: COMPANY.address.postalCode,
      addressCountry: "US",
    },
    contactPoint: { "@type": "ContactPoint", contactType: "customer support", email: COMPANY.supportEmail },
    ...(sameAs.length ? { sameAs } : {}),
  };
}

/** The site itself, in the page's language. */
export function webSiteJsonLd(locale: Locale, description: string): JsonLdObject {
  return {
    "@context": CONTEXT,
    "@type": "WebSite",
    "@id": `${SITE_URL}/#website`,
    name: SITE_NAME,
    url: localeUrl("", locale),
    inLanguage: locale,
    description,
    publisher: { "@id": ORGANIZATION_ID },
  };
}

/** One breadcrumb: its visible name and locale-less route ("" is the home page). */
export type Crumb = { name: string; route: string };

/** BreadcrumbList of the visible breadcrumb trail (the last crumb is the current page). */
export function breadcrumbJsonLd(locale: Locale, crumbs: Crumb[]): JsonLdObject {
  return {
    "@context": CONTEXT,
    "@type": "BreadcrumbList",
    itemListElement: crumbs.map((crumb, index) => ({
      "@type": "ListItem",
      position: index + 1,
      name: crumb.name,
      item: localeUrl(crumb.route, locale),
    })),
  };
}

export type FaqItem = { q: string; a: string };

/** Content-module inline markup (`**bold**`, `[label](href)`) as plain text. */
export function plainText(text: string): string {
  return parseInline(text)
    .map((part) => (part.kind === "link" ? part.label : part.value))
    .join("");
}

/** FAQPage for questions and answers the page shows. */
export function faqJsonLd(items: FaqItem[]): JsonLdObject {
  return {
    "@context": CONTEXT,
    "@type": "FAQPage",
    mainEntity: items.map((item) => ({
      "@type": "Question",
      name: plainText(item.q),
      acceptedAnswer: { "@type": "Answer", text: plainText(item.a) },
    })),
  };
}

/**
 * Serialise for an inline <script type="application/ld+json">. "<", ">" and
 * "&" are escaped so the data can never close the tag or open a comment, and
 * U+2028/U+2029 so the payload stays valid in every JavaScript parser.
 */
export function serializeJsonLd(data: JsonLdObject | JsonLdObject[]): string {
  return JSON.stringify(data)
    .replace(/</g, "\\u003c")
    .replace(/>/g, "\\u003e")
    .replace(/&/g, "\\u0026")
    .replace(/\u2028/g, "\\u2028")
    .replace(/\u2029/g, "\\u2029");
}
