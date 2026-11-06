/**
 * Long-form articles shown under each calculator's results (Figma
 * 482:2010 natal, 1279:1749 path of life, 482:2275 astrology).
 *
 * Inline text may use `**bold**` and `[label](href)` links (internal paths,
 * `#anchors` and `mailto:` only). Ids and structure must be identical in
 * every locale; `npm run test:content` enforces it.
 */
export type ArticleKind = "natal" | "life-path" | "astrology";

export type ArticleImage = {
  /** Absolute https://assets.staryield.net URL registered in r2-images.json. */
  src: string;
  alt: string;
  width: number;
  height: number;
  /** object-position for cropped frames (Figma uses "bottom" for the intro photo). */
  position?: "center" | "bottom";
  /** Display frame from Figma (desktop size in px and corner radius). */
  frame: { width: number; height: number; radius: number };
};

export type ArticleWeight = "light" | "regular" | "medium" | "semibold";

/** Formulas are rendered as MathML by id so content never carries markup. */
export type ArticleFormulaId = "ascendant";

export type ArticleBlock =
  | { type: "p"; text: string; weight?: ArticleWeight }
  | {
      type: "h2";
      id: string;
      text: string;
      /** "lg" = 32px display title in Figma; default = 24px section title. */
      size?: "lg";
      align?: "start" | "center";
    }
  | { type: "h3"; id: string; text: string }
  | { type: "ul"; items: string[] }
  | { type: "ol"; items: string[] }
  | { type: "split"; blocks: ArticleBlock[]; image: ArticleImage }
  | { type: "image"; image: ArticleImage }
  | {
      type: "timeline";
      label: string;
      items: { title: string; period?: string }[];
    }
  | {
      type: "twoColumnList";
      items: { title: string; subtitle: string; body: string[] }[];
    }
  | { type: "faq"; items: { q: string; a: string }[] }
  | { type: "formula"; formula: ArticleFormulaId; label: string };

export type ArticleSection = {
  id: string;
  blocks: ArticleBlock[];
};

export type ArticleContent = {
  kind: ArticleKind;
  /** Accessible name of the article landmark. */
  label: string;
  sections: ArticleSection[];
};
