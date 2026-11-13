// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup } from "@testing-library/react";
import type * as React from "react";

import { renderWithI18n } from "@/test/render";

vi.mock("next/link", () => ({ default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => <a href={href} {...rest}>{children}</a> }));
vi.mock("next/navigation", () => ({ usePathname: () => "/en", useRouter: () => ({ push: vi.fn(), replace: vi.fn() }), useSearchParams: () => new URLSearchParams() }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: "anonymous", user: null }) }));
vi.mock("@/lib/site-config/site-config-provider", () => ({ useSignupFreeMinutes: () => 4, useSiteConfig: () => ({ config: null }) }));
vi.mock("@/components/content/featured-advisors", () => ({ FeaturedAdvisors: () => null }));
vi.mock("@/components/content/use-scroll-to-psychics", () => ({ useScrollToPsychics: () => undefined }));
vi.mock("@/components/reviews/trustpilot-review-collector", () => ({ TrustpilotReviewCollector: () => <div data-testid="trustpilot-widget" /> }));

import About from "@/components/content/about";
import Home from "@/components/content/home";
import Reviews from "@/components/content/reviews";
import Standards from "@/components/content/standards";

afterEach(() => cleanup());

/** Words and figures a card-scheme reviewer would read as a claim that conflicts with "for entertainment purposes only". */
const CLAIMS = /certified|verified|vetted|accurate|accuracy|guarantee|expert (?:guidance|advice)|therap|healing|karm|life-changing|audit|top-rated|10K\+|50\+|4\.8|\b1000\b|millions|Reynolds|Thayna|Surinder|Mariana|competitors|Legit or Scam|follow us for.*master astrologers|master astrologers/i;

describe.each([
  ["Home", Home],
  ["About", About],
  ["Reviews", Reviews],
  ["Standards", Standards],
])("%s page copy (revisions 5.1)", (_name, Page) => {
  it("makes no certification, accuracy, healing, statistic or testimonial claim", () => {
    const { container } = renderWithI18n(<Page />);
    const text = (container.textContent ?? "").replace(/We do not certify or verify any advisor's credentials, abilities or predictions\./, "").replace(/we do not certify or verify advisors' abilities/, "");
    expect(text.match(CLAIMS)).toBeNull();
  });
});

describe("page content decisions", () => {
  it("the Reviews page shows the Trustpilot widget and no hard-coded review cards", () => {
    const { container } = renderWithI18n(<Reviews />);
    expect(container.querySelectorAll('[data-testid="trustpilot-widget"]')).toHaveLength(1);
    expect(container.querySelector('[data-name="Section / Customer Review Cards"]')).toBeNull();
    expect(container.textContent).toContain("Staryield is operated by Staryield Media LLC, 30 N Gould St, Ste N, Sheridan, WY 82801, United States.");
    expect(container.textContent).toContain("support@staryield.net");
  });

  it("the Home page keeps the entertainment statement and the Trustpilot widget", () => {
    const { container } = renderWithI18n(<Home />);
    expect(container.textContent).toContain("Readings are offered for entertainment purposes only.");
    expect(container.querySelectorAll('[data-testid="trustpilot-widget"]')).toHaveLength(1);
  });

  it("the Standards page says Staryield does not certify or verify advisors", () => {
    const { container } = renderWithI18n(<Standards />);
    expect(container.textContent).toContain("We do not certify or verify any advisor's credentials, abilities or predictions.");
  });
});
