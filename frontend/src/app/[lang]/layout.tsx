import type { Metadata } from "next";
import { Cinzel, Inter, Source_Serif_4 } from "next/font/google";
import { cookies } from "next/headers";
import { notFound } from "next/navigation";

import "@/app/globals.css";

import { isLocale, locales } from "@/i18n/config";
import { getDictionary } from "@/i18n/dictionaries";
import { I18nProvider } from "@/i18n/i18n-provider";
import { TooltipProvider } from "@/components/ui/tooltip";
import { createTranslateCopy } from "@/i18n/translate-copy";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { ErrorReporter } from "@/components/feedback/error-reporter";
import { SessionProvider } from "@/lib/auth/session-provider";
import { NotificationsProvider } from "@/lib/notifications/notifications-provider";
import { AttributionCapture } from "@/components/feedback/attribution-capture";
// account settings: app-wide theme, rendered from the sy_theme cookie (no flash).
import { ThemeSync } from "@/components/layout/theme-sync";
import { THEME_COOKIE, parseThemeCookie } from "@/lib/theme";

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
    // Faceless sun mark (revisions 2.0: small logos have no face).
    icons: {
      icon: [
        { url: "https://assets.staryield.net/assets/brand/sun-mark-32-19cd009ec9.png", sizes: "32x32", type: "image/png" },
        { url: "https://assets.staryield.net/assets/brand/sun-mark-192-c139a00d54.png", sizes: "192x192", type: "image/png" },
      ],
      apple: [{ url: "https://assets.staryield.net/assets/brand/sun-mark-180-208bacab8e.png", sizes: "180x180", type: "image/png" }],
    },
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
  const theme = parseThemeCookie((await cookies()).get(THEME_COOKIE)?.value);

  return (
    <html lang={lang} dir="ltr" data-theme={theme}>
      <body
        className={`${cinzel.variable} ${sourceSerif.variable} ${inter.variable} paper-page min-h-dvh`}
      >
        <I18nProvider locale={lang} dictionary={dictionary}>
          <ErrorReporter />
          <AttributionCapture />
          <ToastProvider>
            <SessionProvider>
              <ThemeSync />
              <NotificationsProvider>
                <TooltipProvider>{children}</TooltipProvider>
              </NotificationsProvider>
            </SessionProvider>
          </ToastProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
