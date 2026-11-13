/**
 * Life path portraits for the Path of Life result (Figma 482:2214 "Life
 * Path 6 · The nurturer"). English source copy, translated with `text()`.
 */
export const LIFE_PATH_COPY: Record<number, { title: string; body: string }> = {
  1: {
    title: "The leader",
    body: "Independent, driven and original. Life Path 1s are born to start things and to stand on their own feet. Their dark side is impatience, stubbornness and a fear of depending on anyone.",
  },
  2: {
    title: "The peacemaker",
    body: "Diplomatic, sensitive and cooperative. Life Path 2s build bridges and notice what others miss. Their dark side is over-accommodating, avoiding conflict and losing themselves in others.",
  },
  3: {
    title: "The communicator",
    body: "Expressive, creative and optimistic. Life Path 3s turn feelings into words, art and joy. Their dark side is scattered energy and hiding behind charm instead of facing hard emotions.",
  },
  4: {
    title: "The builder",
    body: "Practical, loyal and disciplined. Life Path 4s create stability through steady, methodical work. Their dark side is rigidity, stubbornness and a fear of sudden change.",
  },
  5: {
    title: "The freedom seeker",
    body: "Curious, adaptable and adventurous. Life Path 5s learn by experience and thrive on variety. Their dark side is restlessness, impulsiveness and a fear of commitment.",
  },
  6: {
    title: "The nurturer",
    body: "Deeply responsible, prioritizing family, domestic life, and acts of service. Life Path 6s are the ultimate caregivers. Their dark side is a tendency to become self-righteous meddlers or professional martyrs.",
  },
  7: {
    title: "The seeker",
    body: "Analytical, intuitive and private. Life Path 7s look for the truth beneath the surface. Their dark side is isolation, cynicism and overthinking instead of trusting their inner voice.",
  },
  8: {
    title: "The powerhouse",
    body: "Ambitious, strategic and resilient. Life Path 8s are drawn to leadership, money and achievement. Their dark side is control, workaholism and measuring worth only by success.",
  },
  9: {
    title: "The humanitarian",
    body: "Compassionate, idealistic and wise. Life Path 9s give generously and see the bigger picture. Their dark side is martyrdom and holding on to the past instead of letting go.",
  },
  11: {
    title: "The intuitive illuminator",
    body: "A master number: highly intuitive, inspired and sensitive. Life Path 11s act as spiritual antennas for those around them. Their challenge is nervous tension and self-doubt that can dim their light.",
  },
  22: {
    title: "The master builder",
    body: "A master number: visionary and practical at once. Life Path 22s can turn big dreams into lasting structures. Their challenge is the pressure of their own potential and fear of falling short.",
  },
  33: {
    title: "The master teacher",
    body: "A master number: devoted to compassion, guidance and selfless service. Life Path 33s uplift others through kindness. Their challenge is burnout from carrying everyone else's burdens.",
  },
};

/** What each number card means (Figma 482:2202 – 482:2213). */
export const NUMBER_CARDS = [
  {
    key: "lifePath",
    label: "Life Path",
    body: "The foundational blueprint of your earthly journey, structuring your natural potential and ultimate purpose.",
  },
  {
    key: "personalYear",
    label: "Personal year",
    body: "The overarching theme and energetic vibration defining your current twelve-month developmental cycle.",
  },
  {
    key: "personalMonth",
    label: "Personal month",
    body: "Your immediate energetic climate, highlighting focused opportunities and acute lessons for this cycle.",
  },
] as const;
