/**
 * Trustpilot "Review Collector" TrustBox (revisions 4.0, §23.7). The ids are
 * public: they appear in the embed code Trustpilot publishes for the business.
 */
import type { Locale } from "@/i18n/config";

export const TRUSTPILOT_TEMPLATE_ID = "56278e9abfbbba0bdcd568bc";
export const TRUSTPILOT_BUSINESS_UNIT_ID = "6ae3c22a0156593a343fe019";
export const TRUSTPILOT_REVIEW_URL = "https://www.trustpilot.com/review/staryield.net";
export const TRUSTPILOT_SCRIPT_URL = "https://widget.trustpilot.com/bootstrap/v5/tp.widget.bootstrap.min.js";
/** Height Trustpilot renders the Review Collector at; the skeleton reserves it. */
export const TRUSTPILOT_WIDGET_HEIGHT = 52;

export const TRUSTPILOT_LOCALES: Record<Locale, string> = {
  en: "en-US",
  fr: "fr-FR",
  es: "es-ES",
};
