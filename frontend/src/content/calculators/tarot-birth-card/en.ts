// Tarot birth card calculator — long-form section under the result (growth:
// calculators). The method is Mary K. Greer's; card numbers follow the
// Rider–Waite–Smith order of the site's artwork (Strength 8, Justice 11).
import type { ArticleContent, ArticleImage } from "@/content/calculators/types";

export const tarotBirthIntroImage: Omit<ArticleImage, "alt"> = {
  src: "https://assets.staryield.net/assets/tarot/the-star.png",
  width: 210,
  height: 355,
  frame: { width: 210, height: 355, radius: 12 },
};

const content: ArticleContent = {
  kind: "tarot-birth-card",
  label: "Understanding your tarot birth card",
  sections: [
    {
      id: "what-is-a-tarot-birth-card",
      blocks: [
        {
          type: "split",
          blocks: [
            { type: "h2", id: "what-is-a-tarot-birth-card", text: "What Is a Tarot Birth Card?" },
            {
              type: "p",
              text: "Your tarot birth card is the Major Arcana card that matches the numbers of your date of birth. The method was popularized by tarot author Mary K. Greer, who brought numerology together with the 22 cards of the Major Arcana: you add up your birth date, reduce the total, and read the matching card as a symbol of the qualities and lessons you carry through life.",
            },
            {
              type: "p",
              weight: "light",
              text: "Most dates give two cards: a **personality card**, which describes how you meet the world, and a **soul card**, which points to your deeper purpose. Some dates give a single card that plays both roles, and one chain, 19/10/1, gives three. Like every reading on Staryield, your birth card is offered for reflection and entertainment, not as a prediction.",
            },
          ],
          image: { ...tarotBirthIntroImage, alt: "The Star tarot card" },
        },
      ],
    },
    {
      id: "how-to-calculate",
      blocks: [
        { type: "h2", id: "how-to-calculate", text: "How to Calculate Your Tarot Birth Card" },
        {
          type: "ol",
          items: [
            "Add your birth month, day and year as whole numbers (for example 11 + 29 + 1990).",
            "Add the digits of the total together. If the result is greater than 22, add its digits again until it is 22 or less.",
            "The number you reach is your personality card: 1 is The Magician, 2 The High Priestess and so on up to 21, The World. A result of 22 is The Fool, which is numbered 0 in the deck.",
            "If that number has two digits, add them together, and again if needed, until you reach a single digit: that is your soul card (for example 17 ⟹ 1 + 7 = 8, Strength). A single-digit result is both your personality card and your soul card.",
          ],
        },
        { type: "h3", id: "worked-example", text: "Worked Example: November 29, 1990" },
        {
          type: "ul",
          items: [
            "Add the date: 11 + 29 + 1990 = 2030",
            "Add the digits: 2 + 0 + 3 + 0 = 5",
            "5 is a single digit, so The Hierophant (V) is both the personality card and the soul card.",
          ],
        },
        { type: "h3", id: "two-card-example", text: "Two Cards: June 15, 1987" },
        {
          type: "ul",
          items: [
            "Add the date: 6 + 15 + 1987 = 2008",
            "Add the digits: 2 + 0 + 0 + 8 = 10, so the personality card is Wheel of Fortune (X)",
            "Reduce once more: 1 + 0 = 1, so the soul card is The Magician (I)",
          ],
        },
        {
          type: "p",
          weight: "light",
          text: "Under your result, “How it adds up” writes out every step for your own date, so you can check the arithmetic yourself.",
        },
      ],
    },
    {
      id: "personality-and-soul-cards",
      blocks: [
        { type: "h2", id: "personality-and-soul-cards", text: "Personality Card and Soul Card" },
        {
          type: "p",
          text: "The cards work as a pair. The higher number is your personality card: the face you show, the talents you use every day and the way you tend to meet challenges. The single-digit number is your soul card: a quieter thread of purpose that you grow into over a lifetime.",
        },
        {
          type: "ul",
          items: [
            "**A single card (2 to 9):** your personality and your soul share the same energy, so its themes run through your life with extra focus.",
            "**Two cards (10 to 18, 20, 21 and 22):** the personality card leads and the soul card beneath it shows the deeper lesson. The Fool (22) pairs with The Emperor (4).",
            "**Three cards (19):** The Sun is the personality card and The Magician the soul card, with Wheel of Fortune (10) as a bridge between them.",
          ],
        },
      ],
    },
    {
      id: "constellations",
      blocks: [
        { type: "h2", id: "constellations", text: "The Nine Tarot Constellations" },
        {
          type: "p",
          text: "Birth cards that end in the same soul card form a constellation, named after its single-digit card. People in the same constellation share the same deeper lesson, even when their personality cards differ.",
        },
        {
          type: "twoColumnList",
          items: [
            { title: "Constellation 1", subtitle: "The Magician", body: ["10/1: Wheel of Fortune and The Magician", "19/10/1: The Sun, Wheel of Fortune and The Magician"] },
            { title: "Constellation 2", subtitle: "The High Priestess", body: ["11/2: Justice and The High Priestess", "20/2: Judgement and The High Priestess", "2: The High Priestess on her own"] },
            { title: "Constellation 3", subtitle: "The Empress", body: ["12/3: The Hanged Man and The Empress", "21/3: The World and The Empress", "3: The Empress on her own"] },
            { title: "Constellation 4", subtitle: "The Emperor", body: ["13/4: Death and The Emperor", "22/4: The Fool and The Emperor", "4: The Emperor on his own"] },
            { title: "Constellation 5", subtitle: "The Hierophant", body: ["14/5: Temperance and The Hierophant", "5: The Hierophant on his own"] },
            { title: "Constellation 6", subtitle: "The Lovers", body: ["15/6: The Devil and The Lovers", "6: The Lovers on their own"] },
            { title: "Constellation 7", subtitle: "The Chariot", body: ["16/7: The Tower and The Chariot", "7: The Chariot on its own"] },
            { title: "Constellation 8", subtitle: "Strength", body: ["17/8: The Star and Strength", "8: Strength on its own"] },
            { title: "Constellation 9", subtitle: "The Hermit", body: ["18/9: The Moon and The Hermit", "9: The Hermit on his own"] },
          ],
        },
        {
          type: "p",
          weight: "light",
          text: "Your result lists the other birth cards of your constellation, so you can see who shares your soul card.",
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
              q: "Do I need my birth time to find my tarot birth card?",
              a: "No. The calculation uses your date of birth only, so the time and place of your birth make no difference. Your date never leaves your browser: the card is worked out on your device.",
            },
            {
              q: "How is this different from my Life Path number?",
              a: "Both start from your date of birth, but they are reduced differently. Your [Life Path number](/calculators/path-of-life) reduces the month, day and year separately and keeps the master numbers 11, 22 and 33, while the birth card adds the whole date first and stops at 22, the number of cards in the Major Arcana.",
            },
            {
              q: "Why is Strength number 8 here when my deck shows Justice as 8?",
              a: "Marseille-style decks number Justice 8 and Strength 11, while the Rider–Waite–Smith tradition, which many modern decks follow, swaps them. This calculator uses the Rider–Waite–Smith order, so 8 is Strength and 11 is Justice. If you prefer the Marseille order, read those two cards the other way round.",
            },
            {
              q: "Is my tarot birth card the same as the tarot card of my zodiac sign?",
              a: "No. Your birth card comes from the numbers of your full date of birth, while each zodiac sign is linked to a Major Arcana card through astrology (Leo with Strength, for example). Two people with the same sign can have different birth cards. For your sign and the rest of your chart, try the [natal chart calculator](/calculators/natal-chart).",
            },
            {
              q: "Can my birth card change over time?",
              a: "No. It is fixed by your date of birth. Some readers also work out a year card by adding your birth month and day to the current year; this calculator shows your birth cards only.",
            },
            {
              q: "Is a tarot birth card a prediction?",
              a: "No. Birth cards are a symbolic way to reflect on your strengths and lessons, and they are provided for entertainment only. They are not a forecast and not a substitute for professional advice.",
            },
          ],
        },
      ],
    },
  ],
};

export default content;
