import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { isLocale } from "@/i18n/config";
import { tarotHubPage } from "@/content/seo/tarot";
import { seoPageMetadata } from "@/content/seo/metadata";
import { BirthDateJump } from "@/components/seo/birth-date-jump";
import { CALCULATOR_ID, SeoArticle } from "@/components/seo/seo-article";

type Props = { params: Promise<{ lang: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { lang } = await params;
  return isLocale(lang) ? seoPageMetadata(tarotHubPage(lang), lang) : {};
}

/** The tarot birth card hub: the method, the 22 cards and the calculator. */
export default async function TarotHubPage({ params }: Props) {
  const { lang } = await params;
  if (!isLocale(lang)) notFound();
  return (
    <SeoArticle
      page={tarotHubPage(lang)}
      locale={lang}
      family="tarot"
      calculator={<BirthDateJump calculator="tarot-birth-card" id={CALCULATOR_ID} />}
    />
  );
}
