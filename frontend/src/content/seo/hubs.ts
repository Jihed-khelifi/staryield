/**
 * The three search hubs: their routes and link labels (English source copy,
 * translated with `text()`). The footer's "Explore" group and the hubs'
 * related links read this one list. Each hub links to the other two.
 *
 * No dictionary is bundled here: the footer is a client component.
 */
import type { TranslateCopy } from "@/i18n/translate-copy";
import type { SeoLink } from "@/content/seo/types";

export type HubKey = "lifePath" | "tarot" | "compatibility";

export const HUB_KEYS: readonly HubKey[] = ["lifePath", "tarot", "compatibility"];

export const HUB_ROUTES: Record<HubKey, string> = {
  lifePath: "/life-path-number",
  tarot: "/tarot-birth-card",
  compatibility: "/compatibility",
};

/** Link labels of the hubs (English source; every string is in copy-{en,fr,es}). */
export const HUB_LABELS: Record<HubKey, string> = {
  lifePath: "Life Path numbers",
  tarot: "Tarot birth cards",
  compatibility: "Zodiac compatibility",
};

/** Heading of a hub's related links. */
export const HUB_MORE = "More free tools";

/** The related links of a hub, in the page's language (`text`): its calculator, then the other two hubs. */
export function hubRelated(text: TranslateCopy, own: HubKey, calculator: SeoLink): { heading: string; links: SeoLink[] } {
  return {
    heading: text(HUB_MORE),
    links: [calculator, ...HUB_KEYS.filter((key) => key !== own).map((key) => ({ label: text(HUB_LABELS[key]), href: HUB_ROUTES[key] }))],
  };
}
