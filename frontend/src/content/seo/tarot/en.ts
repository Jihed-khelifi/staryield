import type { TarotContent, TarotFrame } from "@/content/seo/tarot/types";
import { listOf } from "@/content/seo/names";

const content: TarotContent = {
  "the-fool": {
    keywords: "new beginnings, trust, spontaneity",
    intro:
      "The Fool is the tarot birth card of the free spirit. It is numbered 0 in the Major Arcana, and in the birth card method a total that reduces to 22 is read as The Fool, paired with The Emperor as its soul card.",
    meaning:
      "As a birth card, The Fool points to a life that keeps starting over: curious, trusting and willing to leap before every detail is clear. Tradition sees the figure at the cliff edge as innocence and potential rather than foolishness, someone with everything still ahead.",
    gifts: [
      "Fearlessness in beginnings: they try what others postpone.",
      "Openness and a childlike sense of wonder.",
      "A faith that things can work out, which others find contagious.",
    ],
    shadow: [
      "Recklessness: leaping without checking the ground.",
      "Avoiding commitment or consequences.",
      "A naivety that other people can take advantage of.",
    ],
    note: "The Fool is numbered 0, but in the birth card arithmetic its birth number is 22. That is why its constellation includes The Emperor: 2 + 2 = 4.",
  },
  "the-magician": {
    keywords: "will, skill, resourcefulness",
    intro:
      "The Magician is the tarot birth card of skilful will. It is card number 1, and for modern birth dates it appears as a soul card, reached from the birth numbers 10 and 19, not as the first result of the arithmetic.",
    meaning:
      "As a birth card, The Magician describes someone who makes things happen: communicating well, learning quickly and turning intention into action. The tools on the table in the traditional image stand for the four suits, the resources already available to them.",
    gifts: [
      "Skill and resourcefulness: they work with what they have.",
      "Persuasive, clear communication.",
      "The drive to turn ideas into action.",
    ],
    shadow: [
      "Cleverness without sincerity, or the temptation to manipulate.",
      "Scattering talent across too many projects.",
      "Pride in their own skill that stops them from learning.",
    ],
  },
  "the-high-priestess": {
    keywords: "intuition, mystery, inner knowing",
    intro:
      "The High Priestess is the tarot birth card of intuition and inner knowing. It is card number 2, and it can also appear as the soul card of other birth numbers.",
    meaning:
      "As a birth card, The High Priestess describes someone who observes before speaking and trusts what they sense beneath the surface. She sits between the pillars of the temple guarding what is hidden, and tradition links her to dreams, secrets and patience.",
    gifts: [
      "Deep intuition and a feel for what is left unsaid.",
      "Discretion: others trust them with confidences.",
      "Calm in the face of uncertainty.",
    ],
    shadow: [
      "Withholding: keeping so much inside that others cannot reach them.",
      "Passivity, waiting when action is needed.",
      "Doubting visible facts in favour of vague impressions.",
    ],
  },
  "the-empress": {
    keywords: "nurture, creativity, abundance",
    intro:
      "The Empress is the tarot birth card of fertility, creativity and care. It is card number 3, and it can also appear as the soul card of other birth numbers.",
    meaning:
      "As a birth card, The Empress points to a sensual, generous person who makes things grow, whether a garden, a family, a business or a creative work. Tradition links her to Venus, to nature and to the pleasure of comfort and beauty.",
    gifts: [
      "Nurturing warmth that helps people and projects flourish.",
      "Creativity and a love of beauty.",
      "Generosity and an easy appreciation of life’s pleasures.",
    ],
    shadow: [
      "Smothering care or overindulgence.",
      "Depending on comfort or approval.",
      "Creative energy that all goes into looking after everyone else.",
    ],
  },
  "the-emperor": {
    keywords: "structure, authority, protection",
    intro:
      "The Emperor is the tarot birth card of order and authority. It is card number 4, and it also appears as the soul card of other birth numbers.",
    meaning:
      "As a birth card, The Emperor describes someone who creates structure, sets boundaries and takes responsibility for others. Tradition links him to Aries, to leadership and to the protective, stabilising side of power.",
    gifts: [
      "Leadership and the ability to organise.",
      "A protective instinct and a strong sense of duty.",
      "Clear boundaries and decisive action.",
    ],
    shadow: [
      "Rigidity and the need to control.",
      "Authoritarian streaks or an unwillingness to be questioned.",
      "Emotional distance behind competence.",
    ],
  },
  "the-hierophant": {
    keywords: "tradition, teaching, belonging",
    intro:
      "The Hierophant is the tarot birth card of tradition and teaching. It is card number 5, and it also appears as the soul card of other birth numbers.",
    meaning:
      "As a birth card, The Hierophant describes someone who values shared knowledge, ritual and community, and who often becomes a teacher or mentor. Tradition links him to Taurus, to institutions and to the passing of wisdom from one generation to the next.",
    gifts: [
      "Teaching and mentoring with patience.",
      "Respect for the traditions that give life meaning.",
      "Building trust and belonging in groups.",
    ],
    shadow: [
      "Dogmatism: confusing the usual way with the only way.",
      "Conformity or fear of being different.",
      "Using rules to avoid personal responsibility.",
    ],
  },
  "the-lovers": {
    keywords: "choice, union, values",
    intro:
      "The Lovers is the tarot birth card of choice and union. It is card number 6, and although its name suggests romance, its older meaning is about choosing in line with one’s values.",
    meaning:
      "As a birth card, The Lovers describes someone for whom relationships and decisions are central: choosing partners, paths and principles they can fully stand behind. Tradition links the card to Gemini and to the idea that every real choice defines who we are.",
    gifts: [
      "A capacity for deep, committed connection.",
      "Clarity about their values when it counts.",
      "Ease in dialogue and in seeing both sides.",
    ],
    shadow: [
      "Indecision, or letting others choose for them.",
      "Idealising partners and being disappointed.",
      "Defining themselves too much through relationships.",
    ],
  },
  "the-chariot": {
    keywords: "determination, drive, discipline",
    intro:
      "The Chariot is the tarot birth card of determination. It is card number 7 and shows a driver steering opposing forces toward one goal.",
    meaning:
      "As a birth card, The Chariot describes someone who succeeds through focus, discipline and self-command, learning to hold competing impulses together and move forward. Tradition links the card to Cancer, to protecting what matters and to a victory that has to be earned.",
    gifts: [
      "Focus and determination.",
      "Self-discipline that turns goals into results.",
      "Courage to take the wheel in difficult conditions.",
    ],
    shadow: [
      "Aggression or the need to win at all costs.",
      "Pushing on instead of pausing to reassess.",
      "Hiding vulnerability behind drive.",
    ],
  },
  strength: {
    keywords: "courage, patience, gentle power",
    intro:
      "Strength is the tarot birth card of inner courage. It is card number 8 in the Rider-Waite numbering used on this site, and it shows a person calming a lion with gentleness instead of force.",
    meaning:
      "As a birth card, Strength describes someone whose power is patience, compassion and steady self-mastery. Tradition links the card to Leo and to the idea that taming one’s own fears and impulses is the greatest courage.",
    gifts: [
      "Quiet courage and emotional resilience.",
      "Patience with people and with difficult feelings.",
      "A warmth that steadies others.",
    ],
    shadow: [
      "Self-doubt behind a calm exterior.",
      "Suppressing anger or fear instead of working with it.",
      "Taking on other people’s burdens.",
    ],
    note: "Numbering differs between traditions. The Golden Dawn and the Rider-Waite deck number Strength VIII and Justice XI, matching their astrological attributions; older Marseille-style decks swap them. We use the Rider-Waite numbering that most modern decks follow.",
  },
  "the-hermit": {
    keywords: "wisdom, solitude, guidance",
    intro:
      "The Hermit is the tarot birth card of inner wisdom. It is card number 9 and shows a lone figure holding up a lantern.",
    meaning:
      "As a birth card, The Hermit describes someone who seeks understanding through reflection, solitude and study, and who later lights the way for others. Tradition links the card to Virgo, to discernment and to the kind of knowledge that comes from experience.",
    gifts: [
      "Depth of thought and a gift for discernment.",
      "Comfort with solitude and silence.",
      "Guidance that comes from lived experience.",
    ],
    shadow: [
      "Withdrawal, loneliness or avoiding people.",
      "Perfectionism and over-analysis.",
      "Keeping to themselves what they know.",
    ],
  },
  "wheel-of-fortune": {
    keywords: "cycles, change, opportunity",
    intro:
      "The Wheel of Fortune is the tarot birth card of cycles and change. It is card number 10 and shows a turning wheel, a symbol of rising and falling fortunes.",
    meaning:
      "As a birth card, the Wheel of Fortune describes someone whose life moves in distinct chapters and who learns to adapt as circumstances turn. Tradition links the card to Jupiter, to luck and expansion, and to the reminder that no phase is permanent.",
    gifts: [
      "Adaptability when circumstances shift.",
      "An eye for timing and opportunity.",
      "Optimism and perspective about setbacks.",
    ],
    shadow: [
      "Leaving too much to chance.",
      "Feeling like a victim of circumstances.",
      "Restlessness when things stay still.",
    ],
  },
  justice: {
    keywords: "truth, fairness, accountability",
    intro:
      "Justice is the tarot birth card of fairness and truth. It is card number 11 in the Rider-Waite numbering used on this site, and it shows a figure holding scales and a sword.",
    meaning:
      "As a birth card, Justice describes someone with a strong sense of right and wrong, who weighs situations carefully and takes responsibility for their choices. Tradition links the card to Libra, to balance and to the principle that actions have consequences.",
    gifts: [
      "Fair-mindedness and clear judgment.",
      "Honesty, including honesty with themselves.",
      "A sense of accountability that others can rely on.",
    ],
    shadow: [
      "Harshness: judging themselves or others too sharply.",
      "Rigid thinking about right and wrong.",
      "Difficulty with grey areas and with forgiveness.",
    ],
    note: "Numbering differs between traditions. The Golden Dawn and the Rider-Waite deck number Justice XI and Strength VIII, matching their astrological attributions; older Marseille-style decks swap them. We use the Rider-Waite numbering that most modern decks follow.",
  },
  "the-hanged-man": {
    keywords: "surrender, new perspective, pause",
    intro:
      "The Hanged Man is the tarot birth card of surrender and new perspective. It is card number 12 and shows a figure suspended calmly, upside down, by one foot.",
    meaning:
      "As a birth card, The Hanged Man describes someone who learns by pausing, letting go and seeing things from another angle. Tradition links the card to water and to willing sacrifice, giving up one thing for a deeper understanding.",
    gifts: [
      "Open-mindedness and original points of view.",
      "The patience to wait for the right moment.",
      "Willingness to let go of what no longer serves them.",
    ],
    shadow: [
      "Procrastination presented as patience.",
      "Martyrdom: sacrificing without being asked.",
      "Feeling stuck and unable to act.",
    ],
  },
  death: {
    keywords: "endings, transformation, renewal",
    intro:
      "Death is the tarot birth card of endings and transformation. It is card number 13 and, despite its name, it is not read as literal death.",
    meaning:
      "As a birth card, Death describes someone whose life is shaped by cycles of change: letting go of what has finished so that something new can grow. Tradition links the card to Scorpio, to depth and to the renewal that follows a loss.",
    gifts: [
      "The ability to transform and start over.",
      "Courage to face hard truths.",
      "Depth: they are rarely afraid of difficult subjects.",
    ],
    shadow: [
      "Clinging to what has ended, or cutting ties too abruptly.",
      "Fear of change.",
      "An intensity that can make others uneasy.",
    ],
    note: "Some older decks leave this card unnamed, which is why it is sometimes called the nameless arcana.",
  },
  temperance: {
    keywords: "balance, moderation, harmony",
    intro:
      "Temperance is the tarot birth card of balance and moderation. It is card number 14 and shows an angel pouring liquid between two cups.",
    meaning:
      "As a birth card, Temperance describes someone who blends opposites and finds the middle path: patient, adaptable and soothing in their influence. Tradition links the card to Sagittarius, to the search for meaning and to the art of combining the right amounts.",
    gifts: [
      "Patience and a gift for balance.",
      "A calm that helps resolve conflicts.",
      "The ability to combine different ideas and people.",
    ],
    shadow: [
      "Avoiding extremes so completely that nothing bold happens.",
      "Compromising too much.",
      "Impatience with the slow process of blending.",
    ],
  },
  "the-devil": {
    keywords: "attachment, desire, liberation",
    intro:
      "The Devil is the tarot birth card of attachment and desire. It is card number 15 and, despite its name, it is read as a symbol of the habits and fears that bind us, not as evil.",
    meaning:
      "As a birth card, The Devil describes someone with strong appetites, ambition and magnetism, who learns to tell what they choose from what has a hold on them. Tradition links the card to Capricorn, to material life and to a freedom that begins with seeing that the chains are loosely fitted.",
    gifts: [
      "Passion, drive and a strong will.",
      "Honest awareness of desire and temptation.",
      "Resourcefulness in practical and material matters.",
    ],
    shadow: [
      "Compulsions, dependence or excess.",
      "Cynicism about other people’s motives.",
      "Feeling trapped by roles and obligations.",
    ],
  },
  "the-tower": {
    keywords: "upheaval, revelation, breakthrough",
    intro:
      "The Tower is the tarot birth card of sudden change and revelation. It is card number 16 and shows a tower struck by lightning.",
    meaning:
      "As a birth card, The Tower describes someone whose life tends to include dramatic turning points that clear away what was not solid, and who grows strong by rebuilding on truer foundations. Tradition links the card to Mars, to energy and to breakthroughs that arrive suddenly.",
    gifts: [
      "Resilience: they rebuild well.",
      "An honesty that cuts through illusions.",
      "Courage in a crisis.",
    ],
    shadow: [
      "Drama or conflict that stirs things up unnecessarily.",
      "Resisting change until it is forced.",
      "Fear of instability.",
    ],
  },
  "the-star": {
    keywords: "hope, renewal, inspiration",
    intro:
      "The Star is the tarot birth card of hope and renewal. It is card number 17 and shows a figure pouring water beneath a sky of stars.",
    meaning:
      "As a birth card, The Star describes someone who brings calm, inspiration and trust in the future, often after a difficult time. Tradition links the card to Aquarius, to generosity and to the quiet faith that renewal is possible.",
    gifts: [
      "An optimism that comforts others.",
      "Inspiration and a gift for encouragement.",
      "Serenity and sincerity.",
    ],
    shadow: [
      "Idealism disconnected from practical steps.",
      "Disappointment when hope is not met right away.",
      "Giving freely and forgetting to refill themselves.",
    ],
  },
  "the-moon": {
    keywords: "intuition, imagination, the unconscious",
    intro:
      "The Moon is the tarot birth card of imagination and the unconscious. It is card number 18 and shows a path between two towers beneath a moon.",
    meaning:
      "As a birth card, The Moon describes someone with a vivid inner life, rich intuition and sensitivity to moods and dreams, who learns to separate fear from insight. Tradition links the card to Pisces, to cycles and to what is not yet clear.",
    gifts: [
      "Imagination and artistic sensitivity.",
      "Intuition about people and situations.",
      "Empathy and openness to the unseen.",
    ],
    shadow: [
      "Anxiety, fear or confusion.",
      "Illusion: seeing what they wish or fear.",
      "Moodiness and escapism.",
    ],
  },
  "the-sun": {
    keywords: "joy, vitality, clarity",
    intro:
      "The Sun is the tarot birth card of joy and vitality. It is card number 19 and is traditionally one of the most optimistic cards of the deck.",
    meaning:
      "As a birth card, The Sun describes someone who radiates warmth and enthusiasm, who is open and straightforward, and who is most alive when sharing success and play. Tradition links the card to the Sun, to clarity and to a child-like confidence.",
    gifts: [
      "Warmth and natural optimism.",
      "Openness and honesty.",
      "Energy that encourages others.",
    ],
    shadow: [
      "Naivety or ignoring problems.",
      "Needing attention or approval.",
      "Overconfidence.",
    ],
  },
  judgement: {
    keywords: "awakening, reckoning, calling",
    intro:
      "Judgement is the tarot birth card of awakening and calling. It is card number 20 and shows figures rising as a trumpet sounds.",
    meaning:
      "As a birth card, Judgement describes someone who reviews their life honestly, forgives, and answers a calling to change. Tradition links the card to fire, to renewal and to the moment when the past is understood well enough to be released.",
    gifts: [
      "Honest self-reflection.",
      "The ability to forgive and begin again.",
      "A sense of calling and purpose.",
    ],
    shadow: [
      "Self-criticism and harsh self-judgment.",
      "Dwelling on the past.",
      "Ignoring the call because the change feels too big.",
    ],
  },
  "the-world": {
    keywords: "completion, wholeness, achievement",
    intro:
      "The World is the tarot birth card of completion and wholeness. It is card number 21 and shows a dancing figure within a wreath.",
    meaning:
      "As a birth card, The World describes someone who brings things to completion, connects the local with the global and is at ease in many worlds. Tradition links the card to Saturn, to mastery earned over time and to the satisfaction of a cycle finished well.",
    gifts: [
      "Follow-through and a sense of completion.",
      "A broad perspective and ease with different cultures or fields.",
      "Integration: bringing different parts of life together.",
    ],
    shadow: [
      "Reluctance to finish for fear of what comes next.",
      "Perfectionism: it is never quite complete.",
      "Complacency after success.",
    ],
  },
};

export default content;

const list = (items: string[]) => listOf("en", items);

export const frame: TarotFrame = {
  eyebrow: "Tarot · Birth cards",
  title: (card) => `${card} Tarot Birth Card: Meaning and Constellation`,
  description: (card, keywords) =>
    `${card} as a tarot birth card: ${keywords}. Its number, Golden Dawn attribution, gifts, shadow, constellation and a worked example, plus a free calculator.`,
  h1: (card) => `${card} as a Tarot Birth Card`,
  headings: {
    meaning: (card) => `What ${card} means as a birth card`,
    facts: "Number, numeral and Golden Dawn attribution",
    gifts: (card) => `The gifts of ${card}`,
    shadow: (card) => `The shadow side of ${card}`,
    constellation: (card) => `The constellation of ${card}`,
    example: "A worked example",
  },
  labels: {
    arcana: "Major Arcana number",
    numeral: "Roman numeral",
    birthNumber: "Birth number",
    letter: "Hebrew letter",
    attribution: "Golden Dawn attribution",
    element: "element",
    planet: "planet",
    sign: "zodiac sign",
    total: "Month + day + year",
    personality: "personality card",
    middle: "middle card",
    soul: "soul card",
    both: "personality and soul card",
  },
  attribution: (card, letter, value, kind) =>
    `In the Golden Dawn system, which shaped most modern decks, ${card} corresponds to the Hebrew letter ${letter} and to ${kind === "element" ? `the element ${value}` : kind === "planet" ? (value === "the Sun" || value === "the Moon" ? value : `the planet ${value}`) : `the sign ${value}`}.`,
  constellation: ({ card, birthNumber, chain, soulOf, family, soulOnly }) => {
    const lines: string[] = [];
    if (soulOnly)
      lines.push(`For birth dates from 1900 onward the total never comes to ${birthNumber}, so ${card} is never a first card: it appears as the soul card of ${list(soulOf.map((item) => `${item.number} (${item.card})`))}.`);
    else if (chain.length === 1)
      lines.push(`When your total is ${birthNumber}, ${card} is both your personality card and your soul card.`);
    else if (chain.length === 2)
      lines.push(`When your total is ${birthNumber}, ${card} is your personality card and ${chain[1]} is your soul card.`);
    else
      lines.push(`When your total is ${birthNumber}, you have three cards: ${card} as your personality card, then ${chain[1]}, then ${chain[2]} as your soul card.`);
    if (!soulOnly && soulOf.length)
      lines.push(`${card} is also the soul card of ${list(soulOf.map((item) => `${item.number} (${item.card})`))}.`);
    lines.push(`Its constellation, the cards that share the same soul card, is ${list(family)}.`);
    return lines;
  },
  example: (date) => `Take someone born on ${date}. Add the month, the day and the year, then add the digits of the total until it is 22 or less:`,
  faq: ({ card, entry, constellation, attribution, date, birthNumber, soulOnly, soulOf }) => [
    { q: `What does ${card} mean as a birth card?`, a: entry.meaning },
    {
      q: `How do I know if ${card} is my birth card?`,
      a: soulOnly
        ? `Add the month, the day and the year of your birth, then add the digits of the total until it is 22 or less. ${card} is the soul card of totals ${soulOf}. For example, someone born on ${date} has it. The calculator at the end of this page does it for you.`
        : `Add the month, the day and the year of your birth, then add the digits of the total until it is 22 or less. A total of ${birthNumber} gives ${card}. For example, someone born on ${date} has it. The calculator at the end of this page does it for you.`,
    },
    { q: `What is the constellation of ${card}?`, a: constellation },
    { q: `Which astrological correspondence does ${card} have?`, a: attribution },
    {
      q: "Does a birth card predict my future?",
      a: "No. Birth cards are a tradition for reflection: they describe themes to think about, not events that will happen. They are offered for entertainment purposes only.",
    },
  ],
  related: "Explore other tarot birth cards",
  hubLink: "All tarot birth cards",
  calculatorLink: "Tarot Birth Card Calculator",
  hub: {
    title: "Tarot Birth Cards: Meanings of the 22 Major Arcana",
    description:
      "What each tarot birth card means, from The Magician to The Fool: numbers, Golden Dawn attributions, gifts, shadows and constellations. Find your cards with the free calculator.",
    h1: "Tarot Birth Cards and Their Meanings",
    intro:
      "Your tarot birth cards are the Major Arcana cards that match the sum of your birth date, a method made popular by tarot author Mary K. Greer. Most people have two: a personality card and a soul card. Enter your birth date to find yours, or choose a card below.",
    gridHeading: "The 22 birth cards",
    sections: [
      {
        id: "how-it-works",
        heading: "How tarot birth cards are calculated",
        paragraphs: [
          "Add your birth month, day and year as whole numbers. Then add the digits of the total until the result is 22 or less: that number is your personality card. If it has two digits, add them once more to find your soul card. A total of 22 is The Fool, whose soul card is The Emperor.",
          "Cards 10 to 22 are always personality cards; cards 1 to 9 can be soul cards. Some totals give three cards: 19 leads to The Sun, the Wheel of Fortune and The Magician.",
        ],
      },
      {
        id: "constellations",
        heading: "Constellations",
        paragraphs: [
          "Cards that share the same soul card form a constellation. The Star and Strength belong together because 1 + 7 = 8, just as The Sun, the Wheel of Fortune and The Magician share the soul card 1.",
        ],
      },
    ],
    faq: [
      {
        q: "What is a tarot birth card?",
        a: "It is a Major Arcana card linked to your date of birth. Tarot readers use it as a lifelong theme to reflect on, the way astrologers use a sun sign.",
      },
      {
        q: "What is the difference between the personality card and the soul card?",
        a: "The personality card is the first total of 22 or less and is said to describe how you express yourself. The soul card is that number reduced to a single digit and is said to describe your deeper purpose. When the total is already a single digit, the two are the same card.",
      },
      {
        q: "Why is The Fool numbered 22?",
        a: "The Fool is card 0 of the Major Arcana, but a sum is never 0, so the birth card method gives it the number 22, the position after The World.",
      },
      {
        q: "Which deck do the numbers follow?",
        a: "We use the Rider-Waite numbering, which follows the Golden Dawn: Strength is VIII and Justice is XI. Older Marseille-style decks swap these two cards.",
      },
    ],
    tile: (birthNumber, numeral) => `Birth number ${birthNumber} · ${numeral}`,
  },
  breadcrumb: { hub: "Tarot birth cards" },
  cardAlt: (card) => `${card} tarot card`,
};

