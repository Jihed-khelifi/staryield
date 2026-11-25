import { describe, expect, it } from "vitest";

import { parseInline, resolveHref } from "@/components/content/legal/inline-text";
import { ACCOUNT_FAQ, ASTROLOGERS_FAQ, BILLING_FAQ, CHATTING_FAQ, FAQ_PAGE_ENTRIES, SUPPORT_FAQ, WEBSITE_FAQ, faqAnswerForLabel } from "@/content/faq";
import copyEn from "@/i18n/dictionaries/copy-en.json";
import copyEs from "@/i18n/dictionaries/copy-es.json";
import copyFr from "@/i18n/dictionaries/copy-fr.json";

/**
 * Conversion follow-ups, slice E (docs/platform-extensions.md §27.1): the 22 questions of the FAQ page that used to
 * answer "contact our support team", and the "Free minutes, invites and your account" group, have real answers, in
 * every language, taken from the product's rules (Terms, Refund Policy) and the FAQPage structured data reads them.
 */
const dictionaries = { en: copyEn, fr: copyFr, es: copyEs } as Record<"en" | "fr" | "es", Record<string, string>>;
const hrefs = (text: string) => parseInline(text).flatMap((part) => (part.kind === "link" ? [part.href] : []));

describe("FAQ answers", () => {
  it("answers all 22 questions of the four support groups, in page order", () => {
    expect([WEBSITE_FAQ.length, CHATTING_FAQ.length, ASTROLOGERS_FAQ.length, BILLING_FAQ.length]).toEqual([6, 6, 8, 2]);
    expect(SUPPORT_FAQ).toHaveLength(22);
  });

  it("adds a group on free minutes, invites, e-mail confirmation and the free calculators", () => {
    expect(ACCOUNT_FAQ.length).toBeGreaterThanOrEqual(4);
    const questions = ACCOUNT_FAQ.map((item) => item.q).join(" | ");
    expect(questions).toMatch(/free minutes do I get/i);
    expect(questions).toMatch(/inviting friends/i);
    expect(questions).toMatch(/confirm my e-mail address/i);
    expect(questions).toMatch(/calculators free/i);
  });

  it("has a real answer for every entry, never the support-team placeholder", () => {
    for (const item of FAQ_PAGE_ENTRIES) {
      expect(item.q.trim().length, item.q).toBeGreaterThan(10);
      expect(item.a.trim().length, item.q).toBeGreaterThan(80);
      expect(item.a, item.q).not.toMatch(/^For help with this question/i);
    }
    expect(new Set(FAQ_PAGE_ENTRIES.map((item) => item.q)).size).toBe(FAQ_PAGE_ENTRIES.length);
  });

  for (const locale of ["en", "fr", "es"] as const)
    it(`${locale}: every question and answer has a non-empty translation, with the same links`, () => {
      const copy = dictionaries[locale];
      for (const item of FAQ_PAGE_ENTRIES) {
        for (const key of [item.q, item.a]) {
          expect(copy[key], `${locale}: ${key.slice(0, 60)}`).toBeTruthy();
          if (locale !== "en" && key.length > 60) expect(copy[key], `${locale} is translated: ${key.slice(0, 60)}`).not.toBe(key);
        }
        expect(hrefs(copy[item.a]), `${locale} links: ${item.q}`).toEqual(hrefs(item.a));
      }
    });

  it("only links to internal pages and the support address", () => {
    for (const item of FAQ_PAGE_ENTRIES)
      for (const href of hrefs(item.a)) expect(resolveHref(href, "en"), `${item.q}: ${href}`).not.toBeNull();
  });

  it("uses the wording the page and the payment processor allow", () => {
    for (const item of FAQ_PAGE_ENTRIES) {
      const text = `${item.q} ${item.a}`;
      // faq.test.tsx and docs §25.1/§25.6: no audit/assurance/verification wording, no membership, no claims.
      expect(text, item.q).not.toMatch(/audit|assurance|verification|financial infrastructure|subscription|premium|membership|App Store|Play Store/i);
      expect(text, item.q).not.toMatch(/certified|vetted|accurate|guarantee|therapy|healing|karmic|competitors/i);
    }
  });

  it("states the e-mail confirmation rules of Terms 3.8", () => {
    const answer = SUPPORT_FAQ.find((item) => /confirm STARYIELD Website account's email/.test(item.q))!.a;
    expect(answer).toMatch(/confirm/i);
    expect(answer).toContain("works once");
    expect(answer).toContain("48 hours");
    expect(answer).toMatch(/new link cancels the earlier ones/);
  });

  it("explains the password reset with the reset link", () => {
    const answer = SUPPORT_FAQ.find((item) => /reset staryield Website account's password/.test(item.q))!.a;
    expect(answer).toContain("reset link");
    expect(answer).toContain("works once");
  });

  it("says invite rewards are free minutes, \"currently\" 5 as in Terms 4.20.3, granted at launch on a date we choose", () => {
    const earn = ACCOUNT_FAQ.find((item) => /inviting friends/.test(item.q))!.a;
    expect(earn).toMatch(/confirms their e-mail address/);
    expect(earn).toMatch(/live reading of at least one minute/);
    expect(earn).toContain("currently 5 free chat minutes");
    expect(earn).toContain("the reward and the limit in force");
    expect(ACCOUNT_FAQ.map((item) => item.a).join(" ")).toMatch(/added to your account at launch, on a date we choose/);
  });

  it("finds each of the 22 answers from the label of its Figma question", () => {
    for (const item of SUPPORT_FAQ) {
      expect(faqAnswerForLabel(`↑ ${item.q} TECHNICAL EXECUTION SECURITY`), item.q).toBe(item.a);
      expect(faqAnswerForLabel(`☉ ${item.q} CONSULTATION PROTOCOLS`), item.q).toBe(item.a);
    }
    expect(faqAnswerForLabel("☉ Some question we never answered. TAG")).toBeUndefined();
  });
});
