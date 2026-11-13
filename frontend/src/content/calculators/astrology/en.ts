// Figma 482:2275 — Astrology Results, long-form section (typos such as
// "dailyhoroscopes" / "Ascendantthe" fixed; formula rendered as MathML).
import type { ArticleContent } from "@/content/calculators/types";

const content: ArticleContent = {
  kind: "astrology",
  label: "Understanding your natal chart",
  sections: [
    {
      id: "natal-chart-vs-sun-sign",
      blocks: [
        { type: "h2", id: "natal-chart-vs-sun-sign", size: "lg", text: "What Is a Full Natal Chart vs. Your Sun Sign?" },
        {
          type: "p",
          weight: "light",
          text: "Most daily horoscopes focus solely on your Sun Sign: the constellation the Sun was passing through at the moment of your birth. While your Sun Sign represents your core identity, vital energy, and conscious ego, it is only one component of a much larger cosmic snapshot.",
        },
        {
          type: "p",
          weight: "light",
          text: "A Full Natal Chart (or horoscope) maps the exact geocentric positions of all ten major celestial bodies (Sun, Moon, Mercury, Venus, Mars, Jupiter, Saturn, Uranus, Neptune, and Pluto) across the 360-degree wheel of the zodiac. Furthermore, a complete chart calculates the local space angles—specifically the Ascendant (Rising Sign) and Midheaven—dividing your chart into twelve distinct life domains known as astrological houses.",
        },
        {
          type: "ul",
          items: [
            "Sun Sign: Your primary core purpose and conscious willpower.",
            "Moon Sign: Your internal emotional landscape, automatic instincts, and psychological needs.",
            "Rising Sign (Ascendant): The filter, physical appearance, and primary perspective through which you navigate the outer world.",
          ],
        },
      ],
    },
    {
      id: "birth-time-and-location",
      blocks: [
        { type: "h2", id: "birth-time-and-location", size: "lg", text: "Why Exact Birth Time and Location Matter" },
        {
          type: "p",
          weight: "semibold",
          text: "Calculating an accurate natal chart requires three coordinates: your birth date, geographic birthplace, and minute-exact birth time.",
        },
        {
          type: "p",
          weight: "light",
          text: "Because Earth rotates 360 degrees on its axis every 24 hours, the local sky advances by approximately 1 degree every 4 minutes. This rotation governs the calculation of your Ascendant: the degree of the zodiac rising on the eastern horizon at the exact minute you were born.",
        },
        {
          type: "formula",
          formula: "ascendant",
          label: "The tangent of the Ascendant equals minus the cosine of RAMC, divided by the sine of RAMC times the cosine of epsilon plus the tangent of phi times the sine of epsilon.",
        },
        {
          type: "p",
          weight: "semibold",
          text: "If your birth time is inaccurate by even 15 to 20 minutes, your calculated Ascendant can shift into an entirely different sign, shifting all twelve house cusps and incorrectly placing your natal planets.",
        },
      ],
    },
    {
      id: "house-systems",
      blocks: [
        { type: "h2", id: "house-systems", size: "lg", text: "Understanding House Systems: Placidus vs. Whole Sign" },
        {
          type: "p",
          weight: "semibold",
          text: "Our natal chart engine lets you choose how the space surrounding your birthplace is divided into houses. The two most popular systems are:",
        },
        {
          type: "p",
          weight: "light",
          text: "**Placidus House System:** The standard system in modern Western astrology. It divides the daytime and nighttime arcs of celestial bodies into equal temporal segments. Because it relies on time arcs, house sizes vary, occasionally producing “intercepted signs” at extreme geographic latitudes.",
        },
        {
          type: "p",
          weight: "light",
          text: "**Whole Sign House System:** The primary system utilized in traditional Hellenistic astrology. The sign containing your Ascendant becomes the entire 1st House (0° to 30°), and every subsequent sign forms the next house in sequence. This creates equal 30° houses throughout the entire chart.",
        },
      ],
    },
    {
      id: "faq",
      blocks: [
        { type: "h2", id: "faq", size: "lg", text: "Frequently Asked Questions" },
        {
          type: "faq",
          items: [
            {
              q: "What if I do not know my exact birth time?",
              a: "If your birth time is unknown, select the “Time Unknown” option in our calculator. The engine will set your chart to 12:00 PM (Noon) local time. While this gives you the planet sign placements (such as your Sun and Venus signs), your Rising Sign and specific House Cusps will not be calculated.",
            },
            {
              q: "What is an aspect in a natal chart?",
              a: "Aspects are geometric angles formed between two or more planets measured along the ecliptic wheel. Major aspects include Conjunctions (0 degrees), Sextiles (60 degrees), Squares (90 degrees), Trines (120 degrees), and Oppositions (180 degrees). They indicate how different facets of your personality interact with one another.",
            },
            {
              q: "Why do different astrology sites show different house placements?",
              a: "Variations occur when different websites utilize different House Calculation Systems (such as Placidus vs. Whole Sign). Select your preferred system under our advanced calculation settings.",
            },
          ],
        },
      ],
    },
  ],
};

export default content;
