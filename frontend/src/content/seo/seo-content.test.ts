import { describe, expect, it } from "vitest";

import { locales, type Locale } from "@/i18n/config";
import { lifePath, tarotBirthCards, validBirthDate } from "@/lib/calculators";
import { SIGNS } from "@/lib/astro/zodiac";
import { birthCardsOf, birthNumberOfSlug, constellationOf } from "@/lib/astro/tarot-birth-card";
import registry from "@/data/r2-images.json";
import { LIFE_PATH_COMPATIBLE, LIFE_PATH_EXAMPLE_DATE, LIFE_PATH_NUMBERS, lifePathNeighbours, parseLifePathSlug } from "@/content/seo/life-path/data";
import { lifePathHubPage, lifePathPage, lifePathWorkedExample } from "@/content/seo/life-path";
import { GOLDEN_DAWN, TAROT_EXAMPLE_DATE, TAROT_SLUGS, tarotCardFacts } from "@/content/seo/tarot/data";
import { tarotHubPage, tarotPage } from "@/content/seo/tarot";
import {
  ALL_PAIRS,
  ASPECTS,
  PAIR_SLUGS,
  PAIR_SLUGS_BOTH_ORDERS,
  aspectBetween,
  canonicalPairPath,
  indicatorsFor,
  pairSlug,
  parsePairSlug,
  relatedPairs,
} from "@/content/seo/compatibility/data";
import { compatibilityCopy, compatibilityHubPage, compatibilityPage } from "@/content/seo/compatibility";
import { CARD_NAMES, SIGN_NAMES } from "@/content/seo/names";
import type { SeoBlock, SeoPage } from "@/content/seo/types";
import { sitemapEntries } from "@/lib/seo/sitemap";

const registered = new Set(registry.map((entry) => entry.url));

/** Every visible string of a page, in reading order. */
function pageText(page: SeoPage): string[] {
  const block = (b: SeoBlock): string[] => {
    switch (b.type) {
      case "p":
        return [b.text];
      case "list":
        return b.items;
      case "facts":
        return b.items.flatMap((item) => [item.label, item.value]);
      case "links":
        return b.items.flatMap((item) => [item.label, item.note ?? ""]);
      case "indicators":
        return [...b.items.flatMap((item) => [item.label, item.valueText]), b.note];
    }
  };
  return [
    page.title,
    page.description,
    page.eyebrow,
    page.h1,
    page.intro,
    ...page.sections.flatMap((section) => [section.heading, ...section.blocks.flatMap(block)]),
    ...(page.grid ? [page.grid.heading, ...(page.grid.items ?? []).flatMap((item) => [item.title, item.subtitle])] : []),
    ...page.faq.flatMap((item) => [item.q, item.a]),
    page.related.heading,
    ...page.related.links.map((link) => link.label),
    ...page.breadcrumbs.map((crumb) => crumb.name),
  ];
}

/** Every internal route a page links to. */
function pageLinks(page: SeoPage): string[] {
  return [
    ...page.sections.flatMap((section) => section.blocks.flatMap((b) => (b.type === "links" ? b.items.map((item) => item.href) : []))),
    ...(page.grid?.items ?? []).map((item) => item.href),
    ...(page.grid?.groups ?? []).flatMap((group) => group.links.map((link) => link.href)),
    ...page.related.links.map((link) => link.href),
    ...page.breadcrumbs.map((crumb) => crumb.route),
  ];
}

const indexable = new Set(sitemapEntries().map((entry) => entry.route));

/** Shared quality bar for every programmatic page in every locale. */
function expectWellFormed(page: SeoPage, locale: Locale) {
  const text = pageText(page);
  for (const value of text) {
    expect(value, `${page.route} (${locale})`).not.toMatch(/undefined|NaN|\[object|\{value\d\}|\$\{| {2}|^\s|\s$/);
  }
  expect(page.title.length, `${page.route} title`).toBeLessThanOrEqual(75);
  expect(page.description.length, `${page.route} description`).toBeGreaterThanOrEqual(90);
  expect(page.description.length, `${page.route} description`).toBeLessThanOrEqual(260);
  expect(page.intro.length, `${page.route} direct answer`).toBeGreaterThanOrEqual(100);
  expect(page.faq.length).toBeGreaterThanOrEqual(3);
  expect(page.faq.length).toBeLessThanOrEqual(5);
  for (const item of page.faq) {
    expect(item.q.length).toBeGreaterThan(10);
    expect(item.a.length).toBeGreaterThanOrEqual(60);
  }
  expect(new Set(page.faq.map((item) => item.q)).size).toBe(page.faq.length);
  for (const section of page.sections) {
    expect(section.heading.length).toBeGreaterThan(3);
    expect(section.blocks.length).toBeGreaterThan(0);
  }
  expect(page.breadcrumbs[0].route).toBe("");
  expect(page.breadcrumbs.at(-1)?.route).toBe(page.route);
  for (const href of pageLinks(page)) {
    expect(href === "" || indexable.has(href), `${page.route} links to ${href}`).toBe(true);
  }
  for (const art of page.artwork ?? []) expect(registered.has(art.src), art.src).toBe(true);
  expect(registered.has(page.image.url), page.image.url).toBe(true);
  expect(page.updated).toMatch(/^\d{4}-\d{2}-\d{2}$/);
}

describe("Life Path pages", () => {
  it("has a real example date for every number, reducing to that number", () => {
    for (const n of LIFE_PATH_NUMBERS) {
      const [year, month, day] = LIFE_PATH_EXAMPLE_DATE[n].split("-").map(Number);
      expect(validBirthDate(day, month, year)).toBe(true);
      expect(lifePath(LIFE_PATH_EXAMPLE_DATE[n])).toBe(n);
    }
  });

  it("parses only the twelve slugs", () => {
    expect(LIFE_PATH_NUMBERS.map(String).map(parseLifePathSlug)).toEqual([...LIFE_PATH_NUMBERS]);
    for (const slug of ["0", "10", "13", "07", "44", "x", "", "1.5"]) expect(parseLifePathSlug(slug)).toBeNull();
  });

  it("lists symmetric compatible numbers that all have pages", () => {
    for (const n of LIFE_PATH_NUMBERS) {
      expect(LIFE_PATH_COMPATIBLE[n].length).toBeGreaterThan(0);
      for (const other of LIFE_PATH_COMPATIBLE[n]) {
        expect(LIFE_PATH_NUMBERS).toContain(other);
        expect(other).not.toBe(n);
      }
    }
    expect(lifePathNeighbours(1)).toEqual([2]);
    expect(lifePathNeighbours(9)).toEqual([8, 11]);
    expect(lifePathNeighbours(33)).toEqual([22]);
  });

  it("writes the worked example step by step, ending on the number", () => {
    expect(lifePathWorkedExample("en", 7)).toEqual({
      date: "June 18, 1981",
      lines: ["Month: 6", "Day: 18 → 1 + 8 = 9", "Year: 1981 → 1 + 9 + 8 + 1 = 19 → 1 + 9 = 10 → 1 + 0 = 1", "Total: 6 + 9 + 1 = 16 → 1 + 6 = 7"],
    });
    expect(lifePathWorkedExample("fr", 11).lines.at(-1)).toBe("Total : 9 + 1 + 1 = 11");
    for (const locale of locales)
      for (const n of LIFE_PATH_NUMBERS) expect(lifePathWorkedExample(locale, n).lines.at(-1)).toMatch(new RegExp(`= ${n}$`));
  });

  for (const locale of locales) {
    it(`${locale}: every number has every section, a FAQ and the calculator links`, () => {
      for (const n of LIFE_PATH_NUMBERS) {
        const page = lifePathPage(locale, n);
        expectWellFormed(page, locale);
        expect(page.route).toBe(`/life-path-number/${n}`);
        expect(page.sections.map((s) => s.id)).toEqual(["meaning", "strengths", "challenges", "love", "work", "calculation", "compatible"]);
        expect(page.h1).toContain(String(n));
        const listItems = page.sections.flatMap((s) => s.blocks).filter((b) => b.type === "list");
        expect(listItems.length).toBe(3);
        expect(pageLinks(page)).toEqual(expect.arrayContaining(["/life-path-number", "/calculators/path-of-life"]));
        expect(pageText(page).join(" ").length).toBeGreaterThan(2500);
      }
      expectWellFormed(lifePathHubPage(locale), locale);
      expect(lifePathHubPage(locale).grid?.items).toHaveLength(12);
    });
  }

  it("is translated: French and Spanish never repeat the English copy", () => {
    for (const n of LIFE_PATH_NUMBERS) {
      const en = pageText(lifePathPage("en", n));
      for (const locale of ["fr", "es"] as const) {
        const other = pageText(lifePathPage(locale, n));
        // Arithmetic lines ("Total: 1 + 6 + 3 = 10") may legitimately read the same.
        const same = other.filter((value, index) => value.length > 25 && !value.includes("=") && value === en[index]);
        expect(same, `${locale} ${n}`).toEqual([]);
      }
    }
  });
});

describe("tarot birth card pages", () => {
  it("covers the 22 Major Arcana with Golden Dawn attributions", () => {
    expect(TAROT_SLUGS).toHaveLength(22);
    expect(Object.keys(GOLDEN_DAWN).sort()).toEqual([...TAROT_SLUGS].sort());
    expect(tarotCardFacts("strength")).toMatchObject({ arcana: 8, numeral: "VIII", letter: "Teth", attribution: { kind: "sign", value: "Leo" } });
    expect(tarotCardFacts("justice")).toMatchObject({ arcana: 11, numeral: "XI", letter: "Lamed", attribution: { kind: "sign", value: "Libra" } });
    expect(tarotCardFacts("the-fool")).toMatchObject({ arcana: 0, numeral: "0", birthNumber: 22, chain: [22, 4] });
    expect(tarotCardFacts("the-star")).toMatchObject({ birthNumber: 17, numeral: "XVII", chain: [17, 8], letter: "Tzaddi" });
    expect(tarotCardFacts("nope")).toBeNull();
  });

  it("has a real example date for every card that gives that card", () => {
    for (const slug of TAROT_SLUGS) {
      const number = birthNumberOfSlug(slug)!;
      const date = TAROT_EXAMPLE_DATE[number];
      const [year, month, day] = date.split("-").map(Number);
      expect(validBirthDate(day, month, year), date).toBe(true);
      const cards = tarotBirthCards(date)!.cards;
      // Only The Magician (1) is never a first total for dates from 1900: it is a soul card there.
      if (number === 1) expect(cards.at(-1)).toBe(1);
      else expect(cards[0]).toBe(number);
    }
  });

  it("describes each card's constellation from the arithmetic", () => {
    const star = tarotPage("en", "the-star")!;
    const constellation = star.sections.find((s) => s.id === "constellation")!;
    const text = constellation.blocks.map((b) => (b.type === "p" ? b.text : "")).join(" ");
    expect(text).toContain("When your total is 17, The Star is your personality card and Strength is your soul card.");
    expect(text).toContain("Strength and The Star");
    const magician = tarotPage("en", "the-magician")!.sections.find((s) => s.id === "constellation")!;
    expect(JSON.stringify(magician)).toContain("10 (Wheel of Fortune) and 19 (The Sun)");
    for (const slug of TAROT_SLUGS) {
      const number = birthNumberOfSlug(slug)!;
      const soul = birthCardsOf(number).at(-1)!;
      expect(constellationOf(soul)).toContain(number);
    }
  });

  it("writes the worked example ending on the card", () => {
    const example = tarotPage("en", "the-star")!.sections.find((s) => s.id === "example")!;
    const list = example.blocks.find((b) => b.type === "list");
    expect(list).toEqual({
      type: "list",
      ordered: true,
      items: ["Month + day + year: 1 + 10 + 1959 = 1970", "1 + 9 + 7 + 0 = 17 → The Star (personality card)", "1 + 7 = 8 → Strength (soul card)"],
    });
  });

  for (const locale of locales) {
    it(`${locale}: every card has every section, a FAQ, its artwork and the calculator links`, () => {
      for (const slug of TAROT_SLUGS) {
        const page = tarotPage(locale, slug)!;
        expectWellFormed(page, locale);
        expect(page.route).toBe(`/tarot-birth-card/${slug}`);
        expect(page.sections.map((s) => s.id)).toEqual(["meaning", "facts", "gifts", "shadow", "constellation", "example"]);
        expect(page.h1).toContain(CARD_NAMES[locale][slug]);
        expect(page.artwork?.[0].src).toBe(`https://assets.staryield.net/assets/tarot/${slug}.png`);
        expect(pageLinks(page)).toEqual(expect.arrayContaining(["/tarot-birth-card", "/calculators/tarot-birth-card"]));
        expect(pageText(page).join(" ").length).toBeGreaterThan(2000);
      }
      const hub = tarotHubPage(locale);
      expectWellFormed(hub, locale);
      expect(hub.grid?.items).toHaveLength(22);
    });
  }
});

describe("sign pairs", () => {
  it("has the 78 unordered pairs with canonical slugs in zodiac order", () => {
    expect(ALL_PAIRS).toHaveLength(78);
    expect(new Set(PAIR_SLUGS).size).toBe(78);
    expect(PAIR_SLUGS_BOTH_ORDERS).toHaveLength(78 + 66);
    expect(pairSlug("Leo", "Aries")).toBe("aries-and-leo");
    expect(pairSlug("Aries", "Leo")).toBe("aries-and-leo");
    expect(pairSlug("Pisces", "Pisces")).toBe("pisces-and-pisces");
    expect(parsePairSlug("aries-and-leo")).toEqual({ slug: "aries-and-leo", a: "Aries", b: "Leo", canonical: true });
    expect(parsePairSlug("leo-and-aries")).toEqual({ slug: "aries-and-leo", a: "Aries", b: "Leo", canonical: false });
    for (const slug of ["aries-leo", "aries-and-ophiuchus", "Aries-and-Leo", "aries-and-leo-and-virgo", ""]) expect(parsePairSlug(slug)).toBeNull();
  });

  it("finds the angular relationship of every pair", () => {
    expect(aspectBetween("Aries", "Aries")).toBe("same-sign");
    expect(aspectBetween("Aries", "Taurus")).toBe("semi-sextile");
    expect(aspectBetween("Aries", "Gemini")).toBe("sextile");
    expect(aspectBetween("Aries", "Cancer")).toBe("square");
    expect(aspectBetween("Aries", "Leo")).toBe("trine");
    expect(aspectBetween("Aries", "Virgo")).toBe("quincunx");
    expect(aspectBetween("Aries", "Libra")).toBe("opposition");
    expect(aspectBetween("Pisces", "Aries")).toBe("semi-sextile");
    expect(aspectBetween("Capricorn", "Taurus")).toBe("trine");
    const counts = Object.fromEntries(ASPECTS.map((aspect) => [aspect, ALL_PAIRS.filter((p) => aspectBetween(p.a, p.b) === aspect).length]));
    expect(counts).toEqual({ "same-sign": 12, "semi-sextile": 12, sextile: 12, square: 12, trine: 12, quincunx: 12, opposition: 6 });
  });

  it("gives symmetric, bounded, deterministic indicators", () => {
    for (const a of SIGNS)
      for (const b of SIGNS) {
        expect(indicatorsFor(a, b)).toEqual(indicatorsFor(b, a));
        expect(aspectBetween(a, b)).toBe(aspectBetween(b, a));
        for (const value of Object.values(indicatorsFor(a, b))) {
          expect(value).toBeGreaterThanOrEqual(1);
          expect(value).toBeLessThanOrEqual(5);
        }
      }
    expect(indicatorsFor("Aries", "Leo")).toEqual({ love: 5, friendship: 5, work: 5 });
    expect(indicatorsFor("Aries", "Cancer")).toEqual({ love: 2, friendship: 2, work: 3 });
  });

  it("links related pairs that exist, never the page itself", () => {
    for (const pair of ALL_PAIRS) {
      const related = relatedPairs(pair.a, pair.b);
      expect(related.length).toBeGreaterThanOrEqual(2);
      for (const other of related) {
        expect(PAIR_SLUGS).toContain(other.slug);
        expect(other.slug).not.toBe(pair.slug);
      }
    }
  });

  it("redirects reversed pair paths to the canonical one", () => {
    expect(canonicalPairPath("/en/compatibility/leo-and-aries")).toBe("/en/compatibility/aries-and-leo");
    expect(canonicalPairPath("/compatibility/leo-and-aries")).toBe("/compatibility/aries-and-leo");
    expect(canonicalPairPath("/es/compatibility/pisces-and-cancer/")).toBe("/es/compatibility/cancer-and-pisces");
    for (const path of ["/en/compatibility/aries-and-leo", "/en/compatibility/leo-and-ophiuchus", "/de/compatibility/leo-and-aries", "/en/compatibility", "/en/x/leo-and-aries"])
      expect(canonicalPairPath(path)).toBeNull();
  });

  it("has every building block in every locale", () => {
    for (const locale of locales) {
      const copy = compatibilityCopy[locale];
      expect(Object.keys(copy.aspects)).toHaveLength(7);
      expect(Object.keys(copy.elements)).toHaveLength(10);
      expect(Object.keys(copy.modalities)).toHaveLength(6);
      expect(Object.keys(copy.signs)).toHaveLength(12);
      for (const value of JSON.stringify([copy.aspects, copy.elements, copy.modalities, copy.signs, copy.planets]).match(/"[^"]*"/g) ?? [])
        expect(value.length).toBeGreaterThan(2);
    }
  });

  for (const locale of locales) {
    it(`${locale}: every pair has every section, the indicators, a FAQ and unique text`, () => {
      const bodies = new Set<string>();
      const masked = new Set<string>();
      const names = Object.values(SIGN_NAMES[locale]).sort((x, y) => y.length - x.length);
      for (const pair of ALL_PAIRS) {
        const page = compatibilityPage(locale, pair.slug)!;
        expectWellFormed(page, locale);
        expect(page.route).toBe(`/compatibility/${pair.slug}`);
        expect(page.sections.map((s) => s.id)).toEqual(["overview", "elements", "rulers", "strengths", "friction", "tips", "indicators"]);
        expect(page.h1).toContain(SIGN_NAMES[locale][pair.a]);
        expect(page.h1).toContain(SIGN_NAMES[locale][pair.b]);
        const indicators = page.sections.flatMap((s) => s.blocks).find((b) => b.type === "indicators");
        expect(indicators?.type === "indicators" && indicators.items.map((item) => item.value)).toEqual(
          Object.values(indicatorsFor(pair.a, pair.b)),
        );
        const body = page.sections.map((s) => JSON.stringify(s.blocks)).join("\n");
        bodies.add(body);
        // Unique even with the sign names hidden: the combination of factors and profiles differs.
        masked.add(names.reduce((text, name) => text.split(name).join("§"), body));
        expect(pageLinks(page)).toEqual(expect.arrayContaining(["/compatibility", "/calculators/astrology"]));
      }
      expect(bodies.size).toBe(78);
      expect(masked.size).toBe(78);
      const hub = compatibilityHubPage(locale);
      expectWellFormed(hub, locale);
      expect(hub.grid?.groups).toHaveLength(12);
      expect(new Set(hub.grid?.groups?.flatMap((group) => group.links.map((link) => link.href))).size).toBe(78);
    });
  }

  it("returns no page for reversed or unknown slugs", () => {
    expect(compatibilityPage("en", "leo-and-aries")).toBeNull();
    expect(compatibilityPage("en", "aries-and-nope")).toBeNull();
  });
});

describe("entertainment-only wording", () => {
  /** What a card-scheme reviewer would read as a claim that conflicts with "for entertainment purposes only". */
  const CLAIMS = /certif|verifi|vetted|accura|guarant|garanti|healing|guéri|sanaci|curaci|life-changing|scientifically/i;

  it("makes no certification, accuracy, guarantee or healing claim on any page", () => {
    for (const locale of locales) {
      const pages = [
        lifePathHubPage(locale),
        tarotHubPage(locale),
        compatibilityHubPage(locale),
        ...LIFE_PATH_NUMBERS.map((n) => lifePathPage(locale, n)),
        ...TAROT_SLUGS.map((slug) => tarotPage(locale, slug)!),
        ...PAIR_SLUGS.map((slug) => compatibilityPage(locale, slug)!),
      ];
      for (const page of pages) expect(pageText(page).join(" ").match(CLAIMS), `${locale} ${page.route}`).toBeNull();
    }
  });
});
