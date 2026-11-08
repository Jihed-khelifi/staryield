// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";

const h = vi.hoisted(() => ({
  session: { status: "authenticated", user: { id: "u1", role: "psychic" } } as { status: string; user: { id: string; role: string } | null },
  handler: null as null | ((type: string, data: unknown) => void),
  pathname: "/en/profile",
  sessions: vi.fn(),
  ringtone: { status: "ringing", pending: 0 } as { status: string; pending: number },
  setPending: vi.fn(),
  preload: vi.fn(),
  unlock: vi.fn(),
  showNotification: vi.fn(),
  closeNotification: vi.fn(),
}));

vi.mock("next/link", () => ({ default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => <a href={href} {...rest}>{children}</a> }));
vi.mock("next/navigation", () => ({ usePathname: () => h.pathname }));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => h.session,
  useRealtimeEvents: (_types: readonly string[], fn: (type: string, data: unknown) => void) => {
    h.handler = fn;
  },
}));
vi.mock("@/lib/api/endpoints", () => ({ chatApi: { sessions: (...a: unknown[]) => h.sessions(...a) } }));
vi.mock("@/lib/chat/ringtone", () => ({
  preloadRingtone: () => h.preload(),
  setPendingRequests: (n: number) => h.setPending(n),
  unlockRingtone: () => h.unlock(),
  useRingtoneState: () => h.ringtone,
}));
vi.mock("@/lib/chat/request-notifications", () => ({
  showRequestNotification: (o: unknown) => h.showNotification(o),
  closeRequestNotification: (id: string) => h.closeNotification(id),
}));
vi.mock("@/lib/push", () => ({ pushPermission: () => "granted" }));
vi.mock("@/components/feedback/live-region", () => ({ announce: vi.fn() }));

import { IncomingRequestRinger } from "@/components/chat/incoming-request-ringer";

const dictionary = { ...en, copy } as Dictionary;
const iso = (sec: number) => new Date(Date.now() + sec * 1000).toISOString();
const req = (id: string, name: string, over: object = {}) => ({
  id,
  status: "requested",
  client: { id: `c-${id}`, name },
  requested_at: iso(-2),
  expires_at: iso(60),
  server_time: iso(0),
  ...over,
});
const ui = () => (
  <I18nProvider locale="en" dictionary={dictionary}>
    <IncomingRequestRinger />
  </I18nProvider>
);
const emit = (type: string, data: unknown = {}) => act(() => h.handler?.(type, data));
const lastPending = () => h.setPending.mock.calls.at(-1)?.[0];

function setVisibility(state: "visible" | "hidden") {
  Object.defineProperty(document, "visibilityState", { configurable: true, get: () => state });
}

beforeEach(() => {
  h.session = { status: "authenticated", user: { id: "u1", role: "psychic" } };
  h.handler = null;
  h.pathname = "/en/profile";
  h.ringtone = { status: "ringing", pending: 0 };
  h.sessions.mockReset().mockResolvedValue({ items: [] });
  h.setPending.mockReset();
  h.preload.mockReset();
  h.unlock.mockReset();
  h.showNotification.mockReset().mockResolvedValue(true);
  h.closeNotification.mockReset().mockResolvedValue(undefined);
  setVisibility("visible");
});
afterEach(() => cleanup());

describe("IncomingRequestRinger", () => {
  it("renders nothing and loads nothing for clients", () => {
    h.session = { status: "authenticated", user: { id: "u2", role: "client" } };
    const { container } = render(ui());
    expect(container.innerHTML).toBe("");
    expect(h.sessions).not.toHaveBeenCalled();
    expect(h.preload).not.toHaveBeenCalled();
    expect(h.setPending).not.toHaveBeenCalled();
  });

  it("renders nothing for anonymous visitors", () => {
    h.session = { status: "anonymous", user: null };
    render(ui());
    expect(h.sessions).not.toHaveBeenCalled();
  });

  it("preloads and loads the pending requests for advisors, then rings and shows the banner", async () => {
    h.sessions.mockResolvedValue({ items: [req("a", "Maya")] });
    render(ui());
    expect(h.preload).toHaveBeenCalled();
    expect(h.sessions).toHaveBeenCalledWith("requested");
    // First advisor render of the file: allow for a loaded parallel run.
    const banner = await screen.findByTestId("incoming-request-banner", {}, { timeout: 5000 });
    expect(banner.textContent).toContain("Maya would like to start a live chat.");
    expect(banner.getAttribute("data-ringtone")).toBe("ringing");
    expect(screen.getByRole("link", { name: "Open chat" }).getAttribute("href")).toBe("/en/chatroom");
    expect(lastPending()).toBe(1);
  });

  it("hides the banner in the chatroom but keeps ringing", async () => {
    h.pathname = "/en/chatroom";
    h.sessions.mockResolvedValue({ items: [req("a", "Maya")] });
    render(ui());
    await waitFor(() => expect(lastPending()).toBe(1));
    expect(screen.queryByTestId("incoming-request-banner")).toBeNull();
  });

  it("mirrors the ringtone status on <html> (the chatroom has no banner) and clears it on unmount", async () => {
    h.pathname = "/en/chatroom";
    const { unmount } = render(ui());
    await waitFor(() => expect(document.documentElement.dataset.ringtone).toBe("ringing"));
    unmount();
    expect(document.documentElement.dataset.ringtone).toBeUndefined();
  });

  it("is a labelled region, not a live region (the countdown ticks every second), announced once per request", async () => {
    const { announce } = await import("@/components/feedback/live-region");
    vi.mocked(announce).mockClear();
    render(ui());
    await waitFor(() => expect(h.sessions).toHaveBeenCalled());
    emit("session.requested", { session: req("a", "Maya") });
    const banner = await screen.findByRole("region", { name: "Incoming chat request" });
    expect(banner.getAttribute("aria-live")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(announce).toHaveBeenCalledTimes(1);
    emit("session.requested", { session: req("a", "Maya") });
    expect(announce).toHaveBeenCalledTimes(1);
  });

  it("does not announce in the chatroom, where the request card speaks", async () => {
    const { announce } = await import("@/components/feedback/live-region");
    vi.mocked(announce).mockClear();
    h.pathname = "/en/chatroom";
    render(ui());
    await waitFor(() => expect(h.sessions).toHaveBeenCalled());
    emit("session.requested", { session: req("a", "Maya") });
    await waitFor(() => expect(lastPending()).toBe(1));
    expect(announce).not.toHaveBeenCalled();
  });

  it("a live request rings; accepting stops it", async () => {
    render(ui());
    await waitFor(() => expect(h.sessions).toHaveBeenCalled());
    emit("session.requested", { session: req("a", "Maya") });
    expect(await screen.findByTestId("incoming-request-banner")).toBeTruthy();
    expect(lastPending()).toBe(1);
    emit("session.accepted", { session: req("a", "Maya", { status: "accepted" }) });
    expect(screen.queryByTestId("incoming-request-banner")).toBeNull();
    expect(lastPending()).toBe(0);
  });

  it("shows the newest request and a count of the others", async () => {
    h.sessions.mockResolvedValue({ items: [req("a", "Maya", { requested_at: iso(-30) }), req("b", "Noor", { requested_at: iso(-1) })] });
    render(ui());
    const banner = await screen.findByTestId("incoming-request-banner");
    expect(banner.textContent).toContain("Noor would like");
    expect(banner.textContent).toContain("+1 more");
  });

  it("refetches after hello and resync_required and when the tab becomes visible", async () => {
    render(ui());
    await waitFor(() => expect(h.sessions).toHaveBeenCalledTimes(1));
    emit("hello");
    await waitFor(() => expect(h.sessions).toHaveBeenCalledTimes(2));
    emit("resync_required");
    await waitFor(() => expect(h.sessions).toHaveBeenCalledTimes(3));
    act(() => {
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await waitFor(() => expect(h.sessions).toHaveBeenCalledTimes(4));
  });

  it("a snapshot after a reconnect drops requests that were handled meanwhile", async () => {
    h.sessions.mockResolvedValueOnce({ items: [req("a", "Maya")] });
    render(ui());
    await screen.findByTestId("incoming-request-banner");
    h.sessions.mockResolvedValueOnce({ items: [] });
    emit("hello");
    await waitFor(() => expect(screen.queryByTestId("incoming-request-banner")).toBeNull());
    expect(lastPending()).toBe(0);
  });

  it("offers to turn the ringtone on when blocked, and the tap unlocks", async () => {
    h.ringtone = { status: "blocked", pending: 1 };
    h.sessions.mockResolvedValue({ items: [req("a", "Maya")] });
    render(ui());
    const tap = await screen.findByRole("button", { name: "Tap to turn on the ringtone" });
    fireEvent.click(tap);
    expect(h.unlock).toHaveBeenCalled();
    expect(screen.getByTestId("incoming-request-banner").getAttribute("data-ringtone")).toBe("blocked");
  });

  it("shows no sound button while ringing", async () => {
    h.sessions.mockResolvedValue({ items: [req("a", "Maya")] });
    render(ui());
    await screen.findByTestId("incoming-request-banner");
    expect(screen.queryByRole("button", { name: "Tap to turn on the ringtone" })).toBeNull();
  });

  it("removes a request at its expiry without any event", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      h.sessions.mockResolvedValue({ items: [req("a", "Maya", { expires_at: iso(2) })] });
      render(ui());
      await screen.findByTestId("incoming-request-banner");
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2600);
      });
      expect(screen.queryByTestId("incoming-request-banner")).toBeNull();
      expect(lastPending()).toBe(0);
    } finally {
      vi.useRealTimers();
    }
  });

  it("shows one system notification per request while hidden and closes it when handled", async () => {
    setVisibility("hidden");
    render(ui());
    await waitFor(() => expect(h.sessions).toHaveBeenCalled());
    emit("session.requested", { session: req("a", "Maya") });
    await waitFor(() => expect(h.showNotification).toHaveBeenCalledTimes(1));
    expect(h.showNotification.mock.calls[0][0]).toMatchObject({ sessionId: "a", title: "New chat request", url: "/en/chatroom" });
    emit("session.requested", { session: req("a", "Maya") });
    expect(h.showNotification).toHaveBeenCalledTimes(1);
    emit("session.cancelled", { session: req("a", "Maya", { status: "cancelled" }) });
    await waitFor(() => expect(h.closeNotification).toHaveBeenCalledWith("a"));
  });

  it("shows no system notification while the tab is visible", async () => {
    render(ui());
    await waitFor(() => expect(h.sessions).toHaveBeenCalled());
    emit("session.requested", { session: req("a", "Maya") });
    await screen.findByTestId("incoming-request-banner");
    expect(h.showNotification).not.toHaveBeenCalled();
  });

  it("stops ringing when it unmounts", async () => {
    h.sessions.mockResolvedValue({ items: [req("a", "Maya")] });
    const { unmount } = render(ui());
    await waitFor(() => expect(lastPending()).toBe(1));
    unmount();
    expect(lastPending()).toBe(0);
  });
});
