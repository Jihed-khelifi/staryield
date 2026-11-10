// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { ApiError } from "@/lib/api/errors";
import type { AppNotification } from "@/lib/api/types";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

type Value = ReturnType<typeof import("@/lib/notifications/notifications-provider").useNotifications>;

const h = vi.hoisted(() => ({ value: {} as unknown, desktop: true }));
vi.mock("@/lib/notifications/notifications-provider", () => ({ useNotifications: () => h.value }));
vi.mock("@/lib/use-media-query", () => ({ useMediaQuery: () => h.desktop }));

import { NotificationBell } from "@/components/notifications/notification-bell";

const note = (id: string, over: Partial<AppNotification> = {}): AppNotification => ({
  id,
  kind: "purchase.succeeded",
  data: { credits: 50 },
  link: "/profile",
  created_at: new Date().toISOString(),
  ...over,
});

let fns: ReturnType<typeof makeFns>;
function makeFns() {
  return { ensureLoaded: vi.fn(), loadMore: vi.fn(), refresh: vi.fn(), markRead: vi.fn(), markAllRead: vi.fn(), open: vi.fn() };
}
function set(over: Partial<Value> = {}) {
  h.value = {
    enabled: true,
    items: [],
    unread: 0,
    loaded: false,
    loading: false,
    loadingMore: false,
    hasMore: false,
    error: null,
    ...fns,
    ...over,
  };
}

beforeAll(stubResizeObserver);
beforeEach(() => {
  fns = makeFns();
  h.desktop = true;
  set();
});
afterEach(cleanup);

const bell = () => screen.getByRole("button", { name: /^Notifications/ });

describe("NotificationBell", () => {
  it("renders nothing when notifications are unavailable", () => {
    set({ enabled: false });
    const { container } = renderWithI18n(<NotificationBell />);
    expect(container.querySelector("button")).toBeNull();
  });

  it("shows the unread badge, caps it at 99+ and names it for screen readers", () => {
    set({ unread: 3 });
    const { rerender } = renderWithI18n(<NotificationBell />);
    expect(bell().getAttribute("aria-label")).toBe("Notifications, 3 unread");
    expect(screen.getByTestId("notification-badge").textContent).toBe("3");
    set({ unread: 150 });
    rerender(<NotificationBell />);
    expect(screen.getByTestId("notification-badge").textContent).toBe("99+");
  });

  it("has no badge when nothing is unread", () => {
    renderWithI18n(<NotificationBell />);
    expect(screen.queryByTestId("notification-badge")).toBeNull();
    expect(bell().getAttribute("aria-label")).toBe("Notifications");
  });

  it("loads the list when opened and shows the loading state", async () => {
    set({ loading: true });
    renderWithI18n(<NotificationBell />);
    await userEvent.setup().click(bell());
    expect(fns.ensureLoaded).toHaveBeenCalled();
    expect(await screen.findByRole("status")).toHaveProperty("textContent", expect.stringContaining("Loading…"));
  });

  it("shows the empty state", async () => {
    set({ loaded: true });
    renderWithI18n(<NotificationBell />);
    await userEvent.setup().click(bell());
    expect(await screen.findByText("You're all caught up.")).toBeTruthy();
    expect((screen.getByRole("button", { name: "Mark all as read" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("shows the error with a Try again that refreshes", async () => {
    set({ error: new ApiError({ code: "network_error", message: "x", status: 0 }) });
    renderWithI18n(<NotificationBell />);
    const u = userEvent.setup();
    await u.click(bell());
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Try again");
    await u.click(within(alert).getByRole("button", { name: "Try again" }));
    expect(fns.refresh).toHaveBeenCalledTimes(1);
  });

  it("lists notifications with their copy, marks unread ones, and opens one on click", async () => {
    const items = [
      note("a", { kind: "export.ready", data: {}, link: "/profile/settings" }),
      note("b", { kind: "subscription.canceled", data: { reason: "period_ended" }, read_at: new Date().toISOString() }),
    ];
    set({ items, loaded: true, unread: 1 });
    renderWithI18n(<NotificationBell />);
    const u = userEvent.setup();
    await u.click(bell());
    const list = await screen.findByRole("list", { name: "Notifications" });
    expect(within(list).getByText("Your data export is ready")).toBeTruthy();
    expect(within(list).getByText("Membership ended")).toBeTruthy();
    expect(within(list).getByText("Unread:")).toBeTruthy();
    expect(list.querySelectorAll("[data-unread]").length).toBe(1);
    await u.click(within(list).getByText("Your data export is ready"));
    expect(fns.open).toHaveBeenCalledWith(items[0]);
  });

  it("marks all as read when there are unread notifications", async () => {
    set({ items: [note("a")], loaded: true, unread: 1 });
    renderWithI18n(<NotificationBell />);
    const u = userEvent.setup();
    await u.click(bell());
    await u.click(await screen.findByRole("button", { name: "Mark all as read" }));
    expect(fns.markAllRead).toHaveBeenCalledTimes(1);
  });

  it("offers Load more when there are further pages", async () => {
    set({ items: [note("a")], loaded: true, hasMore: true });
    renderWithI18n(<NotificationBell />);
    const u = userEvent.setup();
    await u.click(bell());
    await u.click(await screen.findByRole("button", { name: "Load more" }));
    expect(fns.loadMore).toHaveBeenCalledTimes(1);
  });

  it("opens the same list in a sheet on phones and is accessible", async () => {
    h.desktop = false;
    set({ items: [note("a")], loaded: true, unread: 1 });
    renderWithI18n(<NotificationBell />);
    await userEvent.setup().click(bell());
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByRole("list", { name: "Notifications" })).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
  });
});
