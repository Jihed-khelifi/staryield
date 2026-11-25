/**
 * The FAQ page's questions and answers (docs/platform-extensions.md §27.1): the rendered page and its FAQPage
 * structured data read the same lists, so search engines are only told what the page shows.
 */
import { LIVE_READINGS_FAQ, type FaqEntry } from "@/content/faq/live-readings";
import { ACCOUNT_FAQ, ASTROLOGERS_FAQ, BILLING_FAQ, CHATTING_FAQ, WEBSITE_FAQ } from "@/content/faq/support-answers";

export type { FaqEntry };
export { ACCOUNT_FAQ, ACCOUNT_FAQ_DESCRIPTION, ACCOUNT_FAQ_HEADING, ASTROLOGERS_FAQ, BILLING_FAQ, CHATTING_FAQ, WEBSITE_FAQ } from "@/content/faq/support-answers";
export { LIVE_READINGS_FAQ };

/** The 22 questions of the Figma blocks, in page order. */
export const SUPPORT_FAQ: readonly FaqEntry[] = [...WEBSITE_FAQ, ...CHATTING_FAQ, ...ASTROLOGERS_FAQ, ...BILLING_FAQ];

/** Every answered question of the page, in page order (the FAQPage structured data). */
export const FAQ_PAGE_ENTRIES: readonly FaqEntry[] = [...SUPPORT_FAQ, ...LIVE_READINGS_FAQ, ...ACCOUNT_FAQ];

/**
 * The answer of a Figma question, found from the `label` of its `ContentLink` ("↑ <question> <TAG WORDS>"; the
 * symbol and the tag words are part of the design's accessible label). Undefined for any other label.
 */
export function faqAnswerForLabel(label: string): string | undefined {
  const text = label.replace(/^\S\s+/, "");
  return SUPPORT_FAQ.find((entry) => text.startsWith(`${entry.q} `) || text === entry.q)?.a;
}
