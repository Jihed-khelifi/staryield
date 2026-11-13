import { describe, expect, it } from "vitest";

import { legalDocuments } from "@/content/legal";
import type { LegalBlock, LegalDocumentContent } from "@/content/legal/types";

const locales = ["en", "fr", "es"] as const;
type Locale = (typeof locales)[number];

const textOf = (block: LegalBlock): string =>
  block.type === "p"
    ? block.text
    : block.type === "ul" || block.type === "ol"
      ? block.items.join("\n")
      : block.type === "table"
        ? [...block.head, ...block.rows.flat()].join("\n")
        : "";

/** The text of one clause: the blocks after its h3 up to the next h3 of the same section (or the whole section for a section id). */
function clause(doc: LegalDocumentContent, id: string): string {
  const section = doc.sections.find((s) => s.id === id);
  if (section) return section.blocks.map(textOf).join("\n");
  for (const s of doc.sections) {
    const start = s.blocks.findIndex((b) => b.type === "h3" && b.id === id);
    if (start < 0) continue;
    const rest = s.blocks.slice(start + 1);
    const end = rest.findIndex((b) => b.type === "h3");
    return (end < 0 ? rest : rest.slice(0, end)).map(textOf).join("\n");
  }
  throw new Error(`no clause ${id}`);
}
const everything = (doc: LegalDocumentContent) => [...doc.intro, ...doc.sections.flatMap((s) => s.blocks)].map(textOf).join("\n");
const tables = (doc: LegalDocumentContent) => doc.sections.flatMap((s) => s.blocks).filter((b): b is Extract<LegalBlock, { type: "table" }> => b.type === "table");

const NICKNAME: Record<Locale, RegExp> = { en: /nickname/i, fr: /surnom/i, es: /apodo/i };
const LIKE: Record<Locale, RegExp> = { en: /\blikes?\b/i, fr: /J’aime/, es: /Me gusta/ };
const OLD_GOOGLE_SENTENCE: Record<Locale, string> = {
  en: "sign-in with Google, Apple",
  fr: "connexion avec Google, Apple",
  es: "inicio de sesión con Google, Apple",
};
const ESSENTIAL: Record<Locale, string> = { en: "Essential", fr: "Essentiel", es: "Esencial" };

describe("revisions 4.0 legal updates", () => {
  it.each(locales.flatMap((l) => (["privacy", "terms"] as const).map((d) => [d, l] as const)))("%s/%s is dated 2026-11-13 (re-dated by revisions 5.1)", (d, l) => {
    expect(legalDocuments[d][l].meta).toMatchObject({ version: "2026-11-13", effective: "2026-11-13", lastUpdated: "2026-11-13" });
  });

  it.each(locales)("%s: neither document links to an external website", (l) => {
    for (const d of ["privacy", "terms"] as const) {
      const all = everything(legalDocuments[d][l]);
      for (const m of all.matchAll(/\]\(([^)]+)\)/g)) expect(m[1]).not.toMatch(/^(https?:)?\/\//);
      expect(all).not.toMatch(/trustpilot\.com|google\.com/i);
    }
  });

  describe("Privacy Policy", () => {
    it.each(locales)("%s: section 2.2 now describes Google sign-in and no longer says it is not offered", (l) => {
      const doc = legalDocuments.privacy[l];
      const text = clause(doc, "data-we-receive-from-third-parties");
      expect(text).toContain("Google");
      expect(text).not.toContain(OLD_GOOGLE_SENTENCE[l]);
      expect(text).toContain("Apple, Facebook");
      expect(text).toContain("(#cookies)");
      // The sentence in the introduction to section 2 and the sign-in statement agree: Google is offered.
      expect(everything(doc)).not.toContain(OLD_GOOGLE_SENTENCE[l]);
    });

    it("en: section 2.2 states exactly what Google gives us and what we do not receive or store", () => {
      const text = clause(legalDocuments.privacy.en, "data-we-receive-from-third-parties");
      expect(text).toContain("Google account identifier, your email address and whether Google has verified that address");
      expect(text).toContain("not your name, picture, contacts or any other Google data");
      expect(text).toContain("We do not receive your Google password and we store no Google access tokens");
      expect(text).toContain("to perform our contract with you");
      expect(text).toContain("only after you confirm with your account password");
      expect(text).toContain("disconnect Google in your account settings, once you have created a password");
      expect(text).toContain("deleted when you disconnect Google or when your account is erased");
    });

    it.each(locales)("%s: replies and likes are stored with the conversation and exported (section 2.5)", (l) => {
      const text = clause(legalDocuments.privacy[l], "live-readings-ratings-and-notifications");
      expect(text).toMatch(LIKE[l]);
      expect(text).toContain("(#data-retention)");
    });

    it.each(locales)("%s: private client nicknames are described with the advisor notes (section 2.7)", (l) => {
      const text = clause(legalDocuments.privacy[l], "advisor-notes");
      expect(text).toMatch(NICKNAME[l]);
      expect(text).toContain("(/contact?topic=privacy)");
    });

    it.each(locales)("%s: staff can view a nickname only with a stated reason, and each view is audited (section 2.7)", (l) => {
      const staffAccess: Record<Locale, RegExp> = {
        en: /Authorised Staryield staff can view a nickname only when they need to.*state a reason first.*audit log/,
        fr: /personnel autorisé de Staryield ne peut consulter un surnom que lorsque cela est nécessaire.*motif.*journal d’audit/,
        es: /personal autorizado de Staryield solo puede consultar un apodo cuando sea necesario.*motivo.*registro de auditoría/,
      };
      expect(clause(legalDocuments.privacy[l], "advisor-notes")).toMatch(staffAccess[l]);
    });

    it.each(locales)("%s: signing out removes the push subscription and read status syncs (sections 2.8 and 2.11)", (l) => {
      const doc = legalDocuments.privacy[l];
      expect(clause(doc, "account-devices-and-preferences")).toMatch(/push/i);
      expect(clause(doc, "chat-notifications-and-drafts")).toMatch(/badge|icon|icône|icono/i);
    });
    it("en: the push and badge wording is exact", () => {
      const doc = legalDocuments.privacy.en;
      expect(clause(doc, "account-devices-and-preferences")).toContain("Signing out removes that browser’s push subscription from our systems.");
      expect(clause(doc, "account-devices-and-preferences")).toContain("Signing out everywhere, changing or resetting your password, and deactivating or closing your account remove all of your push subscriptions.");
      expect(clause(doc, "chat-notifications-and-drafts")).toContain("we collect no new data for it");
    });

    it.each(locales)("%s: Google sign-in and the Trustpilot widget appear in the purposes table, the legitimate interests, sharing, transfers and the US disclosure table", (l) => {
      const doc = legalDocuments.privacy[l];
      const purposes = tables(doc)[0];
      expect(purposes.rows.some((r) => r[0].includes("Google"))).toBe(true);
      expect(purposes.rows.some((r) => r[0].includes("Trustpilot"))).toBe(true);
      expect(purposes.rows.every((r) => r.length === 3)).toBe(true);
      const sharing = clause(doc, "how-we-share-your-personal-data");
      expect(sharing).toContain("Google (");
      expect(sharing).toContain("Trustpilot A/S");
      expect(clause(doc, "legitimate-interests")).toContain("Trustpilot");
      expect(clause(doc, "international-data-transfers")).toMatch(/Google/);
      expect(clause(doc, "international-data-transfers")).toMatch(/Trustpilot A\/S/);
      expect(clause(doc, "third-party-websites-and-applications")).toMatch(/Google[\s\S]*Trustpilot/);
      const disclosure = tables(doc)[1];
      expect(disclosure.rows.flat().join("\n")).toMatch(/Google/);
      expect(disclosure.rows.flat().join("\n")).toMatch(/Trustpilot/);
    });

    it("en: Trustpilot is described as the only third-party embed and loads only without GPC or Do Not Track", () => {
      const sharing = clause(legalDocuments.privacy.en, "how-we-share-your-personal-data");
      expect(sharing).toContain("the only third-party content we embed in our pages");
      expect(sharing).toContain("loaded from Trustpilot’s servers when it scrolls into view on the home page, the reviews page, the footer of our pages and the end-of-chat summary");
      expect(sharing).toContain("Trustpilot then receives your IP address, browser details and the address of the page");
      expect(sharing).toContain("not loaded when your browser sends a Global Privacy Control or Do Not Track signal; a plain link is shown instead");
      expect(sharing).toContain("We never send Trustpilot your account details, your email address or anything about your readings");
      expect(sharing).toContain("We never offer anything in exchange for a review");
      expect(sharing).toContain("we send Google nothing about your readings, purchases or messages");
      expect(sharing).toContain("We do not use third-party analytics, session-recording or advertising tools");
    });

    it.each(locales)("%s: the cookie table lists sy_oa as an essential 10-minute cookie and the Trustpilot frame is addressed", (l) => {
      const doc = legalDocuments.privacy[l];
      const cookies = tables(doc).find((t) => t.head.length === 5)!;
      const row = cookies.rows.find((r) => r[0] === "sy_oa");
      expect(row).toBeTruthy();
      expect(row![1]).toBe("Cookie");
      expect(row![3]).toMatch(/\b10\b/);
      expect(row![4]).toBe(ESSENTIAL[l]);
      expect(clause(doc, "cookies")).toContain("Trustpilot");
    });
    it("en: the sy_oa row has the agreed purpose and duration", () => {
      const cookies = tables(legalDocuments.privacy.en).find((t) => t.head.length === 5)!;
      expect(cookies.rows.find((r) => r[0] === "sy_oa")).toEqual([
        "sy_oa",
        "Cookie",
        "Binds a Google sign-in attempt to this browser so that nobody else can complete it",
        "Up to 10 minutes",
        "Essential",
      ]);
    });

    it.each(locales)("%s: the cookie table also lists the Google intent and push storage keys", (l) => {
      const cookies = tables(legalDocuments.privacy[l]).find((t) => t.head.length === 5)!;
      for (const name of ["staryield.google.intent", "staryield.push.*"]) {
        const row = cookies.rows.find((r) => r[0] === name);
        expect(row).toBeTruthy();
        expect(row![4]).toBe(ESSENTIAL[l]);
      }
      expect(cookies.rows.find((r) => r[0] === "staryield.google.intent")![3]).toMatch(/\b15\b/);
    });
    it("en: the storage rows have the agreed wording", () => {
      const cookies = tables(legalDocuments.privacy.en).find((t) => t.head.length === 5)!;
      expect(cookies.rows.find((r) => r[0] === "staryield.google.intent")).toEqual([
        "staryield.google.intent",
        "Session storage",
        "Remembers whether you were signing in, signing up or connecting Google, so that “Try again” restarts the same step",
        "Up to 15 minutes, or until the tab is closed",
        "Essential",
      ]);
      expect(cookies.rows.find((r) => r[0] === "staryield.push.*")).toEqual([
        "staryield.push.*",
        "Local storage",
        "Remembers when this browser’s push subscription was last sent to us, and a one-way hash of its address, so that it is re-sent only when it changes; removed when you sign out",
        "Until you sign out or turn push notifications off",
        "Essential",
      ]);
    });

    it.each(locales)("%s: keeps exactly three tables", (l) => {
      expect(tables(legalDocuments.privacy[l])).toHaveLength(3);
    });
  });

  describe("Terms of Service", () => {
    it.each(locales)("%s: registering with Google, keeping that account secure and the password-first rule (3.1, 3.5)", (l) => {
      const doc = legalDocuments.terms[l];
      expect(clause(doc, "creating-an-account")).toContain("Google");
      expect(clause(doc, "creating-an-account")).toContain("(#age-restriction)");
      expect(clause(doc, "creating-an-account")).toContain("(/privacy#data-we-receive-from-third-parties)");
      expect(clause(doc, "account-security")).toContain("Google");
    });
    it("en: the Google clauses are exact", () => {
      const doc = legalDocuments.terms.en;
      expect(clause(doc, "creating-an-account")).toContain("The age requirement in [Section 3.3](#age-restriction) and your acceptance of these Terms apply in exactly the same way when you register with Google");
      expect(clause(doc, "account-security")).toContain("anyone who can use your Google account can access your Staryield Account");
      expect(clause(doc, "account-security")).toContain("ask you to create a Staryield password first");
    });

    it.each(locales)("%s: reviews on independent platforms (4.4) mention Trustpilot, no incentives and the advisors exclusion", (l) => {
      expect(clause(legalDocuments.terms[l], "user-reviews-and-testimonials")).toContain("Trustpilot");
    });
    it("en: the review rules are exact", () => {
      const text = clause(legalDocuments.terms.en, "user-reviews-and-testimonials");
      expect(text).toContain("are voluntary and must reflect your genuine experience");
      expect(text).toContain("we never offer credits, discounts or any other benefit for writing a review or for changing or removing one");
      expect(text).toContain("Advisors, employees, contractors and their close relatives must not review Staryield on such platforms");
    });

    it.each(locales)("%s: replies and likes are part of the conversation (4.11)", (l) => {
      expect(clause(legalDocuments.terms[l], "live-readings-messages-and-images")).toMatch(LIKE[l]);
    });
    it("en: a like is not advice, a confirmation or a commitment", () => {
      expect(clause(legalDocuments.terms.en, "live-readings-messages-and-images")).toContain("is not advice, a confirmation or a commitment by that advisor");
    });

    it("en: signing out stops notifications on that device (4.13)", () => {
      expect(clause(legalDocuments.terms.en, "notifications")).toContain("Signing out stops push notifications on that device.");
    });
    it.each(locales)("%s: nicknames are covered by the client notes conduct rules (4.14)", (l) => {
      const text = clause(legalDocuments.terms[l], "client-notes");
      expect(text).toMatch(NICKNAME[l]);
      expect(text).toContain("(/standards)");
      expect(text).toContain("(/privacy#advisor-notes)");
    });
    it("en: nickname conduct", () => {
      const text = clause(legalDocuments.terms.en, "client-notes");
      expect(text).toContain("Nicknames must be respectful and must not be discriminatory or insulting");
      expect(text).toContain("We may remove a nickname that breaches these Terms");
    });

    it.each(locales)("%s: Google and Trustpilot are third-party services (5)", (l) => {
      const text = clause(legalDocuments.terms[l], "third-party-services-materials-and-advertising");
      expect(text).toContain("Google");
      expect(text).toContain("Trustpilot");
      expect(text).toContain("(/privacy#how-we-share-your-personal-data)");
    });
  });
});
