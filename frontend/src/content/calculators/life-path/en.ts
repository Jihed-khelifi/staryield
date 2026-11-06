// Figma 1279:1749 — Path Of Life Results, long-form section (artefacts such as
// "\implies" and "$4$" rendered as real symbols).
import type { ArticleContent } from "@/content/calculators/types";

const content: ArticleContent = {
  kind: "life-path",
  label: "Understanding your Life Path Number",
  sections: [
    {
      id: "what-is-a-life-path-number",
      blocks: [
        { type: "h2", id: "what-is-a-life-path-number", text: "What Is a Life Path Number?" },
        {
          type: "p",
          text: "In Pythagorean numerology, your Life Path Number is the most significant coordinate in your entire chart. Calculated directly from your complete date of birth, it represents your soul’s central blueprint, the core character traits you carry, and the primary lessons you are destined to master during your lifetime.",
        },
        {
          type: "p",
          text: "While your birth name reveals your active skills and inner desires, your Life Path Number establishes the overarching environment and themes you will encounter. Whether you carry a single-digit root vibration (1 through 9) or an intensified Master Number (11, 22, or 33), understanding your number provides clarity on your natural strengths, career alignment, and relationship dynamics.",
        },
      ],
    },
    {
      id: "how-to-calculate",
      blocks: [
        { type: "h2", id: "how-to-calculate", text: "How to Calculate Your Life Path Number Correctly" },
        {
          type: "p",
          weight: "light",
          text: "Not all numerology calculators are built equal. Many automated websites use a simple “linear summation” method: adding every digit of a birth date across in a single row. While fast, this shortcut erases intermediate Master Numbers in roughly 1 in 7 birth dates.",
        },
        {
          type: "p",
          weight: "light",
          text: "Our calculator utilizes the traditional Component Reduction (or Period Cycle) method. This approach reduces the three distinct elements of your birth date—the Month, the Day, and the Year—independently before combining them.",
        },
      ],
    },
    {
      id: "component-reduction-method",
      blocks: [
        { type: "h2", id: "component-reduction-method", text: "The 3-Step Component Reduction Method" },
        {
          type: "ol",
          items: [
            "Reduce the Birth Month: Convert your birth month to a single digit (1 through 9) or preserve Master Number 11 (November).",
            "Reduce the Birth Day: Reduce your day of birth to a single digit or preserve Master Numbers 11 or 22 (e.g., the 11th, 22nd, or 29th).",
            "Reduce the Birth Year: Add all four digits of your birth year together and reduce the sum to a single digit or Master Number.",
            "Sum the Results: Add the three reduced values together and perform a final reduction unless the total equals 11, 22, or 33, in which case the Master Number is preserved.",
          ],
        },
        { type: "h3", id: "worked-example", text: "Worked Example: November 29, 1990" },
        {
          type: "ul",
          items: [
            "Month (November): 11 (Master Number preserved)",
            "Day (29th): 2 + 9 = 11 (Master Number preserved)",
            "Year (1990): 1 + 9 + 9 + 0 = 19 ⟹ 1 + 9 = 10 ⟹ 1 + 0 = 1",
            "Final Sum: 11 + 11 + 1 = 23 ⟹ 2 + 3 = 5 ⟹ Life Path 5",
          ],
        },
      ],
    },
    {
      id: "life-path-meanings",
      blocks: [
        {
          type: "twoColumnList",
          items: [
            { title: "Life Path 1", subtitle: "The Leader", body: ["Innovation, independence, self-reliance, and pioneering vision.", "Guarding against self-doubt, impatience, and authoritarian tendencies."] },
            { title: "Life Path 2", subtitle: "The Peacemaker", body: ["Diplomacy, empathy, active listening, and relationship harmony.", "Managing hypersensitivity, over-accommodation, and codependency."] },
            { title: "Life Path 3", subtitle: "The Communicator", body: ["Creative self-expression, optimism, artistic flare, and charisma.", "Overcoming scattered focus, superficiality, and emotional suppression."] },
            { title: "Life Path 4", subtitle: "The Builder", body: ["Discipline, practical organization, systematic execution, and loyalty.", "Avoiding extreme rigidity, stubbornness, and fear of sudden change."] },
            { title: "Life Path 5", subtitle: "The Freedom Seeker", body: ["Adaptability, resourcefulness, adventurous spirit, and versatility.", "Resisting restlessness, impulsivity, and fear of long-term commitment."] },
            { title: "Life Path 6", subtitle: "The Nurturer", body: ["Community care, domestic harmony, protective responsibility, and balance.", "Learning to serve without slipping into self-martyrdom or control."] },
            { title: "Life Path 7", subtitle: "The Seeker", body: ["Analytical research, spiritual inquiry, intuition, and technical mastery.", "Preventing cynical isolation, overthinking, and emotional detachment."] },
            { title: "Life Path 8", subtitle: "The Powerhouse", body: ["Executive leadership, financial mastery, strategic vision, and authority.", "Balancing material ambition with ethical responsibility and generosity."] },
            { title: "Life Path 9", subtitle: "The Humanitarian", body: ["Universal compassion, artistic maturity, altruism, and completion.", "Letting go of past attachments and establishing healthy boundaries."] },
          ],
        },
      ],
    },
    {
      id: "master-numbers",
      blocks: [
        { type: "h2", id: "master-numbers", text: "The Sacred Master Numbers: 11, 22, and 33" },
        {
          type: "p",
          text: "In numerology, double-digit numbers with matching digits (11, 22, 33) carry heightened vibrational potential. Known as Master Numbers, they operate as higher octaves of their single-digit root bases (2, 4, and 6).",
        },
        {
          type: "ul",
          items: [
            "**Life Path 11/2 (The Intuitive Illuminator):** Combines the peacemaking qualities of Root 2 with high-voltage intuition and visionary insight. Those on Path 11 act as spiritual antennas, though they often navigate high internal nervous energy.",
            "**Life Path 22/4 (The Master Builder):** Takes the structural grounding of Root 4 and scales it to grand proportions. Path 22 individuals possess the rare ability to manifest abstract visionary dreams into tangible, lasting real-world infrastructure.",
            "**Life Path 33/6 (The Master Teacher):** Enhances the nurturing qualities of Root 6 into universal altruism and spiritual service. Path 33 represents selfless guidance and healing, requiring high emotional maturity to prevent burnout.",
          ],
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
              q: "Does an adopted name change my Life Path Number?",
              a: "No. Your Life Path Number is calculated exclusively from your date of birth, making it permanently fixed regardless of name changes or legal adoption.",
            },
            {
              q: "Are Master Numbers better or luckier than single-digit numbers?",
              a: "No. Master Numbers (11, 22, 33) bring greater potential, but they also bring higher psychological pressure and intensified life lessons. A single-digit Life Path 4 or 8 is just as capable of achieving massive success and fulfillment as a Master Number.",
            },
            {
              q: "Why do different numerology websites give me different Life Path numbers?",
              a: "If your birth date contains sub-cycles that equal 11 or 22, simplified calculators that add all digits across in one row often reduce those Master Numbers prematurely. Our calculator uses the strict Component Reduction method to guarantee accurate intermediate calculations.",
            },
          ],
        },
      ],
    },
  ],
};

export default content;
