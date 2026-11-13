import { describe, expect, it } from "vitest";

import advisors from "@/data/advisors.json";
import copyEn from "@/i18n/dictionaries/copy-en.json";
import copyEs from "@/i18n/dictionaries/copy-es.json";
import copyFr from "@/i18n/dictionaries/copy-fr.json";
import en from "@/i18n/dictionaries/en.json";
import es from "@/i18n/dictionaries/es.json";
import fr from "@/i18n/dictionaries/fr.json";

/**
 * Revisions 5.1 (docs/platform-extensions.md §25.6): the public site must say what the Terms say (section 2.1:
 * for entertainment purposes only). A payment processor reads the site against card-scheme website rules, so the
 * display copy carries no certification, verification, accuracy or guarantee claim, no healing or therapy claim,
 * no karma-clearing promise, no hard-coded statistics or testimonials, and no membership or subscription offer.
 *
 * Legal wording is not marketing: the Disclaimer page, the Terms and the standards page may name these words
 * to deny them (e.g. "we do not certify or verify"); those sentences are listed in ALLOWED.
 */
const BANNED: Record<"en" | "fr" | "es", RegExp[]> = {
  en: [/certified (?:specialist|practitioner|psychic|strategic)/i, /verified master/i, /\bvetted\b/i, /profoundly accurate/i, /\baccurate, intuitive/i, /karmic debt/i, /life[- ]changing/i, /alignment audits?/i, /continuous audit/i, /top-rated specialists/i, /\bexpert guidance\b/i, /Crystal Healing/i, /devoted to healing/i, /Deep Healing/i, /regression therapy/i, /natural remedies/i, /Premium (?:subscription|Features)/i, /Web subscription/i, /Legit or Scam/i, /better than competitors/i, /17,419|10K\+|50\+ Countries|Countries Serviced|Average Platform Rating|H\. Reynolds/i, /over 1000|millions of people/i, /\$14 in free credits/i, /3 min free then 40 credits/i],
  fr: [/spécialistes? certifiés?/i, /conseiller[s]? certifié/i, /guérison/i, /dette karmique/i, /audits? (?:continu|d’alignement)/i, /thérapie/i],
  es: [/especialistas? certificados?/i, /curación/i, /deuda kármica/i, /auditor[ií]as? de alineaci[oó]n/i, /terapia de regresi[oó]n/i],
};
// "Premium Membership" stays as the billing-history label of memberships that already exist (the membership surfaces are
// switched off, not removed: docs/platform-extensions.md §25.1), so it is not banned here; the legal documents are checked
// by src/content/legal/revisions-5-1.test.ts.
// Sentences that deny a claim (Disclaimer, Terms 2.x, standards): they may contain the words.
const ALLOWED = [/we do not certify or verify/i, /do not certify/i];

const lists = {
  en: { copy: Object.entries(copyEn).flat(), dict: JSON.stringify(en) },
  fr: { copy: Object.entries(copyFr).flat(), dict: JSON.stringify(fr) },
  es: { copy: Object.entries(copyEs).flat(), dict: JSON.stringify(es) },
};

describe("display copy makes no claim that conflicts with 'for entertainment purposes only'", () => {
  for (const locale of ["en", "fr", "es"] as const) {
    it(`${locale}: copy and structured dictionaries are free of banned claims`, () => {
      const found: string[] = [];
      const strings = [...lists[locale].copy, lists[locale].dict];
      for (const value of strings) {
        if (ALLOWED.some((allowed) => allowed.test(value))) continue;
        for (const re of BANNED[locale]) if (re.test(value)) found.push(`${re}: ${value.slice(0, 100)}`);
      }
      expect(found).toEqual([]);
    });
  }

  it("the English keys (source strings) are free of banned claims too", () => {
    const found = Object.keys(copyEn).filter((key) => !ALLOWED.some((a) => a.test(key)) && BANNED.en.some((re) => re.test(key)));
    expect(found).toEqual([]);
  });

  it("advisors.json carries no hard-coded bios, ratings or statistics", () => {
    for (const advisor of advisors) expect(Object.keys(advisor).sort()).toEqual(["name", "slug"]);
  });

  it("the footer tagline and copyright line are translated in every locale", () => {
    expect(copyEn["Astrology and psychic readings for entertainment purposes only."]).toBeTruthy();
    expect(copyFr["Astrology and psychic readings for entertainment purposes only."]).toContain("divertissement");
    expect(copyEs["Astrology and psychic readings for entertainment purposes only."]).toContain("entretenimiento");
    expect(copyEn["© 2026 Staryield Media LLC. All rights reserved. For entertainment purposes only."]).toBeTruthy();
  });
});
