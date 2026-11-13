import { describe, expect, it } from "vitest";

import registry from "@/data/r2-images.json";
import { legalDocuments } from "@/content/legal";
import { calculatorArticles } from "@/content/calculators";
import {
  checkArticle,
  checkLegalDocument,
  legalAnchorSet,
  linkTargets,
  numberTokens,
} from "@/content/validate";
import type { LegalDocumentContent } from "@/content/legal/types";
import type { ArticleContent } from "@/content/calculators/types";

const anchors = {
  "/privacy": legalAnchorSet(legalDocuments.privacy.en),
  "/terms": legalAnchorSet(legalDocuments.terms.en),
  "/refund-policy": legalAnchorSet(legalDocuments.refund.en), // revisions 5.0
};
const registeredUrls = new Set(registry.map((entry) => entry.url));
const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value)) as T;

describe("legal documents", () => {
  for (const [name, docs] of Object.entries(legalDocuments)) {
    it(`${name}: every locale mirrors English`, () => {
      expect(checkLegalDocument(name, docs, anchors)).toEqual([]);
    });
  }

  it("cross-document links resolve (Terms → Privacy anchors)", () => {
    const targets = legalDocuments.terms.en.sections
      .flatMap((s) => s.blocks)
      .flatMap((b) => (b.type === "p" ? linkTargets(b.text) : []));
    expect(targets).toContain("/privacy");
    expect(targets.every((href) => !href.startsWith("/psychics"))).toBe(true);
  });

  it("uses the corrected entity name and live URL", () => {
    const text = JSON.stringify(legalDocuments);
    expect(text).not.toContain("Spiritual Staryield");
    expect(text).not.toContain("staryield.com/app/privacy");
    for (const locale of ["en", "es", "fr"] as const)
      expect(JSON.stringify(legalDocuments.privacy[locale])).toContain(
        `[https://staryield.net/${locale}/privacy](/privacy)`,
      );
  });

  it("documents the decided advisor-notes policy in every locale", () => {
    for (const locale of ["en", "es", "fr"] as const) {
      const ids = legalAnchorSet(legalDocuments.privacy[locale]);
      expect(ids.has("advisor-notes")).toBe(true);
      expect(ids.has("cookies")).toBe(true);
    }
    const notes = legalDocuments.privacy.en.sections
      .flatMap((s) => s.blocks)
      .find((b) => b.type === "h3" && b.id === "advisor-notes");
    expect(notes).toBeDefined();
    const english = JSON.stringify(legalDocuments.privacy.en);
    expect(english).toContain("not included in the self-service data export");
    expect(english).toContain("structured, machine-readable format");
  });

  it("flags a stale translation (changed number, missing section, bad link)", () => {
    const docs = clone(legalDocuments.privacy) as Record<string, LegalDocumentContent>;
    const retention = docs.es.sections.find((s) => s.id === "data-retention");
    const block = retention?.blocks.find((b) => b.type === "p" && b.text.includes("180"));
    expect(block).toBeDefined();
    if (block?.type === "p") block.text = block.text.replace(/180/g, "90");
    docs.fr.sections.pop();
    docs.en.intro.push({ type: "p", text: "See [here](https://evil.example)." });
    const issues = checkLegalDocument("privacy", docs, anchors);
    expect(issues.some((i) => i.includes("privacy/es: numbers differ"))).toBe(true);
    expect(issues.some((i) => i.includes("privacy/fr: structure differs"))).toBe(true);
    expect(issues.some((i) => i.includes("disallowed link https://evil.example"))).toBe(true);
  });

  it("flags duplicate ids, broken anchors and version drift", () => {
    const docs = clone(legalDocuments.terms) as Record<string, LegalDocumentContent>;
    docs.en.sections[1].id = docs.en.sections[0].id;
    docs.es.meta.version = "2020-01-01";
    docs.en.intro.push({ type: "p", text: "Jump to [nowhere](#does-not-exist)." });
    docs.fr.intro.push({ type: "p", text: "Voir la [politique](/en/privacy)." });
    const issues = checkLegalDocument("terms", docs, anchors);
    expect(issues.some((i) => i.includes("duplicate id"))).toBe(true);
    expect(issues.some((i) => i.includes("meta.version"))).toBe(true);
    expect(issues.some((i) => i.includes("broken anchor #does-not-exist"))).toBe(true);
    expect(issues.some((i) => i.includes("terms/fr: locale-pinned link /en/privacy"))).toBe(true);
  });
});

describe("calculator articles", () => {
  for (const [kind, articles] of Object.entries(calculatorArticles)) {
    it(`${kind}: every locale mirrors English and uses registered R2 images`, () => {
      expect(checkArticle(kind, articles, registeredUrls)).toEqual([]);
    });
  }

  it("renders the worked example with real symbols, not LaTeX artefacts", () => {
    const text = JSON.stringify(calculatorArticles["life-path"]);
    expect(text).not.toMatch(/\\implies|\$\d\$|=\$/);
    expect(text).toContain("11 + 11 + 1 = 23 ⟹ 2 + 3 = 5");
  });

  it("flags unregistered images and structural drift", () => {
    const articles = clone(calculatorArticles.natal) as Record<string, ArticleContent>;
    const split = articles.en.sections[0].blocks[0];
    if (split.type === "split") split.image.src = "https://example.com/x.png";
    articles.fr.sections[1].blocks.pop();
    const issues = checkArticle("natal", articles, registeredUrls);
    expect(issues.some((i) => i.includes("image outside R2"))).toBe(true);
    expect(issues.some((i) => i.includes("natal/fr: structure differs"))).toBe(true);
  });
});

describe("number parity", () => {
  it("normalises Roman-numeral centuries in Spanish and French", () => {
    expect(numberTokens("late 18th-century")).toEqual(["18"]);
    expect(numberTokens("finales del siglo XVIII")).toEqual(["18"]);
    expect(numberTokens("la fin du XVIIIᵉ siècle")).toEqual(["18"]);
  });

  it("ignores link targets and normalises separators", () => {
    expect(numberTokens("See [Section 5](#your-privacy-rights) within 1,000 days")).toEqual(["1000", "5"]);
  });
});
