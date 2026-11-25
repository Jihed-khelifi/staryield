/**
 * growth (seo): the questions and answers of the FAQ page's "Live readings"
 * block, as English source copy translated with `text()`. The page renders
 * them (`components/content/faq.tsx`) and its FAQPage structured data reads
 * the same list, so search engines are only told what the page shows.
 */
export type FaqEntry = { q: string; a: string };

export const LIVE_READINGS_FAQ: readonly FaqEntry[] = [
  {
    q: "How do I rate my advisor?",
    a: "When your live reading ends, the chat summary invites you to rate your advisor from one to five stars and add an optional comment. You can also open the reminder from your notifications. Each reading can be rated once, within 7 days of its end.",
  },
  {
    q: "Can I share photos during a reading?",
    a: "Yes. Use the image button next to the message box, paste an image or drag it into the chat. Only images (PNG, JPEG, WebP or GIF) up to 8 MB can be shared, and location data is removed automatically.",
  },
  {
    q: "Who can see the photos I share?",
    a: "Only you and the advisor in that conversation. Images are stored privately and are never published, and Staryield staff cannot open them through our tools.",
  },
  {
    q: "What does my advisor know about me?",
    a: "Your first name or nickname, profile picture, country, age, your astrological signs and daily horoscope, whether you are online and how many readings you have had together. Advisors never see your email address, balance or purchases.",
  },
  {
    q: "How do notifications work?",
    a: "The bell at the top of the page shows chat requests, reading summaries, order updates and announcements in your language. Select a notification to open it, or mark them all as read.",
  },
];
