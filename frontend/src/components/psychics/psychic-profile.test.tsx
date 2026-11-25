// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { ChatSession, Page, PresenceStatus, Psychic, WaitlistEntry } from "@/lib/api/types";
import { axeViolations, renderWithI18n } from "@/test/render";

const get = vi.fn<(slug: string) => Promise<Psychic>>();
const mine = vi.fn<() => Promise<Page<WaitlistEntry>>>();
const join = vi.fn<(slug: string) => Promise<WaitlistEntry>>();
const sessions = vi.fn<(status?: string) => Promise<Page<ChatSession>>>();
let session: { status: string; user: { id: string; role: string; age_status: string } | null } = { status: "anonymous", user: null };
const handlers = new Map<string, (d: { slug: string; status: PresenceStatus }) => void>();
const statusHandler = (d: { slug: string; status: PresenceStatus }) => handlers.forEach((h) => h(d));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn(), replace: vi.fn() }), usePathname: () => "/en/psychics/theo" }));
vi.mock("@/lib/api/endpoints", () => ({
  psychicsApi: { get: (s: string) => get(s) },
  waitlistApi: { join: (s: string) => join(s), leave: vi.fn(), mine: () => mine() },
  requestsApi: {},
  chatApi: { sessions: (status?: string) => sessions(status) },
}));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ ...session, wallet: null, realtime: null }),
  useRealtimeEvent: (type: string, handler: (d: { slug: string; status: PresenceStatus }) => void) => {
    if (type === "psychic.status_changed") handlers.set(new Error().stack?.split("\n")[2] ?? "x", handler);
  },
}));
// The frames are told apart by markers: the public header carries the menu trigger, the app frame the bottom bar.
vi.mock("@/components/layout/site-header", () => ({
  SiteHeader: ({ publicSite }: { publicSite?: boolean }) => (
    <header data-testid="site-header" data-public={publicSite ? "1" : "0"}>
      {publicSite ? <button type="button" data-testid="mobile-menu-trigger" aria-label="Open menu" /> : null}
    </header>
  ),
  MobileNavigation: () => <nav className="mobile-navigation" aria-label="Main navigation" />,
}));
vi.mock("@/components/layout/site-footer", () => ({ SiteFooter: () => <footer data-testid="site-footer" /> }));
vi.mock("@/components/reviews/psychic-reviews", () => ({ PsychicReviews: () => null }));
vi.mock("@/components/psychics/service-order-form", () => ({ ServiceOrderForm: () => null }));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => ({ show: vi.fn(), showError: vi.fn() }) }));
vi.mock("@/lib/analytics", () => ({ track: vi.fn() }));
vi.mock("@/lib/site-config/site-config-provider", () => ({ useFreeMode: () => false, useSignupFreeMinutes: () => 10 }));

const { PsychicProfile } = await import("@/components/psychics/psychics");

const theo = (status: PresenceStatus): Psychic => ({
  id: "p1",
  slug: "theo",
  name: "Theo",
  headline: "Tarot",
  about: ["About Theo."],
  philosophy: "",
  image_url: "https://advisors.test/theo.jpg",
  zodiac: "Leo",
  skills: ["Tarot"],
  rating: 4.9,
  reviews_count: 12,
  readings_count: 300,
  years_experience: 7,
  rate_per_minute: 30,
  status,
  listed: true,
  services: [],
});

const openSession = (slug: string) =>
  ({ id: `s-${slug}`, status: "active", psychic: { id: `p-${slug}`, role: "psychic", name: slug, slug } }) as unknown as ChatSession;

beforeEach(() => {
  for (const f of [get, mine, join, sessions]) f.mockReset();
  mine.mockResolvedValue({ items: [] });
  sessions.mockResolvedValue({ items: [] });
  handlers.clear();
  session = { status: "anonymous", user: null };
  window.history.replaceState(null, "", "/en/psychics/theo");
});
afterEach(cleanup);

describe("PsychicProfile booking", () => {
  it("offers a visitor the sign-up with the new-member minutes, in the hero and the phone bar", async () => {
    get.mockResolvedValue(theo("online"));
    renderWithI18n(<PsychicProfile slug="theo" />);
    const hero = await screen.findByTestId("profile-ctas");
    const link = within(hero).getByRole("link", { name: /Chat now/ });
    expect(link.getAttribute("href")).toBe("/en/signup?next=%2Fen%2Fchatroom%3Freader%3Dtheo");
    expect(link.textContent).toContain("10 free minutes for new members");
    const bar = screen.getByTestId("booking-bar");
    expect(within(bar).getByRole("link", { name: /Chat now/ })).toBeTruthy();
    expect(bar.className).toContain("booking-bar"); // phones only (globals.css)
    // The page reserves room under the bar so the footer is never covered.
    expect(document.querySelector("main")?.className).toContain("has-booking-bar");
    expect(await axeViolations()).toEqual([]);
  });

  it("shares one waiting-list read and the joined state between the hero and the bar", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client", age_status: "adult" } };
    get.mockResolvedValue(theo("busy"));
    join.mockResolvedValue({ slug: "theo", name: "Theo", image_url: "", status: "busy", joined_at: "2026-11-05T10:00:00Z" });
    renderWithI18n(<PsychicProfile slug="theo" />);
    const hero = await screen.findByTestId("profile-ctas");
    const heroButton = (await within(hero).findByRole("button", { name: /Join the waiting list/ })) as HTMLButtonElement;
    await waitFor(() => expect(heroButton.disabled).toBe(false));
    fireEvent.click(heroButton);
    await within(screen.getByTestId("booking-bar")).findByRole("button", { name: /On the waiting list/ });
    expect(within(hero).getByRole("button", { name: /On the waiting list/ })).toBeTruthy();
    expect(mine).toHaveBeenCalledOnce();
    expect(join).toHaveBeenCalledOnce();
  });

  it("follows the advisor's live status in both places", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client", age_status: "adult" } };
    get.mockResolvedValue(theo("busy"));
    renderWithI18n(<PsychicProfile slug="theo" />);
    await screen.findByTestId("profile-ctas");
    act(() => statusHandler({ slug: "theo", status: "online" }));
    await waitFor(() => expect(screen.getAllByRole("link", { name: /Chat now/ })).toHaveLength(2));
    for (const link of screen.getAllByRole("link", { name: /Chat now/ })) expect(link.getAttribute("href")).toBe("/en/chatroom?reader=theo");
  });

  it("takes a client in a reading with the advisor back to the chat, in the hero and the bar", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client", age_status: "adult" } };
    get.mockResolvedValue(theo("busy"));
    sessions.mockResolvedValue({ items: [openSession("theo")] });
    renderWithI18n(<PsychicProfile slug="theo" />);
    await waitFor(() => expect(screen.getAllByRole("link", { name: /Back to chat/ })).toHaveLength(2));
    for (const link of screen.getAllByRole("link", { name: /Back to chat/ })) expect(link.getAttribute("href")).toBe("/en/chatroom?reader=theo");
    expect(screen.queryByRole("button", { name: /Join the waiting list/ })).toBeNull();
    // Only the open readings are asked for.
    expect(sessions).toHaveBeenCalledWith("requested,accepted,active,disconnected,held");
  });

  it("offers the waiting list when the client's open reading is with another advisor, and reads again when the status changes", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client", age_status: "adult" } };
    get.mockResolvedValue(theo("busy"));
    sessions.mockResolvedValue({ items: [openSession("ramone")] });
    renderWithI18n(<PsychicProfile slug="theo" />);
    const hero = await screen.findByTestId("profile-ctas");
    await within(hero).findByRole("button", { name: /Join the waiting list/ });
    expect(sessions).toHaveBeenCalledOnce();
    // Theo went online, then into a reading with this client: read again.
    sessions.mockResolvedValue({ items: [openSession("theo")] });
    act(() => statusHandler({ slug: "theo", status: "online" }));
    act(() => statusHandler({ slug: "theo", status: "busy" }));
    await within(hero).findByRole("link", { name: /Back to chat/ });
    expect(sessions).toHaveBeenCalledTimes(2);
  });

  it("asks nobody else about readings: visitors, or an advisor who is not busy", async () => {
    get.mockResolvedValue(theo("busy"));
    renderWithI18n(<PsychicProfile slug="theo" />);
    await within(await screen.findByTestId("profile-ctas")).findByRole("link", { name: /Join the waiting list/ });
    cleanup();
    session = { status: "authenticated", user: { id: "u1", role: "client", age_status: "adult" } };
    get.mockResolvedValue(theo("offline"));
    renderWithI18n(<PsychicProfile slug="theo" />);
    await within(await screen.findByTestId("profile-ctas")).findByRole("button", { name: /Notify me when online/ });
    expect(sessions).not.toHaveBeenCalled();
  });

  it("shows advisor accounts why there is no action", async () => {
    session = { status: "authenticated", user: { id: "p1", role: "psychic", age_status: "adult" } };
    get.mockResolvedValue(theo("online"));
    renderWithI18n(<PsychicProfile slug="theo" />);
    await screen.findByTestId("profile-ctas");
    expect(within(screen.getByTestId("profile-ctas")).getByRole("note").textContent).toBe("Advisor accounts can't book readings");
    // Nothing to act on: no phone bar, and no room reserved for it.
    expect(screen.queryByTestId("booking-bar")).toBeNull();
    expect(document.querySelector("main")?.className).not.toContain("has-booking-bar");
    expect(mine).not.toHaveBeenCalled();
  });
});

describe("PsychicProfile new advisor label", () => {
  it("shows \"New advisor\" with no rating, star or review count when nobody has reviewed yet", async () => {
    get.mockResolvedValue({ ...theo("online"), rating: 5, reviews_count: 0 });
    renderWithI18n(<PsychicProfile slug="theo" />);
    const tile = (await screen.findByText("Rating")).closest(".stat-tile") as HTMLElement;
    expect(tile.textContent).toContain("New advisor");
    expect(tile.textContent).not.toContain("5.0");
    expect(tile.textContent).not.toContain("★");
    expect(tile.textContent).not.toContain("0 reviews");
  });

  it("keeps the rating and the review count once there are reviews", async () => {
    get.mockResolvedValue({ ...theo("online"), rating: 4.5, reviews_count: 4 });
    renderWithI18n(<PsychicProfile slug="theo" />);
    const tile = (await screen.findByText("Rating")).closest(".stat-tile") as HTMLElement;
    expect(tile.textContent).toContain("4.5 ★");
    expect(tile.textContent).toContain("4 reviews");
    expect(tile.textContent).not.toContain("New advisor");
  });
});

describe("PsychicProfile frame", () => {
  const publicFrame = () => {
    expect(screen.getByTestId("site-header").getAttribute("data-public")).toBe("1");
    expect(screen.getByTestId("mobile-menu-trigger")).toBeTruthy();
    expect(screen.getByTestId("site-footer")).toBeTruthy();
    expect(document.querySelector(".mobile-navigation")).toBeNull();
    expect(document.querySelector(".public-frame")).not.toBeNull();
    // One main landmark: the profile is the main of the public frame, not nested in another.
    expect(document.querySelectorAll("main")).toHaveLength(1);
  };
  const appFrame = () => {
    expect(screen.getByTestId("site-header").getAttribute("data-public")).toBe("0");
    expect(screen.queryByTestId("site-footer")).toBeNull();
    expect(document.querySelector(".mobile-navigation")).not.toBeNull();
    expect(document.querySelector(".dashboard-page")).not.toBeNull();
  };

  it("gives a signed-out visitor the public header with its menu and the footer, and no bottom bar", async () => {
    get.mockResolvedValue(theo("online"));
    renderWithI18n(<PsychicProfile slug="theo" />);
    await screen.findByTestId("profile-ctas");
    publicFrame();
    expect(document.querySelector("main")?.className).toContain("psychic-profile");
    expect(await axeViolations()).toEqual([]);
  });

  it("keeps the app frame for a signed-in user", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client", age_status: "adult" } };
    get.mockResolvedValue(theo("online"));
    renderWithI18n(<PsychicProfile slug="theo" />);
    await screen.findByTestId("profile-ctas");
    appFrame();
  });

  it("follows the same rule while the profile loads, and when it fails", async () => {
    let resolve: (p: Psychic) => void = () => {};
    get.mockReturnValue(new Promise<Psychic>((r) => (resolve = r)));
    renderWithI18n(<PsychicProfile slug="theo" />);
    publicFrame();
    await act(async () => resolve(theo("online")));
    await screen.findByTestId("profile-ctas");
    cleanup();

    get.mockRejectedValue(new ApiError({ code: "not_found", message: "No such advisor", status: 404 }));
    renderWithI18n(<PsychicProfile slug="ghost" />);
    await screen.findByText("This advisor could not be found.");
    publicFrame();
    cleanup();

    session = { status: "authenticated", user: { id: "u1", role: "client", age_status: "adult" } };
    get.mockReturnValue(new Promise<Psychic>(() => {}));
    renderWithI18n(<PsychicProfile slug="theo" />);
    appFrame();
    cleanup();

    get.mockRejectedValue(new ApiError({ code: "not_found", message: "No such advisor", status: 404 }));
    renderWithI18n(<PsychicProfile slug="ghost" />);
    await screen.findByText("This advisor could not be found.");
    appFrame();
  });
});
