import { describe, expect, it } from "vitest";

import { legalDocuments } from "@/content/legal";
import { LEGAL_VERSIONS } from "@/lib/legal/versions";
import type { LegalBlock, LegalDocumentContent } from "@/content/legal/types";

/**
 * Growth release, Terms of Use: e-mail confirmation (3.8), free calculators and guides (2.5), the booking button
 * (4.17, 4.15, 6.1.4), reminders and the "Get started" checklist (4.18, 4.13), shareable result images (4.19) and
 * the referral program (4.20 and 4.20.1–4.20.9). Every addition has its own anchor; no existing clause was renumbered
 * or removed. The three documents are re-dated 2026-11-25 together with LEGAL_VERSIONS (the purchase consent).
 */
const locales = ["en", "fr", "es"] as const;

describe("growth legal versions", () => {
  it("LEGAL_VERSIONS is 2026-11-25 for the three documents", () => {
    expect(LEGAL_VERSIONS).toEqual({ terms: "2026-11-25", privacy: "2026-11-25", refund: "2026-11-25" });
  });

  it.each(locales)("%s: Terms, Privacy Policy and Refund Policy are dated 2026-11-25 and match LEGAL_VERSIONS", (l) => {
    for (const doc of ["terms", "privacy", "refund"] as const) {
      expect(legalDocuments[doc][l].meta).toMatchObject({ version: "2026-11-25", effective: "2026-11-25", lastUpdated: "2026-11-25" });
    }
  });
});
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
const allBlocks = (doc: LegalDocumentContent) => doc.sections.flatMap((s) => s.blocks);
const allIds = (doc: LegalDocumentContent) =>
  [...doc.intro, ...doc.sections.flatMap((s) => s.blocks)].flatMap((b) => ("id" in b && b.id ? [b.id] : [])).concat(doc.sections.map((s) => s.id));
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
const numbersOf = (doc: LegalDocumentContent, sectionId: string) =>
  doc.sections.find((s) => s.id === sectionId)!.blocks.flatMap((b) => (b.type === "h3" || b.type === "h4" ? [b.n] : []));
const terms = (l: Locale) => legalDocuments.terms[l];

/** The new anchors, their clause numbers and heading levels. */
const NEW_ANCHORS: [id: string, n: string, level: "h3" | "h4"][] = [
  ["free-calculators-and-guides", "2.5", "h3"],
  ["email-address-and-confirmation", "3.8", "h3"],
  ["booking-a-reading", "4.17", "h3"],
  ["reminders-and-getting-started", "4.18", "h3"],
  ["sharing-your-results", "4.19", "h3"],
  ["referral-program", "4.20", "h3"],
  ["referral-who-can-take-part", "4.20.1", "h4"],
  ["referral-how-rewards-are-earned", "4.20.2", "h4"],
  ["referral-reward-size-and-limits", "4.20.3", "h4"],
  ["referral-minutes-added", "4.20.4", "h4"],
  ["referral-free-minutes", "4.20.5", "h4"],
  ["referral-sharing-your-link", "4.20.6", "h4"],
  ["referral-fraud-and-review", "4.20.7", "h4"],
  ["referral-changes-and-ending", "4.20.8", "h4"],
  ["referral-closing-your-account", "4.20.9", "h4"],
];

/** Every anchor the Terms had before this release, in document order (they must all stay). */
const EXISTING_ANCHORS = [
  "binding-arbitration-dispute-resolution", "acceptance-of-terms", "additional-terms-and-policies", "changes-to-these-terms", "changes-to-the-service",
  "important-disclaimers", "entertainment-purposes-no-legal-health-financial-or-other-professional-advice", "advisors-psychics",
  "no-guarantee-of-outcomes-or-service-performance", "calculators-charts-and-horoscopes", "account-registration", "creating-an-account",
  "your-responsibilities", "age-restriction", "account-suspension-or-termination", "account-security", "account-deletion", "disabling-your-account",
  "use-of-the-service", "ownership-and-intellectual-property", "license-to-use-the-service", "user-generated-content", "user-reviews-and-testimonials",
  "user-responsibilities-and-rights", "content-moderation-and-restrictions", "prohibited-conduct", "service-availability-and-modifications",
  "risks-and-disclaimer-of-liability", "customer-support", "live-readings-messages-and-images", "ratings-of-advisors", "notifications", "client-notes",
  "advisor-waiting-lists", "messages-outside-live-readings", "third-party-services-materials-and-advertising", "no-endorsement-or-responsibility",
  "third-party-links-and-advertising", "no-liability-for-third-party-content", "use-of-third-party-services-at-your-own-risk",
  "credits-payments-and-refunds", "balance-terms", "account-balance", "payment-method", "limitations", "free-trials-and-promotional-offers",
  "right-of-withdrawal", "general-terms", "refunds", "chargebacks-and-payment-disputes", "taxes", "advisor-earnings-and-withdrawals",
  "user-representations-and-restrictions", "permitted-use-of-the-service", "prohibited-conduct-2", "respectful-conduct-towards-customer-support",
  "disclaimer-of-warranties", "general-disclaimers", "in-particular-we-do-not-warrant-that", "service-modifications-and-updates",
  "consumer-protection-and-non-waivable-rights", "limitation-of-liability", "limitation-of-aggregate-liability", "waiver-of-unknown-claims-california-residents",
  "jurisdiction-specific-exceptions", "indemnification", "international-use", "informal-dispute-resolution-procedures-and-arbitration",
  "mandatory-pre-filing-notice-procedure", "small-claims-court", "what-is-arbitration", "class-action-and-jury-trial-waiver",
  "except-for-the-mass-filing-procedures-described-below-you-and-we-agree-that", "arbitration-procedure", "decision-of-the-arbitrator", "fees",
  "confidentiality", "settlement-offers-and-offers-of-judgment", "additional-procedures-for-mass-arbitration-filings",
  "opting-out-of-this-arbitration-agreement", "governing-law", "for-eea-and-uk-residents", "for-california-residents", "limitation-on-claims-period",
  "miscellaneous", "contact-information",
];

describe("growth release: Terms of Use", () => {
  describe("anchors and numbering", () => {
    it.each(locales)("%s: every new anchor exists with its number and heading level", (l) => {
      const blocks = allBlocks(terms(l));
      for (const [id, n, level] of NEW_ANCHORS) expect(blocks.find((b) => (b.type === "h3" || b.type === "h4") && b.id === id), id).toMatchObject({ type: level, n });
    });

    it.each(locales)("%s: section and heading ids are identical to English", (l) => {
      expect(allIds(terms(l))).toEqual(allIds(terms("en")));
    });

    it.each(locales)("%s: every existing anchor is kept, in the same order", (l) => {
      const ids = allIds(terms(l)).filter((id) => EXISTING_ANCHORS.includes(id));
      // allIds lists headings before section ids; compare as sets and check the heading order separately.
      expect(new Set(ids)).toEqual(new Set(EXISTING_ANCHORS));
      const headings = allBlocks(terms(l)).flatMap((b) => (b.type === "h3" || b.type === "h4") && EXISTING_ANCHORS.includes(b.id) ? [b.id] : []);
      expect(headings).toEqual(EXISTING_ANCHORS.filter((id) => headings.includes(id)));
    });

    it.each(locales)("%s: new clauses are appended, so the existing numbering is unchanged", (l) => {
      const doc = terms(l);
      expect(doc.sections.map((s) => s.n)).toEqual(["1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16"]);
      expect(numbersOf(doc, "important-disclaimers")).toEqual(["2.1", "2.2", "2.3", "2.4", "2.5"]);
      expect(numbersOf(doc, "account-registration")).toEqual(["3.1", "3.2", "3.3", "3.4", "3.5", "3.6", "3.7", "3.8"]);
      expect(numbersOf(doc, "use-of-the-service")).toEqual([
        ...Array.from({ length: 20 }, (_, i) => `4.${i + 1}`),
        ...Array.from({ length: 9 }, (_, i) => `4.20.${i + 1}`),
      ]);
      expect(numbersOf(doc, "credits-payments-and-refunds")).toEqual(["6.1", "6.1.1", "6.1.2", "6.1.3", "6.1.4", "6.2", "6.3", "6.3.1", "6.3.2", "6.3.3", "6.4"]);
    });

    it.each(locales)("%s: the new clauses link to each other and to the clauses they build on", (l) => {
      const doc = terms(l);
      expect(clause(doc, "email-address-and-confirmation")).toContain("(#referral-how-rewards-are-earned)");
      expect(clause(doc, "email-address-and-confirmation")).toContain("(#reminders-and-getting-started)");
      expect(clause(doc, "referral-how-rewards-are-earned")).toContain("(#email-address-and-confirmation)");
      expect(clause(doc, "referral-how-rewards-are-earned")).toContain("(#free-trials-and-promotional-offers)");
      expect(clause(doc, "notifications")).toContain("(#reminders-and-getting-started)");
      expect(clause(doc, "free-trials-and-promotional-offers")).toContain("(#referral-program)");
      expect(clause(doc, "booking-a-reading")).toContain("(#advisor-waiting-lists)");
      expect(clause(doc, "booking-a-reading")).toContain("(#free-trials-and-promotional-offers)");
      expect(clause(doc, "free-calculators-and-guides")).toContain("(#booking-a-reading)");
      expect(clause(doc, "referral-program")).toContain("(/profile/invite)");
      expect(clause(doc, "reminders-and-getting-started")).toContain("(/profile/settings#notifications)");
    });
  });

  describe("what the clauses say", () => {
    it.each(locales)("%s: 3.8 states the confirmation link's single use, 48-hour expiry and that confirmation is not required to buy", (l) => {
      const text = clause(terms(l), "email-address-and-confirmation");
      expect(text).toMatch(
        {
          en: /works only once, expires after 48 hours[\s\S]*not currently required to use the Service or to buy Credits[\s\S]*the address is confirmed only if the link is used/,
          fr: /ne fonctionne qu’une seule fois, expire au bout de 48 heures[\s\S]*pas nécessaire pour utiliser le Service ni pour acheter des Crédits[\s\S]*l’adresse n’est confirmée que si le lien est utilisé/,
          es: /funciona una sola vez, caduca a las 48 horas[\s\S]*no es necesario confirmar su dirección para usar el Servicio ni para comprar Créditos[\s\S]*la dirección solo se confirma si se utiliza el enlace/,
        }[l],
      );
    });

    it.each(locales)("%s: 2.5 keeps the free tools for entertainment and calls the compatibility scores a heuristic", (l) => {
      const text = clause(terms(l), "free-calculators-and-guides");
      expect(text).toMatch(
        {
          en: /free and can be used without an Account[\s\S]*not predictions[\s\S]*entertainment purposes only[\s\S]*heuristic[\s\S]*not based on data about real couples[\s\S]*only if they agree/,
          fr: /gratuits et utilisables sans Compte[\s\S]*pas des prédictions[\s\S]*fins de divertissement[\s\S]*heuristique[\s\S]*aucune donnée concernant de vrais couples[\s\S]*avec son accord/,
          es: /gratuitas y pueden usarse sin una Cuenta[\s\S]*No son predicciones[\s\S]*fines de entretenimiento[\s\S]*heurística[\s\S]*no se basan en datos sobre parejas reales[\s\S]*solo si esa persona está de acuerdo/,
        }[l],
      );
    });

    it.each(locales)("%s: 4.17 and 4.15 cover who can book, the offer line and the best-effort waiting list", (l) => {
      const doc = terms(l);
      expect(clause(doc, "booking-a-reading")).toMatch(
        {
          en: /aged 18 or older[\s\S]*Advisor and administrator Accounts cannot book readings[\s\S]*one per person[\s\S]*per-minute rate shown when you start the chat/,
          fr: /âgés de 18 ans ou plus[\s\S]*Les Comptes de conseiller et d’administrateur ne peuvent pas réserver de consultations[\s\S]*une seule fois par personne[\s\S]*tarif par minute du conseiller affiché au début du chat/,
          es: /de 18 años o más[\s\S]*Las Cuentas de asesor y de administrador no pueden reservar consultas[\s\S]*una vez por persona[\s\S]*tarifa por minuto del asesor que se muestra al iniciar el chat/,
        }[l],
      );
      expect(clause(doc, "advisor-waiting-lists")).toMatch(
        {
          en: /We send the notice only once, and only if the advisor is still online[\s\S]*does not hold a place for you/,
          fr: /Nous n’envoyons l’alerte qu’une seule fois, et uniquement si le conseiller est toujours en ligne[\s\S]*ne vous réserve aucune place/,
          es: /Enviamos el aviso una sola vez, y solo si el asesor sigue en línea[\s\S]*no le reserva un lugar/,
        }[l],
      );
    });

    it.each(locales)("%s: 4.18 sends promotional reminders by email or push only with the marketing opt-in", (l) => {
      expect(clause(terms(l), "reminders-and-getting-started")).toMatch(
        {
          en: /only if you have turned on “Marketing Updates” \(tips, offers and reminders\)[\s\S]*describe the moment it was sent/,
          fr: /que si vous avez activé « Actualités marketing » \(conseils, offres et rappels\)[\s\S]*correspondent au moment de son envoi/,
          es: /si ha activado «Novedades de marketing» \(consejos, ofertas y recordatorios\)[\s\S]*corresponden al momento en que se envió/,
        }[l],
      );
    });

    it.each(locales)("%s: 4.19 limits shared images to unaltered, personal, non-commercial use without birth data", (l) => {
      expect(clause(terms(l), "sharing-your-results")).toMatch(
        {
          en: /9:16[\s\S]*does not show your birth date, birth time, birthplace or name[\s\S]*unaltered, for personal and non-commercial purposes[\s\S]*remain the property of Staryield or its licensors/,
          fr: /9:16[\s\S]*n’indique ni votre date, ni votre heure, ni votre lieu de naissance, ni votre nom[\s\S]*sans la modifier, à des fins personnelles et non commerciales[\s\S]*restent la propriété de Staryield ou de ses concédants/,
          es: /9:16[\s\S]*No muestra su fecha, hora ni lugar de nacimiento ni su nombre[\s\S]*sin modificarla, con fines personales y no comerciales[\s\S]*siguen siendo propiedad de Staryield o de sus licenciantes/,
        }[l],
      );
    });

    it.each(locales)("%s: 4.20 states the reward conditions, size, cap, when minutes are added, value, disclosure duty and review", (l) => {
      const doc = terms(l);
      expect(clause(doc, "referral-how-rewards-are-earned")).toMatch(
        {
          en: /no more than 30 days[\s\S]*confirms their email address[\s\S]*at least one minute[\s\S]*only the standard welcome offer/,
          fr: /au plus tard 30 jours[\s\S]*confirme son adresse e-mail[\s\S]*au moins une minute[\s\S]*uniquement de l’offre de bienvenue standard/,
          es: /como máximo 30 días[\s\S]*confirma su dirección de correo electrónico[\s\S]*al menos un minuto[\s\S]*únicamente la oferta de bienvenida estándar/,
        }[l],
      );
      expect(clause(doc, "referral-reward-size-and-limits")).toMatch({ en: /currently 5[\s\S]*currently 20/, fr: /actuellement 5[\s\S]*actuellement 20/, es: /actualmente 5[\s\S]*actualmente 20/ }[l]);
      expect(clause(doc, "referral-minutes-added")).toMatch(
        {
          en: /^A reward is added to your Account automatically shortly after your friend qualifies, and we notify you when minutes are added\. Minutes are added only while your Account is active[\s\S]*held for review[\s\S]*added only if we approve it\.$/,
          fr: /^Une récompense est ajoutée automatiquement à votre Compte peu après que votre ami a rempli les conditions, et nous vous prévenons lorsque des minutes sont ajoutées\. Les minutes ne sont ajoutées que tant que votre Compte est actif[\s\S]*retenue pour examen[\s\S]*n’est ajoutée que si nous l’approuvons\.$/,
          es: /^Una recompensa se añade automáticamente a su Cuenta poco después de que su amigo cumpla las condiciones, y le avisamos cuando se añaden minutos\. Los minutos solo se añaden mientras su Cuenta esté activa[\s\S]*retenida para revisión[\s\S]*solo se añade si la aprobamos\.$/,
        }[l],
      );
      expect(clause(doc, "referral-changes-and-ending")).toContain("(#referral-minutes-added)");
      expect(allIds(doc)).not.toContain("referral-beta-and-launch");
      expect(clause(doc, "referral-free-minutes")).toMatch({ en: /no cash value/, fr: /aucune valeur monétaire/, es: /No tienen valor en efectivo/ }[l]);
      expect(clause(doc, "referral-free-minutes")).toContain("(/refund-policy)");
      expect(clause(doc, "referral-sharing-your-link")).toMatch(
        {
          en: /say clearly that you receive free minutes/,
          fr: /indiquez clairement que vous recevez des minutes offertes/,
          es: /indique claramente que recibe minutos gratuitos/,
        }[l],
      );
      expect(clause(doc, "referral-fraud-and-review")).toMatch({ en: /held for review/, fr: /retenus pour examen/, es: /retenidas para revisión/ }[l]);
      expect(clause(doc, "referral-changes-and-ending")).toMatch(
        { en: /does not take away minutes already added/, fr: /ne vous retire ni les minutes déjà ajoutées/, es: /no le quitan los minutos ya añadidos/ }[l],
      );
      expect(clause(doc, "referral-closing-your-account")).toContain("(#account-deletion)");
    });

    it.each(locales)("%s: the hold paragraph of 6.1.4 is still in place and the new free-minute paragraphs follow it", (l) => {
      const text = clause(terms(l), "free-trials-and-promotional-offers");
      const hold = { en: "(the “hold”)", fr: "(la « mise en attente »)", es: "(la «espera»)" }[l];
      const referral = { en: "Free minutes from the referral program.", fr: "Minutes offertes du programme de parrainage.", es: "Minutos gratuitos del programa de referidos." }[l];
      expect(text.indexOf(hold)).toBeGreaterThan(0);
      expect(text.indexOf(referral)).toBeGreaterThan(text.indexOf(hold));
    });

    it.each(locales)("%s: the new text mentions no membership and makes no certification, healing or accuracy claim", (l) => {
      const added = NEW_ANCHORS.filter(([, , level]) => level === "h3").map(([id]) => clause(terms(l), id)).join("\n");
      expect(added).not.toMatch(/membership|membresía|adhésion|Premium/i);
      expect(added).not.toMatch({ en: /certified|healing|therapy|\baccurate\b/i, fr: /certifié|guérison|thérapie|\bprécis\b/i, es: /certificad|curación|terapia|\bprecis[oa]s?\b/i }[l]);
    });
  });
});

/** Follow-ups (fact-check of the growth branch): auto-away, alternative advisors and the invite code in shared links. */
describe("follow-ups: Terms of Use", () => {
  const AUTO_AWAY: Record<Locale, RegExp> = {
    en: /misses several chat requests in a row \(currently 3\) may be set to Away automatically and is told why; an Away advisor is not shown as online until they set themselves Online/,
    fr: /manque plusieurs demandes de chat consécutives \(actuellement 3\) peut être réglé automatiquement sur absent et est informé du motif ; un conseiller absent n’est pas affiché comme en ligne tant qu’il ne s’est pas remis lui-même en ligne/,
    es: /no atiende varias solicitudes de chat seguidas \(actualmente 3\) puede pasar automáticamente a ausente y se le informa del motivo; un asesor ausente no aparece como en línea hasta que él mismo se ponga en línea/,
  };
  const ALTERNATIVES: Record<Locale, RegExp> = {
    en: /After a request that an advisor declined or did not answer, we may suggest other advisors who are online; the same status caveat applies/,
    fr: /Après une demande qu’un conseiller a refusée ou à laquelle il n’a pas répondu, nous pouvons vous suggérer d’autres conseillers en ligne ; la même réserve sur le statut s’applique/,
    es: /Después de una solicitud que un asesor rechazó o a la que no respondió, podemos sugerirle otros asesores que estén en línea; se aplica la misma salvedad sobre el estado/,
  };
  const AWAY_NOTICE: Record<Locale, RegExp> = {
    en: /Advisors are also notified, in the app and, depending on their notification settings, by email and push, if their status is set to Away automatically \(\[Section 4\.17\]\(#booking-a-reading\)\)/,
    fr: /Les conseillers sont également prévenus, dans l’application et, selon leurs paramètres de notification, par e-mail et par notification push, lorsque leur statut est automatiquement réglé sur absent \(\[section 4\.17\]\(#booking-a-reading\)\)/,
    es: /Los asesores también reciben un aviso, en la aplicación y, según su configuración de notificaciones, por correo electrónico y como notificación push, cuando su estado se cambia automáticamente a ausente \(\[Sección 4\.17\]\(#booking-a-reading\)\)/,
  };
  const SHARE_CODE: Record<Locale, RegExp> = {
    en: /If you are signed in as a client while the referral program is running, the link also contains your personal invite code, so that a friend who signs up from it counts as invited \(\[Section 4\.20\]\(#referral-program\)\)/,
    fr: /Si vous êtes connecté en tant que client pendant que le programme de parrainage est en cours, le lien contient aussi votre code d’invitation personnel, afin qu’un ami qui s’inscrit à partir de ce lien soit compté comme invité \(\[section 4\.20\]\(#referral-program\)\)/,
    es: /Si ha iniciado sesión como cliente mientras el programa de referidos está en marcha, el enlace también contiene su código de invitación personal, para que un amigo que se registre desde él cuente como invitado \(\[Sección 4\.20\]\(#referral-program\)\)/,
  };

  it.each(locales)("%s: 4.17 says an advisor who misses several requests in a row is set to Away and why, and that other advisors may be suggested", (l) => {
    const text = clause(terms(l), "booking-a-reading");
    expect(text).toMatch(AUTO_AWAY[l]);
    expect(text).toMatch(ALTERNATIVES[l]);
  });

  it.each(locales)("%s: 4.13 tells advisors they are notified when their status is set to Away automatically", (l) => {
    expect(clause(terms(l), "notifications")).toMatch(AWAY_NOTICE[l]);
  });

  it.each(locales)("%s: 4.19 says a signed-in client's shared link carries the personal invite code", (l) => {
    expect(clause(terms(l), "sharing-your-results")).toMatch(SHARE_CODE[l]);
  });
});

describe("the referral program is no longer in beta and has no launch", () => {
  const OLD_WORDING = /\bbeta\b|bêta|launch|lancement|lancées?\b|lanzamiento|lanzadas?\b|lancemos|lanzar/i;

  it.each(locales)("%s: Terms 4.20 and every referral clause say nothing about a beta or a launch", (l) => {
    const doc = terms(l);
    const referral = [
      clause(doc, "referral-program"),
      ...NEW_ANCHORS.filter(([id]) => id.startsWith("referral-")).map(([id]) => clause(doc, id)),
    ].join("\n");
    expect(referral).not.toMatch(OLD_WORDING);
    const withReferral = allBlocks(doc).map(textOf).filter((t) => /referral|parrain|referid|invite link|lien d’invitation|enlace de invitación/i.test(t));
    expect(withReferral.length).toBeGreaterThan(5);
    for (const t of withReferral) expect(t).not.toMatch(OLD_WORDING);
  });

  it.each(locales)("%s: 4.20 names no launch status on the Invite friends page", (l) => {
    expect(clause(terms(l), "referral-program")).toMatch(
      {
        en: /The program’s current reward and limit are shown on the \[Invite friends\]\(\/profile\/invite\) page of your account\./,
        fr: /La récompense et la limite en vigueur du programme sont indiquées sur la page \[Inviter des amis\]\(\/profile\/invite\) de votre compte\./,
        es: /La recompensa y el límite vigentes del programa se muestran en la página \[Invitar a amigos\]\(\/profile\/invite\) de su cuenta\./,
      }[l],
    );
  });

  it.each(locales)("%s: 4.20.4 is titled for when minutes are added", (l) => {
    const heading = allBlocks(terms(l)).find((b) => b.type === "h4" && b.id === "referral-minutes-added") as { title: string };
    expect(heading.title).toBe({ en: "When Minutes Are Added", fr: "Ajout des minutes", es: "Cuándo se añaden los minutos" }[l]);
  });
});
