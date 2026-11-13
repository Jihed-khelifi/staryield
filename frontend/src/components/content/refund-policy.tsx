import type { Locale } from "@/i18n/config";
import { LegalDocument } from "@/components/content/legal/legal-document";

/**
 * Figma 1360:2766 (desktop) / 1360:2771 (mobile) — Refund & Cancellation Policy.
 * The text lives in src/content/legal/refund/{en,es,fr}.ts.
 */
export default function RefundPolicy({ locale }: { locale: Locale }) {
  return <LegalDocument id="refund" locale={locale} />;
}
