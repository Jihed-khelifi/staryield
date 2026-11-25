// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";

import en from "@/i18n/dictionaries/en.json";
import fr from "@/i18n/dictionaries/fr.json";
import copyEn from "@/i18n/dictionaries/copy-en.json";
import copyFr from "@/i18n/dictionaries/copy-fr.json";

vi.mock("@/i18n/dictionaries", () => ({
  getDictionary: async (locale: string) => (locale === "fr" ? { ...fr, copy: copyFr } : { ...en, copy: copyEn }),
}));

import { generateMetadata } from "@/app/[lang]/[...screen]/page";

const metadata = (lang: string, path: string) => generateMetadata({ params: Promise.resolve({ lang, screen: path.split("/") }) });

// growth (seo): what search engines read on the existing pages.
describe("screen metadata", () => {
  it("gives the calculators keyword titles, descriptions, canonical URLs, alternates and link previews", async () => {
    const meta = await metadata("en", "calculators/path-of-life");
    expect(meta.title).toBe("Life Path Number Calculator");
    expect(meta.description).toMatch(/^Calculate your Life Path number/);
    expect(meta.alternates?.canonical).toBe("https://staryield.net/en/calculators/path-of-life");
    expect(meta.alternates?.languages).toMatchObject({ "x-default": "https://staryield.net/en/calculators/path-of-life" });
    expect(meta.openGraph).toMatchObject({ siteName: "Staryield", url: "https://staryield.net/en/calculators/path-of-life" });
    expect((await metadata("en", "calculators/natal-chart")).title).toBe("Free Natal Chart Calculator");
    expect((await metadata("en", "calculators/astrology")).title).toBe("Zodiac Sign Calculator");
    expect((await metadata("en", "calculators/tarot-birth-card")).title).toBe("Tarot Birth Card Calculator");
  });

  it("translates titles and descriptions", async () => {
    const meta = await metadata("fr", "about");
    expect(meta.title).toBe("À propos de Staryield");
    expect(meta.alternates?.canonical).toBe("https://staryield.net/fr/about");
    expect((meta.openGraph as { locale?: string }).locale).toBe("fr_FR");
  });

  it("keeps private, one-time and utility pages out of the index", async () => {
    expect((await metadata("en", "calculators/path-of-life/results")).robots).toEqual({ index: false, follow: true });
    expect((await metadata("en", "login")).robots).toEqual({ index: false, follow: true });
    expect((await metadata("en", "profile/chart")).robots).toEqual({ index: false, follow: true });
    expect((await metadata("en", "confirm-email")).robots).toEqual({ index: false, follow: false });
    expect((await metadata("en", "confirm-email")).referrer).toBe("no-referrer");
    expect((await metadata("en", "psychics")).robots).toEqual({ index: false });
    expect((await metadata("en", "refunds")).robots).toEqual({ index: false, follow: false });
  });

  it("describes advisor profiles, named after the slug when the advisor is not in the static list", async () => {
    const known = await metadata("en", "psychics/ramone");
    expect(known.title).toBe("Ramone");
    expect(known.description).toContain("Read Ramone’s profile and reviews");
    expect(known.alternates?.canonical).toBe("https://staryield.net/en/psychics/ramone");
    expect((await metadata("en", "psychics/maria-luz")).title).toBe("Maria Luz");
  });
});
