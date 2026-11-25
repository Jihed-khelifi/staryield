import type { CompatibilityCopy } from "@/content/seo/compatibility/types";
import { listOf } from "@/content/seo/names";

const lower = (value: string) => value.toLowerCase();

const copy: CompatibilityCopy = {
  aspects: {
    "same-sign": {
      name: "same sign",
      withArticle: "the same sign",
      angle: "0°, the same sign",
      essence:
        "Two people of the same sign share an element, a modality and a ruling planet. Traditional astrology reads this as instant recognition: you understand each other’s motives, and you also share the same blind spots.",
      strength: "Instant understanding: the same instincts, pace and values make each of you easy for the other to read.",
      friction: "A mirror effect: the traits you share, the difficult ones included, are doubled rather than balanced.",
      tip: "Bring in what neither of you has by nature: friends, interests and habits from outside your shared sign.",
    },
    "semi-sextile": {
      name: "semi-sextile",
      withArticle: "a semi-sextile",
      angle: "30°, neighbouring signs",
      essence:
        "Neighbouring signs form a semi-sextile. They share neither element nor modality, and tradition says each sign grows out of the one before it, so the bond feels close and unfamiliar at the same time.",
      strength: "Complementary skills: each of you has what the other is still learning.",
      friction: "Different instincts: neighbours often misread each other’s priorities and pace.",
      tip: "Treat your differences as a skill swap rather than a contest: ask how the other would handle it.",
    },
    sextile: {
      name: "sextile",
      withArticle: "a sextile",
      angle: "60°, two signs apart",
      essence:
        "Signs two places apart form a sextile and belong to friendly elements, Fire with Air or Earth with Water. Traditional astrology sees it as an easy, cooperative link that rewards effort: opportunities appear when both act on them.",
      strength: "Easy communication and a friendly, cooperative spirit.",
      friction: "Comfort can turn into coasting: the link works best when you actively build on it.",
      tip: "Plan shared projects or trips: a sextile thrives when it has something to do.",
    },
    square: {
      name: "square",
      withArticle: "a square",
      angle: "90°, three signs apart",
      essence:
        "Signs three places apart form a square. They share a modality but not an element, so they act in the same way while chasing different goals. Tradition reads the square as tension that drives growth.",
      strength: "Energy and attraction: the friction keeps things alive and pushes you both to grow.",
      friction: "Power struggles: you may both push at the same moment, in different directions.",
      tip: "Agree on who leads which part of life, and argue about the issue, not about who is right.",
    },
    trine: {
      name: "trine",
      withArticle: "a trine",
      angle: "120°, four signs apart",
      essence:
        "Signs four places apart form a trine and share the same element. Traditional astrology reads the trine as the most harmonious angle: a natural flow of shared values and temperament.",
      strength: "Natural harmony: shared values and a similar temperament make cooperation feel effortless.",
      friction: "Too comfortable: with so little friction, problems can be ignored until they grow.",
      tip: "Keep challenging each other kindly, so that ease does not turn into routine.",
    },
    quincunx: {
      name: "quincunx",
      withArticle: "a quincunx",
      angle: "150°, five signs apart",
      essence:
        "Signs five places apart form a quincunx, also called an inconjunct. They share neither element nor modality, and tradition reads the link as constant adjustment between two very different ways of living.",
      strength: "Fascination with difference: each of you shows the other a way of living they would never have chosen.",
      friction: "Misunderstandings: needs and priorities can feel out of step without either of you being wrong.",
      tip: "Say plainly what you need: with a quincunx little can be taken for granted, and that is fine.",
    },
    opposition: {
      name: "opposition",
      withArticle: "an opposition",
      angle: "180°, opposite signs",
      essence:
        "Opposite signs face each other across the zodiac. They share a modality and their elements complement each other, and traditional astrology reads the opposition as a strong pull between two halves of the same axis.",
      strength: "Magnetic attraction: each of you embodies the qualities the other is missing.",
      friction: "Polarisation: under stress, each of you digs into your own end of the axis.",
      tip: "Look for the middle of the axis: borrow from each other’s approach instead of defending your own.",
    },
  },
  elements: {
    "Fire-Fire": {
      summary: "Two Fire signs share enthusiasm, courage and a taste for action, so the energy between them runs high.",
      strength: "Shared enthusiasm: you push each other to try, to dare and to have fun.",
      friction: "Two fires can flare up at once: quick tempers and competition for the spotlight.",
      tip: "Take turns being the one in front, and cool down before you settle a disagreement.",
    },
    "Fire-Earth": {
      summary:
        "Fire brings spark and vision, Earth brings patience and practical sense. Tradition calls them an uneasy pair, yet together they can turn ideas into real results.",
      strength: "Ideas meet follow-through: one of you starts, the other makes it last.",
      friction: "A question of pace: Fire finds Earth slow, and Earth finds Fire reckless.",
      tip: "Let Fire choose the destination and Earth plan the route, and respect each other’s timing.",
    },
    "Fire-Air": {
      summary: "Air feeds Fire: ideas and conversation fuel enthusiasm and action. Traditional astrology counts Fire and Air as friendly elements.",
      strength: "A lively exchange: ideas quickly become plans and shared adventures.",
      friction: "Restlessness: neither element likes routine, so practical matters can be neglected.",
      tip: "Tackle the dull tasks together, so the fun rests on a solid base.",
    },
    "Fire-Water": {
      summary:
        "Fire is direct and outgoing, Water is sensitive and inward. Tradition sees a challenging mix, yet it can be intense and passionate when both take care.",
      strength: "Passion and depth: Fire brings warmth and courage, Water brings emotional depth.",
      friction: "Fire can hurt Water’s feelings without noticing, and Water’s moods can dampen Fire’s spirit.",
      tip: "Fire can slow down and listen; Water can say clearly what it feels instead of withdrawing.",
    },
    "Earth-Earth": {
      summary: "Two Earth signs share a practical, loyal and patient outlook, and both value stability and things that last.",
      strength: "Reliability: promises are kept, and building a shared life comes naturally.",
      friction: "Routine and stubbornness: life together can become predictable, and neither likes to give way.",
      tip: "Plan regular novelty, such as a trip or a new shared project, and practise compromise on small things.",
    },
    "Earth-Air": {
      summary: "Earth wants the concrete, Air lives in ideas and conversation. Tradition calls them an odd pair, but they can learn a great deal from each other.",
      strength: "Ideas become practical: Air brings perspective, Earth makes it workable.",
      friction: "Earth can find Air unreliable, and Air can find Earth set in its ways.",
      tip: "Agree on what is fixed and what is open, and give each other room in the rest.",
    },
    "Earth-Water": {
      summary: "Water nourishes Earth: emotional depth meets practical care. Traditional astrology counts Earth and Water as friendly elements.",
      strength: "Security and tenderness: you both value loyalty, comfort and a safe home base.",
      friction: "Withdrawal: you can both retreat into silence or worry instead of talking.",
      tip: "Name your worries early: you are both stronger when you share them.",
    },
    "Air-Air": {
      summary: "Two Air signs share curiosity, conversation and a need for mental space; they rarely run out of things to say.",
      strength: "Endless conversation and plenty of freedom for each of you.",
      friction: "Living in your heads: feelings and practical matters get discussed rather than handled.",
      tip: "Turn talk into small, concrete steps, and make time for physical closeness.",
    },
    "Air-Water": {
      summary: "Air thinks and Water feels. Tradition sees a challenging mix of logic and emotion, and each can teach the other a new language.",
      strength: "Head and heart: together you can understand a situation from both sides.",
      friction: "Air can seem detached to Water, and Water can feel overwhelming to Air.",
      tip: "Air can acknowledge feelings before analysing them; Water can put feelings into words.",
    },
    "Water-Water": {
      summary: "Two Water signs share sensitivity, intuition and emotional depth, and they often understand each other without words.",
      strength: "Deep empathy and emotional closeness.",
      friction: "Moods amplify: worries and hurt feelings can swirl without an outside view.",
      tip: "Keep friends and routines that ground you both, and check in before assuming what the other feels.",
    },
  },
  modalities: {
    "Cardinal-Cardinal": {
      summary: "Two cardinal signs are both initiators: each wants to start, decide and lead.",
      friction: "Two leaders can fight over the steering wheel.",
      tip: "Share the lead by area, so that each of you has something to steer.",
    },
    "Cardinal-Fixed": {
      summary: "A cardinal sign starts things and a fixed sign sustains them: one launches, the other makes it last.",
      friction: "The fixed sign resists the cardinal sign’s sudden changes of direction.",
      tip: "Talk changes through before making them, and let the fixed partner set the pace for follow-through.",
    },
    "Cardinal-Mutable": {
      summary: "A cardinal sign sets the direction and a mutable sign adapts it: initiative meets flexibility.",
      friction: "The mutable sign’s changes of mind can frustrate the cardinal sign’s plans.",
      tip: "Let the cardinal partner choose the goal and the mutable partner adjust the route.",
    },
    "Fixed-Fixed": {
      summary: "Two fixed signs are both steady and determined: loyal, persistent and slow to change.",
      friction: "Stubbornness on both sides: disagreements can harden into standoffs.",
      tip: "Agree in advance on how you settle disagreements, and thank each other for giving ground.",
    },
    "Fixed-Mutable": {
      summary: "A fixed sign brings constancy and a mutable sign brings adaptability: stability meets change.",
      friction: "The fixed sign can find the mutable one inconsistent, and the mutable sign can find the fixed one rigid.",
      tip: "Keep a few fixed rituals and leave the rest open to change.",
    },
    "Mutable-Mutable": {
      summary: "Two mutable signs are both flexible and curious: easy-going, adaptable and open to change.",
      friction: "Nobody steers: plans drift and decisions get postponed.",
      tip: "Write down shared goals and dates, so that flexibility does not become drift.",
    },
  },
  signs: {
    Aries: {
      traits: "bold, direct and quick to act",
      brings: "initiative and courage",
      needs: "room to lead and to act on impulse",
      love: "Aries loves openly and passionately, and likes to pursue what it wants.",
    },
    Taurus: {
      traits: "steady, sensual and loyal",
      brings: "patience and a sense of security",
      needs: "stability, comfort and time to decide",
      love: "Taurus loves slowly and loyally, showing affection through presence, touch and care.",
    },
    Gemini: {
      traits: "curious, witty and versatile",
      brings: "conversation, humour and fresh ideas",
      needs: "variety and mental stimulation",
      love: "Gemini loves through words and play, and needs a partner who is also a friend.",
    },
    Cancer: {
      traits: "caring, protective and intuitive",
      brings: "tenderness and emotional loyalty",
      needs: "emotional safety and a sense of home",
      love: "Cancer loves protectively and remembers every kindness, opening up once it feels safe.",
    },
    Leo: {
      traits: "warm, generous and expressive",
      brings: "warmth, loyalty and a sense of fun",
      needs: "appreciation and a place to shine",
      love: "Leo loves grandly and generously, and wants to feel admired in return.",
    },
    Virgo: {
      traits: "attentive, practical and thoughtful",
      brings: "care for the details and practical help",
      needs: "order, usefulness and gentle honesty",
      love: "Virgo loves through helpful acts and attention to detail rather than grand gestures.",
    },
    Libra: {
      traits: "diplomatic, graceful and fair-minded",
      brings: "harmony, charm and a sense of balance",
      needs: "fairness, beauty and true partnership",
      love: "Libra loves with charm and courtesy, and looks for a partnership of equals.",
    },
    Scorpio: {
      traits: "intense, perceptive and private",
      brings: "depth, loyalty and emotional honesty",
      needs: "trust and complete commitment",
      love: "Scorpio loves intensely and loyally, and asks for the same depth in return.",
    },
    Sagittarius: {
      traits: "adventurous, frank and optimistic",
      brings: "enthusiasm, humour and wider horizons",
      needs: "freedom, travel and a sense of meaning",
      love: "Sagittarius loves freely and honestly, and wants a partner who is also a companion on the road.",
    },
    Capricorn: {
      traits: "ambitious, disciplined and dependable",
      brings: "structure, commitment and long-term plans",
      needs: "respect, clear goals and steady progress",
      love: "Capricorn loves seriously and for the long term, proving its feelings through commitment.",
    },
    Aquarius: {
      traits: "independent, inventive and idealistic",
      brings: "originality and a spirit of friendship",
      needs: "independence and room for its ideals",
      love: "Aquarius loves as a friend first, and values freedom and honesty in a relationship.",
    },
    Pisces: {
      traits: "imaginative, empathetic and gentle",
      brings: "compassion, imagination and devotion",
      needs: "understanding and space to dream",
      love: "Pisces loves tenderly and romantically, often sensing what a partner feels before it is said.",
    },
  },
  planets: {
    Sun: "the Sun, the light of identity and vitality",
    Moon: "the Moon, the light of emotion and instinct",
    Mercury: "Mercury, the planet of thought and communication",
    Venus: "Venus, the planet of love, pleasure and harmony",
    Mars: "Mars, the planet of drive and courage",
    Jupiter: "Jupiter, the planet of growth and optimism",
    Saturn: "Saturn, the planet of structure and responsibility",
    Uranus: "Uranus, the planet of change and originality",
    Neptune: "Neptune, the planet of imagination and compassion",
    Pluto: "Pluto, the planet of depth and transformation",
  },
  frame: {
    eyebrow: "Astrology · Compatibility",
    title: (a, b) => `${a} and ${b} Compatibility in Love, Friendship and Work`,
    description: (f) =>
      f.same
        ? `${f.a} and ${f.a} compatibility: two ${f.elementA} signs with the same ruler. Strengths, friction points, practical tips and love, friendship and work scores. Free calculator.`
        : `${f.a} and ${f.b} compatibility: ${f.aspectWithArticle} pairing ${f.elementA} with ${f.elementB}. Strengths, friction points, practical tips and love, friendship and work scores. Free calculator.`,
    h1: (a, b) => `${a} and ${b} Compatibility`,
    intro: (f) =>
      f.same
        ? `Two ${f.a} signs share the same element (${f.elementA}), the same modality (${lower(f.modalityA)}) and the same ruler (${f.rulerA}). In traditional astrology this makes ${f.levelText}, built on instant recognition and a shared set of strengths and blind spots.`
        : `${f.a} and ${f.b} form ${f.aspectWithArticle} (${f.angle}), pairing ${f.elementA} with ${f.elementB} and a ${lower(f.modalityA)} sign with a ${lower(f.modalityB)} one. In traditional astrology this makes ${f.levelText}.`,
    pairName: (f) => (f.same ? `two ${f.a} signs` : `${f.a} and ${f.b}`),
    headings: {
      overview: (pair) => `How ${pair} relate`,
      elements: "Elements and modalities",
      rulers: "Ruling planets",
      strengths: (pair) => `What works between ${pair}`,
      friction: (pair) => `Where ${pair} may clash`,
      tips: "Practical tips",
      indicators: "Love, friendship and work",
    },
    labels: {
      element: (of) => `Element ${of}`,
      modality: (of) => `Modality ${of}`,
      ruler: (of) => `Ruler ${of}`,
      love: "Love",
      friendship: "Friendship",
      work: "Work",
      outOf: (value) => `${value} out of 5`,
    },
    portrait: (f, traitsA, traitsB) => (f.same ? `Both partners are ${traitsA}.` : `${f.a} is ${traitsA}; ${f.b} is ${traitsB}.`),
    brings: (f, bringsA, bringsB) =>
      f.same ? `Both bring ${bringsA}, so there is plenty of it in the relationship.` : `${f.a} brings ${bringsA}, while ${f.b} brings ${bringsB}.`,
    needs: (f, needsA, needsB) =>
      f.same
        ? `Both need ${needsA}, which can mean competing for the same thing.`
        : `${f.a} needs ${needsA}, while ${f.b} needs ${needsB}: making room for both is part of the work.`,
    rulers: (f, planetA, planetB) =>
      f.same
        ? `${f.a} is ruled by ${planetA}. Sharing a ruler means you want the same things from life, and sometimes at the same moment.`
        : f.sharedRuler
          ? `${f.a} and ${f.b} are both ruled by ${planetA}. A shared ruler gives you common ground, even though your elements express it differently.`
          : `${f.a} is ruled by ${planetA}, and ${f.b} by ${planetB}. Each ruler describes what that sign looks for, and blending the two is part of the relationship.`,
    reading: (f) => {
      const scores = f.indicators;
      const max = Math.max(scores.love, scores.friendship, scores.work);
      const names = { love: "love", friendship: "friendship", work: "work" } as const;
      const best = (Object.keys(names) as (keyof typeof names)[]).filter((key) => scores[key] === max).map((key) => names[key]);
      return best.length === 3
        ? `This pairing scores evenly across love, friendship and work (${max} out of 5).`
        : `This pairing is strongest in ${listOf("en", best)} (${max} out of 5).`;
    },
    indicatorNote:
      "These scores are a traditional-astrology heuristic offered for entertainment. They come from the angle between the two signs, their elements, modalities and ruling planets, not from data about real couples. Real relationships depend on the people in them, and a full birth chart says far more than sun signs.",
    levels: {
      harmonious: "one of the most harmonious pairings",
      good: "a good match that grows with effort",
      mixed: "a mixed pairing of attraction and friction",
      demanding: "a demanding pairing that asks for patience on both sides",
    },
    levelShort: { harmonious: "Harmonious", good: "Good match", mixed: "Mixed", demanding: "Demanding" },
    faq: (f, { essence, loveA, loveB, elementSummary }) => [
      {
        q: `Are ${f.a} and ${f.b} compatible?`,
        a: f.same ? `In traditional astrology, two ${f.a} signs make ${f.levelText}. ${essence}` : `In traditional astrology, ${f.a} and ${f.b} make ${f.levelText}. ${essence}`,
      },
      {
        q: `Are ${f.a} and ${f.b} a good match in love?`,
        a: f.same
          ? `${loveA} On our traditional scale, two ${f.a} signs rate ${f.indicators.love} out of 5 for love.`
          : `${loveA} ${loveB} On our traditional scale, this pairing rates ${f.indicators.love} out of 5 for love.`,
      },
      { q: `What do ${f.a} and ${f.b} have in common?`, a: elementSummary },
      {
        q: "Is sun sign compatibility reliable?",
        a: "Sun signs only give a broad picture. Astrologers also compare the Moon, Venus, Mars and the rising sign of both charts, and real relationships depend on the people, not on their signs. This page is offered for entertainment purposes only.",
      },
    ],
    related: "Explore related pairings",
    hubLink: "All zodiac pairings",
    calculatorLink: "Zodiac Sign Calculator",
    hub: {
      title: "Zodiac Compatibility: All 78 Sign Pairings",
      description:
        "Zodiac sign compatibility for all 78 pairings: the angle between the signs, elements, modalities, ruling planets, strengths, friction points and tips. Enter two birth dates to find yours.",
      h1: "Zodiac Sign Compatibility",
      intro:
        "Zodiac compatibility compares two sun signs through the angle between them, their elements, their modalities and their ruling planets. Enter two birth dates to find your pairing, or choose one of the 78 combinations below.",
      gridHeading: "Every sign pairing",
      gridIntro: "Choose your sign, then the sign of your partner, friend or colleague.",
      sections: [
        {
          id: "how-it-works",
          heading: "How we read sign compatibility",
          paragraphs: [
            "Each pairing is read from four traditional factors: the angle between the two signs on the zodiac wheel, their elements (Fire, Earth, Air, Water), their modalities (cardinal, fixed, mutable) and their ruling planets. We use the modern rulers, so Scorpio is ruled by Pluto, Aquarius by Uranus and Pisces by Neptune.",
            "The love, friendship and work scores on each page are derived from those factors with a fixed rule, so the same two signs always get the same answer. They are a heuristic for entertainment, not a measurement.",
          ],
        },
      ],
      aspectsHeading: "The seven sign relationships",
      aspectLine: (name, angle, essence) => `**${name.charAt(0).toUpperCase()}${name.slice(1)}** (${angle}): ${essence}`,
      faq: [
        {
          q: "How is zodiac compatibility calculated?",
          a: "It compares the two sun signs: how far apart they sit on the zodiac wheel, whether their elements and modalities support each other, and which planets rule them. Our calculator finds both signs from two birth dates.",
        },
        {
          q: "What if one of us was born on a cusp?",
          a: "On the days when the Sun changes sign, the sign depends on the time and place of birth. The calculator tells you when that happens and lets you choose, or add your birth time in the zodiac sign calculator.",
        },
        {
          q: "Which signs are the most compatible?",
          a: "Traditional astrology favours signs of the same element (trines) and of friendly elements (sextiles), while opposite signs bring strong attraction with some tension. Every pairing has strengths and friction points, and these pages describe both.",
        },
        {
          q: "Are these compatibility readings predictions?",
          a: "No. They describe traditional astrological themes for reflection and entertainment. They cannot tell you whether a relationship will work.",
        },
      ],
    },
    breadcrumb: { hub: "Zodiac compatibility" },
    imageAlt: "Rays of sunlight breaking through clouds at sunrise",
    signAlt: (sign) => `${sign} zodiac symbol`,
  },
};

export default copy;
