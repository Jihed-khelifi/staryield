import type { Locale } from "@/i18n/config";
import { LegalDocument } from "@/components/content/legal/legal-document";

/**
 * Figma 822:2319 — Screen / Desktop / Legal / Privacy Policy.
 * The text lives in src/content/legal/privacy/{en,es,fr}.ts.
 */
export default function ScreenDesktopLegalPrivacyPolicy({
  locale,
}: {
  locale: Locale;
}) {
  return <LegalDocument id="privacy" locale={locale} />;
}
