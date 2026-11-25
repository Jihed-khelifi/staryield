/**
 * growth (seo): the structured data of the `[...screen]` routes, limited to
 * what each page visibly shows: FAQPage for a calculator's article and for
 * the FAQ page's answered questions (the support answers, the live readings and the account group), and BreadcrumbList where the page shows
 * a breadcrumb trail (the legal policy layout). Pages without a visible
 * trail get no BreadcrumbList.
 */
import { articleKindForCalculator, getCalculatorArticle, type CalculatorRouteKind } from "@/content/calculators";
import { FAQ_PAGE_ENTRIES } from "@/content/faq";
import { getLegalDocument } from "@/content/legal";
import type { LegalDocumentId } from "@/content/legal/types";
import type { Locale } from "@/i18n/config";
import type { TranslateCopy } from "@/i18n/translate-copy";
import { breadcrumbJsonLd, faqJsonLd, type JsonLdObject } from "@/lib/seo/jsonld";

/** The legal document of a route. */
const LEGAL_IDS: Record<string, LegalDocumentId> = { terms: "terms", privacy: "privacy", "refund-policy": "refund" };

export function screenJsonLd(route: string, lang: Locale, text: TranslateCopy): JsonLdObject[] {
  const kind = route.startsWith("calculators/") ? route.slice("calculators/".length) : "";
  if (Object.hasOwn(articleKindForCalculator, kind)) {
    const faq = getCalculatorArticle(kind as CalculatorRouteKind, lang)
      .sections.flatMap((section) => section.blocks)
      .flatMap((block) => (block.type === "faq" ? block.items : []));
    return faq.length ? [faqJsonLd(faq)] : [];
  }
  if (route === "faq") return [faqJsonLd(FAQ_PAGE_ENTRIES.map((item) => ({ q: text(item.q), a: text(item.a) })))];
  const legalId = Object.hasOwn(LEGAL_IDS, route) ? LEGAL_IDS[route] : undefined;
  const trail = legalId ? getLegalDocument(legalId, lang).policy?.breadcrumb : undefined;
  if (trail)
    return [
      breadcrumbJsonLd(lang, [
        { name: trail.home, route: "" },
        { name: trail.current, route: `/${route}` },
      ]),
    ];
  return [];
}
