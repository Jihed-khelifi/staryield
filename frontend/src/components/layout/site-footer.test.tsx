// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import fr from "@/i18n/dictionaries/fr.json";
import copyEn from "@/i18n/dictionaries/copy-en.json";
import copyFr from "@/i18n/dictionaries/copy-fr.json";
import copyEs from "@/i18n/dictionaries/copy-es.json";
import es from "@/i18n/dictionaries/es.json";
import { COMPANY } from "@/config/company";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";

vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: "anonymous", user: null }) }));
vi.mock("next/link", () => ({ default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => <a href={href} {...rest}>{children}</a> }));
vi.mock("@/components/reviews/trustpilot-review-collector", () => ({ TrustpilotReviewCollector: () => null }));

import { SiteFooter } from "@/components/layout/site-footer";

afterEach(() => cleanup());

function renderFooter(locale: "en" | "fr" | "es" = "en") {
  const dictionary = (locale === "en" ? { ...en, copy: copyEn } : locale === "fr" ? { ...fr, copy: copyFr } : { ...es, copy: copyEs }) as Dictionary;
  return render(
    <I18nProvider locale={locale} dictionary={dictionary}>
      <SiteFooter />
    </I18nProvider>,
  );
}

describe("SiteFooter (revisions 5.0)", () => {
  it("links Pricing under STARYIELD and the Refund policy under SUPPORT, keeping the existing items", () => {
    renderFooter();
    const support = within(screen.getByRole("navigation", { name: "Support" }));
    const staryield = within(screen.getByRole("navigation", { name: "Staryield" }));
    expect(support.getByRole("link", { name: "Refund Policy" }).getAttribute("href")).toBe("/en/refund-policy");
    expect(staryield.getByRole("link", { name: "Pricing" }).getAttribute("href")).toBe("/en/pricing");
    expect(support.getAllByRole("link").map((a) => a.getAttribute("href"))).toEqual(["/en/faq", "/en/terms", "/en/privacy", "/en/contact", "/en/refund-policy", "/en/disclaimer"]);
    expect(staryield.getAllByRole("link")).toHaveLength(4);
  });

  it("states the age and entertainment line next to the copyright", () => {
    renderFooter();
    const line = screen.getByTestId("footer-age-line");
    expect(line.textContent).toBe("18+ only · For entertainment purposes only.");
    expect(line.previousElementSibling?.textContent).toContain("© 2026 Staryield Media LLC.");
  });

  it("is translated and keeps the locale in the links", () => {
    renderFooter("fr");
    expect(screen.getByTestId("footer-age-line").textContent).toBe("Réservé aux 18 ans et plus · À des fins de divertissement uniquement.");
    expect(screen.getAllByRole("link").some((a) => a.getAttribute("href") === "/fr/refund-policy")).toBe(true);
    expect(screen.getAllByRole("link").some((a) => a.getAttribute("href") === "/fr/pricing")).toBe(true);
  });

  // revisions 5.1: merchant-account readiness (docs/platform-extensions.md §25.4, §25.6).
  it("shows the company's legal name, postal address, support e-mail and hours (revisions 5.1)", () => {
    renderFooter();
    expect(screen.getByTestId("footer-company-name").textContent).toBe(COMPANY.legalName);
    expect(screen.getByTestId("footer-company-address").textContent).toBe(COMPANY.addressLine);
    expect(screen.getByTestId("footer-support-email").getAttribute("href")).toBe(`mailto:${COMPANY.supportEmail}`);
    expect(screen.getByTestId("footer-support-email").textContent).toBe("support@staryield.net");
    expect(screen.getByTestId("footer-support-hours").textContent).toBe("Support is available 24/7. We reply within 1–2 days.");
    expect(screen.getByTestId("footer-company").tagName).toBe("ADDRESS");
  });

  it("shows the Visa and Mastercard acceptance marks with the translated label", () => {
    renderFooter();
    const cards = screen.getByTestId("accepted-cards");
    expect(within(cards).getByText("We accept")).toBeTruthy();
    expect(within(cards).getAllByRole("img").map((img) => img.getAttribute("alt"))).toEqual(["Visa", "Mastercard"]);
    cleanup();
    renderFooter("fr");
    expect(within(screen.getByTestId("accepted-cards")).getByText("Nous acceptons")).toBeTruthy();
  });

  it.each(["en", "fr", "es"] as const)("%s: states entertainment only in the tagline and the copyright line, with no professional or audit claim", (locale) => {
    const { container } = renderFooter(locale);
    const text = container.textContent ?? "";
    const tagline = {
      en: "Astrology and psychic readings for entertainment purposes only.",
      fr: "Astrologie et consultations de voyance à des fins de divertissement uniquement.",
      es: "Astrología y consultas psíquicas únicamente con fines de entretenimiento.",
    }[locale];
    expect(text).toContain(tagline);
    expect(text).toContain({ en: "© 2026 Staryield Media LLC. All rights reserved. For entertainment purposes only.", fr: "© 2026 Staryield Media LLC. Tous droits réservés. À des fins de divertissement uniquement.", es: "© 2026 Staryield Media LLC. Todos los derechos reservados. Únicamente con fines de entretenimiento." }[locale]);
    expect(text).not.toMatch(/Professional spiritual guidance|alignment audits|audits de|auditorías/i);
    expect(text).toContain(COMPANY.legalName);
    expect(text).toContain(COMPANY.supportEmail);
  });

  it("links the standards page under its new name without the word professional", () => {
    renderFooter();
    expect(within(screen.getByRole("navigation", { name: "Staryield" })).getByRole("link", { name: "Advisor Integrity & Interaction Standards" }).getAttribute("href")).toBe("/en/standards");
  });
});
