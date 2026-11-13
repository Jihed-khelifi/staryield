// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";

vi.mock("next/navigation", () => ({ usePathname: () => "/en/chatroom", useRouter: () => ({ push: vi.fn() }) }));
vi.mock("next/link", () => ({ default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => <a href={href} {...rest}>{children}</a> }));
vi.mock("@/components/layout/site-header", async () => {
  const { Sun, Sparkles, MessageCircle, UserRound } = await import("lucide-react");
  return {
    MOBILE_NAV_LINKS: [
      { label: "Today", href: "/profile", icon: Sun },
      { label: "Psychics", href: "/psychics", icon: Sparkles },
      { label: "Chat", href: "/chatroom", icon: MessageCircle },
      { label: "Profile", href: "/profile/chart", icon: UserRound },
    ],
    isMobileNavActive: (path: string, locale: string, _l: string, href: string) => path === `/${locale}${href}`,
  };
});

const session = vi.hoisted(() => ({ current: { user: { id: "u1", display_name: "Jo", profile: { nickname: "Jo" } }, wallet: { balance: 1234 } } as Record<string, unknown> }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => session.current }));
vi.mock("@/components/layout/user-menu", () => ({ UserAvatar: () => <span data-testid="avatar" /> }));
vi.mock("@/components/notifications/notification-bell", () => ({ NotificationBell: () => <button type="button">Notifications</button> }));
vi.mock("@/components/psychic/use-own-status", () => ({ useOwnStatus: () => ({ availability: "online", status: "online", saving: false, setAvailability: async () => undefined }) }));
vi.mock("@/components/psychic/header-actions", () => ({
  WalletPill: () => <span>Wallet : $10</span>,
  AvailabilityMenu: () => <button type="button" data-testid="availability-menu">Online</button>,
}));
vi.mock("@/components/chat/report-issue", () => ({
  ReportIssueButton: ({ placement }: { placement: string }) => <button type="button" data-testid="report-issue-trigger" data-placement={placement}>Report</button>,
}));

// revisions 5.0: the dialog itself is tested with the top-up; here only what the pills hand to it.
vi.mock("@/components/billing/top-up/credit-top-up", () => ({
  CreditTopUp: ({ context }: { context?: unknown }) => <div data-testid="top-up-stub">{JSON.stringify(context ?? null)}</div>,
}));

import { TopUpProvider } from "@/components/billing/top-up/top-up-provider";
import { SessionTools, SwitcherAccount, SwitcherQuickLinks } from "@/components/chat/chat-mobile";

const dictionary = { ...en, copy } as Dictionary;
const wrap = (node: React.ReactNode) => <I18nProvider locale="en" dictionary={dictionary}>{node}</I18nProvider>;

afterEach(() => cleanup());

describe("SwitcherQuickLinks", () => {
  it("lists the former bottom-bar destinations and marks the current one", () => {
    const onNavigate = vi.fn();
    render(wrap(<SwitcherQuickLinks onNavigate={onNavigate} />));
    const links = screen.getAllByRole("link");
    expect(links.map((l) => l.textContent)).toEqual(["Today", "Psychics", "Chat", "Profile"]);
    expect(links.map((l) => l.getAttribute("href"))).toEqual(["/en/profile", "/en/psychics", "/en/chatroom", "/en/profile/chart"]);
    expect(screen.getByRole("link", { name: "Chat" }).getAttribute("aria-current")).toBe("page");
    fireEvent.click(links[0]);
    expect(onNavigate).toHaveBeenCalled();
  });
});

describe("SessionTools in free mode", () => {
  const wallet = { balance: 40, free_minutes: 0 } as never;
  it("shows the balance and a top-up link normally", () => {
    render(wrap(<SessionTools role="client" wallet={wallet} returnPath="/en/chatroom" onOpenDetails={() => undefined} />));
    expect(screen.getByRole("link").getAttribute("href")).toContain("/en/credits");
  });
  it("replaces them with a Free chat pill when chats are free", () => {
    render(wrap(<SessionTools role="client" wallet={wallet} free returnPath="/en/chatroom" onOpenDetails={() => undefined} />));
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.getByTestId("free-chat-pill").textContent).toBe("Free chat");
  });
});

describe("SessionTools on phones (§20.2)", () => {
  const wallet = { balance: 40, free_minutes: 0 } as never;
  const report = { conversationId: "c1", advisorName: "Theo" };
  it("client: credits pill, then report, then bookmarks", () => {
    render(wrap(<SessionTools role="client" wallet={wallet} returnPath="/en/chatroom" onOpenDetails={() => undefined} report={report} />));
    const row = screen.getByTestId("chat-session-tools");
    const order = Array.from(row.querySelectorAll("a, button")).map((el) => el.getAttribute("data-testid") ?? el.textContent);
    expect(order[0]).toContain("40");
    expect(order[1]).toBe("report-issue-trigger");
    expect(order[2]).toBe("Bookmarks");
    expect(screen.getByTestId("report-issue-trigger").getAttribute("data-placement")).toBe("tools");
  });
  it("client without a conversation has no report button; free chat keeps it", () => {
    const { unmount } = render(wrap(<SessionTools role="client" wallet={wallet} returnPath="/en/chatroom" onOpenDetails={() => undefined} />));
    expect(screen.queryByTestId("report-issue-trigger")).toBeNull();
    unmount();
    render(wrap(<SessionTools role="client" wallet={wallet} free returnPath="/en/chatroom" onOpenDetails={() => undefined} report={report} />));
    expect(screen.getByTestId("free-chat-pill")).toBeTruthy();
    expect(screen.getByTestId("report-issue-trigger")).toBeTruthy();
  });
  it("advisor: earnings pill during a session, compact availability otherwise; never a report button", () => {
    const { unmount } = render(wrap(<SessionTools role="psychic" wallet={null} earned="$4.00" returnPath="/en/chatroom" onOpenDetails={() => undefined} report={report} />));
    expect(screen.getByTestId("session-earnings-pill").textContent).toContain("$4.00");
    expect(screen.queryByTestId("availability-menu")).toBeNull();
    expect(screen.queryByTestId("report-issue-trigger")).toBeNull();
    unmount();
    render(wrap(<SessionTools role="psychic" wallet={null} returnPath="/en/chatroom" onOpenDetails={() => undefined} />));
    expect(screen.getByTestId("availability-menu")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Client details" })).toBeTruthy();
  });
});

describe("SwitcherAccount (drawer account block)", () => {
  it("client: name, bell and a balance link back to the chatroom", () => {
    const onNavigate = vi.fn();
    render(wrap(<SwitcherAccount role="client" returnPath="/en/chatroom?reader=theo" onNavigate={onNavigate} />));
    expect(screen.getByText("Jo")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Notifications" })).toBeTruthy();
    const pill = screen.getByTestId("drawer-balance-pill");
    expect(pill.getAttribute("href")).toBe("/en/credits?next=%2Fen%2Fchatroom%3Freader%3Dtheo");
    expect(pill.textContent).toContain("1,234");
    fireEvent.click(pill);
    expect(onNavigate).toHaveBeenCalled();
  });
  it("client in free mode shows Free chat instead of a link", () => {
    render(wrap(<SwitcherAccount role="client" returnPath="/en/chatroom" free />));
    expect(screen.getByTestId("drawer-free-pill").textContent).toBe("Free chat");
    expect(screen.queryByTestId("drawer-balance-pill")).toBeNull();
  });
  it("advisor: wallet pill and availability menu", () => {
    render(wrap(<SwitcherAccount role="psychic" returnPath="/en/chatroom" />));
    expect(screen.getByText(/Wallet/)).toBeTruthy();
    expect(screen.getByTestId("availability-menu")).toBeTruthy();
    expect(screen.queryByTestId("drawer-balance-pill")).toBeNull();
  });
  it("is phone only (md:hidden)", () => {
    render(wrap(<SwitcherAccount role="client" returnPath="/en/chatroom" />));
    expect(screen.getByTestId("switcher-account").className).toContain("md:hidden");
  });
});

describe("balance pills open the top-up in place (revisions 5.0, §24.7)", () => {
  const withStatus = () => {
    session.current = { ...session.current, status: "authenticated" };
  };
  it("the tools row pill opens the dialog with the live advisor's rate, keeping the href", () => {
    withStatus();
    const wallet = { balance: 40, free_minutes: 0 } as never;
    render(wrap(<TopUpProvider><SessionTools role="client" wallet={wallet} returnPath="/en/chatroom?reader=theo" onOpenDetails={() => undefined} topUp={{ advisorName: "Theo", ratePerMinute: 35 }} /></TopUpProvider>));
    const pill = screen.getByRole("link", { name: /Top up credits/ });
    expect(pill.getAttribute("href")).toBe("/en/credits?next=%2Fen%2Fchatroom%3Freader%3Dtheo");
    expect(fireEvent.click(pill)).toBe(false);
    expect(JSON.parse(screen.getByTestId("top-up-stub").textContent ?? "")).toEqual({ advisorName: "Theo", ratePerMinute: 35 });
  });
  it("the drawer pill closes the drawer and opens the dialog", () => {
    withStatus();
    const onNavigate = vi.fn();
    render(wrap(<TopUpProvider><SwitcherAccount role="client" returnPath="/en/chatroom" onNavigate={onNavigate} topUp={{ advisorName: "Solomon", ratePerMinute: 45 }} /></TopUpProvider>));
    fireEvent.click(screen.getByTestId("drawer-balance-pill"));
    expect(onNavigate).toHaveBeenCalled();
    expect(JSON.parse(screen.getByTestId("top-up-stub").textContent ?? "")).toEqual({ advisorName: "Solomon", ratePerMinute: 45 });
  });
});
