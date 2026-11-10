// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, within } from "@testing-library/react";

import { renderWithI18n } from "@/test/render";

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("next/navigation", () => ({ usePathname: () => "/en", useRouter: () => ({ push: vi.fn(), replace: vi.fn() }), useSearchParams: () => new URLSearchParams() }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: "anonymous" }) }));
vi.mock("@/lib/site-config/site-config-provider", () => ({ useSignupFreeMinutes: () => 0, useSiteConfig: () => ({ config: null }) }));
vi.mock("@/components/content/featured-advisors", () => ({ FeaturedAdvisors: () => null }));
vi.mock("@/components/content/use-scroll-to-psychics", () => ({ useScrollToPsychics: () => undefined }));
vi.mock("@/components/reviews/trustpilot-review-collector", () => ({
  TrustpilotReviewCollector: ({ placement }: { placement: string }) => <div data-testid="trustpilot-widget" data-placement={placement} />,
}));

import ScreenDesktopHome from "@/components/content/home";
import ScreenDesktopReviews from "@/components/content/reviews";
import { SiteFooter } from "@/components/layout/site-footer";

afterEach(() => cleanup());

describe("Trustpilot placements", () => {
  it("the reviews page shows the widget under the Trustpilot heading and no hard-coded figures", () => {
    const { container } = renderWithI18n(<ScreenDesktopReviews />);
    const widget = screen.getByTestId("trustpilot-widget");
    expect(widget.getAttribute("data-placement")).toBe("reviews");
    const heading = screen.getByRole("heading", { level: 1, name: "Trustpilot" });
    expect(heading.compareDocumentPosition(widget) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(screen.getByText("Read what people say about Staryield, or share your own experience.")).toBeTruthy();
    expect(container.textContent).not.toMatch(/17,419|4\.4 rating/);
    expect(screen.getAllByTestId("trustpilot-widget")).toHaveLength(1);
  });

  it("the home page shows a compact widget after the testimonials and statistics", () => {
    const { container } = renderWithI18n(<ScreenDesktopHome />);
    const widget = screen.getByTestId("trustpilot-widget");
    expect(widget.getAttribute("data-placement")).toBe("home");
    const section = container.querySelector('[data-name="Container / Testimonials & Stats"]')!;
    expect(section.contains(widget)).toBe(true);
    // The existing testimonial and statistics content is untouched.
    expect(within(section as HTMLElement).getByText("H. Reynolds")).toBeTruthy();
    expect(within(section as HTMLElement).getByText("Average Platform Rating")).toBeTruthy();
    const stats = section.querySelector('[data-name="Media / Testimonials & Stats"]')!;
    expect(widget.compareDocumentPosition(stats) & Node.DOCUMENT_POSITION_PRECEDING).toBeTruthy();
  });

  it("the footer shows the widget above the copyright line", () => {
    const { container } = renderWithI18n(<SiteFooter />);
    const widget = screen.getByTestId("trustpilot-widget");
    expect(widget.getAttribute("data-placement")).toBe("footer");
    const footer = container.querySelector("footer")!;
    expect(footer.contains(widget)).toBe(true);
    const copyright = screen.getByText(/© 2026 Staryield Media LLC/);
    expect(widget.compareDocumentPosition(copyright) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});
