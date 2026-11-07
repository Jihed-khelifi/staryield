// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";

const h = vi.hoisted(() => ({ session: { status: "authenticated", user: { id: "u1", role: "client" } } as { status: string; user: unknown }, count: 0, path: "/en/psychics" }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => h.session }));
vi.mock("@/lib/chat/chat-unread-provider", () => ({ useChatUnread: () => ({ count: h.count, conversations: 1 }) }));
vi.mock("next/navigation", () => ({ usePathname: () => h.path }));
vi.mock("next/link", () => ({ default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => <a href={href} {...rest}>{children}</a> }));

import { ChatShortcut } from "@/components/layout/chat-shortcut";

const dictionary = { ...en, copy } as Dictionary;
const ui = () => (
  <I18nProvider locale="en" dictionary={dictionary}>
    <ChatShortcut />
  </I18nProvider>
);

beforeEach(() => {
  h.session = { status: "authenticated", user: { id: "u1", role: "client" } };
  h.count = 0;
  h.path = "/en/psychics";
});
afterEach(() => cleanup());

describe("ChatShortcut", () => {
  it("is hidden for anonymous visitors and while the session loads", () => {
    h.session = { status: "anonymous", user: null };
    const { container, rerender } = render(ui());
    expect(container.innerHTML).toBe("");
    h.session = { status: "loading", user: null };
    rerender(ui());
    expect(container.innerHTML).toBe("");
  });

  it("links to the chatroom for clients and advisors, without a badge at zero", () => {
    render(ui());
    const link = screen.getByRole("link", { name: "Chat" });
    expect(link.getAttribute("href")).toBe("/en/chatroom");
    expect(screen.queryByTestId("chat-unread-badge")).toBeNull();
    cleanup();
    h.session = { status: "authenticated", user: { id: "p1", role: "psychic" } };
    render(ui());
    expect(screen.getByTestId("chat-shortcut")).toBeTruthy();
  });

  it("shows the count with a localized aria-label (singular and plural)", () => {
    h.count = 1;
    const { rerender } = render(ui());
    expect(screen.getByRole("link", { name: "Chat, 1 unread message" })).toBeTruthy();
    h.count = 3;
    rerender(ui());
    expect(screen.getByRole("link", { name: "Chat, 3 unread messages" })).toBeTruthy();
    expect(screen.getByTestId("chat-unread-badge").textContent).toBe("3");
  });

  it("caps the badge at 99+", () => {
    h.count = 99;
    const { rerender } = render(ui());
    expect(screen.getByTestId("chat-unread-badge").textContent).toBe("99");
    h.count = 100;
    rerender(ui());
    expect(screen.getByTestId("chat-unread-badge").textContent).toBe("99+");
    expect(screen.getByRole("link", { name: "Chat, 99+ unread messages" })).toBeTruthy();
  });

  it("pops only when the count grows", () => {
    h.count = 2;
    const { rerender } = render(ui());
    expect(screen.getByTestId("chat-unread-badge").getAttribute("data-pop")).toBeNull();
    h.count = 3;
    rerender(ui());
    expect(screen.getByTestId("chat-unread-badge").hasAttribute("data-pop")).toBe(true);
  });

  it("marks the current page", () => {
    h.path = "/en/chatroom";
    render(ui());
    expect(screen.getByTestId("chat-shortcut").getAttribute("aria-current")).toBe("page");
  });
});
