import { describe, expect, it } from "vitest";

import { COMPANY } from "@/config/company";
import { legalDocuments } from "@/content/legal";
import type { LegalBlock, LegalDocumentContent } from "@/content/legal/types";
import { LEGAL_VERSIONS } from "@/lib/legal/versions";

/**
 * Revisions 5.1 (merchant-account readiness, docs/platform-extensions.md §25): memberships are not sold, so the
 * published Terms and Privacy Policy do not describe them (the text is archived under docs/legal-archive/);
 * the purchase consent record is disclosed; the refund timing matches the Refund Policy; support details
 * (e-mail, hours) sit next to the existing contact links.
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
const everything = (doc: LegalDocumentContent) => [...doc.intro, ...doc.sections.flatMap((s) => s.blocks)].map(textOf).join("\n");
const section = (doc: LegalDocumentContent, id: string) => {
  const found = doc.sections.find((s) => s.id === id);
  if (!found) throw new Error(`no section ${id}`);
  return found.blocks.map(textOf).join("\n");
};
const allIds = (doc: LegalDocumentContent) => [...doc.intro, ...doc.sections.flatMap((s) => s.blocks)].flatMap((b) => ("id" in b && b.id ? [b.id] : [])).concat(doc.sections.map((s) => s.id));
const MAILTO = "[support@staryield.net](mailto:support@staryield.net)";
const HOURS: Record<Locale, string> = {
  en: "Support is available 24/7. We reply within 1–2 days.",
  fr: "L’assistance est disponible 24/7. Nous répondons sous 1 à 2 jours.",
  es: "El soporte está disponible 24/7. Respondemos en un plazo de 1 a 2 días.",
};

describe("revisions 5.1 legal updates", () => {
  it.each(locales)("%s: Terms, Privacy Policy and Refund Policy are dated 2026-11-13 and match LEGAL_VERSIONS", (l) => {
    for (const d of ["terms", "privacy", "refund"] as const) {
      expect(legalDocuments[d][l].meta).toMatchObject({ version: "2026-11-13", effective: "2026-11-13", lastUpdated: "2026-11-13" });
      expect(legalDocuments[d][l].meta.version).toBe(LEGAL_VERSIONS[d]);
    }
  });

  describe("memberships are not described", () => {
    const MEMBERSHIP: RegExp = /Premium Membership|membership plan|Membership Plans|auto-renew|renews automatically|Subscription Terms|abonnement Premium|Conditions d['’]abonnement|renouvellement automatique|Membresía Premium|Términos de suscripción|renovación automática|membresía que vaya a renovarse|Billing & subscriptions|Facturation et abonnements|Facturación y suscripciones/i;

    it.each(locales)("%s: Terms and Privacy Policy have no membership or subscription clauses", (l) => {
      for (const d of ["terms", "privacy"] as const) expect(everything(legalDocuments[d][l])).not.toMatch(MEMBERSHIP);
    });

    it.each(locales)("%s: the membership anchors are gone, the linked anchors stay and ids still match across locales", (l) => {
      const ids = allIds(legalDocuments.terms[l]);
      for (const gone of ["subscription-terms", "subscription-options-purchases-and-refunds", "purchases-and-payment-processing", "auto-renewal-subscription-continuity-and-cancellation", "add-on-items-and-additional-services", "promotional-codes", "changes-to-subscription-fees", "failure-to-pay-and-service-termination", "important-notice-regarding-automatic-renewals", "subscription-fees-payments-and-refunds"])
        expect(ids).not.toContain(gone);
      for (const kept of ["right-of-withdrawal", "refunds", "chargebacks-and-payment-disputes", "free-trials-and-promotional-offers", "account-balance", "limitations", "credits-payments-and-refunds"]) expect(ids).toContain(kept);
      for (const d of ["terms", "privacy", "refund"] as const) expect(allIds(legalDocuments[d][l])).toEqual(allIds(legalDocuments[d].en));
    });

    it.each(locales)("%s: section 6 is about credits, with the one-time-purchase sentence and clean numbering", (l) => {
      const s6 = legalDocuments.terms[l].sections.find((s) => s.id === "credits-payments-and-refunds")!;
      expect(s6.n).toBe("6");
      expect(s6.title).not.toMatch(/MEMBERSHIP|ABONNEMENT|MEMBRES|SUSCRIP/);
      const numbers = s6.blocks.flatMap((b) => (b.type === "h3" || b.type === "h4" ? [b.n] : []));
      expect(numbers).toEqual(["6.1", "6.1.1", "6.1.2", "6.1.3", "6.1.4", "6.2", "6.3", "6.3.1", "6.3.2", "6.3.3", "6.4"]);
      const text = textOf(s6.blocks.find((b) => b.type === "p" && /^(Credit packs|Packs de crédits|Paquetes de créditos)\./.test(b.text))!);
      expect(text).toMatch(
        {
          en: /We do not currently offer subscriptions, memberships or automatic top-ups: every purchase is a one-time credit pack\./,
          fr: /Nous ne proposons actuellement ni abonnement, ni adhésion, ni rechargement automatique : chaque achat est un pack de crédits à paiement unique\./,
          es: /Actualmente no ofrecemos suscripciones, membresías ni recargas automáticas: cada compra es un paquete de créditos de pago único\./,
        }[l],
      );
    });

    it.each(locales)("%s: the right of withdrawal stays (6.2) and only concerns credit packs", (l) => {
      const doc = legalDocuments.terms[l];
      const s6 = doc.sections.find((s) => s.id === "credits-payments-and-refunds")!;
      const at = s6.blocks.findIndex((b) => b.type === "h3" && b.id === "right-of-withdrawal");
      expect(at).toBeGreaterThan(0);
      const body = textOf(s6.blocks[at + 1]);
      expect(body).toContain("14");
      expect(body).toContain("(/contact)");
      expect(body).toContain("(/refund-policy)");
      expect(body).not.toMatch(/membership|abonnement|membresía/i);
    });

    it.each(locales)("%s: no other clause refers to a section that no longer exists", (l) => {
      const all = everything(legalDocuments.terms[l]) + everything(legalDocuments.privacy[l]);
      expect(all).not.toMatch(/(Section|section|Sección)\s+6\.2\.\d/);
      expect(all).not.toContain("#subscription-terms");
      expect(all).toContain("(#free-trials-and-promotional-offers)");
    });

    it.each(locales)("%s: the Refund Policy says there are no subscriptions or automatic top-ups", (l) => {
      expect(section(legalDocuments.refund[l], "one-time-purchases")).toMatch(
        { en: /We do not offer subscriptions or automatic top-ups/, fr: /Nous ne proposons ni abonnements ni rechargements automatiques/, es: /No ofrecemos suscripciones ni recargas automáticas/ }[l],
      );
    });
  });

  describe("refund timing is the Refund Policy's", () => {
    it.each(locales)("%s: Terms 6.3.2 no longer says 15 business days and matches Refund Policy section 10", (l) => {
      const terms = everything(legalDocuments.terms[l]);
      expect(terms).not.toMatch(/15 business days|15 jours ouvrables|15 días hábiles/);
      expect(section(legalDocuments.terms[l], "credits-payments-and-refunds")).toMatch(
        { en: /submit it within 5 business days[\s\S]*another 5–10 business days/, fr: /sous 5 jours ouvrés[\s\S]*5 à 10 jours ouvrés supplémentaires/, es: /en un plazo de 5 días hábiles[\s\S]*entre 5 y 10 días hábiles más/ }[l],
      );
      expect(section(legalDocuments.refund[l], "approved-refunds")).toMatch({ en: /within 5 business days/, fr: /sous 5 jours ouvrés/, es: /en un plazo de 5 días hábiles/ }[l]);
    });

    it.each(locales)("%s: the Refund Policy has the EEA, UK and Switzerland withdrawal paragraph linked to the Terms", (l) => {
      const refund = legalDocuments.refund[l];
      const sec = refund.sections.find((s) => s.id === "credit-pack-refunds")!;
      const at = sec.blocks.findIndex((b) => b.type === "h4" && b.id === "refund-right-of-withdrawal");
      expect(at).toBeGreaterThan(0);
      expect(textOf(sec.blocks[at])).toMatch(/EEA|EEE/);
      const body = textOf(sec.blocks[at + 1]);
      expect(body).toContain("14");
      expect(body).toContain("(/terms#right-of-withdrawal)");
      expect(allIds(legalDocuments.terms[l])).toContain("right-of-withdrawal");
    });
  });

  describe("support details", () => {
    it.each(locales)("%s: Terms 16.1 and Privacy section 14 show the support e-mail next to the form links", (l) => {
      const terms = section(legalDocuments.terms[l], "miscellaneous");
      expect(terms).toContain(MAILTO);
      expect(terms).toContain("(/contact)");
      expect(terms).toContain(HOURS[l]);
      const privacy = section(legalDocuments.privacy[l], "contact-us");
      expect(privacy).toContain(MAILTO);
      expect(privacy).toContain("(/contact?topic=privacy)");
      expect(privacy).toContain(HOURS[l]);
    });

    it.each(locales)("%s: the Refund Policy contact section has the e-mail and the support hours", (l) => {
      const contact = section(legalDocuments.refund[l], "contact-us");
      expect(contact).toContain(MAILTO);
      expect(contact).toContain(HOURS[l]);
      expect(contact).toContain("(/contact)");
    });

    it("the legal documents use the company's address and e-mail from config/company.ts", () => {
      expect(everything(legalDocuments.terms.en)).toContain(COMPANY.addressLine);
      expect(COMPANY.supportEmail).toBe("support@staryield.net");
      expect(MAILTO).toBe(`[${COMPANY.supportEmail}](mailto:${COMPANY.supportEmail})`);
    });
  });

  describe("purchase consent record and payment data (Privacy Policy)", () => {
    const CONSENT: Record<Locale, RegExp> = {
      en: /Purchase consent record\.[\s\S]*Terms of Use, Privacy Policy and Refund & Cancellation Policy[\s\S]*version \(date\)[\s\S]*time of your acceptance[\s\S]*one-way hash of your IP address[\s\S]*user agent[\s\S]*payment disputes and chargebacks/,
      fr: /Registre de consentement à l’achat\.[\s\S]*Conditions d’utilisation, notre Politique de confidentialité et notre Politique de remboursement et d’annulation[\s\S]*version \(la date\)[\s\S]*l’heure de votre acceptation[\s\S]*hachage unidirectionnel de votre adresse IP[\s\S]*agent utilisateur[\s\S]*litiges de paiement et aux rétrofacturations/,
      es: /Registro de consentimiento de la compra\.[\s\S]*Términos de uso, nuestra Política de privacidad y nuestra Política de reembolso y cancelación[\s\S]*versión \(la fecha\)[\s\S]*la hora de su aceptación[\s\S]*hash unidireccional de su dirección IP[\s\S]*agente de usuario[\s\S]*disputas de pago y a las devoluciones de cargo/,
    };
    const CARD_RULES: Record<Locale, RegExp> = {
      en: /Payment-method data and card-network rules\.[\s\S]*card brand and last four digits[\s\S]*Visa and Mastercard/,
      fr: /Données du moyen de paiement et règles des réseaux de cartes\.[\s\S]*marque de la carte[\s\S]*Visa et Mastercard/,
      es: /Datos del método de pago y normas de las redes de tarjetas\.[\s\S]*marca de la tarjeta[\s\S]*Visa y Mastercard/,
    };
    const RETENTION: Record<Locale, RegExp> = { en: /purchase consent record is kept with the billing record/, fr: /registre de consentement à l’achat est conservé avec l’enregistrement de facturation/, es: /registro de consentimiento de la compra se conserva junto con el registro de facturación/ };

    it.each(locales)("%s: the consent record and the payment-method data are disclosed, with their purpose, and retention is consistent", (l) => {
      const doc = legalDocuments.privacy[l];
      const collected = section(doc, "categories-of-personal-data-we-collect");
      expect(collected).toMatch(CONSENT[l]);
      expect(collected).toMatch(CARD_RULES[l]);
      expect(section(doc, "data-retention")).toMatch(RETENTION[l]);
    });

    it.each(locales)("%s: the purposes table has a row for the consent record with three cells", (l) => {
      const table = legalDocuments.privacy[l].sections.flatMap((s) => s.blocks).find((b) => b.type === "table");
      if (table?.type !== "table") throw new Error("no table");
      const row = table.rows.at(-1)!;
      expect(row).toHaveLength(table.head.length);
      expect(row[1]).toMatch(/consent|consentimiento|consentement/i);
    });
  });
});
