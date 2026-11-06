/**
 * Zodiac sign and planet glyphs (transparent SVGs exported from the Figma
 * assets page, node 133:442, and served from R2). Revisions 2.0: the old
 * PNG icons had white backgrounds.
 */
export const zodiacIcons: Record<string, string> = {
  aquarius: "https://assets.staryield.net/assets/zodiac/v2/aquarius-f796a3a6bc.svg",
  aries: "https://assets.staryield.net/assets/zodiac/v2/aries-fd711723b9.svg",
  cancer: "https://assets.staryield.net/assets/zodiac/v2/cancer-51e4ceb47f.svg",
  capricorn: "https://assets.staryield.net/assets/zodiac/v2/capricorn-bb33ab6275.svg",
  gemini: "https://assets.staryield.net/assets/zodiac/v2/gemini-0531f9a034.svg",
  leo: "https://assets.staryield.net/assets/zodiac/v2/leo-b976af92a6.svg",
  libra: "https://assets.staryield.net/assets/zodiac/v2/libra-8e5d075d6d.svg",
  pisces: "https://assets.staryield.net/assets/zodiac/v2/pisces-7ffc7a911b.svg",
  sagittarius: "https://assets.staryield.net/assets/zodiac/v2/sagittarius-06e02d23f3.svg",
  scorpio: "https://assets.staryield.net/assets/zodiac/v2/scorpio-91ebdf2e74.svg",
  taurus: "https://assets.staryield.net/assets/zodiac/v2/taurus-4a7439db96.svg",
  virgo: "https://assets.staryield.net/assets/zodiac/v2/virgo-c6d87c9213.svg",
};

export const planetIcons: Record<string, string> = {
  ascendant: "https://assets.staryield.net/assets/planets/v2/ascendant-3d5fd3fdd8.svg",
  jupiter: "https://assets.staryield.net/assets/planets/v2/jupiter-b354447969.svg",
  mars: "https://assets.staryield.net/assets/planets/v2/mars-4120f90aa2.svg",
  mercury: "https://assets.staryield.net/assets/planets/v2/mercury-dec98efec2.svg",
  moon: "https://assets.staryield.net/assets/planets/v2/moon-72dd3db430.svg",
  neptune: "https://assets.staryield.net/assets/planets/v2/neptune-3d0796ab70.svg",
  pluto: "https://assets.staryield.net/assets/planets/v2/pluto-6c283e3a8a.svg",
  saturn: "https://assets.staryield.net/assets/planets/v2/saturn-ee85cef7b9.svg",
  sun: "https://assets.staryield.net/assets/planets/v2/sun-42f09ca443.svg",
  uranus: "https://assets.staryield.net/assets/planets/v2/uranus-7d7c7e2931.svg",
  venus: "https://assets.staryield.net/assets/planets/v2/venus-05c7ab07c9.svg",
};

/** The glyph URL of a sign ("Leo", "leo"), or undefined for an unknown one. */
export function zodiacIconUrl(sign: string | null | undefined): string | undefined {
  return sign ? zodiacIcons[sign.trim().toLowerCase()] : undefined;
}

/** The glyph URL of a planet or angle ("Sun", "Ascendant"), or undefined. */
export function planetIconUrl(body: string | null | undefined): string | undefined {
  return body ? planetIcons[body.trim().toLowerCase()] : undefined;
}
