// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { AppNotification } from "@/lib/api/types";

const h = vi.hoisted(() => ({
  session: { status: "authenticated", user: { id: "u1" } } as { status: string; user: { id: string } | null },
  handlers: {} as Record<string, (d: unknown) => void>,
  api: { list: vi.fn(), unreadCount: vi.fn(), markRead: vi.fn(), markAllRead: vi.fn() },
  toast: vi.fn(),
  push: vi.fn(),
  announce: vi.fn(),
  pathname: "/en/profile",
}));

vi.mock("next/navigation", () => ({ usePathname: () => h.pathname, useRouter: () => ({ push: h.push }) }));
vi.mock("@/i18n/i18n-provider", () => ({ useI18n: () => ({ text: (s: string, v?: Record<string, unknown>) => s.replace(/\{(\w+)\}/g, (_, k) => String(v?.[k] ?? "")), locale: "en" }) }));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => ({ show: h.toast }) }));
vi.mock("@/components/feedback/live-region", () => ({ announce: h.announce }));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => h.session,
  useRealtimeEvent: (type: string, fn: (d: unknown) => void) => {
    h.handlers[type] = fn;
  },
}));
vi.mock("@/lib/api/endpoints", () => ({ notificationsApi: h.api }));

import { NotificationsProvider, VISIBLE_REFETCH_MIN_MS, useNotifications } from "@/lib/notifications/notifications-provider";

const note = (id: string, over: Partial<AppNotification> = {}): AppNotification => ({
  id,
  kind: "purchase.succeeded",
  data: { credits: 10 },
  link: "/profile",
  created_at: "2026-11-01T10:00:00Z",
  ...over,
});

let api: ReturnType<typeof useNotifications>;
function Probe() {
  const value = useNotifications();
  React.useEffect(() => {
    api = value;
  });
  return (
    <div>
      <p data-testid="unread">{value.unread}</p>
      <p data-testid="read">{value.items.filter((n) => n.read_at).map((n) => n.id).join(",")}</p>
      <p data-testid="ids">{value.items.map((n) => n.id).join(",")}</p>
      <p data-testid="enabled">{String(value.enabled)}</p>
      <p data-testid="error">{value.error?.code ?? ""}</p>
    </div>
  );
}
const ui = () => (
  <NotificationsProvider>
    <Probe />
  </NotificationsProvider>
);
const unread = () => screen.getByTestId("unread").textContent;
const read = () => screen.getByTestId("read").textContent;
const ids = () => screen.getByTestId("ids").textContent;
const emit = (type: string, data?: unknown) => act(() => h.handlers[type]?.(data));
function setVisibility(state: "visible" | "hidden") {
  Object.defineProperty(document, "visibilityState", { configurable: true, get: () => state });
  act(() => void document.dispatchEvent(new Event("visibilitychange")));
}
function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}
async function load(items = [note("a"), note("b")]) {
  h.api.list.mockResolvedValue({ items });
  render(ui());
  await waitFor(() => expect(unread()).toBe("2"));
  act(() => api.ensureLoaded());
  await waitFor(() => expect(api.loaded).toBe(true));
}

beforeEach(() => {
  h.session = { status: "authenticated", user: { id: "u1" } };
  h.handlers = {};
  h.pathname = "/en/profile";
  for (const fn of Object.values(h.api)) fn.mockReset();
  h.toast.mockReset();
  h.push.mockReset();
  h.announce.mockReset();
  h.api.unreadCount.mockResolvedValue({ count: 2 });
  h.api.list.mockResolvedValue({ items: [] });
  h.api.markRead.mockResolvedValue({ unread: 1 });
  h.api.markAllRead.mockResolvedValue({ unread: 0 });
  setVisibility("visible");
  window.history.replaceState(null, "", "/en/profile");
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("initial load", () => {
  it("loads the unread count with the session and the list when asked", async () => {
    render(ui());
    await waitFor(() => expect(unread()).toBe("2"));
    expect(h.api.list).not.toHaveBeenCalled();
    h.api.list.mockResolvedValue({ items: [note("a")], next_cursor: "c1" });
    act(() => api.ensureLoaded());
    await waitFor(() => expect(ids()).toBe("a"));
    expect(api.hasMore).toBe(true);
    expect(api.loaded).toBe(true);
  });

  it("does nothing for visitors", () => {
    h.session = { status: "anonymous", user: null };
    render(ui());
    expect(h.api.unreadCount).not.toHaveBeenCalled();
    expect(screen.getByTestId("enabled").textContent).toBe("false");
  });

  it("is disabled when the API has no notifications", async () => {
    h.api.unreadCount.mockRejectedValue(new ApiError({ code: "feature_unavailable", message: "x", status: 404 }));
    render(ui());
    await waitFor(() => expect(screen.getByTestId("enabled").textContent).toBe("false"));
  });

  it("keeps the list error and retries it through refresh()", async () => {
    h.api.list.mockRejectedValueOnce(new ApiError({ code: "network_error", message: "x", status: 0 })).mockResolvedValue({ items: [note("a")] });
    render(ui());
    await waitFor(() => expect(unread()).toBe("2"));
    act(() => api.ensureLoaded());
    await waitFor(() => expect(screen.getByTestId("error").textContent).toBe("network_error"));
    act(() => api.refresh());
    await waitFor(() => expect(ids()).toBe("a"));
    expect(screen.getByTestId("error").textContent).toBe("");
  });
});

describe("notification.created", () => {
  it("adds the notification once (dedupe by id), counts it and toasts it", async () => {
    await load();
    emit("notification.created", { notification: note("c", { kind: "export.ready", data: {}, link: "/profile/settings" }) });
    expect(unread()).toBe("3");
    expect(ids()).toBe("c,a,b");
    emit("notification.created", { notification: note("c") });
    expect(unread()).toBe("3");
    expect(ids()).toBe("c,a,b");
    expect(h.toast).toHaveBeenCalledTimes(1);
    expect(h.toast.mock.calls[0][0].message).toContain("Your data export is ready");
  });

  it("does not count a notification that arrives already read", async () => {
    await load();
    emit("notification.created", { notification: note("c", { read_at: "2026-11-01T10:00:00Z" }) });
    expect(unread()).toBe("2");
  });

  it("is not duplicated when the first page already contains it", async () => {
    await load([note("a"), note("b")]);
    emit("notification.created", { notification: note("a") });
    expect(unread()).toBe("2");
    expect(ids()).toBe("a,b");
  });
});

describe("notification.read", () => {
  it("sets the count and marks the listed ids read", async () => {
    await load();
    emit("notification.read", { ids: ["a"], unread: 1 });
    expect(unread()).toBe("1");
    expect(read()).toBe("a");
  });

  it("marks everything read and sets the count", async () => {
    await load();
    emit("notification.read", { all: true, unread: 0 });
    expect(unread()).toBe("0");
    expect(read()).toBe("a,b");
  });

  it("ignores malformed events", async () => {
    await load();
    emit("notification.read", { ids: ["a"] });
    emit("notification.read", null);
    expect(unread()).toBe("2");
    expect(read()).toBe("");
  });

  it("works before the list is loaded (count only)", async () => {
    render(ui());
    await waitFor(() => expect(unread()).toBe("2"));
    emit("notification.read", { all: true, unread: 0 });
    expect(unread()).toBe("0");
  });
});

describe("refetching the count", () => {
  it("refetches on every hello (first connection and reconnects) and on resync_required", async () => {
    render(ui());
    await waitFor(() => expect(h.api.unreadCount).toHaveBeenCalledTimes(1));
    emit("hello", { reconnected: false });
    expect(h.api.unreadCount).toHaveBeenCalledTimes(2);
    emit("hello", { reconnected: true, resumed: true });
    expect(h.api.unreadCount).toHaveBeenCalledTimes(3);
    emit("resync_required");
    expect(h.api.unreadCount).toHaveBeenCalledTimes(4);
  });

  it("refreshes the loaded list when a reconnect could not replay", async () => {
    await load();
    h.api.list.mockClear();
    emit("hello", { reconnected: true, resumed: false });
    await waitFor(() => expect(h.api.list).toHaveBeenCalledTimes(1));
    emit("resync_required");
    await waitFor(() => expect(h.api.list).toHaveBeenCalledTimes(2));
  });

  it("refetches when the page becomes visible, throttled", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-11-01T10:00:00Z"));
    render(ui());
    await vi.waitFor(() => expect(h.api.unreadCount).toHaveBeenCalledTimes(1));
    setVisibility("hidden");
    expect(h.api.unreadCount).toHaveBeenCalledTimes(1);
    setVisibility("visible");
    expect(h.api.unreadCount).toHaveBeenCalledTimes(2);
    setVisibility("visible");
    expect(h.api.unreadCount).toHaveBeenCalledTimes(2);
    vi.setSystemTime(new Date(Date.now() + VISIBLE_REFETCH_MIN_MS + 1));
    setVisibility("visible");
    expect(h.api.unreadCount).toHaveBeenCalledTimes(3);
  });
});

describe("sequence guard", () => {
  it("drops a slow count response that started before a newer event", async () => {
    const slow = deferred<{ count: number }>();
    h.api.unreadCount.mockResolvedValueOnce({ count: 5 }).mockReturnValueOnce(slow.promise);
    render(ui());
    await waitFor(() => expect(unread()).toBe("5"));
    emit("resync_required");
    emit("notification.read", { all: true, unread: 0 });
    expect(unread()).toBe("0");
    await act(async () => slow.resolve({ count: 5 }));
    expect(unread()).toBe("0");
  });

  it("drops a slow count response that started before a notification.created", async () => {
    const slow = deferred<{ count: number }>();
    h.api.unreadCount.mockResolvedValueOnce({ count: 2 }).mockReturnValueOnce(slow.promise);
    render(ui());
    await waitFor(() => expect(unread()).toBe("2"));
    emit("hello", {});
    emit("notification.created", { notification: note("n9") });
    expect(unread()).toBe("3");
    await act(async () => slow.resolve({ count: 2 }));
    expect(unread()).toBe("3");
  });

  it("applies a response when nothing newer happened", async () => {
    const slow = deferred<{ count: number }>();
    h.api.unreadCount.mockResolvedValueOnce({ count: 2 }).mockReturnValueOnce(slow.promise);
    render(ui());
    await waitFor(() => expect(unread()).toBe("2"));
    emit("hello", {});
    await act(async () => slow.resolve({ count: 7 }));
    expect(unread()).toBe("7");
  });
});

describe("marking read", () => {
  it("is optimistic and takes the server's count", async () => {
    await load();
    h.api.markRead.mockResolvedValue({ unread: 1 });
    await act(async () => api.markRead("a"));
    expect(read()).toBe("a");
    expect(unread()).toBe("1");
    expect(h.api.markRead).toHaveBeenCalledWith(["a"]);
  });

  it("rolls back, shows an error toast and retries from it", async () => {
    await load();
    const failure = deferred<{ unread: number }>();
    h.api.markRead.mockReturnValueOnce(failure.promise);
    let call!: Promise<void>;
    act(() => {
      call = api.markRead("a");
    });
    expect(read()).toBe("a");
    expect(unread()).toBe("1");
    await act(async () => {
      failure.reject(new ApiError({ code: "network_error", message: "x", status: 0 }));
      await call;
    });
    expect(read()).toBe("");
    expect(unread()).toBe("2");
    const toast = h.toast.mock.calls.at(-1)![0];
    expect(toast).toMatchObject({ tone: "error", message: "We couldn't mark that notification as read.", actionLabel: "Try again" });
    h.api.markRead.mockResolvedValue({ unread: 1 });
    await act(async () => toast.onAction());
    expect(read()).toBe("a");
    expect(unread()).toBe("1");
  });

  it("does not call the API for a notification that is already read", async () => {
    await load([note("a", { read_at: "2026-11-01T10:00:00Z" }), note("b")]);
    await act(async () => api.markRead("a"));
    expect(h.api.markRead).not.toHaveBeenCalled();
  });

  it("marks all read optimistically, announces it and keeps the server's count", async () => {
    await load();
    await act(async () => api.markAllRead());
    expect(read()).toBe("a,b");
    expect(unread()).toBe("0");
    expect(h.announce).toHaveBeenCalledWith("All notifications marked as read.");
  });

  it("restores the unread rows and count when marking all read fails", async () => {
    await load();
    const failure = deferred<{ unread: number }>();
    h.api.markAllRead.mockReturnValueOnce(failure.promise);
    let call!: Promise<void>;
    act(() => {
      call = api.markAllRead();
    });
    expect(unread()).toBe("0");
    expect(read()).toBe("a,b");
    await act(async () => {
      failure.reject(new ApiError({ code: "service_unavailable", message: "x", status: 503 }));
      await call;
    });
    expect(read()).toBe("");
    expect(unread()).toBe("2");
    expect(h.toast.mock.calls.at(-1)![0]).toMatchObject({ tone: "error", actionLabel: "Try again" });
    expect(h.announce).toHaveBeenCalledWith("We couldn't mark your notifications as read.");
  });

  it("open() marks read and navigates to the link (or the kind's default)", async () => {
    await load([note("a", { link: "/profile/analytics" }), note("b", { kind: "subscription.canceled", data: { reason: "admin" }, link: "" })]);
    act(() => api.open(api.items[0]));
    expect(h.push).toHaveBeenCalledWith("/en/profile/analytics");
    act(() => api.open(api.items[1]));
    expect(h.push).toHaveBeenLastCalledWith("/en/profile/settings");
    await waitFor(() => expect(read()).toBe("a,b"));
  });
});

describe("an opened push notification (sy_n)", () => {
  it("marks it read on load and removes the parameter, keeping the rest of the URL", async () => {
    window.history.replaceState({ keep: 1 }, "", "/en/chatroom?reader=theo&sy_n=n42#top");
    render(ui());
    await waitFor(() => expect(h.api.markRead).toHaveBeenCalledWith(["n42"]));
    expect(window.location.pathname + window.location.search + window.location.hash).toBe("/en/chatroom?reader=theo#top");
    expect(window.history.state).toEqual({ keep: 1 });
  });

  it("marks it read when the service worker reports that it was opened", async () => {
    const listeners = new Set<(e: MessageEvent) => void>();
    const sw = { addEventListener: (_t: string, fn: (e: MessageEvent) => void) => listeners.add(fn), removeEventListener: (_t: string, fn: (e: MessageEvent) => void) => listeners.delete(fn) };
    Object.defineProperty(navigator, "serviceWorker", { configurable: true, value: sw });
    render(ui());
    await waitFor(() => expect(h.api.unreadCount).toHaveBeenCalled());
    act(() => listeners.forEach((fn) => fn({ data: { type: "notification.opened", id: "n7" } } as MessageEvent)));
    await waitFor(() => expect(h.api.markRead).toHaveBeenCalledWith(["n7"]));
    act(() => listeners.forEach((fn) => fn({ data: { type: "other", id: "n8" } } as MessageEvent)));
    act(() => listeners.forEach((fn) => fn({ data: { type: "notification.opened", id: "../x" } } as MessageEvent)));
    expect(h.api.markRead).toHaveBeenCalledTimes(1);
    // @ts-expect-error cleanup
    delete navigator.serviceWorker;
  });

  it("strips an invalid id without calling the API, and leaves a URL without sy_n alone", async () => {
    window.history.replaceState(null, "", "/en/profile?sy_n=%3Cscript%3E");
    render(ui());
    await waitFor(() => expect(window.location.search).toBe(""));
    expect(h.api.markRead).not.toHaveBeenCalled();
  });

  it("waits for a signed-in user (the parameter stays for the sign-in redirect)", () => {
    h.session = { status: "anonymous", user: null };
    window.history.replaceState(null, "", "/en/profile?sy_n=n42");
    render(ui());
    expect(h.api.markRead).not.toHaveBeenCalled();
    expect(window.location.search).toBe("?sy_n=n42");
  });
});

describe("app badge", () => {
  it("is cleared when the app is visible and when nothing is unread", async () => {
    const clear = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clearAppBadge", { configurable: true, value: clear });
    h.api.unreadCount.mockResolvedValue({ count: 3 });
    render(ui());
    await waitFor(() => expect(unread()).toBe("3"));
    const afterLoad = clear.mock.calls.length;
    expect(afterLoad).toBeGreaterThan(0);
    setVisibility("hidden");
    expect(clear).toHaveBeenCalledTimes(afterLoad);
    setVisibility("visible");
    expect(clear.mock.calls.length).toBeGreaterThan(afterLoad);
    const beforeZero = clear.mock.calls.length;
    emit("notification.read", { all: true, unread: 0 });
    await waitFor(() => expect(clear.mock.calls.length).toBeGreaterThan(beforeZero));
    // @ts-expect-error cleanup
    delete navigator.clearAppBadge;
  });

  it("is a no-op where the Badging API does not exist", async () => {
    render(ui());
    await waitFor(() => expect(unread()).toBe("2"));
    setVisibility("visible");
  });
});

describe("another user", () => {
  it("starts from a fresh state", async () => {
    await load();
    h.session = { status: "authenticated", user: { id: "u2" } };
    h.api.unreadCount.mockResolvedValue({ count: 0 });
    cleanup();
    render(ui());
    await waitFor(() => expect(unread()).toBe("0"));
    expect(ids()).toBe("");
  });
});
