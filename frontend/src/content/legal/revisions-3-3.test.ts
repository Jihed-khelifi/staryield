import { describe, expect, it } from "vitest";

import { legalDocuments } from "@/content/legal";
import type { LegalBlock, LegalDocumentContent } from "@/content/legal/types";

const locales = ["en", "fr", "es"] as const;
const ids = (doc: LegalDocumentContent) =>
  doc.sections.flatMap((s) => [s.id, ...s.blocks]).map((b) => (typeof b === "string" ? b : "id" in b ? (b as Extract<LegalBlock, { id: string }>).id : "")).filter(Boolean);
const flat = (doc: LegalDocumentContent) =>
  doc.sections
    .flatMap((s) => s.blocks)
    .map((b) => ("text" in b ? b.text : "items" in b ? b.items.join(" ") : "rows" in b ? b.rows.flat().join(" ") : ""))
    .join("\n");

const CITY_SWITCH = { en: "Share my birth city with advisors", fr: "Partager ma ville de naissance avec les conseillers", es: "Compartir mi ciudad de nacimiento con los asesores" } as const;
const OLD_SWITCH = { en: "Share birth details with advisors", fr: "Partager mes données de naissance avec les conseillers", es: "Compartir los datos de nacimiento con los asesores" } as const;

describe("revisions 3.3 legal updates", () => {
  it.each(locales.flatMap((l) => (["privacy", "terms"] as const).map((d) => [d, l] as const)))("%s/%s is dated 2026-11-13 (re-dated by revisions 5.1)", (d, l) => {
    expect(legalDocuments[d][l].meta).toMatchObject({ version: "2026-11-13", effective: "2026-11-13", lastUpdated: "2026-11-13" });
  });

  it.each(locales)("%s: both documents have the messages-outside-live-readings clause", (l) => {
    expect(ids(legalDocuments.privacy[l])).toContain("messages-outside-live-readings");
    expect(ids(legalDocuments.terms[l])).toContain("messages-outside-live-readings");
    const terms = flat(legalDocuments.terms[l]);
    expect(terms).toContain("2048");
    expect(terms).toContain("(#prohibited-conduct)");
    expect(terms).toContain("(/privacy#messages-outside-live-readings)");
    expect(flat(legalDocuments.privacy[l])).toContain("(/terms#messages-outside-live-readings)");
  });

  it.each(locales)("%s: date of birth is always shown, birth city is opt-in", (l) => {
    for (const d of ["privacy", "terms"] as const) {
      const all = flat(legalDocuments[d][l]);
      expect(all).toContain(CITY_SWITCH[l]);
      expect(all).not.toContain(OLD_SWITCH[l]);
    }
  });

  it.each(locales)("privacy/%s documents the CSRF cookie lifetime and image resizing", (l) => {
    const all = flat(legalDocuments.privacy[l]);
    const row = legalDocuments.privacy[l].sections.flatMap((s) => s.blocks).flatMap((b) => ("rows" in b ? b.rows : [])).find((r) => r[0] === "sy_csrf");
    expect(row?.[3]).toMatch(/30/);
    expect(all).toContain("2048");
  });
});
