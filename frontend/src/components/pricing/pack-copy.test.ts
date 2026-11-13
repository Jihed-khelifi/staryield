import { describe, expect, it } from "vitest";

import copyEn from "@/i18n/dictionaries/copy-en.json";
import copyFr from "@/i18n/dictionaries/copy-fr.json";
import { createTranslateCopy } from "@/i18n/translate-copy";
import { formatCredits, packEyebrow, packMinutesLine } from "@/components/pricing/pack-copy";

const en = createTranslateCopy(copyEn);
const fr = createTranslateCopy(copyFr);

describe("packEyebrow", () => {
  it("joins the badges with a middle dot, in the order the API sent them", () => {
    expect(packEyebrow({ badges: [{ type: "most_popular" }, { type: "save", percent: 11 }], label: "ignored" }, en)).toBe("MOST POPULAR · SAVE 11%");
    expect(packEyebrow({ badges: [{ type: "best_value" }, { type: "save", percent: 21 }], label: "" }, en)).toBe("BEST VALUE · SAVE 21%");
    expect(packEyebrow({ badges: [{ type: "new_client_offer" }], label: "" }, en)).toBe("NEW CLIENT OFFER");
  });

  it("falls back to the label, then to nothing", () => {
    expect(packEyebrow({ badges: [], label: "One focused question" }, en)).toBe("One focused question");
    expect(packEyebrow({ badges: [], label: "   " }, en)).toBeNull();
    expect(packEyebrow({ badges: [], label: "" }, en)).toBeNull();
  });

  it("is translated", () => {
    expect(packEyebrow({ badges: [{ type: "most_popular" }, { type: "save", percent: 11 }], label: "" }, fr)).toBe("LE PLUS POPULAIRE · ÉCONOMISEZ 11 %");
  });
});

describe("packMinutesLine", () => {
  const range = { min: 30, max: 60 };

  it("shows the minutes at the cheapest and the dearest advisor, truncated to two decimals", () => {
    expect(packMinutesLine(600, range, en, "en")).toBe("≈ 20 min. at 30 credits/min. · ≈ 10 min. at 60 credits/min.");
    expect(packMinutesLine(315, range, en, "en")).toBe("≈ 10.5 min. at 30 credits/min. · ≈ 5.25 min. at 60 credits/min.");
    expect(packMinutesLine(400, range, en, "en")).toBe("≈ 13.33 min. at 30 credits/min. · ≈ 6.66 min. at 60 credits/min.");
  });

  it("uses the live rate range and the locale's decimal separator", () => {
    expect(packMinutesLine(315, { min: 20, max: 40 }, en, "en")).toBe("≈ 15.75 min. at 20 credits/min. · ≈ 7.87 min. at 40 credits/min.");
    expect(packMinutesLine(315, range, fr, "fr")).toBe("≈ 10,5 min à 30 crédits/min · ≈ 5,25 min à 60 crédits/min");
  });
});

describe("formatCredits", () => {
  it("groups thousands for the locale", () => {
    expect(formatCredits(1350, "en")).toBe("1,350");
    expect(formatCredits(2400, "fr").replace(/\s/g, " ")).toBe("2 400");
  });
});
