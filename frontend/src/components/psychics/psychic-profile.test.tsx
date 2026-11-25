// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";

import type { Page, PresenceStatus, Psychic, WaitlistEntry } from "@/lib/api/types";
import { axeViolations, renderWithI18n } from "@/test/render";

const get = vi.fn<(slug: string) => Promise<Psychic>>();
const mine = vi.fn<() => Promise<Page<WaitlistEntry>>>();
const join = vi.fn<(slug: string) => Promise<WaitlistEntry>>();
let session: { status: string; user: { id: string; role: string; age_status: string } | null } = { status: "anonymous", user: null };
const handlers = new Map<string, (d: { slug: string; status: PresenceStatus }) => void>();
const statusHandler = (d: { slug: string; status: PresenceStatus }) => handlers.forEach((h) => h(d));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn(), replace: vi.fn() }), usePathname: () => "/en/psychics/theo" }));
vi.mock("@/lib/api/endpoints", () => ({
  psychicsApi: { get: (s: string) => get(s) },
  waitlistApi: { join: (s: string) => join(s), leave: vi.fn(), mine: () => mine() },
  requestsApi: {},
}));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ ...session, wallet: null, realtime: null }),
  useRealtimeEvent: (type: string, handler: (d: { slug: string; status: PresenceStatus }) => void) => {
    if (type === "psychic.status_changed") handlers.set(new Error().stack?.split("\n")[2] ?? "x", handler);
  },
}));
vi.mock("@/components/layout/site-header", () => ({ SiteHeader: () => null, MobileNavigation: () => null }));
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

beforeEach(() => {
  for (const f of [get, mine, join]) f.mockReset();
  mine.mockResolvedValue({ items: [] });
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
