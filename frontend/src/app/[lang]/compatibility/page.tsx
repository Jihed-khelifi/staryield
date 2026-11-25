import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { isLocale } from "@/i18n/config";
import { compatibilityHubPage } from "@/content/seo/compatibility";
import { seoPageMetadata } from "@/content/seo/metadata";
import { CompatibilityCalculator } from "@/components/seo/compatibility-calculator";
import { CALCULATOR_ID, SeoArticle } from "@/components/seo/seo-article";

type Props = { params: Promise<{ lang: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { lang } = await params;
  return isLocale(lang) ? seoPageMetadata(compatibilityHubPage(lang), lang) : {};
}

/** The compatibility hub: the calculator, all 78 pairings by sign and how they are read. */
export default async function CompatibilityHubPage({ params }: Props) {
  const { lang } = await params;
  if (!isLocale(lang)) notFound();
  return (
    <SeoArticle
      page={compatibilityHubPage(lang)}
      locale={lang}
      family="compatibility"
      calculator={<CompatibilityCalculator id={CALCULATOR_ID} />}
    />
  );
}
