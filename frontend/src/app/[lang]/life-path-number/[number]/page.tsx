import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { isLocale } from "@/i18n/config";
import { LIFE_PATH_NUMBERS, lifePathSlug, parseLifePathSlug } from "@/content/seo/life-path/data";
import { lifePathPage } from "@/content/seo/life-path";
import { seoPageMetadata } from "@/content/seo/metadata";
import { BirthDateJump } from "@/components/seo/birth-date-jump";
import { CALCULATOR_ID, SeoArticle } from "@/components/seo/seo-article";

type Props = { params: Promise<{ lang: string; number: string }> };

/** Only 1–9, 11, 22 and 33 have a page; anything else is a 404. */
export const dynamicParams = false;

export function generateStaticParams() {
  return LIFE_PATH_NUMBERS.map((n) => ({ number: lifePathSlug(n) }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { lang, number } = await params;
  const n = parseLifePathSlug(number);
  return isLocale(lang) && n !== null ? seoPageMetadata(lifePathPage(lang, n), lang) : {};
}

/** One Life Path number: its meaning, a worked example, the FAQ, and the calculator at the end. */
export default async function LifePathNumberPage({ params }: Props) {
  const { lang, number } = await params;
  const n = parseLifePathSlug(number);
  if (!isLocale(lang) || n === null) notFound();
  return (
    <SeoArticle
      page={lifePathPage(lang, n)}
      locale={lang}
      family="life-path"
      calculator={<BirthDateJump calculator="path-of-life" id={CALCULATOR_ID} />}
    />
  );
}
