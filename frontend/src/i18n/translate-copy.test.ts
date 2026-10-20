import { describe, expect, it } from "vitest";

import { createTranslateCopy } from "@/i18n/translate-copy";

const text = createTranslateCopy({ "Hello {value0}": "Bonjour {value0}", Support: "Assistance" });

describe("createTranslateCopy", () => {
  it("translates and interpolates known copy", () => {
    expect(text("Hello {value0}", { value0: "Luna" })).toBe("Bonjour Luna");
    expect(text("SUPPORT")).toBe("ASSISTANCE");
  });

  it("still interpolates unknown copy instead of leaking placeholders", () => {
    expect(text("{value0} credits", { value0: 40 })).toBe("40 credits");
    expect(text("Untranslated")).toBe("Untranslated");
  });

  it("passes non-strings through", () => {
    expect(text(320)).toBe(320);
  });
});
