import type { Metadata } from "next";
import { notFound } from "next/navigation";

import Home from "@/components/content/home";
import { PublicFrame } from "@/components/layout/public-frame";
import { ReferralWelcome } from "@/components/referrals/referral-welcome"; // growth: referrals
import { isLocale } from "@/i18n/config";
import { getDictionary } from "@/i18n/dictionaries";
import { createTranslateCopy } from "@/i18n/translate-copy";
// growth (seo): title, description, canonical, hreflang, link preview and structured data.
import { JsonLd } from "@/lib/seo/json-ld";
import { organizationJsonLd, webSiteJsonLd } from "@/lib/seo/jsonld";
import { DEFAULT_IMAGE_ALT, HOME_SEO } from "@/lib/seo/pages";
import { pageMetadata } from "@/lib/seo/site";

type Props = { params: Promise<{ lang: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { lang } = await params;
  if (!isLocale(lang)) return {};
  const text = createTranslateCopy((await getDictionary(lang)).copy);
  return pageMetadata({
    locale: lang,
    route: "",
    title: text(HOME_SEO.title),
    description: text(HOME_SEO.description),
    imageAlt: text(DEFAULT_IMAGE_ALT),
    absoluteTitle: true,
  });
}

export default async function LangIndexPage({ params }: Props) {
  const { lang } = await params;
  if (!isLocale(lang)) notFound();
  const text = createTranslateCopy((await getDictionary(lang)).copy);
  return (
    <PublicFrame>
      <ReferralWelcome placement="home" />
      <JsonLd data={[organizationJsonLd(), webSiteJsonLd(lang, text(HOME_SEO.description))]} />
      <Home />
    </PublicFrame>
  );
}
