import { describe, expect, it } from "vitest";

import { legalDocuments } from "@/content/legal";
import type { LegalBlock, LegalDocumentContent } from "@/content/legal/types";

const locales = ["en", "fr", "es"] as const;
type Locale = (typeof locales)[number];
type Doc = "terms" | "privacy";

const textOf = (block: LegalBlock): string =>
  block.type === "p" || block.type === "callout"
    ? block.text
    : block.type === "ul" || block.type === "ol"
      ? block.items.join("\n")
      : block.type === "table"
        ? [...block.head, ...block.rows.flat()].join("\n")
        : block.type === "button"
          ? block.label
          : "";

/** The blocks after an h3/h4 up to the next heading of the same or higher level (or the whole section for a section id). */
function clause(doc: LegalDocumentContent, id: string): string {
  const section = doc.sections.find((s) => s.id === id);
  if (section) return section.blocks.map(textOf).join("\n");
  for (const s of doc.sections) {
    const start = s.blocks.findIndex((b) => (b.type === "h3" || b.type === "h4") && b.id === id);
    if (start < 0) continue;
    const level = (s.blocks[start] as { type: "h3" | "h4" }).type;
    const rest = s.blocks.slice(start + 1);
    const end = rest.findIndex((b) => b.type === "h3" || (level === "h4" && b.type === "h4"));
    return (end < 0 ? rest : rest.slice(0, end)).map(textOf).join("\n");
  }
  throw new Error(`no clause ${id}`);
}
const everything = (doc: LegalDocumentContent) => [...doc.intro, ...doc.sections.flatMap((s) => s.blocks)].map(textOf).join("\n");

const doc = (d: Doc | "refund", l: Locale) => legalDocuments[d][l];
const DOCS = locales.flatMap((l) => (["privacy", "terms"] as const).map((d) => [d, l] as const));

describe("revisions 5.0 legal updates", () => {
  it.each(DOCS)("%s/%s is dated 2026-11-13 (re-dated by revisions 5.1)", (d, l) => {
    expect(doc(d, l).meta).toMatchObject({ version: "2026-11-13", effective: "2026-11-13", lastUpdated: "2026-11-13" });
  });

  it.each(locales)("%s: the Refund Policy is dated 2026-11-13 (re-dated by revisions 5.1)", (l) => {
    expect(doc("refund", l).meta).toMatchObject({ version: "2026-11-13", effective: "2026-11-13", lastUpdated: "2026-11-13" });
  });

  describe("Terms", () => {
    const ORDER: Record<Locale, string> = {
      // revisions 5.1: membership Credits are gone from the published Terms.
      en: "purchased Credits first, then bonus Credits",
      fr: "d’abord les Crédits achetés, puis les Crédits bonus",
      es: "primero los Créditos comprados y luego los Créditos de bonificación",
    };
    const NO_AUTO_TOPUP: Record<Locale, RegExp> = { en: /never top up your Balance automatically/, fr: /ne rechargeons jamais votre Solde automatiquement/, es: /nunca recargamos su Saldo automáticamente/ };
    const AFTER_PAYMENT: Record<Locale, RegExp> = { en: /only after your payment has succeeded/, fr: /uniquement une fois votre paiement réussi/, es: /solo una vez que el pago se ha realizado correctamente/ };
    const WELCOME: Record<Locale, RegExp> = { en: /Welcome Pack[\s\S]*once per customer and once per payment method[\s\S]*cannot be combined with other promotions[\s\S]*not restored if you are refunded/, fr: /Pack de bienvenue[\s\S]*une fois par client et une fois par moyen de paiement[\s\S]*ne peut pas être combiné avec d’autres promotions[\s\S]*pas rétabli si vous en êtes remboursé/, es: /Paquete de bienvenida[\s\S]*una vez por cliente y una vez por método de pago[\s\S]*no puede combinarse con otras promociones[\s\S]*No se restablece si se le reembolsa/ };
    const SAVINGS: Record<Locale, RegExp> = { en: /Save 11%[\s\S]*standard pack rate[\s\S]*round a saving down/, fr: /11 %[\s\S]*tarif de pack standard[\s\S]*arrondissons toujours une économie à l’inférieur/, es: /11 %[\s\S]*tarifa de paquete estándar[\s\S]*redondeamos el ahorro a la baja/ };
    const OLD_NON_REFUNDABLE: Record<Locale, string> = {
      en: "purchases are non-refundable",
      fr: "les achats ne sont pas remboursables",
      es: "las compras no son reembolsables",
    };
    const OLD_TRIAL_BILLING: Record<Locale, string> = {
      en: "your access to the Service will automatically continue and you will be billed",
      fr: "votre accès au Service continuera automatiquement et les frais applicables vous seront facturés",
      es: "su acceso al Servicio continuará automáticamente y se le facturarán",
    };
    const AGE: Record<Locale, RegExp> = { en: /date of birth you give[\s\S]*restrict your Account until our team has reviewed it/, fr: /date de naissance que vous indiquez[\s\S]*restreignons votre compte jusqu’à son examen par notre équipe/, es: /fecha de nacimiento que facilita[\s\S]*restringiremos su Cuenta hasta que nuestro equipo la haya revisado/ };
    const FREE_MINUTES: Record<Locale, RegExp> = { en: /require no payment method[\s\S]*never convert into a charge/, fr: /ne nécessitent aucun moyen de paiement[\s\S]*ne se transforment jamais en prélèvement/, es: /no requieren ningún método de pago[\s\S]*nunca se convierten en un cargo/ };

    it.each(locales)("%s: purchased vs bonus credits and the consumption order are disclosed (6.1.1)", (l) => {
      const text = clause(doc("terms", l), "account-balance");
      expect(text).toContain(ORDER[l]);
    });

    it.each(locales)("%s: credit packs are one-time, never auto top-up, added only after payment, savings basis and rounding (6.1.1)", (l) => {
      const text = clause(doc("terms", l), "account-balance");
      expect(text).toMatch(NO_AUTO_TOPUP[l]);
      expect(text).toMatch(AFTER_PAYMENT[l]);
      expect(text).toMatch(SAVINGS[l]);
      expect(text).toContain("(/pricing)");
    });

    it.each(locales)("%s: the Welcome Pack terms are stated (6.1.1)", (l) => {
      expect(clause(doc("terms", l), "account-balance")).toMatch(WELCOME[l]);
    });

    it.each(locales)("%s: 6.1.3 and 6.3.1 point to the Refund Policy instead of saying purchases are non-refundable", (l) => {
      const limitations = clause(doc("terms", l), "limitations");
      const refunds = clause(doc("terms", l), "refunds");
      expect(limitations).toContain("(/refund-policy)");
      expect(refunds).toContain("(/refund-policy)");
      expect(refunds).not.toContain(OLD_NON_REFUNDABLE[l]);
      // Custom-reading hold returns are kept.
      expect(refunds).toContain("(/contact)");
      expect(everything(doc("terms", l))).not.toContain(OLD_NON_REFUNDABLE[l]);
    });

    // revisions 5.1: the membership free-trial paragraph is archived; free minutes and free mode stay as 6.1.4.
    it.each(locales)("%s: free minutes never convert into a charge (6.1.4)", (l) => {
      const trial = clause(doc("terms", l), "free-trials-and-promotional-offers");
      expect(trial).not.toContain(OLD_TRIAL_BILLING[l]);
      expect(trial).toMatch(FREE_MINUTES[l]);
    });

    it.each(locales)("%s: age section says eligibility comes from the date of birth and under-18 accounts are restricted until review (3.3)", (l) => {
      const text = clause(doc("terms", l), "age-restriction");
      expect(text).toMatch(AGE[l]);
      expect(text).toContain("18");
      expect(text).toContain("(/contact)");
    });
  });

  describe("Privacy Policy", () => {
    const HASH: Record<Locale, RegExp> = { en: /one-way keyed hash[\s\S]*never store your card number/, fr: /hachage unidirectionnel à clé[\s\S]*ne conservons jamais votre numéro de carte/, es: /hash unidireccional con clave[\s\S]*Nunca guardamos el número de su tarjeta/ };
    const REFUND: Record<Locale, RegExp> = { en: /Refund requests\.[\s\S]*Credits used and remaining[\s\S]*reason[\s\S]*how it was resolved/, fr: /Demandes de remboursement\.[\s\S]*Crédits utilisés et restants[\s\S]*motif[\s\S]*traitée/, es: /Solicitudes de reembolso\.[\s\S]*Créditos utilizados y restantes[\s\S]*motivo[\s\S]*resolvió/ };
    const LOTS: Record<Locale, RegExp> = { en: /Billing records and credit lots[\s\S]*purchased or bonus/, fr: /Registres de facturation et lots de crédits[\s\S]*achetés ou bonus/, es: /Registros de facturación y lotes de créditos[\s\S]*comprados o de bonificación/ };
    const DOB: Record<Locale, RegExp> = { en: /Age check\.[\s\S]*at least 18[\s\S]*restricted[\s\S]*reviewed by our team/, fr: /Contrôle de l’âge\.[\s\S]*au moins 18 ans[\s\S]*restreint[\s\S]*examinés par notre équipe/, es: /Comprobación de la edad\.[\s\S]*al menos 18 años[\s\S]*restringida[\s\S]*revisadas por nuestro equipo/ };
    const RETENTION: Record<Locale, RegExp> = { en: /Billing and refund records[\s\S]*financial records[\s\S]*card hash/, fr: /Registres de facturation et de remboursement[\s\S]*documents financiers[\s\S]*hachage unidirectionnel de la carte/, es: /Registros de facturación y de reembolsos[\s\S]*registros financieros[\s\S]*hash unidireccional de la tarjeta/ };

    it.each(locales)("%s: refund requests, the welcome-offer card hash, credit lots and the age check are described in the data categories", (l) => {
      const text = clause(doc("privacy", l), "data-you-directly-provide-to-us");
      expect(text).toMatch(HASH[l]);
      expect(text).toMatch(REFUND[l]);
      expect(text).toMatch(LOTS[l]);
      expect(text).toMatch(DOB[l]);
    });

    it.each(locales)("%s: the purposes table has a row for each new purpose with legal basis", (l) => {
      const table = doc("privacy", l).sections.flatMap((s) => s.blocks).find((b) => b.type === "table");
      expect(table?.type).toBe("table");
      if (table?.type !== "table") return;
      expect(table.rows.length).toBeGreaterThanOrEqual(3 + 3);
      for (const row of table.rows.slice(-3)) expect(row).toHaveLength(table.head.length);
    });

    it.each(locales)("%s: billing and refund records are kept like financial records; the age restriction review is described (Sections 6 and 8)", (l) => {
      expect(clause(doc("privacy", l), "data-retention")).toMatch(RETENTION[l]);
      const age = clause(doc("privacy", l), "age-limitation");
      expect(age).toContain("18");
      expect(age).toMatch(/restricted|restreint|restringid/);
    });
  });

  describe("Refund Policy", () => {
    const sections = locales.map((l) => doc("refund", l).sections.map((s) => s.id));

    it("has the 14 numbered sections of the design with identical anchors in every locale", () => {
      expect(sections[0]).toHaveLength(14);
      expect(sections[1]).toEqual(sections[0]);
      expect(sections[2]).toEqual(sections[0]);
    });

    // Confirmed by the business on 2026-11-13: no placeholder is left in the published policy.
    const confirmed: Record<string, string[]> = {
      en: ["within 2 business days", "within 5 business days after approval", "as STARYIELD MEDIA LLC"],
      fr: ["sous 2 jours ouvrés", "sous 5 jours ouvrés après", "sous la mention STARYIELD MEDIA LLC"],
      es: ["en un plazo de 2 días hábiles", "en un plazo de 5 días hábiles tras", "como STARYIELD MEDIA LLC"],
    };
    it.each(locales)("%s: states the confirmed delays and statement descriptor, with no placeholder left", (l) => {
      const all = everything(doc("refund", l));
      for (const value of confirmed[l]) expect(all).toContain(value);
      for (const placeholder of ["[2]", "[5]", "[NOTDEFINEDYET]"]) expect(all).not.toContain(placeholder);
    });

    it("English follows the design wording for the key rules", () => {
      const all = everything(doc("refund", "en"));
      expect(all).toContain("You may request a refund within 14 calendar days of the original purchase");
      expect(all).toContain("Refund amount = actual pack price paid − value of purchased credits already used.");
      expect(all).toContain("Promotional eligibility may not be restored after a refund");
      expect(all).toContain("The trial requires no purchase, does not convert into a subscription, and does not result in a charge.");
    });

    it.each(locales)("%s: support e-mail links are mailto and the contact page is /contact", (l) => {
      const all = everything(doc("refund", l));
      expect(all).toContain("[support@staryield.net](mailto:support@staryield.net)");
      expect(all).toContain("(/contact)");
    });
  });
});
