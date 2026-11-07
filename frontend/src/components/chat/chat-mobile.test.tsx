// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";

vi.mock("next/navigation", () => ({ usePathname: () => "/en/chatroom" }));
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

import { SessionTools, SwitcherQuickLinks } from "@/components/chat/chat-mobile";

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
