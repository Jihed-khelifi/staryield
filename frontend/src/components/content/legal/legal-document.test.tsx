// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import { LegalDocument } from "@/components/content/legal/legal-document";
import {
  parseInline,
  resolveHref,
} from "@/components/content/legal/inline-text";
import { legalDocuments } from "@/content/legal";
import type { Locale } from "@/i18n/config";

afterEach(cleanup);

const locales: Locale[] = ["en", "es", "fr"];

function renderDoc(id: "privacy" | "terms", locale: Locale) {
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
    expect(container.querySelectorAll("time[datetime='2026-11-09']")).toHaveLength(4);
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

  it("links the Terms notice to the subscription terms and refunds anchors", () => {
    const container = renderDoc("terms", "es");
    expect(container.querySelector("a[href='#subscription-terms']")).not.toBeNull();
    expect(container.querySelector("a[href='#refunds']")).not.toBeNull();
    expect(container.querySelector("[id='subscription-terms']")).not.toBeNull();
    expect(container.querySelector("[id='refunds']")).not.toBeNull();
  });

  it("keeps the content data-driven: all sections render", () => {
    const container = renderDoc("terms", "en");
    expect(container.querySelectorAll("section h2").length).toBe(legalDocuments.terms.en.sections.length);
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
