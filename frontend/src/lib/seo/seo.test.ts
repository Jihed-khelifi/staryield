import { describe, expect, it } from "vitest";

import copyEn from "@/i18n/dictionaries/copy-en.json";
import copyEs from "@/i18n/dictionaries/copy-es.json";
import copyFr from "@/i18n/dictionaries/copy-fr.json";
import { FAMILY_COPY } from "@/components/seo/copy";
import { buildRobots, DISALLOWED_PATHS } from "@/lib/seo/robots";
import { ADVISOR_SITEMAP_PATH, buildAdvisorSitemap, buildSitemap, sitemapEntries } from "@/lib/seo/sitemap";
import { screenJsonLd } from "@/lib/seo/screen-json-ld";
import { createTranslateCopy } from "@/i18n/translate-copy";
import { LIVE_READINGS_FAQ } from "@/content/faq/live-readings";
import { breadcrumbJsonLd, faqJsonLd, organizationJsonLd, plainText, serializeJsonLd, webSiteJsonLd } from "@/lib/seo/jsonld";
import { ADVISOR_DESCRIPTION, AUTH_ROUTES, CALCULATOR_ROUTES, DEFAULT_IMAGE_ALT, HOME_SEO, PAGE_SEO } from "@/lib/seo/pages";
import { DEFAULT_SEO_IMAGE, SITE_URL, SKY_SEO_IMAGE, languageAlternates, localeUrl, pageAlternates, pageMetadata, tarotSeoImage, twitterCardFor } from "@/lib/seo/site";
import { calculatorResultHref } from "@/lib/seo/calculator-links";
import { HUB_LABELS, HUB_MORE } from "@/content/seo/hubs";
import { SIGNS, TAROT_CARDS } from "@/lib/astro/zodiac";

describe("site URLs", () => {
  it("builds absolute locale URLs, alternates and x-default", () => {
    expect(SITE_URL).toBe("https://staryield.net");
    expect(localeUrl("", "fr")).toBe("https://staryield.net/fr");
    expect(localeUrl("/about", "es")).toBe("https://staryield.net/es/about");
    expect(languageAlternates("/compatibility/aries-and-leo")).toEqual({
      en: "https://staryield.net/en/compatibility/aries-and-leo",
      fr: "https://staryield.net/fr/compatibility/aries-and-leo",
      es: "https://staryield.net/es/compatibility/aries-and-leo",
      "x-default": "https://staryield.net/en/compatibility/aries-and-leo",
    });
    expect(pageAlternates("/faq", "es").canonical).toBe("https://staryield.net/es/faq");
  });

  it("describes a page: canonical, hreflang, Open Graph and Twitter card", () => {
    const metadata = pageMetadata({ locale: "fr", route: "/life-path-number/7", title: "Chemin de vie 7", description: "Une description.", imageAlt: "Alt" });
    expect(metadata.title).toBe("Chemin de vie 7");
    expect(metadata.alternates?.canonical).toBe("https://staryield.net/fr/life-path-number/7");
    expect(Object.keys(metadata.alternates?.languages ?? {})).toEqual(["en", "fr", "es", "x-default"]);
    expect(metadata.openGraph).toMatchObject({
      type: "website",
      siteName: "Staryield",
      locale: "fr_FR",
      alternateLocale: ["en_US", "es_ES"],
      title: "Chemin de vie 7 — Staryield",
      url: "https://staryield.net/fr/life-path-number/7",
      images: [{ url: DEFAULT_SEO_IMAGE.url, width: 640, height: 427, alt: "Alt" }],
    });
    expect(metadata.twitter).toMatchObject({ card: "summary_large_image", title: "Chemin de vie 7 — Staryield" });
    // A picture narrower than X's large card (the 210 px tarot card) gets the small "summary" card.
    expect(twitterCardFor(tarotSeoImage("the-star"))).toBe("summary");
    expect(twitterCardFor(SKY_SEO_IMAGE)).toBe("summary_large_image");
    const card = pageMetadata({ locale: "en", route: "/tarot-birth-card/the-star", title: "t", description: "d", imageAlt: "a", image: tarotSeoImage("the-star") });
    expect(card.twitter).toMatchObject({ card: "summary" });
    // An image without known dimensions (an advisor's photo) is described without them.
    const photo = pageMetadata({ locale: "en", route: "/psychics/theo", title: "Theo", description: "d", imageAlt: "Theo", image: { url: "https://assets.staryield.net/p/theo.webp" } });
    expect(photo.openGraph?.images).toEqual([{ url: "https://assets.staryield.net/p/theo.webp", alt: "Theo" }]);
    expect(photo.twitter).toMatchObject({ card: "summary_large_image" });
    expect(pageMetadata({ locale: "en", route: "", title: "Home title", description: "d", imageAlt: "a", absoluteTitle: true }).title).toEqual({ absolute: "Home title" });
  });

  it("opens the calculators with the date in the fragment", () => {
    expect(calculatorResultHref("en", "path-of-life", "1981-06-18")).toBe("/en/calculators/path-of-life#v=1&d=1981-06-18");
    expect(calculatorResultHref("es", "tarot-birth-card", "1959-01-10")).toBe("/es/calculators/tarot-birth-card#v=1&d=1959-01-10");
  });
});

describe("sitemap", () => {
  const sitemap = buildSitemap();

  it("lists every indexable page in the three locales with absolute URLs and alternates", () => {
    const entries = sitemapEntries();
    // Home, the static pages, 4 calculators, 3 hubs, 12 numbers, 22 cards and 78 pairs.
    expect(entries).toHaveLength(1 + Object.keys(PAGE_SEO).length + 3 + 12 + 22 + 78);
    expect(sitemap).toHaveLength(entries.length * 3);
    expect(new Set(sitemap.map((entry) => entry.url)).size).toBe(sitemap.length);
    for (const entry of sitemap) {
      expect(entry.url).toMatch(/^https:\/\/staryield\.net\/(en|fr|es)(\/|$)/);
      expect(Object.keys(entry.alternates?.languages ?? {})).toEqual(["en", "fr", "es", "x-default"]);
      for (const href of Object.values(entry.alternates?.languages ?? {})) expect(href).toMatch(/^https:\/\/staryield\.net\//);
      expect(String(entry.lastModified)).toMatch(/^\d{4}-\d{2}-\d{2}$/);
      expect(entry.priority).toBeGreaterThan(0);
    }
  });

  it("includes the calculators (tarot birth card too) and the programmatic pages, never private pages", () => {
    const urls = new Set(sitemap.map((entry) => entry.url));
    for (const route of CALCULATOR_ROUTES) expect(urls.has(`https://staryield.net/en/${route}`)).toBe(true);
    for (const path of ["/en/life-path-number", "/fr/life-path-number/33", "/es/tarot-birth-card/the-star", "/en/compatibility", "/en/compatibility/aries-and-leo", "/fr/compatibility/pisces-and-pisces"])
      expect(urls.has(`https://staryield.net${path}`), path).toBe(true);
    for (const url of urls) {
      expect(url).not.toMatch(/\/(profile|chatroom|credits|signup|login|psychics|support|unsubscribe|confirm-email)(\/|$)/);
      expect(url).not.toMatch(/leo-and-aries|\/results$/);
    }
    for (const route of AUTH_ROUTES) expect(urls.has(`https://staryield.net/en/${route}`)).toBe(false);
  });

  it("dates entries from the content, not from the request", () => {
    expect(buildSitemap()).toEqual(sitemap);
  });
});

describe("robots.txt", () => {
  it("allows public pages, blocks private and utility paths, and points to the sitemap", () => {
    const robots = buildRobots();
    expect(robots.sitemap).toEqual(["https://staryield.net/sitemap.xml", `https://staryield.net${ADVISOR_SITEMAP_PATH}`]);
    const rules = Array.isArray(robots.rules) ? robots.rules[0] : robots.rules;
    expect(rules.userAgent).toBe("*");
    expect(rules.allow).toBe("/");
    expect(rules.disallow).toEqual([...DISALLOWED_PATHS]);
    for (const path of ["/*/profile", "/*/chatroom", "/*/credits", "/*/signup/", "/go/", "/*/verify-email", "/*/unsubscribe", "/*/support/", "/*/confirm-email"])
      expect(DISALLOWED_PATHS).toContain(path);
    // Nothing indexable is blocked.
    const blocked = (path: string) =>
      DISALLOWED_PATHS.some((rule) => new RegExp(`^${rule.replace(/[.+?^${}()|[\]\\]/g, "\\$&").replace(/\*/g, "[^/]+")}`).test(path));
    for (const entry of sitemapEntries()) expect(blocked(`/en${entry.route}`), entry.route).toBe(false);
    expect(blocked("/en/psychics/theo")).toBe(false);
    expect(blocked(ADVISOR_SITEMAP_PATH)).toBe(false);
    expect(blocked("/en/profile/chart")).toBe(true);
    expect(blocked("/fr/signup/password")).toBe(true);
  });
});

describe("JSON-LD", () => {
  it("serialises valid JSON that cannot close its script tag", () => {
    const separator = String.fromCharCode(0x2028);
    const data = faqJsonLd([{ q: "</script><script>alert(1)</script>", a: `A & B${separator}C` }]);
    const json = serializeJsonLd(data);
    expect(json).not.toContain("<");
    expect(json).not.toContain(">");
    expect(json).not.toContain("&");
    expect(json).not.toContain(separator);
    expect(JSON.parse(json)).toEqual(data);
  });

  it("builds the breadcrumbs with absolute URLs and positions", () => {
    expect(breadcrumbJsonLd("es", [{ name: "Inicio", route: "" }, { name: "Caminos de vida", route: "/life-path-number" }])).toEqual({
      "@context": "https://schema.org",
      "@type": "BreadcrumbList",
      itemListElement: [
        { "@type": "ListItem", position: 1, name: "Inicio", item: "https://staryield.net/es" },
        { "@type": "ListItem", position: 2, name: "Caminos de vida", item: "https://staryield.net/es/life-path-number" },
      ],
    });
  });

  it("turns content markup into plain text for FAQ answers", () => {
    expect(plainText("See **this** [page](/faq).")).toBe("See this page.");
    expect(faqJsonLd([{ q: "Q?", a: "**Yes**, [read](/about)." }]).mainEntity).toEqual([
      { "@type": "Question", name: "Q?", acceptedAnswer: { "@type": "Answer", text: "Yes, read." } },
    ]);
  });

  it("describes the organisation and the site with what the footer shows", () => {
    const organization = organizationJsonLd();
    expect(organization).toMatchObject({ "@type": "Organization", name: "Staryield", legalName: "Staryield Media LLC", url: "https://staryield.net" });
    expect(organization).not.toHaveProperty("aggregateRating");
    expect((organization.sameAs as string[]).every((url) => url.startsWith("https://"))).toBe(true);
    expect(webSiteJsonLd("fr", "Description")).toMatchObject({ "@type": "WebSite", url: "https://staryield.net/fr", inLanguage: "fr" });
  });
});

describe("page copy", () => {
  const dictionaries = { en: copyEn, fr: copyFr, es: copyEs } as Record<string, Record<string, string>>;
  const has = (copy: Record<string, string>, key: string) => Object.keys(copy).some((k) => k.toLowerCase() === key.toLowerCase());
  const strings = [
    HOME_SEO.title,
    HOME_SEO.description,
    DEFAULT_IMAGE_ALT,
    ADVISOR_DESCRIPTION,
    ...Object.values(PAGE_SEO).flatMap((seo) => [seo.title, seo.description]),
    ...Object.values(FAMILY_COPY).flatMap((family) => [family.cta.heading, family.cta.body, family.note, family.calculator.heading, family.calculator.intro]),
    // Hub labels (footer "Explore" group and the hubs' related links), the advisor fallbacks.
    ...Object.values(HUB_LABELS),
    HUB_MORE,
    "Advisor profile",
    // Names the search pages take from the dictionaries.
    ...SIGNS,
    ...Object.values(TAROT_CARDS),
    "Home",
  ];

  it("has every title, description and interface string in English, French and Spanish", () => {
    for (const [locale, copy] of Object.entries(dictionaries))
      for (const value of strings) expect(has(copy, value), `${locale}: ${value}`).toBe(true);
  });

  it("gives every indexable page a unique title and description", () => {
    const titles = Object.values(PAGE_SEO).map((seo) => seo.title);
    const descriptions = Object.values(PAGE_SEO).map((seo) => seo.description);
    expect(new Set(titles).size).toBe(titles.length);
    expect(new Set(descriptions).size).toBe(descriptions.length);
    for (const seo of Object.values(PAGE_SEO)) expect(seo.updated).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });
});

describe("advisors' sitemap", () => {
  it("lists every advisor profile in the three locales with alternates and no invented date", () => {
    const sitemap = buildAdvisorSitemap(["ramone", "theo"]);
    expect(sitemap.map((entry) => entry.url)).toEqual([
      "https://staryield.net/en/psychics/ramone",
      "https://staryield.net/fr/psychics/ramone",
      "https://staryield.net/es/psychics/ramone",
      "https://staryield.net/en/psychics/theo",
      "https://staryield.net/fr/psychics/theo",
      "https://staryield.net/es/psychics/theo",
    ]);
    expect(sitemap[1].alternates?.languages).toEqual(languageAlternates("/psychics/ramone"));
    expect(sitemap.every((entry) => entry.lastModified === undefined)).toBe(true);
    expect(buildAdvisorSitemap([])).toEqual([]);
  });
});

describe("structured data of the existing pages", () => {
  const text = createTranslateCopy(copyFr);

  it("describes a calculator's visible FAQ, without breadcrumbs the page does not show", () => {
    const data = screenJsonLd("calculators/path-of-life", "fr", text);
    expect(data.map((item) => item["@type"])).toEqual(["FAQPage"]);
  });

  it("describes the FAQ page's answered questions, translated", () => {
    const [faq] = screenJsonLd("faq", "fr", text) as { "@type": string; mainEntity: { name: string }[] }[];
    expect(faq["@type"]).toBe("FAQPage");
    expect(faq.mainEntity.map((item) => item.name)).toEqual(LIVE_READINGS_FAQ.map((item) => (copyFr as Record<string, string>)[item.q]));
  });

  it("gives breadcrumbs only to the legal pages, which show a trail", () => {
    // The refund policy's layout shows "Home / Refund policy"; the other pages show no trail.
    const [crumbs] = screenJsonLd("refund-policy", "fr", text) as { "@type": string; itemListElement: { item: string }[] }[];
    expect(crumbs["@type"]).toBe("BreadcrumbList");
    expect(crumbs.itemListElement.map((item) => item.item)).toEqual(["https://staryield.net/fr", "https://staryield.net/fr/refund-policy"]);
    for (const route of ["about", "pricing", "contact", "reviews", "privacy", "terms", "calculators/natal-chart", "constructor", "__proto__"])
      expect(screenJsonLd(route, "fr", text).some((item) => item["@type"] === "BreadcrumbList"), route).toBe(false);
  });
});
