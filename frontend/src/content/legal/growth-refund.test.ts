import { describe, expect, it } from "vitest";

import { legalDocuments } from "@/content/legal";
import type { LegalBlock, LegalDocumentContent } from "@/content/legal/types";

/**
 * Growth release, Refund Policy: §2 describes the configured sign-up offer instead of a fixed "four free reading
 * minutes", says that offers on booking buttons, pages and reminders follow the same rules, and adds free mode
 * (`#refund-free-mode`) and the free calculators, result images and guide pages (`#refund-free-tools`); §6 adds
 * free minutes (`#refund-free-minutes`) and referral reward minutes (`#refund-referral-rewards`); §12 says free
 * minutes and pending rewards are forfeited on final closure. The 14 sections and every existing anchor are kept.
 * The version and dates are bumped by the lead with LEGAL_VERSIONS, so they are not asserted here.
 */
const locales = ["en", "fr", "es"] as const;
type Locale = (typeof locales)[number];

const textOf = (block: LegalBlock): string =>
  block.type === "p" || block.type === "callout"
    ? block.text
    : block.type === "ul" || block.type === "ol"
      ? block.items.join("\n")
      : block.type === "table"
        ? [...block.head, ...block.rows.flat()].join("\n")
        : block.type === "button"
          ? block.label
          : "title" in block
            ? block.title
            : "";
const allIds = (doc: LegalDocumentContent) =>
  [...doc.intro, ...doc.sections.flatMap((s) => s.blocks)].flatMap((b) => ("id" in b && b.id ? [b.id] : [])).concat(doc.sections.map((s) => s.id));
const sectionOf = (doc: LegalDocumentContent, id: string) => {
  const found = doc.sections.find((s) => s.id === id);
  if (!found) throw new Error(`no section ${id}`);
  return found;
};
const section = (doc: LegalDocumentContent, id: string) => sectionOf(doc, id).blocks.map(textOf).join("\n");
/** The paragraphs after an h4 up to the next heading or the end of its section. */
const clause = (doc: LegalDocumentContent, id: string) => {
  const owner = doc.sections.find((s) => s.blocks.some((b) => (b.type === "h3" || b.type === "h4") && b.id === id));
  if (!owner) throw new Error(`no clause ${id}`);
  const blocks = owner.blocks;
  const start = blocks.findIndex((b) => (b.type === "h3" || b.type === "h4") && b.id === id);
  const rest = blocks.slice(start + 1);
  const end = rest.findIndex((b) => b.type === "h3" || b.type === "h4");
  return (end < 0 ? rest : rest.slice(0, end)).map(textOf).join("\n");
};
const refund = (l: Locale) => legalDocuments.refund[l];

/** The new anchors and the section each one belongs to. */
const NEW_ANCHORS: [id: string, section: string][] = [
  ["refund-free-mode", "free-trial"],
  ["refund-free-tools", "free-trial"],
  ["refund-free-minutes", "bonus-and-promotional-credits"],
  ["refund-referral-rewards", "bonus-and-promotional-credits"],
];

const SECTIONS = [
  "one-time-purchases", "free-trial", "credit-delivery", "credit-pack-refunds", "welcome-pack-and-promotions",
  "bonus-and-promotional-credits", "technical-problems-and-incorrect-charges", "concerns-about-a-reading",
  "requesting-a-refund", "approved-refunds", "payment-disputes", "account-closure", "policy-updates", "contact-us",
];
const EXISTING_HEADINGS = ["refund-unused-packs", "refund-partial-packs", "refund-completed-readings", "refund-right-of-withdrawal"];

describe("growth release: Refund Policy", () => {
  describe("anchors and structure", () => {
    it.each(locales)("%s: the 14 sections keep their ids and numbers, and the existing headings stay", (l) => {
      const doc = refund(l);
      expect(doc.sections.map((s) => s.id)).toEqual(SECTIONS);
      expect(doc.sections.map((s) => s.n)).toEqual(SECTIONS.map((_, i) => String(i + 1)));
      const ids = allIds(doc);
      for (const id of EXISTING_HEADINGS) expect(ids).toContain(id);
    });

    it.each(locales)("%s: every new anchor is an h4 in its section, and ids are identical to English", (l) => {
      const doc = refund(l);
      for (const [id, sectionId] of NEW_ANCHORS)
        expect(sectionOf(doc, sectionId).blocks.find((b) => b.type === "h4" && b.id === id), id).toBeDefined();
      expect(allIds(doc)).toEqual(allIds(refund("en")));
    });

    it("the Terms anchors the new text links to exist", () => {
      const terms = allIds(legalDocuments.terms.en);
      for (const id of ["free-trials-and-promotional-offers", "free-calculators-and-guides", "referral-program", "account-deletion"]) expect(terms).toContain(id);
    });
  });

  describe("§2 sign-up offer", () => {
    const FIXED_AMOUNT: Record<Locale, RegExp> = { en: /\bfour\b/i, fr: /\bquatre\b/i, es: /\bcuatro\b/i };

    it.each(locales)("%s: describes the configured offer, with no fixed number of minutes", (l) => {
      const first = textOf(sectionOf(refund(l), "free-trial").blocks[0]);
      expect(first).not.toMatch(FIXED_AMOUNT[l]);
      expect(first).not.toMatch(/\d/);
      expect(first).toContain("(/pricing)");
      expect(first).toMatch(
        {
          en: /When our sign-up offer is available[\s\S]*Staryield sets the number of minutes[\s\S]*during sign-up[\s\S]*change the offer or switch it off at any time[\s\S]*only to accounts created afterwards[\s\S]*new accounts only, one per person/,
          fr: /Lorsque notre offre d’inscription est disponible[\s\S]*Staryield fixe ce nombre de minutes[\s\S]*lors de l’inscription[\s\S]*modifier l’offre ou la désactiver à tout moment[\s\S]*qu’aux comptes créés ensuite[\s\S]*réservée aux nouveaux comptes, à raison d’une seule par personne/,
          es: /Cuando nuestra oferta de registro está disponible[\s\S]*Staryield fija la cantidad de minutos[\s\S]*durante el registro[\s\S]*cambiar la oferta o desactivarla en cualquier momento[\s\S]*solo se aplica a las cuentas creadas después[\s\S]*solo para cuentas nuevas, una por persona/,
        }[l],
      );
    });

    it("keeps the design sentence on the trial and the hold paragraph", () => {
      const text = section(refund("en"), "free-trial");
      expect(text).toContain("The trial requires no purchase, does not convert into a subscription, and does not result in a charge.");
      expect(text).toContain("Free trial minutes have no cash or refund value.");
      expect(text).toContain("Nothing is charged for the hold");
    });

    it.each(locales)("%s: offers on buttons, pages and reminders follow the same rules", (l) => {
      expect(section(refund(l), "free-trial")).toMatch(
        {
          en: /booking buttons, pages or reminders, such as “free minutes for new members”, describe the offer in force when they were shown/,
          fr: /boutons de réservation, les pages ou les rappels, comme « minutes gratuites pour les nouveaux membres », décrivent l’offre en vigueur/,
          es: /botones de reserva, las páginas o los recordatorios, como «minutos gratis para nuevos miembros», describen la oferta vigente/,
        }[l],
      );
    });

    it.each(locales)("%s: free mode and the free tools involve no payment", (l) => {
      const doc = refund(l);
      const freeMode = clause(doc, "refund-free-mode");
      expect(freeMode).toContain("(/terms#free-trials-and-promotional-offers)");
      expect(freeMode).toMatch(
        {
          en: /uses no free minutes or Credits and involves no payment[\s\S]*Credits you buy during free mode are covered by this policy/,
          fr: /ne consomme ni minutes offertes ni Crédits et n’entraîne aucun paiement[\s\S]*Crédits que vous achetez pendant le mode gratuit restent couverts/,
          es: /no consume minutos gratuitos ni Créditos y no implica ningún pago[\s\S]*Créditos que compre durante el modo gratuito quedan cubiertos/,
        }[l],
      );
      const tools = clause(doc, "refund-free-tools");
      expect(tools).toContain("(/terms#free-calculators-and-guides)");
      expect(tools).toMatch(
        {
          en: /full natal chart[\s\S]*tarot birth card[\s\S]*zodiac compatibility[\s\S]*result images[\s\S]*guide pages[\s\S]*no payment and uses no Credits or free minutes/,
          fr: /thème natal complet[\s\S]*carte de naissance du tarot[\s\S]*compatibilité des signes[\s\S]*images de résultats[\s\S]*pages de guide[\s\S]*aucun paiement et ne consomme ni Crédits ni minutes offertes/,
          es: /carta natal completa[\s\S]*carta de nacimiento del tarot[\s\S]*compatibilidad de signos[\s\S]*imágenes de resultados[\s\S]*páginas de guía[\s\S]*no implica ningún pago ni consume Créditos ni minutos gratuitos/,
        }[l],
      );
    });
  });

  describe("§6 free minutes and referral rewards", () => {
    it.each(locales)("%s: free minutes have no cash value and cannot be refunded or transferred", (l) => {
      const text = clause(refund(l), "refund-free-minutes");
      expect(text).toMatch(
        {
          en: /promotional[\s\S]*only for live chat readings[\s\S]*no cash value and cannot be refunded, exchanged for money or Credits, sold or transferred[\s\S]*nothing to refund in money[\s\S]*do not reduce the refundable amount of a credit pack/,
          fr: /promotionnelles[\s\S]*qu’à des consultations par chat en direct[\s\S]*aucune valeur en espèces et ne peuvent être ni remboursées, ni échangées contre de l’argent ou des Crédits, ni vendues, ni transférées[\s\S]*rien à rembourser en argent[\s\S]*ne réduisent pas le montant remboursable d’un pack de crédits/,
          es: /promocionales[\s\S]*para consultas por chat en vivo[\s\S]*No tienen valor en efectivo y no pueden reembolsarse, canjearse por dinero o Créditos, venderse ni transferirse[\s\S]*nada que reembolsar en dinero[\s\S]*no reducen el importe reembolsable de un paquete de créditos/,
        }[l],
      );
    });

    it.each(locales)("%s: a referral reward is added shortly after the friend qualifies, survive the friend's refund and may be forfeited for abuse", (l) => {
      const text = clause(refund(l), "refund-referral-rewards");
      expect(text).toContain("(/terms#referral-program)");
      expect(text).toMatch(
        {
          en: /A reward is added to your account shortly after your friend qualifies; until then, a pending reward cannot be used, refunded or exchanged\.[\s\S]*refund or payment dispute by the friend you invited does not change them[\s\S]*withhold or reject it[\s\S]*abusing the program, its pending rewards and free minutes may be forfeited/,
          fr: /Une récompense est ajoutée à votre compte peu après que votre ami a rempli les conditions ; d’ici là, une récompense en attente ne peut être ni utilisée, ni remboursée, ni échangée\.[\s\S]*contestation de paiement ultérieurs de la part de l’ami que vous avez invité ne les modifient pas[\s\S]*la retenir ou la refuser[\s\S]*abus du programme, ses récompenses en attente et ses minutes offertes peuvent être perdues/,
          es: /Una recompensa se añade a su cuenta poco después de que su amigo cumpla las condiciones; hasta entonces, una recompensa pendiente no puede usarse, reembolsarse ni canjearse\.[\s\S]*disputa de pago posteriores del amigo que invitó no los modifican[\s\S]*retenerla o rechazarla[\s\S]*abusar del programa, sus recompensas pendientes y sus minutos gratuitos pueden perderse/,
        }[l],
      );
    });
  });

  describe("§12 account closure", () => {
    it.each(locales)("%s: free minutes and rewards not yet added are forfeited on final closure; pack refund rights stay", (l) => {
      const text = section(refund(l), "account-closure");
      expect(text).toContain("(/terms#account-deletion)");
      expect(text).toMatch(
        {
          en: /not refunded or paid out when an account is closed[\s\S]*referral rewards not yet added are forfeited[\s\S]*does not affect the refund rights for purchased credit packs/,
          fr: /ni remboursées ni versées en argent lors de la clôture d’un compte[\s\S]*récompenses de parrainage non encore ajoutées sont perdues[\s\S]*n’affecte pas les droits à remboursement des packs de crédits achetés/,
          es: /no se reembolsan ni se pagan en dinero cuando se cierra una cuenta[\s\S]*recompensas por referidos aún no añadidas[\s\S]*no afecta a los derechos de reembolso de los paquetes de créditos comprados/,
        }[l],
      );
    });
  });

  it.each(locales)("%s: the new text makes no membership, outcome or accuracy claim", (l) => {
    const doc = refund(l);
    const added = [
      textOf(sectionOf(doc, "free-trial").blocks[0]),
      ...NEW_ANCHORS.map(([id]) => clause(doc, id)),
      section(doc, "account-closure"),
    ].join("\n");
    expect(added).not.toMatch(/membership|membresía|adhésion|abonnement mensuel|Premium/i);
    expect(added).not.toMatch(
      { en: /guarantee|certified|healing|therapy|\baccurate\b/i, fr: /garanti|certifié|guérison|thérapie|\bprécis\b/i, es: /garantiz|certificad|curación|terapia|\bprecis[oa]s?\b/i }[l],
    );
  });
});

describe("the referral program is no longer in beta and has no launch (Refund Policy)", () => {
  const OLD_WORDING = /\bbeta\b|bêta|launch|lancement|lancées?\b|lanzamiento|lanzadas?\b|lancemos|lanzar/i;

  it.each(locales)("%s: nothing in the Refund Policy says beta or launch about referral rewards", (l) => {
    const doc = refund(l);
    expect(clause(doc, "refund-referral-rewards")).not.toMatch(OLD_WORDING);
    const withReferral = doc.sections.flatMap((s) => s.blocks).map(textOf).filter((t) => /referral|parrain|referid|invit/i.test(t));
    expect(withReferral.length).toBeGreaterThan(1);
    for (const t of withReferral) expect(t).not.toMatch(OLD_WORDING);
  });
});
