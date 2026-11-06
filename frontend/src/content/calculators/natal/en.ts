// Figma 482:2010 (node 1258:1317) — Natal Chart Results, long-form section.
import type { ArticleContent, ArticleImage } from "@/content/calculators/types";

export const natalIntroImage: Omit<ArticleImage, "alt"> = {
  src: "https://assets.staryield.net/assets/articles/natal/twelve-houses-intro-0cf68eb2.png",
  width: 640,
  height: 427,
  position: "bottom",
  frame: { width: 426, height: 289, radius: 13 },
};
export const natalDiagramImage: Omit<ArticleImage, "alt"> = {
  src: "https://assets.staryield.net/assets/articles/natal/energy-mapping-diagram-d92680b7.png",
  width: 736,
  height: 920,
  frame: { width: 373, height: 466, radius: 8 },
};

const content: ArticleContent = {
  kind: "natal",
  label: "About the Twelve Houses Tarot Spread",
  sections: [
    {
      id: "twelve-houses-spread",
      blocks: [
        {
          type: "split",
          blocks: [
            {
              type: "p",
              weight: "light",
              text: "The systematic integration of astrological geometry with cartomantic symbolism represents one of the most enduring developments in Western divinatory traditions. Among the various layouts designed to unify these systems, the Twelve Houses Tarot Spread serves as a comprehensive diagnostic framework. By mapping tarot cards onto the twelve divisions of the astrological chart, this layout projects the multi-faceted nature of human experience onto an organized wheel.",
            },
          ],
          image: { ...natalIntroImage, alt: "Tarot cards spread out on a patterned cloth" },
        },
      ],
    },
    {
      id: "history",
      blocks: [
        {
          type: "p",
          text: "The practice of arranging divinatory cards in an astrological wheel evolved over more than two centuries, moving from late 18th-century French cartomancy through Victorian esoteric societies to 20th-century popular publications.",
        },
        {
          type: "timeline",
          label: "Historical timeline",
          items: [
            { title: "Etteilla", period: "(1785–1791)" },
            { title: "Golden Dawn", period: "(1888)" },
            { title: "Russell Grant", period: "(1992)" },
            { title: "Digital Web Calculators" },
          ],
        },
        { type: "h3", id: "etteilla", text: "The Pre-Esoteric and Etteilla Eras (1785–1791)" },
        {
          type: "p",
          weight: "light",
          text: "The formal merging of tarot with astrology was initiated by Jean-Baptiste Alliette (1738–1791), an influential French cartomancer who published under the pseudonym Etteilla. Prior to Etteilla’s work, fortune-tellers typically pulled individual cards sequentially from the top of a deck, reading each in isolation. In his 1770 text Manière de se récréer avec un jeu des cartes and his subsequent tarot publications between 1785 and 1791—including Jeu des tarots ou le livre de Thot—Etteilla established structural cartomancy.",
        },
        {
          type: "p",
          weight: "light",
          text: "Etteilla introduced the earliest documented 12-house astrological wheel layout, which he designated as the “Horoscope Spread” or “Spread of Twelve”. He assigned specific zodiac signs, planetary bodies, and elemental temperaments to individual cards, demonstrating that laying cards within designated astrological houses alters their baseline interpretations according to the specific domain governed by each house.",
        },
        { type: "h3", id: "golden-dawn", text: "The Golden Dawn Synthesis (1888)" },
        {
          type: "p",
          weight: "light",
          text: "While Etteilla established early correspondences, the Hermetic Order of the Golden Dawn—founded in 1888—codified the esoteric synthesis of tarot, astrology, and Kabbalah. The Golden Dawn system mapped the 22 Major Arcana directly onto the 22 paths of the Tree of Life, corresponding to the 12 Zodiac signs, 7 classical planets, and 3 primary elements.",
        },
        {
          type: "p",
          weight: "light",
          text: "In the Golden Dawn’s master divinatory ritual, known as the Opening of the Key, the second operation explicitly requires dealing the deck into twelve distinct piles corresponding to the astrological houses of heaven. This operation served a specific diagnostic purpose: the house in which the querent’s Significator card landed identified the precise life domain where the underlying inquiry was manifesting.",
        },
        { type: "h3", id: "russell-grant", text: "Late 20th-Century Popularization and Russell Grant’s Astro-Tarot (1992)" },
        {
          type: "p",
          weight: "light",
          text: "In 1992, British astrologer Russell Grant released Russell Grant’s Astro-Tarot Pack, published by Virgin Books and illustrated by Kay Smith. Grant simplified esoteric cartomancy for a broader audience by designing a custom 64-card hybrid deck.",
        },
        {
          type: "p",
          weight: "light",
          text: "Grant modified the standard 78-card structure by omitting the numbered Minor Arcana pip cards (2 through 10) while retaining the 22 Major Arcana and the 4 Aces. To complete the 64-card set, he added specialized cards:",
        },
        {
          type: "ul",
          items: [
            "12 cards representing the signs of the Western Zodiac.",
            "12 cards depicting the animals of the Chinese Zodiac.",
            "14 cards representing planetary bodies, lunar phases (New Moon and Full Moon), and key astrological angles (Ascendant, Midheaven, and North Node).",
          ],
        },
        {
          type: "p",
          weight: "light",
          text: "Grant’s guidebook featured the Twelve Houses Spread as its foundational analytical spread. By pairing an accessible deck design with positional house definitions, Grant demonstrated how readers could utilize astrological positions to ground card meanings within concrete life areas, such as employment, real estate, and personal relationships.",
        },
      ],
    },
    {
      id: "structural-mechanics",
      blocks: [
        {
          type: "h2",
          id: "structural-mechanics",
          size: "lg",
          align: "center",
          text: "Structural Mechanics and Architectural Foundations of the Spread",
        },
        {
          type: "split",
          blocks: [
            {
              type: "p",
              weight: "medium",
              text: "The Twelve Houses Spread is constructed by placing twelve cards in a counterclockwise wheel that mirrors the natural sequence of the tropical zodiac. The arrangement begins with the First House at the traditional Ascendant position (9 o’clock) and proceeds sequentially around the circle, ending with the Twelfth House.",
            },
            {
              type: "p",
              text: "Interpreting the Twelve Houses Spread effectively requires evaluating structural relationships across the entire chart rather than analyzing twelve isolated cards. Expert readers utilize cross-house triangulation and polarity dynamics to extract nuanced readings.",
            },
          ],
          image: {
            ...natalDiagramImage,
            alt: "“Energy Mapping Tarot Reading” diagram: twelve card positions arranged in a wheel and numbered 1 to 12",
          },
        },
        {
          type: "twoColumnList",
          items: [
            { title: "1st House", subtitle: "Mars / Aries (Ascendant) Self & Identity", body: ["Physical appearance, core personality, overall vitality, personal projects, and immediate outlook. Serves as the primary anchor for the overall inquiry."] },
            { title: "2nd House", subtitle: "Venus / Taurus Material Values & Money", body: ["Personal finances, liquid assets, earned income, personal possessions, material security, self-worth, and major purchases."] },
            { title: "3rd House", subtitle: "Mercury / Gemini Mind & Communication", body: ["Daily mental focus, short-distance travel, immediate communication, siblings, neighbors, writing, and prevailing daily thoughts."] },
            { title: "4th House", subtitle: "Moon / Cancer (IC) Home & Ancestry", body: ["Domestic life, physical residence, family origins, emotional security, psychological foundations, real estate moves, and maternal ties."] },
            { title: "5th House", subtitle: "Sun / Leo Creativity & Romance", body: ["New romance, leisure pursuits, casual relationships, creative expression, hobbies, entertainment, self-expression, and children."] },
            { title: "6th House", subtitle: "Mercury / Virgo Daily Labor & Wellness", body: ["Everyday employment conditions, routine service, personal habits, physical health matters, workplace environment, and daily responsibilities."] },
            { title: "7th House", subtitle: "Venus / Libra (Descendant) Committed Partnerships", body: ["Formal marriage, business partnerships, binding contracts, public interactions, legal alliances, and open opponents."] },
            { title: "8th House", subtitle: "Pluto / Scorpio Transformation & Shared Assets", body: ["Shared finances, joint debts, taxes, inheritances, deep emotional investments, psychological rebirth, intimacy, and life cycles."] },
            { title: "9th House", subtitle: "Jupiter / Sagittarius Higher Mind & Expansion", body: ["Higher education, legal affairs, long-distance travel, philosophical beliefs, publishing, moral frameworks, and perspective shifts."] },
            { title: "10th House", subtitle: "Saturn / Capricorn (MC) Public Status & Career", body: ["Long-term vocational ambitions, professional reputation, public standing, duty, corporate authority, and paternal influence."] },
            { title: "11th House", subtitle: "Uranus / Aquarius Social Networks & Ambitions", body: ["Social circles, group affiliations, long-term ideals, community involvement, future plans, and potential romantic interests arising from friendships."] },
            { title: "12th House", subtitle: "Neptune / Pisces Subconscious & Seclusion", body: ["Solitude, hidden motives, unconscious patterns, spiritual retreat, unseen obstacles, hidden fears, and developments waiting to emerge."] },
          ],
        },
      ],
    },
    {
      id: "advanced-techniques",
      blocks: [
        {
          type: "h2",
          id: "advanced-techniques",
          size: "lg",
          text: "Advanced Interpretive Techniques: Triads, Axes, and Diagnostic Dynamics",
        },
        { type: "h3", id: "cross-house-triangulation", text: "Cross-House Triangulation" },
        {
          type: "p",
          weight: "semibold",
          text: "Triangulation involves analyzing linked houses together to gain deep context on specific personal topics.",
        },
        {
          type: "ul",
          items: [
            "**The Relationship Triad (Houses 5, 7, and 8):** Analyzing House 5 reveals initial romantic interest, attraction, and casual dating. House 7 dictates formal commitments, legal marriage, and long-term partnership dynamics. House 8 reflects underlying emotional intimacy, sexual compatibility, and shared financial obligations. Favorable cards in House 5 paired with challenging cards in House 7 indicate exciting initial romance that encounters structural friction when moving toward long-term commitment.",
            "**The Career and Achievement Axis (Houses 6 and 10):** House 6 governs daily tasks, routine employment, and immediate job satisfaction. House 10 represents long-term career ambition, public reputation, and professional achievement. Discrepancies between these houses highlight occupational imbalances; strong cards in House 10 combined with stressful cards in House 6 signal professional recognition achieved at the cost of grueling daily labor.",
            "**The Cognitive and Perspective Axis (Houses 3 and 9):** House 3 details daily thoughts, immediate worries, and local interactions. House 9 represents overarching philosophy, higher education, and broader outlook. When a querent experiences mental overload in House 3, negative cards in House 9 indicate that an overly narrow perspective is obstructing personal growth.",
          ],
        },
        { type: "h3", id: "polarity-axes", text: "Polarity Axes and Structural Tension" },
        {
          type: "p",
          weight: "semibold",
          text: "Astrological houses exist in six complementary pairs directly opposing each other on the wheel. Analyzing these opposing axes reveals core tensions and balancing requirements:",
        },
        {
          type: "ul",
          items: [
            "**1st House / 7th House (The Axis of Identity):** Balances personal autonomy and self-directed goals against the obligations and compromises required by committed partnerships.",
            "**2nd House / 8th House (The Axis of Resources):** Opposes individual earnings and personal property with shared investments, joint debts, taxes, and marital assets.",
            "**3rd House / 9th House (The Axis of Mind):** Juxtaposes concrete local information and daily thinking with abstract philosophy, higher learning, and worldviews.",
            "**4th House / 10th House (The Axis of Origin and Status):** Balances private domestic life, family security, and psychological roots with public career goals, reputation, and social duty.",
          ],
        },
      ],
    },
  ],
};

export default content;
