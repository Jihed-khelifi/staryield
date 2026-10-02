import type { Metadata } from "next";
import { Cinzel, Inter, Source_Serif_4 } from "next/font/google";
import { notFound } from "next/navigation";

import "@/app/globals.css";

import { isLocale, locales } from "@/i18n/config";
import { getDictionary } from "@/i18n/dictionaries";
import { I18nProvider } from "@/i18n/i18n-provider";
import { TooltipProvider } from "@/components/ui/tooltip";
import { createTranslateCopy } from "@/i18n/translate-copy";

/** Stand-in for Castellar, which is not available as a webfont. */
const cinzel = Cinzel({
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  variable: "--font-cinzel",
  display: "swap",
});

const sourceSerif = Source_Serif_4({
  subsets: ["latin"],
  variable: "--font-source-serif",
  display: "swap",
});

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  display: "swap",
});

type LayoutParams = { params: Promise<{ lang: string }> };

export function generateStaticParams() {
  return locales.map((lang) => ({ lang }));
}

export async function generateMetadata({
  params,
}: LayoutParams): Promise<Metadata> {
  const { lang } = await params;
  if (!isLocale(lang)) return {};

  const dictionary = await getDictionary(lang);
  const text = createTranslateCopy(dictionary.copy);

  return {
    title: {
      default: `${dictionary.nav.brand} — ${text("Grounded wisdom. Personalized guidance.")}`,
      template: `%s — ${dictionary.nav.brand}`,
    },
    icons: { icon: "https://assets.staryield.net/favicon.ico" },
    description: text(
      "Explore free astrology and numerology calculators, connect with psychics, and discover your cosmic profile.",
    ),
  };
}

export default async function LangLayout({
  children,
  params,
}: LayoutParams & { children: React.ReactNode }) {
  const { lang } = await params;
  if (!isLocale(lang)) notFound();

  const dictionary = await getDictionary(lang);

  return (
    <html lang={lang} dir="ltr">
      <body
        className={`${cinzel.variable} ${sourceSerif.variable} ${inter.variable} paper-page min-h-dvh`}
      >
        <I18nProvider locale={lang} dictionary={dictionary}>
          <TooltipProvider>{children}</TooltipProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
