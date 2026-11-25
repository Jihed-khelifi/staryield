import type { Metadata } from "next";
import { notFound, permanentRedirect } from "next/navigation";

import { isLocale } from "@/i18n/config";
import { PAIR_SLUGS_BOTH_ORDERS, parsePairSlug } from "@/content/seo/compatibility/data";
import { compatibilityPage } from "@/content/seo/compatibility";
import { seoPageMetadata } from "@/content/seo/metadata";
import { CompatibilityCalculator } from "@/components/seo/compatibility-calculator";
import { CALCULATOR_ID, SeoArticle } from "@/components/seo/seo-article";

type Props = { params: Promise<{ lang: string; pair: string }> };

/**
 * The 78 canonical pairs ("aries-and-leo") have a page; the 66 reversed slugs
 * ("leo-and-aries") are accepted only to redirect permanently (the proxy
 * normally does it first, in one hop). Anything else is a 404.
 */
export const dynamicParams = false;

export function generateStaticParams() {
  return PAIR_SLUGS_BOTH_ORDERS.map((pair) => ({ pair }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { lang, pair } = await params;
  const page = isLocale(lang) ? compatibilityPage(lang, pair) : null;
  return page && isLocale(lang) ? seoPageMetadata(page, lang) : {};
}

/** One sign pairing: how the signs relate, strengths, friction, tips, scores, the FAQ and the calculator. */
export default async function CompatibilityPairPage({ params }: Props) {
  const { lang, pair } = await params;
  if (!isLocale(lang)) notFound();
  const parsed = parsePairSlug(pair);
  if (!parsed) notFound();
  if (!parsed.canonical) permanentRedirect(`/${lang}/compatibility/${parsed.slug}`);
  const page = compatibilityPage(lang, parsed.slug);
  if (!page) notFound();
  return (
    <SeoArticle
      page={page}
      locale={lang}
      family="compatibility"
      calculator={<CompatibilityCalculator id={CALCULATOR_ID} current={parsed.slug} />}
    />
  );
}
