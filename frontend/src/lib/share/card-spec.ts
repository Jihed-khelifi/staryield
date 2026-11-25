/**
 * What a share image says, per calculator, as plain data. The spec is built
 * from the result in the viewer's language and carries only what is meant to
 * be shared: card and sign names, numbers and a few words. It never holds the
 * birth date, time, place or any name, so nothing personal can reach the image
 * (the card-spec tests assert it).
 */
import type { TranslateCopy } from "@/i18n/translate-copy";
import type { CalculatorResult } from "@/components/calculators/calculate";
import { LIFE_PATH_COPY } from "@/lib/astro/life-path-copy";
import { cardNumeral } from "@/lib/astro/tarot-birth-steps";
import { SIGN_ARTWORK, isSign } from "@/lib/astro/zodiac";
import type { CalculatorKind } from "@/lib/schemas/calculator";

/** The site's address as printed on every image. */
export const SHARE_DOMAIN = "staryield.net";

export type ShareArtwork = {
  /** Public R2 artwork URL. */
  url: string;
  /** Width over height, used when the image reports no size. */
  aspect: number;
  /**
   * How it is drawn: "card" is a framed picture (tarot artwork, side by side
   * when there are several); "emblem" is a glyph drawn in gold inside the
   * card's medallion (a zodiac sign; the two signs of a cusp side by side).
   */
  style: "card" | "emblem";
  /** Small caption under the picture (a Roman numeral). */
  caption?: string;
};

export type ShareCardSpec = {
  kind: CalculatorKind;
  /** Small capitals above the headline ("Tarot birth card"). */
  eyebrow: string;
  /** "My tarot birth card is The Star". */
  headline: string;
  /** Sentences under the headline. */
  lines: string[];
  /** A large figure in the medallion when there is no artwork ("7"). */
  badge?: string;
  artwork: ShareArtwork[];
  /** Text of the native share sheet: the headline and where to find yours. */
  shareText: string;
  /** What the image shows, for assistive technology (preview alt text). */
  alt: string;
};

/** The tarot artwork's frame (the site's card images are 210 × 355). */
const TAROT_ASPECT = 210 / 355;

function finish(spec: Omit<ShareCardSpec, "shareText" | "alt">, text: TranslateCopy): ShareCardSpec {
  const detail = [spec.headline, ...spec.lines].join(". ");
  return {
    ...spec,
    shareText: text("{value0} — find yours at {value1}", { value0: spec.headline, value1: SHARE_DOMAIN }),
    alt: text("Share image: {value0}", { value0: detail }),
  };
}

/** The zodiac glyphs of the signs (R2 artwork) as the card's emblems; unknown signs are left out. */
function signEmblems(...signs: string[]): ShareArtwork[] {
  return signs.filter(isSign).map((sign) => {
    const art = SIGN_ARTWORK[sign];
    return { url: art.src, aspect: art.width / art.height, style: "emblem" as const };
  });
}

type SunCertainty = { uncertain?: boolean; alternatives?: string[] } | { sun_sign_uncertain?: boolean; sun_sign_alternatives?: string[] };

/**
 * The two signs of a cusp birth day, in the order the Sun crossed them, when
 * the birth time is unknown and the Sun changed signs that day (the sun sign
 * result's `uncertain`, the natal chart's `sun_sign_uncertain`); null when
 * the Sun sign is certain.
 */
export function sunCuspOf(sun: SunCertainty): [string, string] | null {
  const [uncertain, alternatives] = "sun_sign_uncertain" in sun ? [sun.sun_sign_uncertain, sun.sun_sign_alternatives] : "uncertain" in sun ? [sun.uncertain, sun.alternatives] : [false, undefined];
  const signs = alternatives ?? [];
  const first = signs[0];
  const last = signs[signs.length - 1];
  return uncertain && first && last && first !== last ? [first, last] : null;
}

/**
 * "Sun in Pisces · Moon in Libra · Taurus rising": only the parts that are
 * known. A cusp day without a birth time names both possible Sun signs
 * ("Sun in Aquarius or Pisces"); an uncertain Moon is left out.
 */
export function natalHeadline(
  chart: { sun_sign: string; sun_sign_uncertain?: boolean; sun_sign_alternatives?: string[]; moon_sign?: string; moon_sign_uncertain?: boolean; rising_sign?: string },
  text: TranslateCopy,
): string {
  const cusp = sunCuspOf(chart);
  const parts = [cusp ? text("Sun in {value0} or {value1}", { value0: text(cusp[0]), value1: text(cusp[1]) }) : text("Sun in {value0}", { value0: text(chart.sun_sign) })];
  if (chart.moon_sign && !chart.moon_sign_uncertain) parts.push(text("Moon in {value0}", { value0: text(chart.moon_sign) }));
  if (chart.rising_sign) parts.push(text("{value0} rising", { value0: text(chart.rising_sign) }));
  return parts.join(" · ");
}

/** The share card of a calculator result, in the language of `text`. */
export function shareCardSpec(result: CalculatorResult, text: TranslateCopy): ShareCardSpec {
  switch (result.kind) {
    case "tarot-birth-card": {
      const cards = result.tarot.birthCards;
      const personality = cards[0];
      const soul = cards[cards.length - 1];
      return finish(
        {
          kind: result.kind,
          eyebrow: text("Tarot birth card"),
          headline: text("My tarot birth card is {value0}", { value0: text(personality.name) }),
          lines:
            cards.length > 1
              ? [text("Soul card: {value0}", { value0: text(soul.name) })]
              : [text("Personality and soul card in one")],
          artwork: cards.map((card) => ({ url: card.image, aspect: TAROT_ASPECT, style: "card", caption: cardNumeral(card) })),
        },
        text,
      );
    }
    case "path-of-life": {
      const number = result.numbers.lifePath.number;
      const copy = LIFE_PATH_COPY[number];
      return finish(
        {
          kind: result.kind,
          eyebrow: text("Path of life"),
          headline: text("My life path number is {value0}", { value0: number }),
          lines: copy ? [text(copy.title)] : [],
          badge: String(number),
          artwork: [],
        },
        text,
      );
    }
    case "astrology": {
      const sign = result.sun.sun_sign;
      const cusp = sunCuspOf(result.sun);
      if (cusp)
        // A cusp day without a birth time: both signs, and no element or modality (they differ).
        return finish(
          {
            kind: result.kind,
            eyebrow: text("Zodiac sign"),
            headline: text("I was born on the {value0}–{value1} cusp", { value0: text(cusp[0]), value1: text(cusp[1]) }),
            lines: [],
            artwork: signEmblems(...cusp),
          },
          text,
        );
      return finish(
        {
          kind: result.kind,
          eyebrow: text("Zodiac sign"),
          headline: text("I'm a {value0}", { value0: text(sign) }),
          lines: [[result.sun.element, result.sun.modality].filter(Boolean).map((word) => text(word)).join(" · ")].filter(Boolean),
          artwork: signEmblems(sign),
        },
        text,
      );
    }
    case "natal-chart": {
      const chart = result.natal.chart;
      return finish(
        {
          kind: result.kind,
          eyebrow: text("My birth chart"),
          headline: natalHeadline(chart, text),
          lines: [],
          artwork: signEmblems(...(sunCuspOf(chart) ?? [chart.sun_sign])),
        },
        text,
      );
    }
  }
}
