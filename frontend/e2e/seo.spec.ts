/**
 * growth (seo): technical SEO and the programmatic search pages, against the
 * real stack: robots.txt and the sitemap, canonical / hreflang / Open Graph /
 * JSON-LD on a hub and a detail page of each family, the reversed-pair
 * redirect, the embedded calculators (birth date → calculator with the date in
 * the fragment; two dates → sun signs from the API → the pair page), cusp
 * birthdays, and phone layouts without horizontal scrolling (@responsive).
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { expectNoHorizontalScroll, quietReportPrompt } from "./helpers";

const SITE = "https://staryield.net";

test.beforeEach(({ page }) => quietReportPrompt(page));

/** The head tags a search engine reads, and the parsed JSON-LD. */
async function headOf(page: Page) {
  return page.evaluate(() => {
    const attr = (selector: string, name: string) => document.head.querySelector(selector)?.getAttribute(name) ?? null;
    return {
      title: document.title,
      description: attr('meta[name="description"]', "content"),
      canonical: attr('link[rel="canonical"]', "href"),
      robots: attr('meta[name="robots"]', "content"),
      alternates: Object.fromEntries(
        [...document.head.querySelectorAll('link[rel="alternate"][hreflang]')].map((link) => [link.getAttribute("hreflang"), link.getAttribute("href")]),
      ),
      ogTitle: attr('meta[property="og:title"]', "content"),
      ogUrl: attr('meta[property="og:url"]', "content"),
      ogImage: attr('meta[property="og:image"]', "content"),
      ogLocale: attr('meta[property="og:locale"]', "content"),
      twitterCard: attr('meta[name="twitter:card"]', "content"),
      jsonLd: [...document.querySelectorAll('script[type="application/ld+json"]')].flatMap((script) => JSON.parse(script.textContent ?? "null")),
    };
  });
}

function expectAlternates(alternates: Record<string, string | null>, route: string) {
  expect(alternates).toEqual({
    en: `${SITE}/en${route}`,
    fr: `${SITE}/fr${route}`,
    es: `${SITE}/es${route}`,
    "x-default": `${SITE}/en${route}`,
  });
}

async function fillDates(page: Page, scope: string, dates: [string, string, string][]) {
  const groups = page.getByTestId(scope).locator("[data-slot=date-parts-input]");
  for (const [index, [day, month, year]] of dates.entries()) {
    const group = groups.nth(index);
    await group.locator("input").nth(0).fill(day);
    await group.locator("input").nth(1).fill(month);
    await group.locator("input").nth(2).fill(year);
  }
}

test.describe("crawler files", () => {
  test("robots.txt allows public pages, blocks private paths and points to the sitemap", async ({ request }) => {
    const response = await request.get("/robots.txt");
    expect(response.status()).toBe(200);
    expect(response.headers()["content-type"]).toContain("text/plain");
    const body = await response.text();
    expect(body).toContain("User-Agent: *");
    expect(body).toContain("Allow: /");
    for (const path of ["/*/profile", "/*/chatroom", "/*/credits", "/*/signup/", "/go/", "/*/unsubscribe", "/*/verify-email", "/*/support/"])
      expect(body).toContain(`Disallow: ${path}`);
    expect(body).toContain(`Sitemap: ${SITE}/sitemap.xml`);
  });

  test("sitemap.xml lists every indexable page in three locales with hreflang alternates", async ({ request }) => {
    const response = await request.get("/sitemap.xml");
    expect(response.status()).toBe(200);
    expect(response.headers()["content-type"]).toContain("xml");
    const xml = await response.text();
    const locs = [...xml.matchAll(/<loc>([^<]+)<\/loc>/g)].map((match) => match[1]);
    expect(locs.length).toBe(390);
    expect(new Set(locs).size).toBe(locs.length);
    for (const url of [
      `${SITE}/en`,
      `${SITE}/fr/calculators/path-of-life`,
      `${SITE}/es/calculators/tarot-birth-card`,
      `${SITE}/en/life-path-number/22`,
      `${SITE}/fr/tarot-birth-card/the-star`,
      `${SITE}/es/compatibility/aries-and-leo`,
    ])
      expect(locs).toContain(url);
    expect(locs.some((url) => /\/(profile|login|signup|chatroom)(\/|$)/.test(url))).toBe(false);
    expect(xml).toContain(`<xhtml:link rel="alternate" hreflang="x-default" href="${SITE}/en/compatibility/aries-and-leo" />`);
    expect(xml).toMatch(/<lastmod>\d{4}-\d{2}-\d{2}<\/lastmod>/);
  });
});

const PAGES = [
  { path: "/en/life-path-number", route: "/life-path-number", h1: "Life Path Numbers and Their Meanings", locale: "en_US" },
  { path: "/fr/life-path-number/7", route: "/life-path-number/7", h1: "Chemin de vie 7 : Le Chercheur de vérité", locale: "fr_FR" },
  { path: "/es/tarot-birth-card", route: "/tarot-birth-card", h1: "Las cartas de nacimiento del tarot y su significado", locale: "es_ES" },
  { path: "/en/tarot-birth-card/the-star", route: "/tarot-birth-card/the-star", h1: "The Star as a Tarot Birth Card", locale: "en_US" },
  { path: "/fr/compatibility", route: "/compatibility", h1: "Compatibilité des signes du zodiaque", locale: "fr_FR" },
  { path: "/en/compatibility/aries-and-leo", route: "/compatibility/aries-and-leo", h1: "Aries and Leo Compatibility", locale: "en_US" },
];

test.describe("search pages", { tag: "@responsive" }, () => {
  for (const item of PAGES) {
    test(`${item.path}: H1, canonical, hreflang, link preview, JSON-LD and layout`, async ({ page }) => {
      const response = await page.goto(item.path);
      expect(response?.status()).toBe(200);
      await expect(page.getByRole("heading", { level: 1 })).toHaveText(item.h1);
      await expect(page.getByTestId("seo-answer")).not.toBeEmpty();
      const head = await headOf(page);
      expect(head.canonical).toBe(`${SITE}${item.path}`);
      expectAlternates(head.alternates, item.route);
      expect(head.description?.length).toBeGreaterThan(90);
      expect(head.title).toMatch(/ — Staryield$/);
      expect(head.ogUrl).toBe(`${SITE}${item.path}`);
      expect(head.ogLocale).toBe(item.locale);
      expect(head.ogImage).toMatch(/^https:\/\/assets\.staryield\.net\/assets\//);
      expect(head.twitterCard).toBe("summary_large_image");
      expect(head.robots).toBeNull();
      const types = head.jsonLd.map((data: { "@type": string }) => data["@type"]);
      expect(types).toEqual(["BreadcrumbList", "FAQPage"]);
      const breadcrumbs = head.jsonLd[0] as { itemListElement: { item: string }[] };
      expect(breadcrumbs.itemListElement.at(-1)?.item).toBe(`${SITE}${item.path}`);
      // The FAQ the structured data describes is on the page.
      const faq = head.jsonLd[1] as { mainEntity: { name: string }[] };
      for (const question of faq.mainEntity) await expect(page.getByTestId("seo-faq").getByRole("heading", { name: question.name })).toBeVisible();
      await expect(page.getByTestId("seo-calculator")).toBeVisible();
      await expect(page.getByTestId("seo-entertainment-note")).toBeVisible();
      await expectNoHorizontalScroll(page);
    });
  }

  test("the pages pass axe (detail page with its calculator)", async ({ page }) => {
    await page.goto("/en/compatibility/aries-and-leo");
    await expectNoA11yViolations(page, ['[data-testid="seo-page"]']);
    await page.goto("/en/tarot-birth-card/the-star");
    await expectNoA11yViolations(page, ['[data-testid="seo-page"]']);
  });

  test("unknown numbers, cards and pairs are 404s", async ({ request }) => {
    for (const path of ["/en/life-path-number/13", "/en/tarot-birth-card/the-joker", "/en/compatibility/aries-and-ophiuchus"])
      expect((await request.get(path)).status(), path).toBe(404);
  });
});

test.describe("redirects", () => {
  test("a reversed pair goes permanently to the canonical page, in one hop", async ({ page, request }) => {
    const reversed = await request.get("/fr/compatibility/leo-and-aries", { maxRedirects: 0 });
    expect(reversed.status()).toBe(308);
    expect(new URL(reversed.headers().location, "http://localhost").pathname).toBe("/fr/compatibility/aries-and-leo");
    const bare = await request.get("/compatibility/pisces-and-cancer", { maxRedirects: 0, headers: { "accept-language": "es" } });
    expect(bare.status()).toBe(308);
    expect(new URL(bare.headers().location, "http://localhost").pathname).toBe("/es/compatibility/cancer-and-pisces");
    await page.goto("/en/compatibility/leo-and-aries");
    await expect(page).toHaveURL(/\/en\/compatibility\/aries-and-leo$/);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Aries and Leo Compatibility");
  });
});

test.describe("embedded calculators", { tag: "@responsive" }, () => {
  test("the Life Path page's form validates, then opens the calculator with the date in the fragment", async ({ page }) => {
    await page.goto("/en/life-path-number/7");
    const form = page.getByTestId("seo-birth-form");
    await form.scrollIntoViewIfNeeded();
    await page.getByTestId("seo-birth-submit").click();
    await expect(form.getByText("Enter your date of birth.")).toBeVisible();
    await expect(form.locator('input[name="birth.day"]')).toHaveAttribute("aria-invalid", "true");
    await fillDates(page, "seo-birth-form", [["18", "06", "1981"]]);
    await page.getByTestId("seo-birth-submit").click();
    await page.waitForURL(/\/en\/calculators\/path-of-life#v=1&d=1981-06-18$/);
    await expect(page.getByTestId("number-lifePath")).toHaveText("7");
    await expect(page.getByTestId("life-path-title")).toHaveText("Life path 7");
  });

  test("the tarot page's form opens the tarot birth card calculator with the date in the fragment", async ({ page }) => {
    await page.goto("/fr/tarot-birth-card/the-star");
    await fillDates(page, "seo-birth-form", [["10", "01", "1959"]]);
    await page.getByTestId("seo-birth-submit").click();
    await page.waitForURL(/\/fr\/calculators\/tarot-birth-card#v=1&d=1959-01-10$/);
  });

  test("the compatibility calculator finds both signs and opens the pair, or confirms it in place", async ({ page }) => {
    const signRequests: string[] = [];
    page.on("request", (request) => {
      if (request.url().endsWith("/api/v1/astrology/sun-sign")) signRequests.push(request.postData() ?? "");
    });
    await page.goto("/en/compatibility");
    await page.getByTestId("compatibility-submit").click();
    await expect(page.getByTestId("compatibility-form").getByText("Enter your date of birth.")).toHaveCount(2);
    // Leo first, Aries second: the canonical page is still aries-and-leo.
    await fillDates(page, "compatibility-form", [["05", "08", "1988"], ["10", "04", "1990"]]);
    await page.getByTestId("compatibility-submit").click();
    await page.waitForURL(/\/en\/compatibility\/aries-and-leo$/);
    expect(signRequests.map((body) => JSON.parse(body).birth_date).sort()).toEqual(["1988-08-05", "1990-04-10"]);

    // Already on the pair's page: confirmed in place.
    await fillDates(page, "compatibility-form", [["10", "04", "1990"], ["05", "08", "1988"]]);
    await page.getByTestId("compatibility-submit").click();
    await expect(page.getByTestId("compatibility-here")).toContainText("Your signs are Aries and Leo: this is your pairing.");
    await expect(page).toHaveURL(/\/en\/compatibility\/aries-and-leo$/);

    // A cusp birthday (20 April 1990): both possible signs, a link to settle it, then the chosen pair.
    await fillDates(page, "compatibility-form", [["20", "04", "1990"], ["05", "08", "1988"]]);
    await page.getByTestId("compatibility-submit").click();
    const cusp = page.getByTestId("compatibility-cusp");
    await expect(cusp.getByRole("radio", { name: "Aries" })).toBeVisible();
    await expect(cusp.getByRole("radio", { name: "Taurus" })).toBeVisible();
    await expect(cusp.getByRole("link", { name: /Add a birth time/ })).toHaveAttribute("href", "/en/calculators/astrology#v=1&d=1990-04-20");
    await cusp.getByRole("radio", { name: "Taurus" }).check();
    await page.getByTestId("compatibility-continue").click();
    await page.waitForURL(/\/en\/compatibility\/taurus-and-leo$/);
    await expectNoHorizontalScroll(page);
  });
});

test.describe("site pages", () => {
  test("the home page describes the organisation and the site", async ({ page }) => {
    await page.goto("/fr");
    const head = await headOf(page);
    expect(head.canonical).toBe(`${SITE}/fr`);
    expectAlternates(head.alternates, "");
    expect(head.title).toBe("Staryield — Calculateurs d’astrologie gratuits et consultations de voyance privées");
    expect(head.jsonLd.map((data: { "@type": string }) => data["@type"])).toEqual(["Organization", "WebSite"]);
  });

  test("a calculator has a keyword title, a canonical URL, breadcrumbs and its FAQ as structured data", async ({ page }) => {
    await page.goto("/en/calculators/path-of-life");
    const head = await headOf(page);
    expect(head.title).toBe("Life Path Number Calculator — Staryield");
    expect(head.canonical).toBe(`${SITE}/en/calculators/path-of-life`);
    expectAlternates(head.alternates, "/calculators/path-of-life");
    expect(head.jsonLd.map((data: { "@type": string }) => data["@type"])).toEqual(["BreadcrumbList", "FAQPage"]);
    // The answers are in the page (hidden until opened), so crawlers read them.
    const faq = head.jsonLd[1] as { mainEntity: { acceptedAnswer: { text: string } }[] };
    expect(await page.getByTestId("article-faq").textContent()).toContain(faq.mainEntity[0].acceptedAnswer.text.slice(0, 40));
    // Its article links to the Life Path hub.
    await expect(page.locator('[data-calculator-article] a[href="/en/life-path-number"]')).toHaveCount(1);
  });

  test("private and one-time pages stay out of the index", async ({ page }) => {
    await page.goto("/en/login");
    expect((await headOf(page)).robots).toBe("noindex, follow");
    await page.goto("/en/unsubscribe");
    expect((await headOf(page)).robots).toBe("noindex, nofollow");
  });

  test("the footer links the three hubs", async ({ page }) => {
    await page.goto("/es/about");
    const explore = page.getByRole("navigation", { name: "Explorar" });
    await expect(explore.getByRole("link")).toHaveText(["Caminos de vida", "Cartas de nacimiento del tarot", "Compatibilidad de signos"]);
    await explore.getByRole("link", { name: "Compatibilidad de signos" }).click();
    await expect(page).toHaveURL(/\/es\/compatibility$/);
  });
});
