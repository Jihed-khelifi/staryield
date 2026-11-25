// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";

import en from "@/i18n/dictionaries/en.json";
import fr from "@/i18n/dictionaries/fr.json";
import copyEn from "@/i18n/dictionaries/copy-en.json";
import copyFr from "@/i18n/dictionaries/copy-fr.json";
import { DEFAULT_SEO_IMAGE } from "@/lib/seo/site";

vi.mock("@/i18n/dictionaries", () => ({
  getDictionary: async (locale: string) => (locale === "fr" ? { ...fr, copy: copyFr } : { ...en, copy: copyEn }),
}));
const fetchAdvisor = vi.hoisted(() => vi.fn());
vi.mock("@/lib/psychics/server", async (importOriginal) => ({ ...(await importOriginal<typeof import("@/lib/psychics/server")>()), fetchAdvisor }));

import ScreenPage, { generateMetadata } from "@/app/[lang]/[...screen]/page";

const params = (lang: string, path: string) => ({ params: Promise.resolve({ lang, screen: path.split("/") }) });
const metadata = (lang: string, path: string) => generateMetadata(params(lang, path));
const NOT_FOUND = { digest: "NEXT_HTTP_ERROR_FALLBACK;404" };

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

  it("describes an advisor profile with what the API confirms: the advisor's name and photo", async () => {
    fetchAdvisor.mockResolvedValue({
      found: true,
      advisor: { slug: "maria-luz", name: "María Luz", headline: "Tarot", image_url: "https://assets.staryield.net/advisors/maria.webp" },
    });
    const meta = await metadata("en", "psychics/maria-luz");
    expect(fetchAdvisor).toHaveBeenCalledWith("maria-luz");
    expect(meta.title).toBe("María Luz");
    expect(meta.description).toContain("Read María Luz’s profile and reviews");
    expect(meta.robots).toBeUndefined();
    expect(meta.alternates?.canonical).toBe("https://staryield.net/en/psychics/maria-luz");
    expect(meta.openGraph).toMatchObject({ images: [{ url: "https://assets.staryield.net/advisors/maria.webp", alt: "María Luz" }] });
    // A photo that is not served over https falls back to the default preview.
    fetchAdvisor.mockResolvedValue({ found: true, advisor: { slug: "theo", name: "Theo", headline: "", image_url: "http://localhost:8085/x.png" } });
    expect((await metadata("en", "psychics/theo")).openGraph).toMatchObject({ images: [{ url: DEFAULT_SEO_IMAGE.url }] });
  });

  it("never turns the words of an unknown advisor URL into a title, description or preview: it is a 404", async () => {
    fetchAdvisor.mockResolvedValue({ found: false });
    await expect(metadata("en", "psychics/staryield-refund-desk-call-1-800-555-0199")).rejects.toMatchObject(NOT_FOUND);
    await expect(ScreenPage(params("en", "psychics/staryield-refund-desk-call-1-800-555-0199"))).rejects.toMatchObject(NOT_FOUND);
  });

  it("keeps a profile out of the index, with a neutral title, while the API cannot answer", async () => {
    fetchAdvisor.mockResolvedValue(null);
    const meta = await metadata("en", "psychics/some-advisor");
    expect(meta).toEqual({ title: "Advisor profile", robots: { index: false, follow: true } });
    // Malformed slugs are never looked up.
    fetchAdvisor.mockClear();
    await expect(ScreenPage(params("en", "psychics/Not_A_Slug"))).rejects.toMatchObject(NOT_FOUND);
    expect(fetchAdvisor).not.toHaveBeenCalled();
  });

  it("treats Object.prototype names as unknown routes (404), not as pages", async () => {
    for (const route of ["constructor", "toString", "__proto__", "hasOwnProperty", "valueOf"]) {
      await expect(ScreenPage(params("en", route)), route).rejects.toMatchObject(NOT_FOUND);
      const meta = await metadata("en", route);
      expect(meta.title, route).toBe("Home");
      expect(meta.description, route).toBeUndefined();
    }
  });
});
