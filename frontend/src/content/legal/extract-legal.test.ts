import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterAll, describe, expect, it } from "vitest";

import {
  LOCALES,
  buildDocument,
  classify,
  collectTexts,
  contentWords,
  createLookup,
  projectLocale,
  referencedElsewhere,
  sourceWords,
  toModule,
} from "../../../scripts/extract-legal.mjs";

// A miniature of the old Figma export: every sentence wrapped in text().
const FIXTURE = `
export default function Legal({ text }) {
  return (
    <main>
      <div data-name="Section / Hero">
        <span className="badge">{text("LEGAL")}</span>
        <h1>{text("Privacy Policy")}</h1>
      </div>
      <div data-name="Table Of Contents">
        <p>{text("Table of contents")}</p>
        <a href="#nowhere">{text("1. Who we are")}</a>
      </div>
      <p>{text("Last updated: April 2, 2026")}</p>
      <p>{text("This notice explains how we use your data.")}</p>
      <h2 id="who-we-are">{text("1. Who we are")}</h2>
      <p className="font-semibold">{text("1.1. Our company")}</p>
      <p>{text("We are a small team of 12 people.")}</p>
      <p>{text("• Email address")}</p>
      <p>{text("• Phone number")}</p>
      <p>{text("CONTACT US")}</p>
      <h2 id="your-rights">{text("2. Your rights")}</h2>
      <p>{text("2.1.1 Right to access")}</p>
      <p>{text("You may ask for a copy within 30 days.")}</p>
    </main>
  );
}
`;

const SOURCES = [
  "LEGAL",
  "Privacy Policy",
  "Table of contents",
  "1. Who we are",
  "Last updated: April 2, 2026",
  "This notice explains how we use your data.",
  "1.1. Our company",
  "We are a small team of 12 people.",
  "• Email address",
  "• Phone number",
  "Contact us",
  "2. Your rights",
  "2.1.1 Right to access",
  "You may ask for a copy within 30 days.",
];
const ES = [
  "LEGAL",
  "Política de privacidad",
  "Índice",
  "1. Quiénes somos",
  "Última actualización: 2 de abril de 2026",
  "Este aviso explica cómo usamos sus datos.",
  "1.1. Nuestra empresa",
  "Somos un pequeño equipo de 12 personas.",
  "• Dirección de correo electrónico",
  "• Número de teléfono",
  "Contáctenos",
  "2. Sus derechos",
  "2.1.1 Derecho de acceso",
  "Puede solicitar una copia en un plazo de 30 días.",
];
const FR = [
  "MENTIONS LÉGALES",
  "Politique de confidentialité",
  "Table des matières",
  "1. Qui nous sommes",
  "Dernière mise à jour : 2 avril 2026",
  "Cet avis explique comment nous utilisons vos données.",
  "1.1. Notre société",
  "Nous sommes une petite équipe de 12 personnes.",
  "• Adresse e-mail",
  "• Numéro de téléphone",
  "Nous contacter",
  "2. Vos droits",
  "2.1.1 Droit d'accès",
  undefined, // deliberately untranslated
];

const dictionary = (values: (string | undefined)[]) =>
  Object.fromEntries(
    SOURCES.flatMap((key, index) => (values[index] === undefined ? [] : [[key, values[index]]])),
  );
const lookups = {
  en: createLookup(dictionary(SOURCES)),
  es: createLookup(dictionary(ES)),
  fr: createLookup(dictionary(FR)),
};
const META = { version: "2026-11-04", effective: "2026-11-04", lastUpdated: "2026-11-04" };

const entries = collectTexts(FIXTURE, "fixture.tsx");
const built = buildDocument(entries, lookups, { docId: "privacy" });

describe("extract-legal (fixture round trip)", () => {
  it("collects every text() literal in document order with its JSX context", () => {
    expect(entries.map((entry: { text: string }) => entry.text)).toHaveLength(15);
    expect(classify(entries[0])).toEqual({ kind: "badge" });
    expect(classify(entries[1])).toEqual({ kind: "title" });
    expect(classify(entries[3])).toEqual({ kind: "toc" });
    expect(classify(entries[4])).toMatchObject({ kind: "date", label: "Last updated" });
    expect(classify(entries[6])).toMatchObject({ kind: "section", id: "who-we-are", n: "1" });
  });

  it("builds sections, headings and lists with stable ids and drops the TOC", () => {
    const en = projectLocale(built.doc, "en", META);
    expect(en.meta).toMatchObject({ badge: "LEGAL", title: "Privacy Policy", version: "2026-11-04" });
    expect(en.intro).toEqual([{ type: "p", text: "This notice explains how we use your data." }]);
    expect(en.sections.map((s: { id: string }) => s.id)).toEqual(["who-we-are", "your-rights"]);
    expect(en.sections[0]).toEqual({
      id: "who-we-are",
      n: "1",
      title: "Who we are",
      blocks: [
        { type: "h3", id: "our-company", n: "1.1", title: "Our company" },
        { type: "p", text: "We are a small team of 12 people." },
        { type: "ul", items: ["Email address", "Phone number"] },
        { type: "h3", id: "contact-us", title: "CONTACT US" },
      ],
    });
    expect(en.sections[1].blocks[0]).toEqual({
      type: "h4",
      id: "right-to-access",
      n: "2.1.1",
      title: "Right to access",
    });
    expect(built.tocStrings).toEqual(["Table of contents", "1. Who we are"]);
  });

  it("translates exactly like the runtime lookup, preserving upper case", () => {
    const es = projectLocale(built.doc, "es", META);
    expect(es.sections[0].title).toBe("Quiénes somos");
    expect(es.sections[0].blocks[3]).toMatchObject({ id: "contact-us", title: "CONTÁCTENOS" });
    expect(es.sections[0].blocks[2]).toEqual({
      type: "ul",
      items: ["Dirección de correo electrónico", "Número de teléfono"],
    });
    expect(es.labels.toc).toBe("Índice");
  });

  it("reports untranslated strings instead of silently dropping them", () => {
    expect(built.missing).toEqual([
      { locale: "fr", value: "You may ask for a copy within 30 days." },
    ]);
  });

  it("keeps the same number of words in every locale", () => {
    for (const locale of LOCALES) {
      const lookup = (value: string) =>
        locale === "en"
          ? value
          : (lookups[locale as "es" | "fr"](value).value ?? value);
      const content = projectLocale(built.doc, locale, META);
      expect(contentWords(content)).toBe(sourceWords(entries, lookup));
    }
  });

  it("detects a translated heading that lost its clause number", () => {
    const broken = ES.map((value, index) => (index === 11 ? "Sus derechos (2)" : value));
    const brokenLookups = { ...lookups, es: createLookup(dictionary(broken)) };
    const rebuilt = buildDocument(entries, brokenLookups, { docId: "privacy" });
    const lookup = (value: string) => brokenLookups.es(value).value ?? value;
    const content = projectLocale(rebuilt.doc, "es", META);
    expect(contentWords(content)).not.toBe(sourceWords(entries, lookup));
  });

  it("emits a typed module whose data parses back to the same content", () => {
    const content = projectLocale(built.doc, "fr", META);
    const source = toModule(content);
    expect(source).toContain('import type { LegalDocumentContent } from "@/content/legal/types";');
    const json = source.slice(source.indexOf(" = ") + 3, source.lastIndexOf(";\n\nexport default"));
    expect(JSON.parse(json)).toEqual(content);
  });

  it("collects the migrated keys and keeps those still used elsewhere", () => {
    expect(built.keys).toContain("Contact us");
    expect(built.keys).toContain("Last updated: April 2, 2026");
    const dir = mkdtempSync(join(tmpdir(), "extract-legal-"));
    tempDirs.push(dir);
    const other = join(dir, "footer.tsx");
    writeFileSync(other, `export const Footer = () => text("Privacy  policy");\n`);
    expect([...referencedElsewhere([other], built.keys)]).toEqual(["Privacy Policy"]);
  });
});

const tempDirs: string[] = [];
afterAll(() => {
  for (const dir of tempDirs) rmSync(dir, { recursive: true, force: true });
});
