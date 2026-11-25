import { describe, expect, it } from "vitest";

import { legalDocuments } from "@/content/legal";
import type { LegalBlock, LegalDocumentContent } from "@/content/legal/types";
import { LEGAL_VERSIONS } from "@/lib/legal/versions";

/**
 * Revisions 6.0 (docs/platform-extensions.md §26.8): the free-trial hold (Terms §6.1.4, §6.4), Stripe card payments,
 * invoices and credit notes (Terms §6.1.2), payment disputes and fraud warnings (Terms §6.3.2, Refund Policy),
 * calculator report e-mails and marketing consent, Stripe as processor and independent controller, dispute evidence,
 * invoice retention and the export of hidden replies (Privacy Policy). All dated 2026-11-19.
 */
const locales = ["en", "fr", "es"] as const;
type Locale = (typeof locales)[number];
type Doc = "terms" | "privacy" | "refund";

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
const allBlocks = (doc: LegalDocumentContent) => doc.sections.flatMap((s) => s.blocks);
const allIds = (doc: LegalDocumentContent) => [...doc.intro, ...doc.sections.flatMap((s) => s.blocks)].flatMap((b) => ("id" in b && b.id ? [b.id] : [])).concat(doc.sections.map((s) => s.id));
const section = (doc: LegalDocumentContent, id: string) => {
  const found = doc.sections.find((s) => s.id === id);
  if (!found) throw new Error(`no section ${id}`);
  return found.blocks.map(textOf).join("\n");
};
/** The paragraphs after a heading up to the next heading of the same or higher level. */
const clause = (doc: LegalDocumentContent, id: string) => {
  const blocks = allBlocks(doc);
  const start = blocks.findIndex((b) => (b.type === "h3" || b.type === "h4") && b.id === id);
  if (start < 0) throw new Error(`no clause ${id}`);
  const level = (blocks[start] as { type: "h3" | "h4" }).type;
  const rest = blocks.slice(start + 1);
  const end = rest.findIndex((b) => b.type === "h3" || (level === "h4" && b.type === "h4"));
  return (end < 0 ? rest : rest.slice(0, end)).map(textOf).join("\n");
};
const d = (doc: Doc, l: Locale) => legalDocuments[doc][l];

describe("revisions 6.0 legal updates", () => {
  it("LEGAL_VERSIONS is 2026-11-19 or later for the three documents (re-dated by growth)", () => {
    for (const doc of ["terms", "privacy", "refund"] as const) expect(LEGAL_VERSIONS[doc] >= "2026-11-19").toBe(true);
  });

  it.each(locales)("%s: Terms, Privacy Policy and Refund Policy are dated 2026-11-19 or later and match LEGAL_VERSIONS (re-dated by growth)", (l) => {
    for (const doc of ["terms", "privacy", "refund"] as const) {
      const { version, effective, lastUpdated } = d(doc, l).meta;
      expect(version >= "2026-11-19").toBe(true);
      expect([effective, lastUpdated]).toEqual([version, version]);
      expect(version).toBe(LEGAL_VERSIONS[doc]);
    }
  });

  it.each(locales)("%s: section and heading ids stay identical across locales and the 6.x numbering is unchanged", (l) => {
    for (const doc of ["terms", "privacy", "refund"] as const) expect(allIds(d(doc, l))).toEqual(allIds(d(doc, "en")));
    const s6 = d("terms", l).sections.find((s) => s.id === "credits-payments-and-refunds")!;
    const numbers = s6.blocks.flatMap((b) => (b.type === "h3" || b.type === "h4" ? [b.n] : []));
    expect(numbers).toEqual(["6.1", "6.1.1", "6.1.2", "6.1.3", "6.1.4", "6.2", "6.3", "6.3.1", "6.3.2", "6.3.3", "6.4"]);
    expect(allIds(d("terms", l))).toContain("free-trials-and-promotional-offers");
  });

  describe("Terms of Use", () => {
    const HOLD: Record<Locale, RegExp> = {
      en: /\(the “hold”\)[\s\S]*put on hold for a short time, currently one minute[\s\S]*those replies are hidden from you[\s\S]*the next minute is charged at the advisor’s per-minute rate[\s\S]*the reading ends, nothing is charged[\s\S]*reveal the hidden replies for the price of one minute at that advisor’s rate[\s\S]*told the price before you pay[\s\S]*Free minutes never convert into a charge/,
      fr: /\(la « mise en attente »\)[\s\S]*mise en attente pour une courte durée, actuellement une minute[\s\S]*ces réponses vous restent masquées[\s\S]*la minute suivante est facturée au tarif par minute du conseiller[\s\S]*la consultation prend fin, rien n’est facturé[\s\S]*révéler les réponses masquées pour le prix d’une minute au tarif de ce conseiller[\s\S]*Le prix vous est toujours indiqué avant que vous payiez[\s\S]*Les minutes offertes ne se transforment jamais en prélèvement/,
      es: /\(la «espera»\)[\s\S]*se pone en espera durante un breve tiempo, actualmente un minuto[\s\S]*esas respuestas permanecen ocultas para usted[\s\S]*el minuto siguiente se cobra a la tarifa por minuto del asesor[\s\S]*la consulta termina, no se cobra nada[\s\S]*revelar las respuestas ocultas por el precio de un minuto a la tarifa de ese asesor[\s\S]*Siempre se le indica el precio antes de pagar[\s\S]*Los minutos gratuitos nunca se convierten en un cargo/,
    };
    it.each(locales)("%s: 6.1.4 keeps its anchor and describes the hold after the two existing paragraphs", (l) => {
      const doc = d("terms", l);
      expect(allIds(doc)).toContain("free-trials-and-promotional-offers");
      const text = clause(doc, "free-trials-and-promotional-offers");
      expect(text).toMatch(HOLD[l]);
      // The existing introductory-minutes and free-mode paragraphs come first.
      const intro = { en: "Introductory free minutes.", fr: "Minutes offertes de bienvenue.", es: "Minutos gratuitos de bienvenida." }[l];
      expect(text.indexOf(intro)).toBe(0);
      expect(text.indexOf(intro)).toBeLessThan(text.search(HOLD[l]));
    });

    it.each(locales)("%s: 6.1.2 says card payments are processed by Stripe, 3-D Secure, no card storage, invoices and credit notes", (l) => {
      const text = clause(d("terms", l), "payment-method");
      expect(text).toMatch(
        {
          en: /processed by Stripe[\s\S]*directly in fields provided by Stripe; Staryield never receives or stores your full card number or security code[\s\S]*3-D Secure[\s\S]*Invoices and credit notes\. For every credit purchase we issue a Staryield invoice, and for every refund a credit note\. Both are available in your account/,
          fr: /traités par Stripe[\s\S]*directement dans des champs fournis par Stripe ; Staryield ne reçoit ni ne conserve jamais le numéro complet de votre carte[\s\S]*3-D Secure[\s\S]*Factures et avoirs\. Pour chaque achat de crédits, nous émettons une facture Staryield, et pour chaque remboursement un avoir\. Les deux sont disponibles dans votre compte/,
          es: /procesa Stripe[\s\S]*directamente en campos proporcionados por Stripe; Staryield nunca recibe ni almacena el número completo de su tarjeta[\s\S]*3-D Secure[\s\S]*Facturas y notas de crédito\. Por cada compra de créditos emitimos una factura de Staryield y, por cada reembolso, una nota de crédito\. Ambas están disponibles en su cuenta/,
        }[l],
      );
    });

    it.each(locales)("%s: 6.3.2 freezes the unused Credits of a disputed payment, pauses purchases, restores them and mentions fraud warnings", (l) => {
      const text = clause(d("terms", l), "chargebacks-and-payment-disputes");
      expect(text).toMatch(
        {
          en: /While a payment dispute is open\.[\s\S]*freeze the unused Credits bought with that payment and pause new purchases[\s\S]*Credits from other purchases, are not affected[\s\S]*resolved in your favour, the frozen Credits are restored[\s\S]*warns us that a payment may be fraudulent, we may refund that payment and review your account/,
          fr: /Pendant un litige de paiement\.[\s\S]*geler les Crédits non utilisés achetés avec ce paiement et suspendre les nouveaux achats[\s\S]*Crédits issus d’autres achats, ne sont pas concernés[\s\S]*résolu en votre faveur, les Crédits gelés sont rétablis[\s\S]*nous signale qu’un paiement pourrait être frauduleux, nous pouvons rembourser ce paiement et examiner votre compte/,
          es: /Mientras hay una disputa de pago abierta\.[\s\S]*congelar los Créditos sin usar comprados con ese pago y pausar las nuevas compras[\s\S]*Créditos de otras compras no se ven afectados[\s\S]*se resuelve a su favor, los Créditos congelados se restablecen[\s\S]*nos avisa de que un pago podría ser fraudulento, podemos reembolsar ese pago y revisar su cuenta/,
        }[l],
      );
      // The refund-timing paragraph of revisions 5.1 is still there.
      expect(text).toMatch({ en: /submit it within 5 business days/, fr: /sous 5 jours ouvrés/, es: /en un plazo de 5 días hábiles/ }[l]);
    });

    it.each(locales)("%s: 6.4 says advisors earn nothing for hold time unless the client continues or reveals", (l) => {
      const text = clause(d("terms", l), "advisor-earnings-and-withdrawals");
      expect(text).toMatch(
        {
          en: /Readings on hold\.[\s\S]*you earn nothing for the time on hold[\s\S]*next minute is billed normally[\s\S]*reveals your hidden replies, you earn your agreed share of one minute at your list rate/,
          fr: /Consultations en attente\.[\s\S]*vous ne gagnez rien pour le temps d’attente[\s\S]*minute suivante est facturée normalement[\s\S]*révèle ensuite vos réponses masquées, vous percevez la part convenue d’une minute à votre tarif de référence/,
          es: /Consultas en espera\.[\s\S]*no gana nada por el tiempo en espera[\s\S]*minuto siguiente se factura con normalidad[\s\S]*revela después sus respuestas ocultas, usted recibe su parte acordada de un minuto a su tarifa de lista/,
        }[l],
      );
      expect(text).toContain("(#free-trials-and-promotional-offers)");
    });

    it.each(locales)("%s: memberships are still not mentioned and the new text adds no subscription clause", (l) => {
      const added = [clause(d("terms", l), "payment-method"), clause(d("terms", l), "free-trials-and-promotional-offers"), clause(d("terms", l), "advisor-earnings-and-withdrawals")].join("\n");
      expect(added).not.toMatch(/membership|membresía|adhésion|Premium/i);
      expect(everything(d("terms", l))).not.toMatch(/Premium Membership|abonnement Premium|Membresía Premium/i);
    });
  });

  describe("Privacy Policy", () => {
    it.each(locales)("%s: 2.14 and 2.15 exist with their anchors and numbers", (l) => {
      const blocks = allBlocks(d("privacy", l));
      expect(blocks.find((b) => b.type === "h3" && b.id === "calculator-report-emails")).toMatchObject({ n: "2.14" });
      expect(blocks.find((b) => b.type === "h3" && b.id === "marketing-emails")).toMatchObject({ n: "2.15" });
    });

    it.each(locales)("%s: calculator report e-mails say what is kept, what is not, why and for how long", (l) => {
      const text = clause(d("privacy", l), "calculator-report-emails");
      expect(text).toMatch(
        {
          en: /Your birth details are used to calculate the report and send it, and are kept, encrypted, only until the email has been sent[\s\S]*What we keep\.[\s\S]*your language and country, which calculators you used[\s\S]*consent record[\s\S]*one-way hash of your IP address[\s\S]*We do not keep your birth details in this record[\s\S]*Why we use it, and for how long\.[\s\S]*your consent[\s\S]*Asking for a report does not subscribe you to marketing[\s\S]*until you ask us to erase it/,
          fr: /Vos données de naissance servent à calculer le rapport et à l’envoyer, et ne sont conservées, chiffrées, que jusqu’à l’envoi de l’e-mail[\s\S]*Ce que nous conservons\.[\s\S]*votre langue et votre pays, les calculateurs que vous avez utilisés[\s\S]*registre de consentement[\s\S]*hachage unidirectionnel de votre adresse IP[\s\S]*Nous ne conservons pas vos données de naissance dans cet enregistrement[\s\S]*Pourquoi nous l’utilisons et pendant combien de temps\.[\s\S]*votre consentement[\s\S]*Demander un rapport ne vous abonne pas au marketing[\s\S]*jusqu’à ce que vous nous demandiez de l’effacer/,
          es: /Sus datos de nacimiento se utilizan para calcular el informe y enviarlo, y se conservan, cifrados, solo hasta que se envía el correo[\s\S]*Qué conservamos\.[\s\S]*su idioma y su país, las calculadoras que ha utilizado[\s\S]*registro de consentimiento[\s\S]*hash unidireccional de su dirección IP[\s\S]*No conservamos sus datos de nacimiento en este registro[\s\S]*Para qué lo usamos y durante cuánto tiempo\.[\s\S]*su consentimiento[\s\S]*Pedir un informe no le suscribe al marketing[\s\S]*hasta que nos pida borrarlo/,
        }[l],
      );
      expect(text).toContain("(/contact?topic=privacy)");
    });

    it.each(locales)("%s: the birth-data paragraph (2.1) points to 2.14 instead of saying birth details are never stored", (l) => {
      const text = clause(d("privacy", l), "data-you-directly-provide-to-us");
      expect(text).toContain("(#calculator-report-emails)");
      expect(text).not.toMatch({ en: /runs entirely in your browser/, fr: /fonctionne entièrement dans votre navigateur/, es: /se ejecuta íntegramente en su navegador/ }[l]);
    });

    it.each(locales)("%s: marketing e-mails need consent, may be pre-ticked only where the law allows, carry an unsubscribe link and are never sold", (l) => {
      const text = clause(d("privacy", l), "marketing-emails");
      expect(text).toMatch(
        {
          en: /only with your consent[\s\S]*Where the law allows it[\s\S]*already be ticked[\s\S]*you can untick it[\s\S]*In other countries the box starts unticked[\s\S]*Every marketing email contains an unsubscribe link[\s\S]*We never sell your email address/,
          fr: /qu’avec votre consentement[\s\S]*Lorsque la loi le permet[\s\S]*déjà être cochée[\s\S]*vous pouvez la décocher[\s\S]*Dans les autres pays, la case est décochée au départ[\s\S]*Chaque e-mail marketing contient un lien de désabonnement[\s\S]*Nous ne vendons jamais votre adresse e-mail/,
          es: /Solo enviamos estos correos con su consentimiento[\s\S]*Cuando la ley lo permite[\s\S]*ya marcada[\s\S]*puede desmarcarla[\s\S]*En los demás países la casilla aparece sin marcar[\s\S]*Cada correo comercial incluye un enlace para darse de baja[\s\S]*Nunca vendemos su dirección de correo electrónico/,
        }[l],
      );
      expect(text).toContain("(/profile/settings)");
    });

    it.each(locales)("%s: Stripe is named with the data shared, Stripe.js, processor and independent controller roles, its privacy policy and transfers", (l) => {
      const sharing = section(d("privacy", l), "how-we-share-your-personal-data");
      expect(sharing).toMatch(
        {
          en: /Payment processing: Card payments for credits are processed by Stripe[\s\S]*your email address, your name, the amount and currency[\s\S]*Stripe\.js[\s\S]*js\.stripe\.com[\s\S]*device and network data[\s\S]*for fraud prevention[\s\S]*as our service provider and, for fraud prevention and to meet its own legal obligations, as an independent controller[\s\S]*https:\/\/stripe\.com\/privacy/,
          fr: /Traitement des paiements : les paiements par carte des crédits sont traités par Stripe[\s\S]*votre adresse e-mail, votre nom, le montant et la devise[\s\S]*Stripe\.js[\s\S]*js\.stripe\.com[\s\S]*données d’appareil et de réseau[\s\S]*pour prévenir la fraude[\s\S]*en tant que notre prestataire de services et, pour prévenir la fraude et respecter ses propres obligations légales, en tant que responsable de traitement indépendant[\s\S]*https:\/\/stripe\.com\/privacy/,
          es: /Procesamiento de pagos: los pagos con tarjeta de créditos los procesa Stripe[\s\S]*su correo electrónico, su nombre, el importe y la moneda[\s\S]*Stripe\.js[\s\S]*js\.stripe\.com[\s\S]*datos del dispositivo y de la red[\s\S]*para prevenir el fraude[\s\S]*como nuestro proveedor de servicios y, para prevenir el fraude y cumplir sus propias obligaciones legales, como responsable del tratamiento independiente[\s\S]*https:\/\/stripe\.com\/privacy/,
        }[l],
      );
      expect(section(d("privacy", l), "international-data-transfers")).toContain("Stripe");
      // The external address is plain text (the legal documents only allow internal links).
      expect(sharing).not.toContain("](https://stripe.com");
    });

    it.each(locales)("%s: dispute evidence goes to Stripe and the card networks and never includes message content", (l) => {
      expect(section(d("privacy", l), "how-we-share-your-personal-data")).toMatch(
        {
          en: /Payment disputes\. If you or your bank dispute a payment, we send our payment processor Stripe, which passes it to the card networks and your bank[\s\S]*record of the terms you accepted with the purchase[\s\S]*a log of the readings you used after the purchase[\s\S]*and the invoice\. We never send the content of your messages/,
          fr: /Litiges de paiement\. Si vous ou votre banque contestez un paiement, nous transmettons à notre prestataire de paiement Stripe, qui les communique aux réseaux de cartes et à votre banque[\s\S]*registre des conditions que vous avez acceptées lors de l’achat[\s\S]*un journal des consultations que vous avez utilisées après l’achat[\s\S]*et la facture\. Nous n’envoyons jamais le contenu de vos messages/,
          es: /Disputas de pago\. Si usted o su banco disputan un pago, enviamos a nuestro procesador de pagos Stripe, que los transmite a las redes de tarjetas y a su banco[\s\S]*registro de las condiciones que aceptó con la compra[\s\S]*un registro de las consultas que utilizó después de la compra[\s\S]*y la factura\. Nunca enviamos el contenido de sus mensajes/,
        }[l],
      );
    });

    it.each(locales)("%s: invoices are described (2.1) and kept for accounting and tax obligations even after erasure (8)", (l) => {
      expect(clause(d("privacy", l), "data-you-directly-provide-to-us")).toMatch(
        {
          en: /Invoices and credit notes\. For every credit purchase we issue an invoice, and for every refund a credit note/,
          fr: /Factures et avoirs\. Pour chaque achat de crédits, nous émettons une facture, et pour chaque remboursement un avoir/,
          es: /Facturas y notas de crédito\. Por cada compra de créditos emitimos una factura y, por cada reembolso, una nota de crédito/,
        }[l],
      );
      expect(section(d("privacy", l), "data-retention")).toMatch(
        {
          en: /Invoices and credit notes\. Invoices and credit notes are accounting records\. We keep them for as long as accounting and tax laws require, even after you close or erase your account, on the basis of our legal obligations[\s\S]*Calculator report email records are kept until you ask us to erase them/,
          fr: /Factures et avoirs\. Les factures et les avoirs sont des documents comptables\. Nous les conservons aussi longtemps que l’exigent les lois comptables et fiscales, même après la clôture ou l’effacement de votre compte, sur la base de nos obligations légales[\s\S]*Les enregistrements des e-mails de rapport des calculateurs sont conservés jusqu’à ce que vous nous demandiez de les effacer/,
          es: /Facturas y notas de crédito\. Las facturas y las notas de crédito son registros contables\. Las conservamos durante el tiempo que exijan las leyes contables y fiscales, incluso después de que cierre o borre su cuenta, sobre la base de nuestras obligaciones legales[\s\S]*Los registros de los correos con el informe de las calculadoras se conservan hasta que nos pida borrarlos/,
        }[l],
      );
    });

    it.each(locales)("%s: payment disputes, fraud warnings and risk signals are disclosed as data received from Stripe", (l) => {
      expect(clause(d("privacy", l), "data-we-receive-from-third-parties")).toMatch(
        {
          en: /Payment disputes, fraud warnings and risk signals\.[\s\S]*a risk level and score, the country of the card and the result of any 3-D Secure authentication/,
          fr: /Litiges de paiement, alertes de fraude et signaux de risque\.[\s\S]*un niveau et un score de risque, le pays de la carte et le résultat d’une éventuelle authentification 3-D Secure/,
          es: /Disputas de pago, avisos de fraude y señales de riesgo\.[\s\S]*un nivel y una puntuación de riesgo, el país de la tarjeta y el resultado de cualquier autenticación 3-D Secure/,
        }[l],
      );
    });

    it.each(locales)("%s: the personal-data export includes the replies hidden by a paused reading, and 2.13 says so", (l) => {
      expect(clause(d("privacy", l), "right-to-data-portability")).toMatch(
        {
          en: /conversations \(including replies and likes, and the advisor’s replies that were hidden from you while a reading was on hold\)/,
          fr: /vos conversations \(avec les réponses et les mentions « J’aime », ainsi que les réponses du conseiller qui vous ont été masquées pendant une mise en attente\)/,
          es: /sus conversaciones \(con las respuestas y los «Me gusta», y las respuestas del asesor que se le ocultaron durante una espera\)/,
        }[l],
      );
      expect(clause(d("privacy", l), "messages-outside-live-readings")).toMatch(
        {
          en: /Replies hidden during a hold\.[\s\S]*stored like any other message but are hidden from you[\s\S]*Hidden replies are included in your data export/,
          fr: /Réponses masquées pendant une mise en attente\.[\s\S]*enregistrées comme n’importe quel autre message mais vous restent masquées[\s\S]*Les réponses masquées figurent dans votre export de données/,
          es: /Respuestas ocultas durante una espera\.[\s\S]*se almacenan como cualquier otro mensaje, pero permanecen ocultas para usted[\s\S]*Las respuestas ocultas se incluyen en su exportación de datos/,
        }[l],
      );
    });

    it.each(locales)("%s: the purposes table has rows for the report e-mails, invoices and fraud evidence, and the consent-record row is still last", (l) => {
      const table = allBlocks(d("privacy", l)).find((b) => b.type === "table");
      if (table?.type !== "table") throw new Error("no table");
      for (const row of table.rows) expect(row).toHaveLength(table.head.length);
      const col0 = table.rows.map((r) => r[0]).join("\n");
      expect(col0).toMatch({ en: /calculator report[\s\S]*invoices and credit notes[\s\S]*prevent payment fraud/, fr: /rapport de calculateur[\s\S]*factures et des avoirs[\s\S]*prévenir la fraude au paiement/, es: /informe de la calculadora[\s\S]*facturas y notas de crédito[\s\S]*prevenir el fraude en los pagos/ }[l]);
      expect(table.rows.at(-1)![1]).toMatch(/consent|consentimiento|consentement/i);
    });

    it.each(locales)("%s: the sending-service bullet covers invoices and report e-mails, a marketing provider bullet exists and the U.S. table lists the Stripe.js data", (l) => {
      const sharing = section(d("privacy", l), "how-we-share-your-personal-data");
      expect(sharing).toMatch({ en: /receipts with their invoices, the calculator reports you ask for/, fr: /les reçus et leurs factures, les rapports des calculateurs que vous demandez/, es: /recibos con sus facturas, los informes de las calculadoras que solicita/ }[l]);
      expect(sharing).toContain("(#marketing-emails)");
      expect(section(d("privacy", l), "privacy-notice-for-u-s-residents")).toMatch({ en: /Payment processors \(device and network data collected while you pay/, fr: /Prestataires de paiement \(données d’appareil et de réseau collectées pendant que vous payez/, es: /Procesadores de pagos \(datos del dispositivo y de la red recogidos mientras paga/ }[l]);
    });

    it.each(locales)("%s: the legitimate-interest text covers fraud prevention and dispute evidence and the e-mail choice record", (l) => {
      const li = clause(d("privacy", l), "legitimate-interests");
      expect(li).toMatch({ en: /prevent payment fraud and to send evidence to our payment processor and the card networks[\s\S]*When remembering your email choices/, fr: /prévenir la fraude au paiement et transmettre des éléments de preuve[\s\S]*Lorsque nous mémorisons vos choix d’e-mail/, es: /prevenir el fraude en los pagos y enviar pruebas a nuestro procesador de pagos[\s\S]*Al recordar sus elecciones de correo electrónico/ }[l]);
    });
  });

  describe("Refund Policy", () => {
    it.each(locales)("%s: card refunds go back to the original card through Stripe with a credit note", (l) => {
      expect(section(d("refund", l), "approved-refunds")).toMatch(
        {
          en: /Card refunds\. A refund of a card payment goes back to the original card through Stripe\. We submit it to Stripe within 5 business days after approval[\s\S]*another 5–10 business days[\s\S]*we issue a credit note/,
          fr: /Remboursements par carte\. Le remboursement d’un paiement par carte est crédité sur la carte d’origine via Stripe\. Nous le transmettons à Stripe sous 5 jours ouvrés après l’approbation[\s\S]*5 à 10 jours ouvrés supplémentaires[\s\S]*nous émettons un avoir/,
          es: /Reembolsos con tarjeta\. El reembolso de un pago con tarjeta se devuelve a la tarjeta original a través de Stripe\. Lo enviamos a Stripe en un plazo de 5 días hábiles tras su aprobación[\s\S]*entre 5 y 10 días hábiles más[\s\S]*emitimos una nota de crédito/,
        }[l],
      );
    });

    it.each(locales)("%s: disputes link to Terms 6.3.2 and a fraud warning can lead to a refund", (l) => {
      const text = section(d("refund", l), "payment-disputes");
      expect(text).toContain("(/terms#chargebacks-and-payment-disputes)");
      expect(allIds(d("terms", l))).toContain("chargebacks-and-payment-disputes");
      expect(text).toMatch(
        {
          en: /unused Credits bought with the disputed payment may be frozen and new purchases paused[\s\S]*cannot also be refunded[\s\S]*warns us that a payment may be fraudulent, we may refund it to the original card straight away and review your account[\s\S]*credit note/,
          fr: /Crédits non utilisés achetés avec le paiement contesté peuvent être gelés et les nouveaux achats suspendus[\s\S]*ne peut pas être également remboursé[\s\S]*nous signale qu’un paiement pourrait être frauduleux, nous pouvons le rembourser immédiatement sur la carte d’origine et examiner votre compte[\s\S]*avoir/,
          es: /Créditos sin usar comprados con el pago disputado pueden congelarse y las nuevas compras pausarse[\s\S]*no puede reembolsarse además[\s\S]*nos avisa de que un pago podría ser fraudulento, podemos reembolsarlo de inmediato a la tarjeta original y revisar su cuenta[\s\S]*nota de crédito/,
        }[l],
      );
      // The statement descriptor paragraph of revisions 5.0 is still there.
      expect(text).toContain("STARYIELD MEDIA LLC");
    });

    it.each(locales)("%s: the free-trial section explains the hold, links to Terms 6.1.4 and says nothing is charged for it", (l) => {
      const text = section(d("refund", l), "free-trial");
      expect(text).toContain("(/terms#free-trials-and-promotional-offers)");
      expect(text).toMatch(
        {
          en: /put on hold for a short time instead of ending[\s\S]*Nothing is charged for the hold/,
          fr: /mise en attente pendant une courte durée au lieu de se terminer[\s\S]*La mise en attente n’est pas facturée/,
          es: /se pone en espera durante un breve tiempo en lugar de terminar[\s\S]*La espera no se cobra/,
        }[l],
      );
    });
  });
});
