/**
 * Headings, labels and sentence templates of the calculators' result views
 * that the e-mailed report prints too (revisions 6.0, §26.4). English source
 * copy, translated with `text()` like every other string
 * (src/i18n/dictionaries/copy-*.json).
 *
 * This file is one of the sources of `npm run gen:report-copy`, which writes
 * backend/internal/mail/reportcopy/report-copy.json (the API embeds it); a
 * vitest test fails when the committed JSON is stale. Keep it free of
 * imports: the generator transpiles it on its own.
 */
export const REPORT_LABELS = {
  /** Title of every result ("Natal alignment calculated for: 30 / 04 / 1945"). */
  headline: "Natal alignment calculated for:",
  birthTimeUnknown: "Birth time unknown",
  // Natal chart: the Twelve Houses spread.
  spreadTitle: "Your spread",
  spreadIntro: "A 12-card spread exploring the current landscape, themes, and guidance for your journey.",
  spreadUncertain: "Without an exact birth time, cards with a dashed frame may change. Add your time and place for a more detailed spread.",
  cardsHeading: "Your cards and why they were drawn",
  houseNumber: "House {value0}",
  reversed: "Reversed",
  cardPlanet: "{value0} in {value1}, in the {value2}",
  cardRetrograde: "retrograde, so the card is reversed",
  cardCusp: "Cusp sign: {value0} (no planet in this house)",
  cardMayChange: "This card may change with your exact birth time.",
  housesLine: "Houses: {value0}.",
  housesUnknown: "Birth time unknown: houses are counted whole-sign from your Sun.",
  housesFallback: "Placidus is undefined this close to the poles, so Porphyry houses were used.",
  wallTimeSkipped: "That clock time did not exist there (the clocks went forward), so the time just before the change was used.",
  wallTimeAmbiguous: "That clock time happened twice there (the clocks went back), so the earlier one was used.",
  wallTimeLmt: "Your birthplace kept local mean time then, so its own mean time was used.",
  natalCta: "Get a better understanding of what this spread means with the guidance of one of our amazing psychics!",
  clickHere: "Click here!",
  // Astrology: the sun sign.
  zodiacTitle: "Your zodiac",
  cuspTitle: "You were born on a cusp: the Sun moved from {value0} into {value1} that day.",
  cuspNote: "We show {value0}, the sign for your date at noon. Add your birth time and place to know for sure.",
  sunSignLead: "According to your alignment, the sun resides in the sign of",
  signFacts: "{value0} is ruled by {value1} and belongs to the {value2} element ({value3}).",
  zodiacCtaTitle: "Want to discover your planetary coordinates?",
  zodiacCtaBody: "Sign up to unlock details on your elemental balance, ascending, moon and sun signs, and complete alignment data.",
  signUp: "Sign in / up",
  openMyChart: "Open my chart",
  // Path of life: the numbers.
  numbersTitle: "Your numbers",
  personalCycles: "Personal cycles for {value0}, your local date.",
  lifePathTitle: "Life path {value0}",
  howItAddsUp: "How it adds up:",
  month: "Month",
  day: "Day",
  year: "Year",
  stepLine: "{value0}: {value1}",
  stepTotal: "Total: {value0}",
  lifePathCtaTitle: "Get a deeper understanding of what a life path really means and the alignment that comes with it",
  lifePathCtaBody: "Chat with one of our psychic advisors to explore your chart and upcoming planetary transitions, for entertainment purposes only.",
  // What the API's calculators answer (names the views translate with text()).
  disclaimer: "Offered for reflection and entertainment. Your choices remain your own.",
} as const;

export type ReportLabelKey = keyof typeof REPORT_LABELS;

/** House systems the natal calculator can answer (the API's `house_system_used`). */
export const HOUSE_SYSTEM_NAMES: Record<string, string> = {
  placidus: "Placidus",
  whole_sign: "Whole Sign",
  equal: "Equal",
  porphyry: "Porphyry",
};

/** Bodies that can source a tarot card (the API's `source.body`). */
export const CARD_BODIES = ["Sun", "Moon", "Mercury", "Venus", "Mars", "Jupiter", "Saturn", "Uranus", "Neptune", "Pluto"] as const;

/** Elements and modalities the sun-sign answer names. */
export const ELEMENTS = ["Fire", "Earth", "Air", "Water"] as const;
export const MODALITIES = ["Cardinal", "Fixed", "Mutable"] as const;
