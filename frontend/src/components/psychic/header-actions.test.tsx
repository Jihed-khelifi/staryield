// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";
import type { OwnStatus } from "@/components/psychic/use-own-status";

const mocks = vi.hoisted(() => ({
  psychicsApi: { balance: vi.fn(), get: vi.fn(), setAvailability: vi.fn() },
  session: { user: null as unknown, setUser: vi.fn(), signOut: vi.fn(), realtime: null },
}));
vi.mock("@/lib/api/endpoints", () => ({ psychicsApi: mocks.psychicsApi }));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => mocks.session,
  useRealtimeEvent: () => undefined,
}));
vi.mock("@/components/notifications/notification-bell", () => ({ NotificationBell: () => null }));

import { AdvisorHeaderActions, AdvisorMenu, AvailabilityMenu, WalletPill } from "@/components/psychic/header-actions";

const RAMONE = {
  id: "u1",
  role: "psychic",
  email: "ramone@advisors.staryield.net",
  display_name: "Ramone",
  profile: { nickname: "", avatar_url: "" },
  psychic: { slug: "ramone", name: "Ramone", availability: "online" },
};

beforeAll(stubResizeObserver);
afterEach(cleanup);
beforeEach(() => {
  for (const fn of Object.values(mocks.psychicsApi)) fn.mockReset();
  mocks.session.setUser.mockReset();
  mocks.session.signOut.mockReset();
  mocks.session.user = structuredClone(RAMONE);
});

const own = (over: Partial<OwnStatus> = {}): OwnStatus => ({
  availability: "online",
  status: "online",
  saving: false,
  setAvailability: vi.fn(async () => undefined),
  ...over,
});

function openMenu(trigger: HTMLElement) {
  fireEvent.keyDown(trigger, { key: "Enter" });
}

describe("AvailabilityMenu", () => {
  it("names the state in text and switches between online, away and offline", async () => {
    const state = own();
    renderWithI18n(<AvailabilityMenu own={state} />);
    const trigger = screen.getByTestId("availability-menu");
    expect(trigger.getAttribute("aria-label")).toBe("Availability: Online");
    expect(trigger.textContent).toContain("Online");

    openMenu(trigger);
    const menu = await screen.findByRole("menu");
    const items = within(menu).getAllByRole("menuitemradio");
    expect(items.map((i) => i.textContent)).toEqual(["Online", "Away", "Offline"]);
    expect(items[0].getAttribute("aria-checked")).toBe("true");
    expect(await axeViolations(menu)).toEqual([]);

    fireEvent.click(screen.getByTestId("availability-away"));
    expect(state.setAvailability).toHaveBeenCalledWith("away");
  });

  it("shows Busy read-only while a live chat is open", async () => {
    const state = own({ status: "busy" });
    renderWithI18n(<AvailabilityMenu own={state} />);
    const trigger = screen.getByTestId("availability-menu");
    expect(trigger.getAttribute("data-status")).toBe("busy");
    expect(trigger.textContent).toContain("Busy");
    openMenu(trigger);
    expect(await screen.findByRole("note")).toBeTruthy();
    expect(screen.getByText("You are in a live chat. Clients see you as busy until it ends.")).toBeTruthy();
    for (const item of screen.getAllByRole("menuitemradio")) expect(item.getAttribute("aria-disabled")).toBe("true");
    fireEvent.click(screen.getByTestId("availability-offline"));
    expect(state.setAvailability).not.toHaveBeenCalled();
  });
});

describe("AdvisorHeaderActions", () => {
  it("saves a new availability optimistically", async () => {
    mocks.psychicsApi.balance.mockResolvedValue({ available_cents: 100_000, available_credits: 50_000, currency: "usd", reserved_credits: 0, lifetime_net_credits: 0 });
    mocks.psychicsApi.get.mockResolvedValue({ slug: "ramone", status: "online" });
    let finish!: () => void;
    mocks.psychicsApi.setAvailability.mockImplementation(
      () => new Promise((resolve) => (finish = () => resolve({ slug: "ramone", status: "offline" }))),
    );
    renderWithI18n(
      <ToastProvider>
        <AdvisorHeaderActions />
      </ToastProvider>,
    );
    expect(screen.getByTestId("wallet-pill").textContent).toBe("Wallet : …");
    await waitFor(() => expect(screen.getByTestId("wallet-pill").textContent).toBe("Wallet : 1,000 USD"));

    const trigger = screen.getByTestId("availability-menu");
    openMenu(trigger);
    fireEvent.click(await screen.findByTestId("availability-offline"));
    expect(mocks.psychicsApi.setAvailability).toHaveBeenCalledWith("offline");
    await waitFor(() => expect(trigger.getAttribute("data-status")).toBe("offline"));
    expect(trigger.getAttribute("aria-busy")).toBe("true");
    finish();
    await waitFor(() => expect(mocks.session.setUser).toHaveBeenCalled());
    expect(mocks.session.setUser.mock.calls[0][0].psychic.availability).toBe("offline");
  });

  it("rolls back and explains when saving fails", async () => {
    mocks.psychicsApi.balance.mockResolvedValue({ available_cents: 0, available_credits: 0, currency: "USD", reserved_credits: 0, lifetime_net_credits: 0 });
    mocks.psychicsApi.get.mockResolvedValue({ slug: "ramone", status: "online" });
    mocks.psychicsApi.setAvailability.mockRejectedValue(new ApiError({ code: "network_error", message: "x", status: 0 }));
    const online = vi.spyOn(window.navigator, "onLine", "get").mockReturnValue(false);
    renderWithI18n(
      <ToastProvider>
        <AdvisorHeaderActions />
      </ToastProvider>,
    );
    const trigger = screen.getByTestId("availability-menu");
    openMenu(trigger);
    fireEvent.click(await screen.findByTestId("availability-away"));
    expect(await screen.findByText("You appear to be offline. Check your connection and try again.")).toBeTruthy();
    online.mockRestore();
    expect(trigger.getAttribute("data-status")).toBe("online");
    expect(mocks.session.setUser).not.toHaveBeenCalled();
  });
});

describe("WalletPill", () => {
  it("links to earnings and keeps cents when there are any", async () => {
    mocks.psychicsApi.balance.mockResolvedValue({ available_cents: 2_550, available_credits: 1_275, currency: "EUR", reserved_credits: 0, lifetime_net_credits: 0 });
    renderWithI18n(<WalletPill />);
    const pill = screen.getByTestId("wallet-pill");
    await waitFor(() => expect(pill.textContent).toBe("Wallet : 25.50 EUR"));
    expect(pill.getAttribute("href")).toBe("/en/profile/analytics");
    // Full label by default (the header's compact pill hides it visually below lg).
    expect(pill.querySelector(".max-lg\\:sr-only")).toBeNull();
  });
});

describe("AdvisorMenu", () => {
  it("offers go offline, settings and log out under the advisor's name", async () => {
    const state = own();
    renderWithI18n(<AdvisorMenu own={state} />);
    openMenu(screen.getByTestId("user-menu-trigger"));
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByText("Ramone")).toBeTruthy();
    expect(within(menu).getByRole("menuitem", { name: "Settings" }).getAttribute("href")).toBe("/en/profile/settings");
    expect(await axeViolations(menu)).toEqual([]);
    fireEvent.click(within(menu).getByRole("menuitem", { name: "Go offline" }));
    expect(state.setAvailability).toHaveBeenCalledWith("offline");
  });

  it("offers go online when offline, and logs out", async () => {
    const state = own({ availability: "offline", status: "offline" });
    renderWithI18n(<AdvisorMenu own={state} />);
    openMenu(screen.getByTestId("user-menu-trigger"));
    expect(await screen.findByRole("menuitem", { name: "Go online" })).toBeTruthy();
    fireEvent.click(screen.getByRole("menuitem", { name: "Log out" }));
    expect(mocks.session.signOut).toHaveBeenCalled();
  });
});
