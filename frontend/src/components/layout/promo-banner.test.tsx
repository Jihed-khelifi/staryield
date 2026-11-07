// @vitest-environment jsdom
import type * as React from "react";
import { act, cleanup, fireEvent, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { SiteBanner, SiteConfig } from "@/lib/api/types";
import { axeViolations, renderWithI18n } from "@/test/render";

const session = vi.hoisted(() => ({ status: "anonymous" as "anonymous" | "authenticated" | "loading" }));
const authHint = vi.hoisted(() => ({ value: false }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: session.status }) }));
vi.mock("@/lib/api/client", () => ({ readAuthHint: () => authHint.value }));
vi.mock("@/lib/api/endpoints", () => ({ siteApi: { config: vi.fn(() => new Promise(() => {})) } }));

import { bannerLink, filterBanners, PromoBanner } from "@/components/layout/promo-banner";
import { SiteConfigProvider } from "@/lib/site-config/site-config-provider";

function banner(id: string, over: Partial<SiteBanner> = {}): SiteBanner {
  return { id, text: `Text ${id}`, cta_label: `Go ${id}`, action: { type: "link", href: `/signup` }, audience: "all", ...over };
}
function site(items: SiteBanner[], rotation = 6): SiteConfig {
  return {
    banners: { rotation_seconds: rotation, items },
    signup_offer: { enabled: true, free_minutes: 5 },
    free_mode: { active: false },
    generated_at: "2026-11-07T00:00:00Z",
  };
}
function mount(items: SiteBanner[], rotation = 6) {
  return renderWithI18n(
    <SiteConfigProvider initial={site(items, rotation)}>
      <PromoBanner />
    </SiteConfigProvider>,
  );
}
const active = () => screen.getAllByTestId("promo-slide").find((s) => s.dataset.active === "true")!;

beforeEach(() => {
  vi.useFakeTimers();
  session.status = "anonymous";
  authHint.value = false;
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("bannerLink", () => {
  it("prefixes internal paths with the locale", () => {
    expect(bannerLink(banner("a", { action: { type: "link", href: "/signup" } }), "fr")).toEqual({ href: "/fr/signup", external: false });
    expect(bannerLink(banner("a", { action: { type: "link", href: "/" } }), "es")).toEqual({ href: "/es", external: false });
    expect(bannerLink(banner("a", { action: { type: "link", href: "/#psychics" } }), "en")).toEqual({ href: "/en#psychics", external: false });
  });
  it("keeps https URLs external", () => {
    expect(bannerLink(banner("a", { action: { type: "link", href: "https://staryield.net/x" } }), "en")).toEqual({ href: "https://staryield.net/x", external: true });
  });
  it("is null for none, empty and unsafe values", () => {
    expect(bannerLink(banner("a", { action: { type: "none", href: "" } }), "en")).toBeNull();
    for (const href of ["", "javascript:alert(1)", "//evil.test", "http://x.test", "data:text/html,1", "/\\evil"])
      expect(bannerLink(banner("a", { action: { type: "link", href } }), "en"), href).toBeNull();
  });
});

describe("filterBanners", () => {
  const all = [banner("a"), banner("b", { audience: "anonymous" }), banner("c", { audience: "signed_in" })];
  it("keeps all + anonymous for visitors", () => expect(filterBanners(all, false).map((b) => b.id)).toEqual(["a", "b"]));
  it("keeps all + signed_in for members", () => expect(filterBanners(all, true).map((b) => b.id)).toEqual(["a", "c"]));
});

describe("PromoBanner", () => {
  it("renders nothing without banners", () => {
    mount([]);
    expect(screen.queryByTestId("promo-bar")).toBeNull();
  });

  it("renders nothing when no banner fits the audience", () => {
    mount([banner("a", { audience: "signed_in" })]);
    expect(screen.queryByTestId("promo-bar")).toBeNull();
  });

  it("shows a single banner without rotation controls", async () => {
    mount([banner("a")]);
    expect(screen.getByRole("link", { name: /Text a/ }).getAttribute("href")).toBe("/en/signup");
    expect(screen.queryByTestId("promo-toggle")).toBeNull();
    await act(async () => vi.advanceTimersByTime(60_000));
    expect(active().textContent).toContain("Text a");
  });

  it("rotates every rotation_seconds and wraps around", async () => {
    mount([banner("a"), banner("b"), banner("c")], 5);
    expect(active().textContent).toContain("Text a");
    await act(async () => vi.advanceTimersByTime(4_900));
    expect(active().textContent).toContain("Text a");
    await act(async () => vi.advanceTimersByTime(200));
    expect(active().textContent).toContain("Text b");
    await act(async () => vi.advanceTimersByTime(5_000));
    expect(active().textContent).toContain("Text c");
    await act(async () => vi.advanceTimersByTime(5_000));
    expect(active().textContent).toContain("Text a");
  });

  it("hides inactive slides from assistive technology and keyboard", () => {
    mount([banner("a"), banner("b")]);
    const inactive = screen.getAllByTestId("promo-slide").find((s) => s.dataset.active === "false")!;
    expect(inactive.getAttribute("aria-hidden")).toBe("true");
    expect(inactive.className).toContain("invisible");
  });

  it("pauses on hover and resumes with a fresh interval on leave", async () => {
    mount([banner("a"), banner("b")], 5);
    const bar = screen.getByTestId("promo-bar");
    fireEvent.mouseEnter(bar);
    await act(async () => vi.advanceTimersByTime(30_000));
    expect(active().textContent).toContain("Text a");
    fireEvent.mouseLeave(bar);
    await act(async () => vi.advanceTimersByTime(5_100));
    expect(active().textContent).toContain("Text b");
  });

  it("pauses while a link has keyboard focus", async () => {
    mount([banner("a"), banner("b")], 5);
    const link = within0();
    fireEvent.focus(link);
    await act(async () => vi.advanceTimersByTime(20_000));
    expect(active().textContent).toContain("Text a");
    fireEvent.blur(link, { relatedTarget: document.body });
    await act(async () => vi.advanceTimersByTime(5_100));
    expect(active().textContent).toContain("Text b");
  });

  it("has a pause/play control (WCAG 2.2.2)", async () => {
    mount([banner("a"), banner("b")], 5);
    const toggle = screen.getByTestId("promo-toggle");
    expect(toggle.getAttribute("aria-label")).toBe("Pause promotions");
    fireEvent.click(toggle);
    expect(toggle.getAttribute("aria-label")).toBe("Play promotions");
    expect(toggle.getAttribute("aria-pressed")).toBe("true");
    await act(async () => vi.advanceTimersByTime(60_000));
    expect(active().textContent).toContain("Text a");
    fireEvent.click(toggle);
    await act(async () => vi.advanceTimersByTime(5_100));
    expect(active().textContent).toContain("Text b");
  });

  it("resumes on Play even while the play/pause button keeps keyboard focus", async () => {
    mount([banner("a"), banner("b")], 5);
    const toggle = screen.getByTestId("promo-toggle");
    fireEvent.focus(toggle);
    fireEvent.click(toggle); // pause
    await act(async () => vi.advanceTimersByTime(30_000));
    expect(active().textContent).toContain("Text a");
    fireEvent.click(toggle); // play: the focused toggle must not keep it paused
    await act(async () => vi.advanceTimersByTime(5_100));
    expect(active().textContent).toContain("Text b");
  });

  it("still rotates with prefers-reduced-motion and drops the fade classes", async () => {
    vi.stubGlobal("matchMedia", (query: string) => ({ matches: query.includes("reduce"), addEventListener() {}, removeEventListener() {}, media: query }));
    mount([banner("a"), banner("b")], 5);
    expect(screen.getAllByTestId("promo-slide")[0].className).toContain("motion-reduce:transition-none");
    await act(async () => vi.advanceTimersByTime(5_100));
    expect(active().textContent).toContain("Text b");
    vi.unstubAllGlobals();
  });

  it("opens external https actions in a new tab with noopener", () => {
    mount([banner("a", { action: { type: "link", href: "https://staryield.net/blog" } })]);
    const a = screen.getByRole("link", { name: /Text a/ });
    expect(a.getAttribute("target")).toBe("_blank");
    expect(a.getAttribute("rel")).toBe("noopener noreferrer");
  });

  it("renders text only for the none action", () => {
    mount([banner("a", { action: { type: "none", href: "" } })]);
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.getByText("Text a")).toBeTruthy();
  });

  it("shows the CTA label only from the sm breakpoint and omits it when empty", () => {
    mount([banner("a"), banner("b", { cta_label: "" })]);
    expect(screen.getByText("Go a").className).toContain("hidden sm:inline");
    expect(screen.queryByText(/^Go b$/)).toBeNull();
  });

  it("filters by audience for visitors and members", () => {
    const items = [banner("a", { audience: "anonymous" }), banner("b", { audience: "signed_in" })];
    mount(items);
    expect(screen.getAllByTestId("promo-slide")).toHaveLength(1);
    expect(screen.getByText("Text a")).toBeTruthy();
    cleanup();
    session.status = "authenticated";
    mount(items);
    expect(screen.getByText("Text b")).toBeTruthy();
    expect(screen.queryByText("Text a")).toBeNull();
  });

  it("uses the auth hint while the session is loading", () => {
    const items = [banner("a", { audience: "anonymous" }), banner("b", { audience: "signed_in" })];
    session.status = "loading";
    authHint.value = true;
    mount(items);
    expect(screen.getByText("Text b")).toBeTruthy();
    expect(screen.queryByText("Text a")).toBeNull();
  });

  it("is an accessible labelled region", async () => {
    vi.useRealTimers();
    mount([banner("a"), banner("b")]);
    expect(screen.getByRole("region", { name: "Promotions" })).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
  });
});

function within0() {
  return screen.getAllByRole("link")[0];
}
