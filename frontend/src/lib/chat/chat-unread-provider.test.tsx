// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";

const h = vi.hoisted(() => ({
  session: { status: "authenticated", user: { id: "u1" } } as { status: string; user: { id: string } | null },
  handlers: {} as Record<string, (d: unknown) => void>,
  unreadCount: vi.fn(),
}));

vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => h.session,
  useRealtimeEvent: (type: string, fn: (d: unknown) => void) => {
    h.handlers[type] = fn;
  },
}));
vi.mock("@/lib/api/endpoints", () => ({ chatApi: { unreadCount: (...a: unknown[]) => h.unreadCount(...a) } }));

import { ChatUnreadProvider, useChatUnread } from "@/lib/chat/chat-unread-provider";

function Probe() {
  const { count, conversations } = useChatUnread();
  return <p data-testid="v">{`${count}/${conversations}`}</p>;
}
const ui = () => (
  <ChatUnreadProvider>
    <Probe />
  </ChatUnreadProvider>
);
const value = () => screen.getByTestId("v").textContent;

function setVisibility(state: "visible" | "hidden") {
  Object.defineProperty(document, "visibilityState", { configurable: true, get: () => state });
  document.dispatchEvent(new Event("visibilitychange"));
}

beforeEach(() => {
  h.session = { status: "authenticated", user: { id: "u1" } };
  h.handlers = {};
  h.unreadCount.mockReset().mockResolvedValue({ count: 3, conversations: 2 });
  document.title = "Chatroom — Staryield";
  setVisibility("visible");
});
afterEach(() => cleanup());

describe("ChatUnreadProvider", () => {
  it("fetches the count when authenticated", async () => {
    render(ui());
    await waitFor(() => expect(value()).toBe("3/2"));
    expect(h.unreadCount).toHaveBeenCalledTimes(1);
  });

  it("does not fetch for anonymous visitors", () => {
    h.session = { status: "anonymous", user: null };
    render(ui());
    expect(h.unreadCount).not.toHaveBeenCalled();
    expect(value()).toBe("0/0");
  });

  it("follows realtime chat.unread, including a drop to zero", async () => {
    render(ui());
    await waitFor(() => expect(value()).toBe("3/2"));
    act(() => h.handlers["chat.unread"]({ count: 7, conversations: 3, conversation_id: "c1", conversation_unread: 4 }));
    expect(value()).toBe("7/3");
    act(() => h.handlers["chat.unread"]({ count: 0, conversations: 0 }));
    expect(value()).toBe("0/0");
  });

  it("refetches after a reconnect, on resync_required and when the page becomes visible", async () => {
    render(ui());
    await waitFor(() => expect(h.unreadCount).toHaveBeenCalledTimes(1));
    act(() => h.handlers["hello"]({ reconnected: false }));
    expect(h.unreadCount).toHaveBeenCalledTimes(1);
    act(() => h.handlers["hello"]({ reconnected: true }));
    expect(h.unreadCount).toHaveBeenCalledTimes(2);
    act(() => h.handlers["resync_required"](undefined));
    expect(h.unreadCount).toHaveBeenCalledTimes(3);
    act(() => setVisibility("visible"));
    expect(h.unreadCount).toHaveBeenCalledTimes(4);
  });

  it("swallows failures and keeps the last count", async () => {
    render(ui());
    await waitFor(() => expect(value()).toBe("3/2"));
    h.unreadCount.mockRejectedValue(new Error("boom"));
    act(() => h.handlers["resync_required"](undefined));
    await act(async () => {});
    expect(value()).toBe("3/2");
  });

  it("clamps garbage values", async () => {
    h.unreadCount.mockResolvedValue({ count: -4, conversations: "x" });
    render(ui());
    await act(async () => {});
    expect(value()).toBe("0/0");
  });

  it("resets on sign-out", async () => {
    const { rerender } = render(ui());
    await waitFor(() => expect(value()).toBe("3/2"));
    h.session = { status: "anonymous", user: null };
    rerender(ui());
    expect(value()).toBe("0/0");
    // And a different user never sees the previous count.
    h.session = { status: "authenticated", user: { id: "u2" } };
    h.unreadCount.mockResolvedValue({ count: 1, conversations: 1 });
    rerender(ui());
    expect(value()).toBe("0/0");
    await waitFor(() => expect(value()).toBe("1/1"));
  });

  it("prefixes the tab title while hidden and restores it when visible", async () => {
    render(ui());
    await waitFor(() => expect(value()).toBe("3/2"));
    act(() => setVisibility("hidden"));
    expect(document.title).toBe("(3) Chatroom — Staryield");
    act(() => h.handlers["chat.unread"]({ count: 120, conversations: 4 }));
    expect(document.title).toBe("(99+) Chatroom — Staryield");
    act(() => h.handlers["chat.unread"]({ count: 0, conversations: 0 }));
    expect(document.title).toBe("Chatroom — Staryield");
    act(() => h.handlers["chat.unread"]({ count: 2, conversations: 1 }));
    act(() => setVisibility("visible"));
    expect(document.title).toBe("Chatroom — Staryield");
  });
});
