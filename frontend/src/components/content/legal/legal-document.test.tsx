// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";

import { LegalDocument } from "@/components/content/legal/legal-document";
import {
  parseInline,
  resolveHref,
} from "@/components/content/legal/inline-text";
import { legalDocuments } from "@/content/legal";
import type { Locale } from "@/i18n/config";

afterEach(cleanup);

const locales: Locale[] = ["en", "es", "fr"];

function renderDoc(id: "privacy" | "terms" | "refund", locale: Locale) {
  const { container } = render(<LegalDocument id={id} locale={locale} />);
  return container;
}

function ids(container: HTMLElement) {
  return [...container.querySelectorAll("[id]")].map((el) => el.id);
}

describe("LegalDocument", () => {
  for (const id of ["privacy", "terms"] as const) {
    for (const locale of locales) {
      it(`${id}/${locale}: unique ids and every TOC link targets an existing id`, () => {
        const container = renderDoc(id, locale);
        const all = ids(container);
        expect(new Set(all).size).toBe(all.length);
        const toc = container.querySelector("nav#toc");
        expect(toc).not.toBeNull();
        const tocLinks = [...toc!.querySelectorAll("a")].map((a) => a.getAttribute("href")!);
        expect(tocLinks.length).toBeGreaterThan(10);
        for (const href of tocLinks) {
          expect(href.startsWith("#")).toBe(true);
          expect(container.querySelector(`[id="${href.slice(1)}"]`)).not.toBeNull();
        }
      });

      it(`${id}/${locale}: links are internal, anchors or mailto — never /psychics`, () => {
        const container = renderDoc(id, locale);
        for (const anchor of container.querySelectorAll("a")) {
          const href = anchor.getAttribute("href")!;
          expect(href).toMatch(/^(#|\/(en|es|fr)\/|mailto:)/);
          expect(href).not.toMatch(/\/psychics/);
          if (href.startsWith("/") && !href.includes("/privacy") && !href.includes("/terms"))
            expect(href.startsWith(`/${locale}/`)).toBe(true);
        }
      });
    }

    it(`${id}: anchors are identical in every locale`, () => {
      const [en, ...others] = locales.map((locale) => {
        const container = renderDoc(id, locale);
        const result = ids(container);
        cleanup();
        return result;
      });
      for (const other of others) expect(other).toEqual(en);
    });
  }

  it("shows the dates in the hero and at the end, with semantic structure", () => {
    const container = renderDoc("privacy", "en");
    expect(screen.getAllByText("Last updated")).toHaveLength(2);
    expect(screen.getAllByText("Effective as of")).toHaveLength(2);
    expect(container.querySelectorAll("time[datetime='2026-11-13']")).toHaveLength(4);
    expect(screen.getByRole("heading", { level: 1, name: "Privacy Policy" })).toBeTruthy();
    expect(container.querySelector("article[aria-labelledby='legal-title']")).not.toBeNull();
    expect(screen.getByRole("heading", { level: 2, name: /^11\. COOKIES/ })).toBeTruthy();
  });

  it("renders the cookie table with column headers", () => {
    const container = renderDoc("privacy", "fr");
    const table = container.querySelector("[id='cookies']")?.closest("section")?.querySelector("table");
    expect(table).not.toBeNull();
    expect(table!.querySelectorAll("thead th[scope='col']")).toHaveLength(5);
    expect(table!.textContent).toContain("sy_rt");
  });

  // revisions 5.1: the membership notice and "Subscription Terms" are gone; the withdrawal and refund anchors stay.
  it("keeps the refunds and right-of-withdrawal anchors and has no subscription-terms anchor", () => {
    const container = renderDoc("terms", "es");
    expect(container.querySelector("a[href='#refunds']")).not.toBeNull();
    expect(container.querySelector("[id='right-of-withdrawal']")).not.toBeNull();
    expect(container.querySelector("[id='refunds']")).not.toBeNull();
    expect(container.querySelector("[id='subscription-terms']")).toBeNull();
    expect(container.querySelector("a[href='#subscription-terms']")).toBeNull();
  });

  it("keeps the content data-driven: all sections render", () => {
    const container = renderDoc("terms", "en");
    expect(container.querySelectorAll("section h2").length).toBe(legalDocuments.terms.en.sections.length);
  });
});

// revisions 5.0: the Refund Policy and its reading-card (policy) layout.
describe("LegalDocument policy layout (Refund Policy)", () => {
  for (const locale of locales) {
    it(`refund/${locale}: unique ids, sidebar and phone contents lists target existing sections`, () => {
      const container = renderDoc("refund", locale);
      const all = ids(container);
      expect(new Set(all).size).toBe(all.length);
      for (const selector of ["nav#toc", "nav#toc-mobile"]) {
        const links = [...container.querySelectorAll(`${selector} a`)].map((a) => a.getAttribute("href")!);
        expect(links).toHaveLength(14);
        for (const href of links) expect(container.querySelector(`[id="${href.slice(1)}"]`)).not.toBeNull();
      }
    });

    it(`refund/${locale}: only internal, anchor and mailto links, with the locale prefix`, () => {
      const container = renderDoc("refund", locale);
      for (const anchor of container.querySelectorAll("a")) {
        const href = anchor.getAttribute("href")!;
        expect(href).toMatch(/^(#|\/(en|es|fr)(\/|$)|mailto:support@staryield\.net$)/);
        if (href.startsWith("/")) expect(href.startsWith(`/${locale}`)).toBe(true);
      }
    });
  }

  it("refund anchors are identical in every locale", () => {
    const [first, ...others] = locales.map((locale) => {
      const result = ids(renderDoc("refund", locale));
      cleanup();
      return result;
    });
    for (const other of others) expect(other).toEqual(first);
  });

  it("renders the header, actions, dates and related links like the design", () => {
    const container = renderDoc("refund", "en");
    expect(container.querySelector("[data-legal-layout='policy']")).not.toBeNull();
    expect(screen.getByRole("heading", { level: 1, name: "Refund & Cancellation Policy" })).toBeTruthy();
    expect(screen.getByText(/STARYIELD · POLICIES/)).toBeTruthy();
    expect(container.querySelectorAll("time[datetime='2026-11-13']")).toHaveLength(2);
    expect(container.textContent).toContain("Effective date: 13.11.2026");
    expect(container.textContent).toContain("Last updated: 13.11.2026");
    expect(screen.getAllByRole("link", { name: "Contact Support" }).map((a) => a.getAttribute("href"))).toEqual(["/en/contact", "/en/contact"]);
    expect(screen.getByRole("link", { name: "Go to Pricing" }).getAttribute("href")).toBe("/en/pricing");
    const related = container.querySelector("nav[aria-labelledby='legal-related-title']")!;
    expect([...related.querySelectorAll("a")].map((a) => [a.textContent, a.getAttribute("href")])).toEqual([
      ["Terms of Service", "/en/terms"],
      ["Privacy Policy", "/en/privacy"],
      ["Pricing and Credits", "/en/pricing"],
    ]);
    expect(screen.getByRole("link", { name: "Contact page" }).getAttribute("href")).toBe("/en/contact");
    expect(screen.getAllByRole("link", { name: "support@staryield.net" }).every((a) => a.getAttribute("href") === "mailto:support@staryield.net")).toBe(true);
  });

  it("has a sticky contents sidebar on desktop and a collapsible list on phones", () => {
    const container = renderDoc("refund", "en");
    const aside = container.querySelector("aside")!;
    expect(aside.className).toContain("lg:sticky");
    expect(aside.className).toContain("hidden");
    expect(aside.querySelector("h2")?.textContent).toBe("On this page");
    const details = container.querySelector("details")!;
    expect(details.className).toContain("lg:hidden");
    expect(details.open).toBe(false);
    expect(details.querySelector("summary")?.textContent).toBe("On this page");
    expect(container.querySelector("nav[aria-label='Refund Policy']")?.textContent).toContain("Home");
  });

  it("uses a 920px card (760px of text), 16px+ body text and 8px corners", () => {
    const article = renderDoc("refund", "en").querySelector("article")!;
    expect(article.className).toContain("lg:max-w-[920px]");
    expect(article.className).toContain("md:px-20");
    expect(article.className).toContain("rounded-md");
    expect(article.className).toMatch(/text-base/);
    expect(article.className).not.toMatch(/text-\[1[0-5]px\]/);
  });

  it("shows the callout, the request checklist and the verbatim placeholders", () => {
    const container = renderDoc("refund", "en");
    expect(container.textContent).toContain("Refund amount = actual pack price paid − value of purchased credits already used.");
    expect(container.querySelectorAll("#requesting-a-refund ~ ul li, #requesting-a-refund + p + ul li").length).toBeGreaterThanOrEqual(0);
    expect(within(container.querySelector("section[aria-labelledby='requesting-a-refund']") as HTMLElement).getAllByRole("listitem")).toHaveLength(7);
    for (const value of ["within 2 business days", "within 5 business days", "as STARYIELD MEDIA LLC"]) expect(container.textContent).toContain(value);
    for (const placeholder of ["[2]", "[5]", "[NOTDEFINEDYET]"]) expect(container.textContent).not.toContain(placeholder);
  });

  it("keeps Privacy and Terms on the original centred layout", () => {
    expect(renderDoc("privacy", "en").querySelector("[data-legal-layout]")).toBeNull();
    expect(renderDoc("terms", "en").querySelector("details")).toBeNull();
  });
});

describe("inline text", () => {
  it("parses links and bold spans", () => {
    expect(parseInline("See **this** and [that](/contact).")).toEqual([
      { kind: "text", value: "See " },
      { kind: "bold", value: "this" },
      { kind: "text", value: " and " },
      { kind: "link", label: "that", href: "/contact" },
      { kind: "text", value: "." },
    ]);
  });

  it("prefixes the locale on internal paths and refuses external URLs", () => {
    expect(resolveHref("/contact?topic=privacy", "fr")).toBe("/fr/contact?topic=privacy");
    expect(resolveHref("/en/privacy", "es")).toBe("/en/privacy");
    expect(resolveHref("#refunds", "en")).toBe("#refunds");
    expect(resolveHref("mailto:privacy@example.com", "en")).toBe("mailto:privacy@example.com");
    expect(resolveHref("https://evil.example", "en")).toBeNull();
    expect(resolveHref("//evil.example", "en")).toBeNull();
    expect(resolveHref("javascript:alert(1)", "en")).toBeNull();
  });
});
