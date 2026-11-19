import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import { buildReportCopy, REPORT_COPY_PATH } from "../../../scripts/gen-report-copy.mjs";
import en from "@/i18n/dictionaries/copy-en.json";
import es from "@/i18n/dictionaries/copy-es.json";
import fr from "@/i18n/dictionaries/copy-fr.json";
import { CARD_BODIES, ELEMENTS, HOUSE_SYSTEM_NAMES, MODALITIES, REPORT_LABELS } from "@/lib/astro/report-labels";

// revisions 6.0 (§26.4): the API prints the calculators' report copy from one JSON
// generated out of these sources (`npm run gen:report-copy`), embedded by the Go
// package backend/internal/mail/reportcopy. The pattern of company.ts/company.go: when the
// sources change and the file is not regenerated, this test fails.

const COMMITTED = new URL(REPORT_COPY_PATH, new URL("../../../scripts/", import.meta.url));

describe("the report copy generated for the API", () => {
  it("is up to date: run `npm run gen:report-copy` and commit the file", async () => {
    const fresh: string = await buildReportCopy();
    expect(readFileSync(COMMITTED, "utf8") === fresh).toBe(true);
  });

  it("has a stable key order and a trailing newline", async () => {
    const json: string = await buildReportCopy();
    expect(json.endsWith("}\n")).toBe(true);
    const parsed = JSON.parse(json);
    expect(Object.keys(parsed)).toEqual([
      "version", "labels", "signs", "tarot", "houses", "life_path", "numbers", "house_systems", "bodies", "elements", "modalities", "translations",
    ]);
    for (const locale of ["fr", "es"]) {
      const keys = Object.keys(parsed.translations[locale]);
      expect(keys).toEqual([...keys].sort());
    }
    expect(Object.keys(parsed.life_path)).toEqual(["1", "2", "3", "4", "5", "6", "7", "8", "9", "11", "22", "33"]);
    expect(parsed.signs).toHaveLength(12);
    expect(parsed.houses).toHaveLength(12);
    expect(Object.keys(parsed.tarot)).toHaveLength(22);
  });

  it("translates every string the report prints into French and Spanish", async () => {
    const parsed = JSON.parse(await buildReportCopy());
    const strings = [
      ...Object.values(REPORT_LABELS), ...Object.values(HOUSE_SYSTEM_NAMES), ...CARD_BODIES, ...ELEMENTS, ...MODALITIES,
    ].map((s) => s.trim().replace(/\s+/g, " "));
    for (const locale of ["fr", "es"] as const) {
      for (const s of strings) {
        expect(parsed.translations[locale][s], `${locale}: ${s}`).toBeTruthy();
        // the generator reads the page's own dictionaries, so the report cannot say something else
        const dictionary = (locale === "fr" ? fr : es) as Record<string, string>;
        const known = Object.keys(dictionary).find((k) => k.trim().replace(/\s+/g, " ").toLowerCase() === s.toLowerCase());
        expect(known && dictionary[known]).toBe(parsed.translations[locale][s]);
      }
    }
  });

  it("only lists labels the page dictionaries know in English too", () => {
    const known = new Set(Object.keys(en).map((k) => k.trim().replace(/\s+/g, " ").toLowerCase()));
    for (const label of Object.values(REPORT_LABELS)) expect(known.has(label.trim().replace(/\s+/g, " ").toLowerCase()), label).toBe(true);
  });
});
