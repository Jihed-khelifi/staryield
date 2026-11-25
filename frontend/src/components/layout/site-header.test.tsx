// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { renderWithI18n, stubResizeObserver } from "@/test/render";

const h = vi.hoisted(() => ({
  session: { status: "anonymous", user: null, wallet: null } as { status: string; user: { id: string; role: string; email: string; display_name: string; profile: { nickname: string; avatar_url: string } } | null; wallet: { balance: number } | null },
}));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("next/navigation", () => ({ usePathname: () => "/en/psychics/theo" }));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ ...h.session, signOut: vi.fn() }),
}));
vi.mock("@/components/layout/promo-banner", () => ({ PromoBanner: () => null }));
vi.mock("@/components/layout/chat-shortcut", () => ({ ChatShortcut: () => null }));
vi.mock("@/components/layout/locale-switcher", () => ({ LocaleSwitcher: () => null }));
vi.mock("@/components/notifications/notification-bell", () => ({ NotificationBell: () => null }));
vi.mock("@/components/psychic/header-actions", () => ({ AdvisorHeaderActions: () => null, WalletPill: () => null }));
vi.mock("@/components/billing/top-up/top-up-provider", () => ({ TopUpLink: ({ children }: { children: React.ReactNode }) => <span>{children}</span> }));

import { SiteHeader } from "@/components/layout/site-header";

beforeAll(stubResizeObserver);
afterEach(() => {
  cleanup();
  h.session = { status: "anonymous", user: null, wallet: null };
});

const client = { id: "u1", role: "client", email: "jo@staryield.test", display_name: "Jo", profile: { nickname: "", avatar_url: "" } };

describe("public header", () => {
  it("links Pricing in the desktop navigation and in the menu drawer", async () => {
    const u = userEvent.setup();
    renderWithI18n(<SiteHeader publicSite />);
    const nav = screen.getByRole("navigation", { name: "Menu" });
    expect(within(nav).getByRole("link", { name: "Pricing" }).getAttribute("href")).toBe("/en/pricing");
    await u.click(screen.getByTestId("mobile-menu-trigger"));
    const drawer = await screen.findByRole("dialog");
    expect(within(drawer).getByRole("link", { name: "Pricing" }).getAttribute("href")).toBe("/en/pricing");
  });

  it("shows a signed-out visitor a Sign up link in the bar itself, shown below lg only, next to the menu trigger", () => {
    renderWithI18n(<SiteHeader publicSite />);
    const compact = screen.getByTestId("header-signup-compact");
    expect(compact.getAttribute("href")).toBe("/en/signup");
    expect(compact.textContent).toBe("Sign Up");
    expect(compact.className).toContain("lg:hidden");
    // The desktop pair (Log In, Sign Up) stays hidden below lg; the compact link is outside it.
    expect(compact.closest(".lg\\:flex")).toBeNull();
    expect(compact.closest(".header-actions")).toBe(screen.getByTestId("mobile-menu-trigger").closest(".header-actions"));
    expect(screen.getByTestId("mobile-menu-trigger").className).toContain("lg:hidden");
  });

  it("shows a signed-in user neither Sign up nor Log in", () => {
    h.session = { status: "authenticated", user: client, wallet: { balance: 10 } };
    renderWithI18n(<SiteHeader publicSite />);
    expect(screen.queryByTestId("header-signup-compact")).toBeNull();
    expect(screen.queryByRole("link", { name: /^Sign Up/ })).toBeNull();
    expect(screen.queryByRole("link", { name: /^Log In/ })).toBeNull();
  });

  it("keeps Pricing out of the signed-in app header", () => {
    h.session = { status: "authenticated", user: client, wallet: { balance: 10 } };
    renderWithI18n(<SiteHeader />);
    expect(screen.queryByRole("link", { name: "Pricing" })).toBeNull();
    expect(screen.queryByTestId("header-signup-compact")).toBeNull();
  });
});
