// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import es from "@/i18n/dictionaries/es.json";
import copyEn from "@/i18n/dictionaries/copy-en.json";
import copyEs from "@/i18n/dictionaries/copy-es.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";

vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: "anonymous", user: null }) }));
vi.mock("next/link", () => ({ default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => <a href={href} {...rest}>{children}</a> }));
vi.mock("@/components/reviews/trustpilot-review-collector", () => ({ TrustpilotReviewCollector: () => null }));

import { SiteFooter } from "@/components/layout/site-footer";

afterEach(() => cleanup());

// growth (seo): internal links from every page to the three search hubs.
describe("SiteFooter Explore group", () => {
  it("links the Life Path, tarot birth card and compatibility hubs", () => {
    render(
      <I18nProvider locale="en" dictionary={{ ...en, copy: copyEn } as Dictionary}>
        <SiteFooter />
      </I18nProvider>,
    );
    const explore = within(screen.getByRole("navigation", { name: "Explore" }));
    expect(explore.getByRole("heading").textContent).toBe("EXPLORE");
    expect(explore.getAllByRole("link").map((a) => [a.textContent, a.getAttribute("href")])).toEqual([
      ["Life Path numbers", "/en/life-path-number"],
      ["Tarot birth cards", "/en/tarot-birth-card"],
      ["Zodiac compatibility", "/en/compatibility"],
    ]);
  });

  it("is translated", () => {
    render(
      <I18nProvider locale="es" dictionary={{ ...es, copy: copyEs } as Dictionary}>
        <SiteFooter />
      </I18nProvider>,
    );
    const explore = within(screen.getByRole("navigation", { name: "Explorar" }));
    expect(explore.getAllByRole("link").map((a) => a.textContent)).toEqual(["Caminos de vida", "Cartas de nacimiento del tarot", "Compatibilidad de signos"]);
  });
});
