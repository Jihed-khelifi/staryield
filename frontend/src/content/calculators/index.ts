import type { Locale } from "@/i18n/config";
import type {
  ArticleBlock,
  ArticleContent,
  ArticleKind,
} from "@/content/calculators/types";
import natalEn from "@/content/calculators/natal/en";
import natalEs from "@/content/calculators/natal/es";
import natalFr from "@/content/calculators/natal/fr";
import lifePathEn from "@/content/calculators/life-path/en";
import lifePathEs from "@/content/calculators/life-path/es";
import lifePathFr from "@/content/calculators/life-path/fr";
import astrologyEn from "@/content/calculators/astrology/en";
import astrologyEs from "@/content/calculators/astrology/es";
import astrologyFr from "@/content/calculators/astrology/fr";

/** Server-only content: render through <CalculatorArticle> from a server component. */
export const calculatorArticles: Record<ArticleKind, Record<Locale, ArticleContent>> = {
  natal: { en: natalEn, es: natalEs, fr: natalFr },
  "life-path": { en: lifePathEn, es: lifePathEs, fr: lifePathFr },
  astrology: { en: astrologyEn, es: astrologyEs, fr: astrologyFr },
};

/** Route segment (`/calculators/<kind>`) → article. */
export const articleKindForCalculator = {
  "natal-chart": "natal",
  "path-of-life": "life-path",
  astrology: "astrology",
} as const satisfies Record<string, ArticleKind>;

export type CalculatorRouteKind = keyof typeof articleKindForCalculator;

export function getCalculatorArticle(
  kind: ArticleKind | CalculatorRouteKind,
  locale: Locale,
): ArticleContent {
  const articleKind =
    kind in articleKindForCalculator
      ? articleKindForCalculator[kind as CalculatorRouteKind]
      : (kind as ArticleKind);
  return calculatorArticles[articleKind][locale] ?? calculatorArticles[articleKind].en;
}

/** Heading ids an article defines (rendered with a `<kind>-` prefix). */
export function articleAnchorIds(content: ArticleContent): string[] {
  const ids: string[] = [];
  const visit = (blocks: ArticleBlock[]) => {
    for (const block of blocks) {
      if (block.type === "h2" || block.type === "h3") ids.push(block.id);
      if (block.type === "split") visit(block.blocks);
    }
  };
  for (const section of content.sections) visit(section.blocks);
  return ids;
}
