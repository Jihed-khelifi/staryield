import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { isLocale } from "@/i18n/config";
import { lifePathHubPage } from "@/content/seo/life-path";
import { seoPageMetadata } from "@/content/seo/metadata";
import { BirthDateJump } from "@/components/seo/birth-date-jump";
import { CALCULATOR_ID, SeoArticle } from "@/components/seo/seo-article";

type Props = { params: Promise<{ lang: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { lang } = await params;
  return isLocale(lang) ? seoPageMetadata(lifePathHubPage(lang), lang) : {};
}

/** The Life Path hub: what the numbers mean, the twelve numbers and the calculator. */
export default async function LifePathHubPage({ params }: Props) {
  const { lang } = await params;
  if (!isLocale(lang)) notFound();
  return (
    <SeoArticle
      page={lifePathHubPage(lang)}
      locale={lang}
      family="life-path"
      calculator={<BirthDateJump calculator="path-of-life" id={CALCULATOR_ID} />}
    />
  );
}
