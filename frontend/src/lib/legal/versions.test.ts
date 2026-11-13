import { describe, expect, it } from "vitest";

import { getLegalDocument } from "@/content/legal";
import { LEGAL_VERSIONS, purchaseConsent } from "@/lib/legal/versions";

describe("LEGAL_VERSIONS", () => {
  it.each(["en", "fr", "es"] as const)("matches the documents' meta.version (%s)", (locale) => {
    expect(LEGAL_VERSIONS.terms).toBe(getLegalDocument("terms", locale).meta.version);
    expect(LEGAL_VERSIONS.privacy).toBe(getLegalDocument("privacy", locale).meta.version);
    expect(LEGAL_VERSIONS.refund).toBe(getLegalDocument("refund", locale).meta.version);
  });

  it("is what a purchase sends as consent", () => {
    expect(purchaseConsent()).toEqual({ terms: LEGAL_VERSIONS.terms, privacy: LEGAL_VERSIONS.privacy, refund: LEGAL_VERSIONS.refund });
  });
});
