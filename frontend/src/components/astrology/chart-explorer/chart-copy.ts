/**
 * English source copy for natal chart interpretations (translated through
 * `text()` from copy-*.json). Descriptions are composed from the planet, the
 * sign it occupies and its house, so they always match the computed chart.
 */
export const planetMeaning: Record<string, string> = {
  Ascendant: "Your ascendant is the “mask” you present when meeting others — the first impression you make and the way you begin new things.",
  Sun: "The Sun reflects your core identity, vitality, and conscious sense of self.",
  Moon: "The Moon describes your emotional needs, your instincts, and what helps you feel safe.",
  Mercury: "Mercury governs how you think, learn, and communicate.",
  Venus: "Venus rules love, beauty, pleasure, and what you value most.",
  Mars: "Mars shapes how you assert yourself, pursue your goals, and express desire.",
  Jupiter: "Jupiter points to growth, generosity, and where opportunity tends to find you.",
  Saturn: "Saturn represents structure, responsibility, and the lessons that build mastery.",
  Uranus: "Uranus is a generational influence tied to change, independence, and invention.",
  Neptune: "Neptune is a generational influence tied to dreams, imagination, and compassion.",
  Pluto: "Pluto is a generational force of transformation, depth, and renewal.",
};

export const signFlavor: Record<string, string> = {
  Aries: "In Aries, this energy is direct, courageous, and eager to begin.",
  Taurus: "In Taurus, it seeks steadiness, comfort, and lasting value.",
  Gemini: "In Gemini, it is curious, quick, and fed by conversation.",
  Cancer: "In Cancer, it is protective, intuitive, and rooted in care.",
  Leo: "In Leo, it shines through warmth, creativity, and generous self-expression.",
  Virgo: "In Virgo, it is precise, practical, and devoted to improvement.",
  Libra: "In Libra, it looks for harmony, fairness, and meaningful partnership.",
  Scorpio: "In Scorpio, it runs deep, intense, and quietly transformative.",
  Sagittarius: "In Sagittarius, it is adventurous, hopeful, and searching for meaning.",
  Capricorn: "In Capricorn, it is patient, disciplined, and focused on what endures.",
  Aquarius: "In Aquarius, it is independent, inventive, and community-minded.",
  Pisces: "In Pisces, it is gentle, imaginative, and deeply empathetic.",
};

export const houseTheme: Record<number, string> = {
  1: "self-image and new beginnings",
  2: "resources and self-worth",
  3: "communication and everyday learning",
  4: "home and family",
  5: "creativity, play, and romance",
  6: "daily routines, work, and wellbeing",
  7: "partnerships and close relationships",
  8: "intimacy, shared resources, and transformation",
  9: "beliefs, travel, and higher learning",
  10: "career and public life",
  11: "friendships, community, and long-term hopes",
  12: "solitude, reflection, and the inner world",
};

export const planetSymbol: Record<string, string> = {
  Ascendant: "↑",
  Sun: "☉",
  Moon: "☽",
  Mercury: "☿",
  Venus: "♀",
  Mars: "♂",
  Jupiter: "♃",
  Saturn: "♄",
  Uranus: "♅",
  Neptune: "♆",
  Pluto: "♇",
};
