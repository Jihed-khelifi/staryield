import { describe, expect, it } from "vitest";

import { legalDocuments } from "@/content/legal";
import type { LegalDocumentContent } from "@/content/legal/types";

const locales = ["en", "fr", "es"] as const;
const section = (doc: LegalDocumentContent, id: string) => {
  const i = doc.sections.findIndex((s) => s.blocks.some((b) => "id" in b && b.id === id));
  return doc.sections[i].blocks;
};
const flat = (doc: LegalDocumentContent) => doc.sections.flatMap((s) => s.blocks).map((b) => ("text" in b ? b.text : "items" in b ? b.items.join(" ") : "")).join("\n");

const REPORT = { en: "Report an issue", fr: "Signaler un problème", es: "Informar de un problema" } as const;
const NOT_TOLD = { en: "not told", fr: "n’est pas informé", es: "no es informado" } as const;
const TRANSCRIPT = { en: "transcript", fr: "transcription", es: "transcripción" } as const;

describe("revisions 3.1 legal updates (Report an issue)", () => {
  it.each(locales)("privacy/%s explains conversation reports in the support-requests section", (l) => {
    const doc = legalDocuments.privacy[l];
    const blocks = section(doc, "support-requests");
    const para = blocks.map((b) => ("text" in b ? b.text : "")).find((t) => t.includes(REPORT[l]));
    expect(para).toBeTruthy();
    expect(para).toContain(NOT_TOLD[l]);
    expect(para).toContain(TRANSCRIPT[l]);
    expect(para).toContain("(/support/requests)");
  });

  it.each(locales)("terms/%s mentions Report an issue next to reporting content, linking #support-requests", (l) => {
    const doc = legalDocuments.terms[l];
    const para = flat(doc).split("\n").find((t) => t.includes(REPORT[l]));
    expect(para).toBeTruthy();
    expect(para).toContain("(/privacy#support-requests)");
  });

  it.each(locales.flatMap((l) => (["privacy", "terms"] as const).map((d) => [d, l] as const)))("%s/%s keeps a consistent ISO version and dates", (d, l) => {
    const m = legalDocuments[d][l].meta;
    expect(m.version).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(m).toMatchObject({ effective: m.version, lastUpdated: m.version });
    expect(m.version).toBe(legalDocuments[d].en.meta.version);
  });
});
