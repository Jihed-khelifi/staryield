/**
 * What each Major Arcana card means as a birth card: three keywords and a
 * short reading, keyed by birth number (1–22; 22 is The Fool). English source
 * copy, translated with `text()` from copy-*.json (tarot-birth-steps.test.ts
 * checks that every string exists in all three locales). For entertainment,
 * like the rest of the calculator.
 */
export type TarotBirthCopy = { keywords: string; meaning: string };

export const TAROT_BIRTH_COPY: Record<number, TarotBirthCopy> = {
  1: {
    keywords: "Willpower, skill, initiative",
    meaning:
      "You were born to make things happen. Resourceful and persuasive, you turn ideas into action and learn quickly by doing. Your lesson is to use your talent with integrity and to finish what you start.",
  },
  2: {
    keywords: "Intuition, mystery, inner knowing",
    meaning:
      "You sense what is left unsaid and often understand things before you can explain them. Calm and observant, you carry a quiet depth. Your lesson is to trust your inner voice and to share what you know instead of keeping it hidden.",
  },
  3: {
    keywords: "Nurture, creativity, abundance",
    meaning:
      "You bring warmth, beauty and growth to whatever you touch, from a home to a project. Generous and sensual, you help people and ideas flourish. Your lesson is to care for others without forgetting yourself.",
  },
  4: {
    keywords: "Structure, authority, stability",
    meaning:
      "You build solid foundations and take responsibility naturally. Others turn to you for order, protection and a clear plan. Your lesson is to lead with fairness and to stay open to change instead of controlling everything.",
  },
  5: {
    keywords: "Tradition, teaching, belonging",
    meaning:
      "You value shared beliefs, learning and meaningful rituals, and you are often the one who explains and mentors. You find strength in community and in wisdom handed down. Your lesson is to question what you were taught and to follow what is true for you.",
  },
  6: {
    keywords: "Choice, union, values",
    meaning:
      "Relationships and decisions of the heart shape your path. You are drawn to deep connection and to living by your values. Your lesson is to choose with honesty rather than out of fear or habit, and to love without losing yourself.",
  },
  7: {
    keywords: "Drive, determination, victory",
    meaning:
      "You are focused, ambitious and willing to work for what you want. You steer opposing forces in one direction and keep moving when others stall. Your lesson is to balance your will with flexibility and to enjoy the journey, not only the win.",
  },
  8: {
    keywords: "Courage, patience, compassion",
    meaning:
      "Your power is gentle but steady: you tame fear with patience and kindness rather than force. People feel safe near your calm. Your lesson is to trust your own resilience and to be as kind to yourself as you are to others.",
  },
  9: {
    keywords: "Solitude, wisdom, guidance",
    meaning:
      "You look for meaning beneath the surface and need time alone to find it. Thoughtful and self-sufficient, you light the way for others with what you learn. Your lesson is to share your wisdom and to stay connected while you search.",
  },
  10: {
    keywords: "Cycles, change, luck",
    meaning:
      "Your life moves in cycles, with turning points that arrive when you least expect them. You are adaptable and often in the right place at the right time. Your lesson is to ride change with an open mind and to take part in shaping your luck.",
  },
  11: {
    keywords: "Fairness, truth, balance",
    meaning:
      "You have a strong sense of right and wrong and a gift for weighing both sides. Honest and clear-minded, you want choices to have fair consequences. Your lesson is to take responsibility for your decisions and to temper judgement with compassion.",
  },
  12: {
    keywords: "Surrender, perspective, pause",
    meaning:
      "You see life from an unusual angle and learn by pausing rather than pushing. Letting go, even of a plan, often reveals what you were missing. Your lesson is to accept waiting as part of growth and to make your sacrifices consciously.",
  },
  13: {
    keywords: "Endings, transformation, renewal",
    meaning:
      "You know that something must end for something new to begin, and you are no stranger to deep change. Resilient and unafraid of the truth, you help others through transitions. In tarot this card is about transformation, never about literal death.",
  },
  14: {
    keywords: "Balance, patience, harmony",
    meaning:
      "You blend opposites into harmony and bring calm to tense situations. Patient and moderate, you bring together what others keep apart. Your lesson is to find your own middle path and to keep going gently, step by step.",
  },
  15: {
    keywords: "Desire, attachment, freedom",
    meaning:
      "You feel the pull of passion, pleasure and ambition strongly, and you notice what holds people back. Honest about your appetites, you can turn intensity into power. Your lesson is to recognize the habits and ties that bind you and to choose freedom. The card is about attachment, not evil.",
  },
  16: {
    keywords: "Upheaval, revelation, breakthrough",
    meaning:
      "Your life has had sudden turns that cleared away what was not solid. You are strong in a crisis and honest about what must change. Your lesson is to build on what is true, so that change brings liberation rather than shock.",
  },
  17: {
    keywords: "Hope, renewal, inspiration",
    meaning:
      "You carry hope and a quiet faith that things can get better. Open, generous and inspiring, you help others believe in tomorrow. Your lesson is to trust your own light and to keep giving without emptying yourself.",
  },
  18: {
    keywords: "Dreams, intuition, illusion",
    meaning:
      "You are sensitive, imaginative and tuned to moods and hidden currents, and your dreams speak to you clearly. Your lesson is to tell intuition from fear and to walk through uncertainty without letting it fool you.",
  },
  19: {
    keywords: "Joy, vitality, success",
    meaning:
      "You radiate warmth and optimism, and others enjoy being around you. Confident and open, you thrive in the light. Your lesson is to keep your enthusiasm honest and to share your good fortune generously.",
  },
  20: {
    keywords: "Awakening, calling, forgiveness",
    meaning:
      "You are drawn to answer a calling and to start over with a clearer sense of purpose. Reflective and forgiving, you help others see their own worth. Your lesson is to forgive the past, yourself included, and to act on what you now know.",
  },
  21: {
    keywords: "Completion, wholeness, achievement",
    meaning:
      "You like to see things through and to feel part of something larger. Well rounded and at ease in the world, you bring cycles to a satisfying close. Your lesson is to celebrate what you have completed and to stay curious about what comes next.",
  },
  22: {
    keywords: "New beginnings, trust, freedom",
    meaning:
      "You approach life with openness, curiosity and a leap-of-faith spirit. Spontaneous and unburdened, you start fresh when others hesitate. Your lesson is to pair your trust with a little foresight and to enjoy the journey.",
  },
};
