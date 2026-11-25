/**
 * The three search hubs: their routes and, per locale, their link labels
 * (the same words as the footer's "Explore" group). Each hub links to the
 * other two.
 */
import type { Locale } from "@/i18n/config";
import type { SeoLink } from "@/content/seo/types";

export type HubKey = "lifePath" | "tarot" | "compatibility";

export const HUB_ROUTES: Record<HubKey, string> = {
  lifePath: "/life-path-number",
  tarot: "/tarot-birth-card",
  compatibility: "/compatibility",
};

const HUB_LABELS: Record<Locale, Record<HubKey, string> & { more: string }> = {
  en: { lifePath: "Life Path numbers", tarot: "Tarot birth cards", compatibility: "Zodiac compatibility", more: "More free tools" },
  fr: { lifePath: "Chemins de vie", tarot: "Cartes de naissance du tarot", compatibility: "Compatibilité des signes", more: "Plus d’outils gratuits" },
  es: { lifePath: "Caminos de vida", tarot: "Cartas de nacimiento del tarot", compatibility: "Compatibilidad de signos", more: "Más herramientas gratuitas" },
};

/** The related links of a hub: its calculator, then the other two hubs. */
export function hubRelated(locale: Locale, own: HubKey, calculator: SeoLink): { heading: string; links: SeoLink[] } {
  const labels = HUB_LABELS[locale];
  return {
    heading: labels.more,
    links: [
      calculator,
      ...(Object.keys(HUB_ROUTES) as HubKey[]).filter((key) => key !== own).map((key) => ({ label: labels[key], href: HUB_ROUTES[key] })),
    ],
  };
}
