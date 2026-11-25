import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { isLocale } from "@/i18n/config";
import { TAROT_SLUGS } from "@/content/seo/tarot/data";
import { tarotPage } from "@/content/seo/tarot";
import { seoPageMetadata } from "@/content/seo/metadata";
import { BirthDateJump } from "@/components/seo/birth-date-jump";
import { CALCULATOR_ID, SeoArticle } from "@/components/seo/seo-article";

type Props = { params: Promise<{ lang: string; card: string }> };

/** Only the 22 Major Arcana slugs have a page. */
export const dynamicParams = false;

export function generateStaticParams() {
  return TAROT_SLUGS.map((card) => ({ card }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { lang, card } = await params;
  const page = isLocale(lang) ? tarotPage(lang, card) : null;
  return page && isLocale(lang) ? seoPageMetadata(page, lang) : {};
}

/** One tarot birth card: meaning, attribution, constellation, a worked example, the FAQ and the calculator. */
export default async function TarotCardPage({ params }: Props) {
  const { lang, card } = await params;
  if (!isLocale(lang)) notFound();
  const page = tarotPage(lang, card);
  if (!page) notFound();
  return (
    <SeoArticle page={page} locale={lang} family="tarot" calculator={<BirthDateJump calculator="tarot-birth-card" id={CALCULATOR_ID} />} />
  );
}
