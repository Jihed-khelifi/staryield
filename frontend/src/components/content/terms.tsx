import type { Locale } from "@/i18n/config";
import { LegalDocument } from "@/components/content/legal/legal-document";

/**
 * Figma 813:2308 — Screen / Desktop / Legal / Terms Of Use.
 * The text lives in src/content/legal/terms/{en,es,fr}.ts.
 */
export default function ScreenDesktopLegalTermsOfUse({
  locale,
}: {
  locale: Locale;
}) {
  return <LegalDocument id="terms" locale={locale} />;
}
