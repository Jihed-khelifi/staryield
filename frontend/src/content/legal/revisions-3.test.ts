import { describe, expect, it } from "vitest";

import { legalDocuments } from "@/content/legal";
import type { LegalBlock, LegalDocumentContent } from "@/content/legal/types";

const locales = ["en", "fr", "es"] as const;

function ids(doc: LegalDocumentContent): string[] {
  return [...doc.intro, ...doc.sections.flatMap((s) => [{ type: "h3", id: s.id } as LegalBlock, ...s.blocks])]
    .filter((b): b is Extract<LegalBlock, { id: string }> => "id" in b)
    .map((b) => b.id);
}
function text(doc: LegalDocumentContent): string {
  return doc.sections
    .flatMap((s) => s.blocks)
    .map((b) => ("text" in b ? b.text : "items" in b ? b.items.join(" ") : ""))
    .join("\n");
}

describe("revisions 3.0 legal updates", () => {
  it.each(locales.flatMap((l) => (["privacy", "terms"] as const).map((d) => [d, l] as const)))("%s/%s has a consistent ISO version and dates", (d, l) => {
    const m = legalDocuments[d][l].meta;
    expect(m.version).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(m).toMatchObject({ effective: m.version, lastUpdated: m.version });
    expect(m.version).toBe(legalDocuments[d].en.meta.version);
  });

  it.each(locales)("privacy/%s covers the support desk, calculator measurement, chat drafts and free mode", (l) => {
    const doc = legalDocuments.privacy[l];
    for (const id of ["support-requests", "calculator-usage-measurement", "chat-notifications-and-drafts", "free-access-periods"]) expect(ids(doc)).toContain(id);
    const all = text(doc);
    // 30-day private links, 24 months after closing, 25 months of measurement, 7 days of drafts.
    for (const n of ["30", "24", "25", "7"]) expect(all).toMatch(new RegExp(`\\b${n}\\b`));
    expect(all).toContain("/support/requests");
  });

  it.each(locales)("terms/%s covers introductory free minutes, free mode and support communications", (l) => {
    const doc = legalDocuments.terms[l];
    const all = text(doc);
    expect(all).toContain("#support-requests");
    expect(all).toMatch(/free mode|mode gratuit|modo gratuito/);
    expect(ids(doc)).toContain("free-trials-and-promotional-offers");
  });

  it("keeps section and heading ids identical across locales", () => {
    for (const d of ["privacy", "terms"] as const) {
      expect(ids(legalDocuments[d].fr)).toEqual(ids(legalDocuments[d].en));
      expect(ids(legalDocuments[d].es)).toEqual(ids(legalDocuments[d].en));
    }
  });

  it("links only to allowed internal paths", () => {
    for (const d of ["privacy", "terms"] as const)
      for (const l of locales) for (const m of text(legalDocuments[d][l]).matchAll(/\]\(([^)]+)\)/g)) expect(m[1]).toMatch(/^(#[a-z0-9-]+|\/[a-zA-Z0-9/_?=&#.-]*|mailto:[^\s@]+@[^\s@]+)$/);
  });
});
