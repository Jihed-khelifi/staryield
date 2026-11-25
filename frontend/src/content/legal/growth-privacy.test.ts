import { describe, expect, it } from "vitest";

import { legalDocuments } from "@/content/legal";
import type { LegalBlock, LegalDocumentContent } from "@/content/legal/types";

/**
 * Growth features (Privacy Policy): e-mail address confirmation (2.16), the referral program (2.17), reminders and the
 * "Get started" checklist (2.18), the free calculators and search pages (2.19), share images (2.20), the new analytics
 * events (2.21), e-mail delivery records and test sends (2.22), with the matching purposes, legitimate interests,
 * rights, retention, storage and share-sheet text. The English text prevails; French and Spanish mirror it.
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
const allBlocks = (doc: LegalDocumentContent) => doc.sections.flatMap((s) => s.blocks);
const allIds = (doc: LegalDocumentContent) =>
  [...doc.intro, ...doc.sections.flatMap((s) => s.blocks)].flatMap((b) => ("id" in b && b.id ? [b.id] : [])).concat(doc.sections.map((s) => s.id));
const section = (doc: LegalDocumentContent, id: string) => {
  const found = doc.sections.find((s) => s.id === id);
  if (!found) throw new Error(`no section ${id}`);
  return found.blocks.map(textOf).join("\n");
};
/** The blocks after a heading up to the next heading of the same or higher level. */
const clause = (doc: LegalDocumentContent, id: string) => {
  const blocks = allBlocks(doc);
  const start = blocks.findIndex((b) => (b.type === "h3" || b.type === "h4") && b.id === id);
  if (start < 0) throw new Error(`no clause ${id}`);
  const level = (blocks[start] as { type: "h3" | "h4" }).type;
  const rest = blocks.slice(start + 1);
  const end = rest.findIndex((b) => b.type === "h3" || (level === "h4" && b.type === "h4"));
  return (end < 0 ? rest : rest.slice(0, end)).map(textOf).join("\n");
};
const tables = (doc: LegalDocumentContent) => allBlocks(doc).flatMap((b) => (b.type === "table" ? [b] : []));
const privacy = (l: Locale) => legalDocuments.privacy[l];

const NEW_CLAUSES = [
  ["email-address-verification", "2.16"],
  ["referral-program", "2.17"],
  ["lifecycle-messages", "2.18"],
  ["free-calculators-and-search-pages", "2.19"],
  ["share-images", "2.20"],
  ["feature-usage-measurement", "2.21"],
  ["email-delivery-records", "2.22"],
] as const;

describe("growth: Privacy Policy", () => {
  it.each(locales)("%s: the new clauses 2.16 to 2.22 follow 2.15 in section 2 with their anchors and numbers", (l) => {
    const s2 = privacy(l).sections.find((s) => s.id === "categories-of-personal-data-we-collect")!;
    const headings = s2.blocks.flatMap((b) => (b.type === "h3" ? [[b.id, b.n] as const] : []));
    expect(headings.slice(-NEW_CLAUSES.length - 1)).toEqual([["marketing-emails", "2.15"], ...NEW_CLAUSES]);
  });

  it.each(locales)("%s: every anchor is identical to English and the existing anchors are unchanged", (l) => {
    expect(allIds(privacy(l))).toEqual(allIds(privacy("en")));
    for (const id of ["calculator-report-emails", "marketing-emails", "legitimate-interests", "cookies", "data-retention", "right-to-data-portability"])
      expect(allIds(privacy(l))).toContain(id);
  });

  it.each(locales)("%s: links to the new clauses resolve inside the document", (l) => {
    const ids = new Set(allIds(privacy(l)));
    const json = JSON.stringify(privacy(l));
    for (const [id] of NEW_CLAUSES.filter(([id]) => id !== "free-calculators-and-search-pages" && id !== "share-images" && id !== "email-delivery-records"))
      expect(json).toContain(`](#${id})`);
    for (const [, href] of json.matchAll(/\]\((#[a-z0-9-]+)\)/g)) expect(ids.has(href.slice(1))).toBe(true);
  });

  it.each(locales)("%s: keeps exactly three tables, with full rows", (l) => {
    expect(tables(privacy(l))).toHaveLength(3);
    for (const table of tables(privacy(l))) for (const row of table.rows) expect(row).toHaveLength(table.head.length);
  });

  it.each(locales)("%s: 2.16 describes the confirmation link: hashed single-use code, 48 hours, fragment, audit, admin resend", (l) => {
    expect(clause(privacy(l), "email-address-verification")).toMatch(
      {
        en: /welcome email with a link to confirm[\s\S]*transactional message[\s\S]*only a one-way hash of its secret code, never the code itself[\s\S]*used once and expires after 48 hours[\s\S]*at most one link a minute and 5 links in any 24 hours[\s\S]*audit record[\s\S]*keyed hash of the IP address[\s\S]*after the “#”, which browsers do not send to servers[\s\S]*administrators can see whether and when your address was confirmed and can send you a new link, but they never see the link itself[\s\S]*about a day after the link expires/,
        fr: /e-mail de bienvenue contenant un lien pour confirmer[\s\S]*message transactionnel[\s\S]*qu’un hachage unidirectionnel de son code secret, jamais le code lui-même[\s\S]*ne peut servir qu’une fois et expire au bout de 48 heures[\s\S]*au plus un lien par minute et 5 liens sur toute période de 24 heures[\s\S]*journal d’audit[\s\S]*hachage à clé de l’adresse IP[\s\S]*après le « # », que les navigateurs n’envoient ni aux serveurs[\s\S]*administrateurs peuvent voir si votre adresse a été confirmée et à quelle date, et peuvent vous envoyer un nouveau lien, mais ils ne voient jamais le lien lui-même[\s\S]*environ un jour après l’expiration du lien/,
        es: /correo de bienvenida con un enlace para confirmar[\s\S]*mensaje transaccional[\s\S]*solo guardamos un hash unidireccional de su código secreto, nunca el código en sí[\s\S]*solo puede usarse una vez y caduca a las 48 horas[\s\S]*como máximo un enlace por minuto y 5 enlaces en cualquier periodo de 24 horas[\s\S]*registro de auditoría[\s\S]*hash con clave de la dirección IP[\s\S]*sigue a «#», que los navegadores no envían a los servidores[\s\S]*administradores pueden ver si su dirección se ha confirmado y cuándo, y pueden enviarle un enlace nuevo, pero nunca ven el enlace en sí[\s\S]*aproximadamente un día después de que caduque el enlace/,
      }[l],
    );
  });

  it.each(locales)("%s: 2.17 lists the referral data, the abuse checks, what inviters see, retention, erasure and export", (l) => {
    expect(clause(privacy(l), "referral-program")).toMatch(
      {
        en: /8-character code[\s\S]*keyed hash of the IP address the friend signed up from \(never the address itself\)[\s\S]*last 30 days[\s\S]*last 24 hours[\s\S]*random browser identifier[\s\S]*not credited[\s\S]*held for a person on our team to review[\s\S]*Inviters see only totals[\s\S]*never tell an inviter who signed up or who had a reading[\s\S]*never name a friend[\s\S]*share sheet[\s\S]*local storage for up to 30 days[\s\S]*never the code itself[\s\S]*unfinished referral is deleted[\s\S]*without any link to the friend’s account and without the IP hash[\s\S]*referrals\.json[\s\S]*never who invited you or whom you invited/,
        fr: /code de 8 caractères[\s\S]*hachage à clé de l’adresse IP depuis laquelle l’ami s’est inscrit \(jamais l’adresse elle-même\)[\s\S]*30 derniers jours[\s\S]*24 dernières heures[\s\S]*identifiant aléatoire du navigateur[\s\S]*n’est pas prise en compte[\s\S]*mise en attente pour qu’un membre de notre équipe l’examine[\s\S]*ne voient que des totaux[\s\S]*n’indiquons jamais à une personne qui invite qui s’est inscrit ni qui a eu une consultation[\s\S]*ne nomment jamais un ami[\s\S]*menu de partage[\s\S]*stockage local de votre navigateur pendant 30 jours au maximum[\s\S]*jamais le code lui-même[\s\S]*parrainage non abouti est supprimé[\s\S]*sans aucun lien avec le compte de l’ami et sans le hachage de l’adresse IP[\s\S]*referrals\.json[\s\S]*jamais l’identité de la personne qui vous a invité/,
        es: /código de 8 caracteres[\s\S]*hash con clave de la dirección IP desde la que se registró el amigo \(nunca la dirección en sí\)[\s\S]*últimos 30 días[\s\S]*últimas 24 horas[\s\S]*identificador aleatorio del navegador[\s\S]*no se tiene en cuenta[\s\S]*queda retenido para que una persona de nuestro equipo lo revise[\s\S]*solo ve totales[\s\S]*Nunca le decimos a quien invita quién se registró ni quién tuvo una consulta[\s\S]*nunca nombran a ningún amigo[\s\S]*menú para compartir[\s\S]*almacenamiento local de su navegador durante un máximo de 30 días[\s\S]*nunca el código en sí[\s\S]*se elimina una invitación que no se completó[\s\S]*sin ningún vínculo con la cuenta del amigo y sin el hash de la IP[\s\S]*referrals\.json[\s\S]*nunca quién le invitó ni a quién invitó usted/,
      }[l],
    );
  });

  it.each(locales)("%s: 2.18 gives the data used, the lawful bases (legitimate interest, transactional, consent), the caps and retention", (l) => {
    expect(clause(privacy(l), "lifecycle-messages")).toMatch(
      {
        en: /Your sign-up date, whether your email address is confirmed[\s\S]*when you last signed in, and the number of free minutes in your wallet[\s\S]*legitimate interest[\s\S]*transactional message[\s\S]*promotional: we send them by email or push only if you have turned on Marketing updates[\s\S]*company name and postal address[\s\S]*at most one of these messages every 48 hours[\s\S]*at most once every 30 days and no more than 3 times[\s\S]*restricted by the age check[\s\S]*7 days unless our team sets another period, of up to 30 days[\s\S]*never sent twice[\s\S]*only totals and rates[\s\S]*kept for as long as your account exists/,
        fr: /Votre date d’inscription, le fait que votre adresse e-mail soit confirmée[\s\S]*votre dernière connexion et le nombre de minutes offertes dans votre portefeuille[\s\S]*intérêt légitime[\s\S]*message transactionnel[\s\S]*promotionnels : nous ne les envoyons par e-mail ou par notification push que si vous avez activé « Actualités marketing »[\s\S]*nom et l’adresse postale de notre société[\s\S]*au plus un de ces messages toutes les 48 heures[\s\S]*au plus une fois tous les 30 jours et au maximum 3 fois[\s\S]*restreints par la vérification de l’âge[\s\S]*7 jours, sauf si notre équipe fixe une autre durée, de 30 jours au plus[\s\S]*jamais envoyé deux fois[\s\S]*que des totaux et des taux[\s\S]*conservés tant que votre compte existe/,
        es: /Su fecha de registro, si su dirección de correo electrónico está confirmada[\s\S]*cuándo inició sesión por última vez y el número de minutos gratuitos de su monedero[\s\S]*interés legítimo[\s\S]*mensaje transaccional[\s\S]*promocionales: solo se los enviamos por correo electrónico o push si ha activado «Novedades de marketing»[\s\S]*nombre y la dirección postal de nuestra empresa[\s\S]*como máximo uno de estos mensajes cada 48 horas[\s\S]*como máximo una vez cada 30 días y no más de 3 veces[\s\S]*restringidas por la comprobación de edad[\s\S]*7 días, salvo que nuestro equipo fije otro periodo, de hasta 30 días[\s\S]*nunca se envíe dos veces[\s\S]*solo muestra totales y porcentajes[\s\S]*se conservan mientras exista su cuenta/,
      }[l],
    );
    expect(clause(privacy(l), "lifecycle-messages")).toContain("(/profile/settings)");
  });

  it.each(locales)("%s: 2.19 says the tarot birth card stays in the browser, fragments are never sent and the compatibility dates are not stored", (l) => {
    const text = clause(privacy(l), "free-calculators-and-search-pages");
    expect(text).toMatch(
      {
        en: /in the body of the request, not in the web address[\s\S]*not stored[\s\S]*works out your cards in your browser: the date of birth you enter is not sent to us and is not stored[\s\S]*after the “#”, which browsers do not send to servers[\s\S]*sends each of the two dates of birth you enter \(the date only, without a name or a time\)[\s\S]*not stored[\s\S]*only with that person’s agreement[\s\S]*never the dates you enter/,
        fr: /dans le corps de la requête, et non dans l’adresse web[\s\S]*ne sont pas conservés[\s\S]*détermine vos cartes dans votre navigateur : la date de naissance que vous saisissez ne nous est pas envoyée et n’est pas conservée[\s\S]*après le « # », que les navigateurs n’envoient ni aux serveurs[\s\S]*chacune des deux dates de naissance que vous saisissez \(la date seule, sans nom ni heure\)[\s\S]*ne sont pas conservées[\s\S]*qu’avec son accord[\s\S]*jamais les dates que vous saisissez/,
        es: /en el cuerpo de la solicitud, no en la dirección web[\s\S]*no se conservan[\s\S]*obtiene sus cartas en su navegador: la fecha de nacimiento que introduce no se nos envía ni se conserva[\s\S]*sigue a «#», que los navegadores no envían a los servidores[\s\S]*cada una de las dos fechas de nacimiento que introduce \(solo la fecha, sin nombre ni hora\)[\s\S]*no se conservan[\s\S]*solo con su consentimiento[\s\S]*nunca las fechas que introduce/,
      }[l],
    );
    expect(text).toContain("(#calculator-report-emails)");
  });

  it.each(locales)("%s: 2.20 says share images are drawn on the device, carry no birth details and the share sheet hands them to the chosen app", (l) => {
    expect(clause(privacy(l), "share-images")).toMatch(
      {
        en: /9:16[\s\S]*drawn on your device; we do not receive or keep it[\s\S]*never your date, time or place of birth or your name[\s\S]*share sheet passes the image, a short text and a link to the app you pick[\s\S]*its own terms and privacy policy[\s\S]*never contains your birth details/,
        fr: /9:16[\s\S]*dessinée sur votre appareil ; nous ne la recevons pas et ne la conservons pas[\s\S]*jamais vos date, heure ou lieu de naissance ni votre nom[\s\S]*menu de partage de votre appareil transmet l’image, un court texte et un lien à l’application que vous choisissez[\s\S]*propre politique de confidentialité[\s\S]*ne contient jamais vos données de naissance/,
        es: /9:16[\s\S]*se dibuja en su dispositivo; no la recibimos ni la conservamos[\s\S]*nunca su fecha, hora o lugar de nacimiento ni su nombre[\s\S]*menú para compartir de su dispositivo pasa la imagen, un texto breve y un enlace a la aplicación que elija[\s\S]*propia política de privacidad[\s\S]*nunca contiene sus datos de nacimiento/,
      }[l],
    );
  });

  it.each(locales)("%s: 2.21 lists the new analytics events and honours Do Not Track, Global Privacy Control and the analytics switch", (l) => {
    const text = clause(privacy(l), "feature-usage-measurement");
    expect(text).toMatch(
      {
        en: /Booking button[\s\S]*Share images[\s\S]*“Get started” checklist[\s\S]*Invitations: that an invitation link was opened[\s\S]*never the code itself[\s\S]*Calculators and search pages[\s\S]*Do Not Track or Global Privacy Control signal, or if you switch analytics off/,
        fr: /Bouton de réservation[\s\S]*Images de partage[\s\S]*Liste « Commencer »[\s\S]*Invitations : l’ouverture d’un lien d’invitation[\s\S]*jamais le code lui-même[\s\S]*Calculateurs et pages de recherche[\s\S]*Do Not Track ou Global Privacy Control, ou si vous désactivez les statistiques/,
        es: /Botón de reserva[\s\S]*Imágenes para compartir[\s\S]*Lista «Empezar»[\s\S]*Invitaciones: que se abrió un enlace de invitación[\s\S]*nunca el código en sí[\s\S]*Calculadoras y páginas de búsqueda[\s\S]*Do Not Track o Global Privacy Control, o si desactiva la analítica/,
      }[l],
    );
    expect(text).toContain("(/profile/settings)");
  });

  it.each(locales)("%s: 2.22 describes delivery records, the admin overview without recipients and the audited test send", (l) => {
    expect(clause(privacy(l), "email-delivery-records")).toMatch(
      {
        en: /keyed hash of the recipient’s address[\s\S]*stored encrypted, and only until the email has been sent or has finally failed[\s\S]*deleted 30 days after that[\s\S]*never shows recipients’ addresses or the content of emails[\s\S]*test email to an address they choose[\s\S]*audit log records it with the address partly masked/,
        fr: /hachage à clé de l’adresse du destinataire[\s\S]*stockés chiffrés, et uniquement jusqu’à ce que l’e-mail ait été envoyé ou ait définitivement échoué[\s\S]*supprimé 30 jours plus tard[\s\S]*n’affiche jamais l’adresse des destinataires ni le contenu des e-mails[\s\S]*e-mail de test à l’adresse de leur choix[\s\S]*journal d’audit l’enregistre avec l’adresse partiellement masquée/,
        es: /hash con clave de la dirección del destinatario[\s\S]*se guardan cifrados, y solo hasta que el correo se ha enviado o ha fallado definitivamente[\s\S]*se elimina 30 días después[\s\S]*nunca muestra las direcciones de los destinatarios ni el contenido de los correos[\s\S]*correo de prueba a la dirección que elijan[\s\S]*registro de auditoría la anota con la dirección parcialmente oculta/,
      }[l],
    );
  });

  it.each(locales)("%s: the purposes table gains the growth rows before the consent-record row, which stays last", (l) => {
    const [purposes] = tables(privacy(l));
    const col0 = purposes.rows.map((r) => r[0]).join("\n");
    expect(col0).toMatch(
      {
        en: /confirm that you control your email address[\s\S]*run the referral program[\s\S]*check invitations for abuse[\s\S]*send reminders in the app[\s\S]*promotional reminders by email and push[\s\S]*compatibility calculator[\s\S]*booking button, share images[\s\S]*delivery records of our emails/,
        fr: /Confirmer que vous contrôlez votre adresse e-mail[\s\S]*Gérer le programme de parrainage[\s\S]*Contrôler les invitations[\s\S]*rappels dans l’application[\s\S]*rappels promotionnels par e-mail et par notification push[\s\S]*calculateur de compatibilité[\s\S]*bouton de réservation, des images de partage[\s\S]*distribution de nos e-mails/,
        es: /Confirmar que usted controla su dirección de correo electrónico[\s\S]*Gestionar el programa de invitaciones[\s\S]*Controlar las invitaciones[\s\S]*recordatorios en la aplicación[\s\S]*recordatorios promocionales por correo electrónico y push[\s\S]*calculadora de compatibilidad[\s\S]*botón de reserva, las imágenes para compartir[\s\S]*registros de entrega de nuestros correos/,
      }[l],
    );
    const promo = purposes.rows.find((r) => /promotional|promotionnels|promocionales/.test(r[0]))!;
    expect(promo[2]).toMatch(/consent|consentement|consentimiento/i);
    expect(purposes.rows.at(-1)![1]).toMatch(/consent|consentimiento|consentement/i);
  });

  it.each(locales)("%s: legitimate interests cover e-mail confirmation, abuse checks, in-app reminders, another person's date, measurement and delivery", (l) => {
    expect(clause(privacy(l), "legitimate-interests")).toMatch(
      {
        en: /When confirming email addresses[\s\S]*When checking invitations for abuse[\s\S]*When sending reminders in the app[\s\S]*another person’s date of birth[\s\S]*When measuring how the booking button[\s\S]*When keeping email delivery records/,
        fr: /Lorsque nous confirmons les adresses e-mail[\s\S]*Lorsque nous contrôlons les invitations[\s\S]*Lorsque nous envoyons des rappels dans l’application[\s\S]*date de naissance d’une autre personne[\s\S]*Lorsque nous mesurons l’utilisation du bouton de réservation[\s\S]*Lorsque nous conservons les enregistrements de distribution/,
        es: /Al confirmar direcciones de correo electrónico[\s\S]*Al controlar las invitaciones[\s\S]*Al enviar recordatorios en la aplicación[\s\S]*fecha de nacimiento de otra persona[\s\S]*Al medir el uso del botón de reserva[\s\S]*Al conservar los registros de entrega/,
      }[l],
    );
  });

  it.each(locales)("%s: rights, retention, the e-mail provider and third-party apps mention the growth features", (l) => {
    expect(clause(privacy(l), "right-to-withdraw-consent")).toContain("(#lifecycle-messages)");
    expect(clause(privacy(l), "right-to-data-portability")).toContain("referrals.json");
    expect(section(privacy(l), "data-retention")).toContain("(#referral-program)");
    expect(section(privacy(l), "data-retention")).toContain("(#lifecycle-messages)");
    expect(section(privacy(l), "how-we-share-your-personal-data")).toMatch({ en: /link to confirm your email address/, fr: /lien de confirmation de votre adresse e-mail/, es: /enlace para confirmar su dirección de correo electrónico/ }[l]);
    expect(section(privacy(l), "third-party-websites-and-applications")).toMatch({ en: /share sheet/, fr: /menu de partage/, es: /menú para compartir/ }[l]);
    expect(clause(privacy(l), "live-readings-ratings-and-notifications")).toContain("(#cookies)");
  });

  it.each(locales)("%s: the storage table lists the referral code (30 days), the banner flag and the pending waiting-list request (1 hour)", (l) => {
    const cookies = tables(privacy(l)).find((t) => t.head.length === 5)!;
    const row = (name: string) => cookies.rows.find((r) => r[0] === name);
    const referral = row("staryield.referral")!;
    expect(referral[3]).toMatch(/\b30\b/);
    expect(referral[4]).toBe({ en: "Invitations", fr: "Invitations", es: "Invitaciones" }[l]);
    expect(row("staryield.referral.welcome-dismissed")).toBeTruthy();
    const intent = row("staryield.booking.intent")!;
    expect(intent[1]).toBe({ en: "Session storage", fr: "Stockage de session", es: "Almacenamiento de sesión" }[l]);
    expect(intent[3]).toMatch(/\b1\b/);
    expect(section(privacy(l), "cookies")).toMatch({ en: /whether or not analytics are allowed/, fr: /que les statistiques soient autorisées ou non/, es: /tanto si la analítica está permitida como si no/ }[l]);
  });

  it("en: the storage rows have the agreed wording", () => {
    const cookies = tables(privacy("en")).find((t) => t.head.length === 5)!;
    expect(cookies.rows.find((r) => r[0] === "staryield.referral")).toEqual([
      "staryield.referral",
      "Local storage",
      "Keeps the invitation code of the link that brought you, and when you first opened it, so that the friend who invited you can be credited if you sign up; removed when you sign up",
      "Up to 30 days",
      "Invitations",
    ]);
    expect(cookies.rows.find((r) => r[0] === "staryield.booking.intent")).toEqual([
      "staryield.booking.intent",
      "Session storage",
      "Remembers that you asked to join an advisor’s waiting list before you had an account, so that you can be added to it after you sign up",
      "Up to 1 hour, or until the tab is closed",
      "Essential",
    ]);
  });

  it.each(locales)("%s: the new text makes no accuracy, prediction or healing claim", (l) => {
    const added = NEW_CLAUSES.map(([id]) => clause(privacy(l), id)).join("\n");
    expect(added).not.toMatch(/accura|predict|healing|exactitude|prédi|guéri|exactitud|predic|sanaci/i);
  });
});

/** Follow-ups (fact-check of the growth branch): invite code in shared links, notices, lifecycle data, auto-away, UTM, funnel. */
describe("follow-ups: Privacy Policy", () => {
  const EXPECT: Record<string, Record<Locale, RegExp[]>> = {
    "share-images": {
      en: [/If you are signed in as a client while the referral program is running, the link also contains your personal invite code, so that a friend who signs up from it counts as invited \(\[Section 2\.17\]\(#referral-program\)\)/],
      fr: [/Si vous êtes connecté en tant que client pendant que le programme de parrainage est en cours, le lien contient aussi votre code d’invitation personnel, afin qu’un ami qui s’inscrit à partir de ce lien soit compté comme invité/],
      es: [/Si ha iniciado sesión como cliente mientras el programa de referidos está en marcha, el enlace también contiene su código de invitación personal, para que un amigo que se registre desde él cuente como invitado/],
    },
    "referral-program": {
      en: [
        /created the first time you view your invitation link or open the share window/,
        /when a friend signs up \(not when a sign-up is held for review or rejected\) and when free minutes are added to their wallet/,
        /The message about added minutes is an account message, also sent by email whatever the inviter’s notification settings; the other messages follow their notification settings/,
      ],
      fr: [
        /la première fois que vous affichez votre lien d’invitation ou que vous ouvrez la fenêtre de partage/,
        /lorsqu’une inscription est retenue pour examen ou rejetée\) et lorsque des minutes offertes sont ajoutées à son portefeuille/,
        /Le message concernant les minutes ajoutées est un message de compte, également envoyé par e-mail quels que soient les paramètres de notification de la personne qui invite/,
      ],
      es: [
        /la primera vez que ve su enlace de invitación o abre la ventana para compartir/,
        /o es rechazado\) y cuando se añaden minutos gratuitos a su monedero/,
        /El mensaje sobre los minutos añadidos es un mensaje de cuenta, que también se envía por correo electrónico sea cual sea la configuración de notificaciones de quien invita/,
      ],
    },
    "lifecycle-messages": {
      en: [/a reading summary: the date of your first and of your last completed reading and how many you have had/, /This summary is deleted when your account is erased/],
      fr: [/un résumé de ses consultations : la date de votre première et de votre dernière consultation terminée, et leur nombre/, /Ce résumé est supprimé lors de l’effacement de votre compte/],
      es: [/un resumen de sus consultas: la fecha de su primera y de su última consulta completada y cuántas ha tenido/, /Este resumen se elimina cuando se borra su cuenta/],
    },
    "right-to-data-portability": {
      en: [/the records of the reminders we sent you \(lifecycle-messages\.json\), and your account file \(account\.json\) shows when you hid the “Get started” checklist/],
      fr: [/les enregistrements des rappels que nous vous avons envoyés \(lifecycle-messages\.json\), et le fichier de votre compte \(account\.json\) indique quand vous avez masqué la liste « Commencer »/],
      es: [/los registros de los recordatorios que le enviamos \(lifecycle-messages\.json\), y el archivo de su cuenta \(account\.json\) indica cuándo ocultó la lista «Empezar»/],
    },
    "live-readings-ratings-and-notifications": {
      en: [/We also count the chat requests you missed in a row, to set you to Away automatically after a few/],
      fr: [/Nous comptons aussi les demandes de chat que vous avez manquées à la suite, afin de vous passer automatiquement sur absent après quelques-unes/],
      es: [/También contamos las solicitudes de chat que no atendió seguidas, para pasarle automáticamente a ausente tras unas pocas/],
    },
    "marketing-attribution": {
      en: [/Links in our e-mails may carry campaign parameters, which are handled in the same way/],
      fr: [/Les liens de nos e-mails peuvent comporter des paramètres de campagne, qui sont traités de la même manière/],
      es: [/Los enlaces de nuestros correos electrónicos pueden llevar parámetros de campaña, que se tratan de la misma manera/],
    },
    "calculator-usage-measurement": {
      en: [/In our reports, events recorded from your browser before you signed up are counted together with your account/],
      fr: [/Dans nos rapports, les événements enregistrés depuis votre navigateur avant votre inscription sont comptés avec votre compte/],
      es: [/En nuestros informes, los eventos registrados desde tu navegador antes de que te registraras se cuentan junto con tu cuenta/],
    },
  };

  it.each(locales)("%s: the follow-up sentences are in their clauses (invite code in links, notices, reading summary, export, auto-away, UTM, funnel)", (l) => {
    for (const [id, byLocale] of Object.entries(EXPECT)) {
      const text = clause(privacy(l), id);
      for (const re of byLocale[l]) expect(text, `${l} #${id}`).toMatch(re);
    }
  });

  it("en: the reward e-mail is described as an account message and the old notification wording is gone", () => {
    const text = clause(privacy("en"), "referral-program");
    expect(text).not.toMatch(/when a friend joins or completes a reading/);
    expect(text).not.toMatch(/by email when free minutes are added to their wallet, according to their notification settings/);
  });
});

describe("the referral program is no longer in beta and has no launch (Privacy Policy)", () => {
  const OLD_WORDING = /\bbeta\b|bêta|launch|lancement|lancées?\b|lanzamiento|lanzadas?\b|lancemos|lanzar/i;

  it.each(locales)("%s: nothing in the Privacy Policy says beta or launch about the referral program", (l) => {
    const doc = privacy(l);
    expect(clause(doc, "referral-program")).not.toMatch(OLD_WORDING);
    const sentences = allBlocks(doc)
      .flatMap((b) => textOf(b).split(/(?<=[.;:])\s+/))
      .filter((t) => /referral|parrain|referid|invite link|lien d’invitation|enlace de invitación|reward|récompense|recompensa/i.test(t));
    expect(sentences.length).toBeGreaterThan(3);
    for (const t of sentences) expect(t).not.toMatch(OLD_WORDING);
  });

  it.each(locales)("%s: the message about added minutes is an account message and the other messages follow the notification settings", (l) => {
    expect(clause(privacy(l), "referral-program")).toMatch(
      {
        en: /Messages\. We tell the inviter, in the app and by push if push notifications are on, when a friend signs up \(not when a sign-up is held for review or rejected\) and when free minutes are added to their wallet\. The message about added minutes is an account message, also sent by email whatever the inviter’s notification settings; the other messages follow their notification settings\. These are service messages about the program, not marketing\./,
        fr: /Messages\. Nous prévenons la personne qui invite[\s\S]*et lorsque des minutes offertes sont ajoutées à son portefeuille\. Le message concernant les minutes ajoutées est un message de compte, également envoyé par e-mail[\s\S]*les autres messages suivent ses paramètres de notification\./,
        es: /Mensajes\. Avisamos a quien invita[\s\S]*y cuando se añaden minutos gratuitos a su monedero\. El mensaje sobre los minutos añadidos es un mensaje de cuenta, que también se envía por correo electrónico[\s\S]*los demás mensajes siguen su configuración de notificaciones\./,
      }[l],
    );
  });
});
