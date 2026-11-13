// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const h = vi.hoisted(() => ({
  user: { id: "u1", role: "client", email: "jo@staryield.test", display_name: "Jo", profile: { nickname: "", avatar_url: "" } },
}));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("next/navigation", () => ({ usePathname: () => "/en/profile" }));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ status: "authenticated", user: h.user, wallet: { balance: 10 }, signOut: vi.fn() }),
}));
vi.mock("@/components/layout/promo-banner", () => ({ PromoBanner: () => null }));
vi.mock("@/components/layout/chat-shortcut", () => ({ ChatShortcut: () => null }));
vi.mock("@/components/layout/locale-switcher", () => ({ LocaleSwitcher: () => null }));
vi.mock("@/components/notifications/notification-bell", () => ({ NotificationBell: () => null }));
vi.mock("@/components/psychic/header-actions", () => ({ AdvisorHeaderActions: () => null, WalletPill: () => null }));
vi.mock("@/components/billing/top-up/top-up-provider", () => ({ TopUpLink: ({ children }: { children: React.ReactNode }) => <span>{children}</span> }));

import { SiteHeader } from "@/components/layout/site-header";
import { HelpSupportBlock, UserMenu } from "@/components/layout/user-menu";

beforeAll(stubResizeObserver);
afterEach(cleanup);

const HOURS = "Support is available 24/7. We reply within 1–2 days.";

describe("customer-service details on signed-in pages (§25.4)", () => {
  it("the account menu has Help & support with the support e-mail, a Contact link and the hours", async () => {
    renderWithI18n(<UserMenu />);
    fireEvent.keyDown(screen.getByTestId("user-menu-trigger"), { key: "Enter" });
    const email = await screen.findByTestId("help-support-email");
    expect(email.getAttribute("href")).toBe("mailto:support@staryield.net");
    expect(email.textContent).toBe("support@staryield.net");
    expect(screen.getByTestId("help-support-contact").getAttribute("href")).toBe("/en/contact");
    expect(screen.getByTestId("help-support-hours").textContent).toBe(HOURS);
    expect(screen.getByText("Help & support")).toBeTruthy();
    // The old entries and sign-out are still there.
    expect(screen.getByRole("menuitem", { name: "Sign out" })).toBeTruthy();
    expect(screen.getByRole("menuitem", { name: "Settings" })).toBeTruthy();
  });

  it("the mobile account sheet shows the same details", async () => {
    const u = userEvent.setup();
    renderWithI18n(<SiteHeader />);
    await u.click(screen.getByTestId("mobile-menu-trigger"));
    const block = await screen.findByTestId("help-support");
    expect(within(block).getByRole("link", { name: "support@staryield.net" }).getAttribute("href")).toBe("mailto:support@staryield.net");
    expect(within(block).getByRole("link", { name: "Contact us" }).getAttribute("href")).toBe("/en/contact");
    expect(within(block).getByTestId("help-support-hours").textContent).toBe(HOURS);
    expect(await axeViolations()).toEqual([]);
  });

  it("the block closes the sheet when the Contact link is used", async () => {
    const onNavigate = vi.fn();
    renderWithI18n(<HelpSupportBlock onNavigate={onNavigate} />);
    await userEvent.setup().click(screen.getByTestId("help-support-contact"));
    expect(onNavigate).toHaveBeenCalled();
  });
});
