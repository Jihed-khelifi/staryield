// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { FeaturedAdvisor, FeaturedList, Page, PresenceStatus, WaitlistEntry } from "@/lib/api/types";
import { axeViolations, renderWithI18n } from "@/test/render";

const featured = vi.fn<(limit?: number, signal?: AbortSignal) => Promise<FeaturedList>>();
const join = vi.fn<(slug: string) => Promise<WaitlistEntry>>();
const leave = vi.fn<(slug: string) => Promise<void>>();
const mine = vi.fn<() => Promise<Page<WaitlistEntry>>>();
const push = vi.fn();
const toastShow = vi.fn();
const toastError = vi.fn();
let session: { status: string; user: { id: string; role: string } | null } = { status: "anonymous", user: null };
// Every subscriber (the cards and the waiting-list provider) hears a change, one slot per call site.
const handlers = new Map<string, (d: { slug: string; status: PresenceStatus }) => void>();
const statusHandler = (d: { slug: string; status: PresenceStatus }) => handlers.forEach((h) => h(d));

vi.mock("@/lib/api/endpoints", () => ({
  psychicsApi: { featured: (limit?: number, signal?: AbortSignal) => featured(limit, signal) },
  waitlistApi: { join: (s: string) => join(s), leave: (s: string) => leave(s), mine: () => mine() },
}));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ ...session, wallet: null, realtime: null }),
  useRealtimeEvent: (type: string, handler: (d: { slug: string; status: PresenceStatus }) => void) => {
    if (type === "psychic.status_changed") handlers.set(new Error().stack?.split("\n")[2] ?? "x", handler);
  },
}));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => ({ show: toastShow, showError: toastError }) }));
vi.mock("@/lib/analytics", () => ({ track: vi.fn() }));

const { FeaturedAdvisors, reviewCountLabel } = await import("@/components/content/featured-advisors");

function advisor(slug: string, status: PresenceStatus, reviews = 423): FeaturedAdvisor {
  return {
    slug,
    name: slug[0].toUpperCase() + slug.slice(1),
    headline: "Spiritual Strategy & Intuitive Insights",
    image_url: `https://advisors.test/${slug}.jpg`,
    zodiac: "Leo",
    skills: ["Tarot"],
    rating: 4.9,
    reviews_count: reviews,
    readings_count: 900,
    years_experience: 7,
    rate_per_minute: 30,
    status,
    featured: false,
    cta: status === "online" ? "chat" : "waitlist",
  };
}

const three = (): FeaturedList => ({
  items: [advisor("ramone", "online"), advisor("theo", "busy", 7), advisor("solomon", "offline", 0)],
  generated_at: "2026-11-05T12:00:00Z",
});

/** A client's waiting-list buttons are held until the one read of the lists is back. */
async function ready(name: string, pattern: RegExp) {
  const button = (await within(card(name)).findByRole("button", { name: pattern })) as HTMLButtonElement;
  await waitFor(() => expect(button.disabled).toBe(false));
  return button;
}

function card(name: string) {
  return screen.getByRole("heading", { name }).closest("article") as HTMLElement;
}

beforeEach(() => {
  for (const f of [featured, join, leave, mine, push, toastShow, toastError]) f.mockReset();
  session = { status: "anonymous", user: null };
  handlers.clear();
  mine.mockResolvedValue({ items: [] });
});
afterEach(cleanup);

describe("reviewCountLabel", () => {
  it("rounds down to tens with a plus", () => {
    expect(reviewCountLabel(0, "en")).toBe("0");
    expect(reviewCountLabel(7, "en")).toBe("7");
    expect(reviewCountLabel(423, "en")).toBe("420+");
    expect(reviewCountLabel(12_345, "en")).toBe("12,340+");
  });
});

describe("FeaturedAdvisors", () => {
  it("shows a loading state, then cards with the right call to action", async () => {
    featured.mockResolvedValue(three());
    renderWithI18n(<FeaturedAdvisors />);
    expect(screen.getByRole("status").textContent).toBe("Loading advisors…");
    expect(await screen.findByRole("heading", { name: "Ramone" })).toBeTruthy();
    expect(featured.mock.calls[0][0]).toBe(3);

    const ramone = within(card("Ramone"));
    expect(ramone.getByText("Active Now")).toBeTruthy();
    expect(ramone.getByText("30 credits / min")).toBeTruthy();
    expect(ramone.getByText("4.9 (420+ reviews)")).toBeTruthy();
    expect(ramone.getByRole("link", { name: /Chat now/ }).getAttribute("href")).toBe("/en/signup?next=%2Fen%2Fchatroom%3Freader%3Dramone");

    expect(within(card("Theo")).getByText("Busy")).toBeTruthy();
    expect(within(card("Theo")).getByText("4.9 (7 reviews)")).toBeTruthy();
    expect(within(card("Solomon")).getByText("New advisor")).toBeTruthy();
    expect(within(card("Solomon")).getByRole("link", { name: /Notify me when online/ })).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
  });

  it("puts each card's parts on rows shared with the cards beside it, so they line up whatever the headline", async () => {
    const list = three();
    list.items[1].headline = "A much longer headline that wraps onto several lines. ".repeat(5);
    featured.mockResolvedValue(list);
    renderWithI18n(<FeaturedAdvisors />);
    await screen.findByRole("heading", { name: "Ramone" });
    const cards = screen.getAllByTestId("featured-advisor");
    for (const c of cards) {
      // One child per shared row (the layout itself is measured in e2e/advisor-cards.spec.ts).
      expect(c.className).toMatch(/(^|\s)grid-rows-subgrid(\s|$)/);
      expect(c.className).toMatch(/(^|\s)row-span-5(\s|$)/);
      expect([...c.children].map((el) => el.getAttribute("data-testid") ?? el.tagName.toLowerCase())).toEqual([
        "div",
        "featured-advisor-intro",
        "featured-advisor-divider",
        "dl",
        "book-button",
      ]);
    }
    // Desktop: one column per card, so fewer advisors stay centred.
    expect(cards[0].parentElement!.style.getPropertyValue("--featured-columns")).toBe("3");
  });

  it("gives the row one desktop column per card when fewer advisors are listed", async () => {
    featured.mockResolvedValue({ ...three(), items: three().items.slice(0, 2) });
    renderWithI18n(<FeaturedAdvisors />);
    await screen.findByRole("heading", { name: "Theo" });
    expect(screen.getAllByTestId("featured-advisor")[0].parentElement!.style.getPropertyValue("--featured-columns")).toBe("2");
  });

  it("sends visitors to sign up before joining a waiting list, and reads no waiting list for them", async () => {
    featured.mockResolvedValue(three());
    renderWithI18n(<FeaturedAdvisors />);
    await screen.findByRole("heading", { name: "Theo" });
    const link = within(card("Theo")).getByRole("link", { name: /Join the waiting list/ });
    expect(link.getAttribute("href")).toBe("/en/signup?next=%2Fen%2Fpsychics%2Ftheo");
    expect(join).not.toHaveBeenCalled();
    expect(mine).not.toHaveBeenCalled();
  });

  it("lets a client join and leave, keeping joined lists from the server", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client" } };
    featured.mockResolvedValue(three());
    mine.mockResolvedValue({ items: [{ slug: "solomon", name: "Solomon", image_url: "", status: "offline", joined_at: "2026-11-01T00:00:00Z" }] });
    join.mockResolvedValue({ slug: "theo", name: "Theo", image_url: "", status: "busy", joined_at: "2026-11-05T12:00:00Z" });
    leave.mockResolvedValue(undefined);
    renderWithI18n(<FeaturedAdvisors />);
    await screen.findByRole("heading", { name: "Theo" });
    await waitFor(() => expect(within(card("Solomon")).getByRole("button", { name: /On the waiting list/ })).toBeTruthy());
    expect(mine).toHaveBeenCalledOnce(); // one read serves every card

    fireEvent.click(await ready("Theo", /Join the waiting list/));
    await waitFor(() => expect(within(card("Theo")).getByRole("button", { name: /On the waiting list/ })).toBeTruthy());
    expect(join).toHaveBeenCalledWith("theo");
    expect(toastShow).toHaveBeenCalledWith(expect.objectContaining({ tone: "success" }));

    fireEvent.click(within(card("Solomon")).getByRole("button", { name: /On the waiting list/ }));
    await waitFor(() => expect(within(card("Solomon")).getByRole("button", { name: /Notify me when online/ })).toBeTruthy());
    expect(leave).toHaveBeenCalledWith("solomon");
  });

  it("switches the card to online when the advisor came online meanwhile: button, badge and order", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client" } };
    featured.mockResolvedValue(three());
    join.mockRejectedValue(new ApiError({ code: "advisor_available", message: "online", status: 409 }));
    renderWithI18n(<FeaturedAdvisors />);
    await screen.findByRole("heading", { name: "Theo" });
    fireEvent.click(await ready("Theo", /Join the waiting list/));
    await waitFor(() => expect(within(card("Theo")).getByRole("link", { name: /Chat now/ })).toBeTruthy());
    expect(toastShow).toHaveBeenCalledWith(expect.objectContaining({ tone: "info", message: "Theo is online now. Start your chat." }));
    expect(within(card("Theo")).getByText("Active Now")).toBeTruthy();
    expect(screen.getAllByRole("heading", { level: 3 }).map((h) => h.textContent)).toEqual(["Ramone", "Theo", "Solomon"]);
    // A patch, not a reload (the API's selection may lag the 409 by a few seconds).
    expect(featured).toHaveBeenCalledOnce();
    // Theo takes another client: the live change wins again, badge and button alike.
    act(() => statusHandler({ slug: "theo", status: "busy" }));
    expect(within(card("Theo")).getByText("Busy")).toBeTruthy();
    expect(await within(card("Theo")).findByRole("button", { name: /Join the waiting list/ })).toBeTruthy();
  });

  it("reports other join errors without changing the card, and no retry where it cannot help", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client" } };
    featured.mockResolvedValue(three());
    const limit = new ApiError({ code: "limit_reached", message: "You can wait for up to 20 advisors at a time.", status: 409 });
    join.mockRejectedValue(limit);
    renderWithI18n(<FeaturedAdvisors />);
    await screen.findByRole("heading", { name: "Theo" });
    fireEvent.click(await ready("Theo", /Join the waiting list/));
    await waitFor(() => expect(toastError).toHaveBeenCalledWith(limit));
    expect(toastShow).not.toHaveBeenCalled();
    expect(within(card("Theo")).getByRole("button", { name: /Join the waiting list/ })).toBeTruthy();
  });

  it("follows live status changes", async () => {
    featured.mockResolvedValue(three());
    renderWithI18n(<FeaturedAdvisors />);
    await screen.findByRole("heading", { name: "Ramone" });
    act(() => statusHandler({ slug: "ramone", status: "busy" }));
    expect(within(card("Ramone")).getByRole("link", { name: /Join the waiting list/ })).toBeTruthy();
    act(() => statusHandler({ slug: "solomon", status: "online" }));
    expect(within(card("Solomon")).getByRole("link", { name: /Chat now/ })).toBeTruthy();
    expect(within(card("Solomon")).getByRole("link", { name: /Chat now/ }).getAttribute("href")).toContain("/signup?next=");
  });

  it("re-sorts the cards by status tier on live changes and refreshes the selection", async () => {
    featured.mockResolvedValue(three());
    renderWithI18n(<FeaturedAdvisors />);
    await screen.findByRole("heading", { name: "Ramone" });
    const names = () => screen.getAllByRole("heading", { level: 3 }).map((h) => h.textContent);
    expect(names()).toEqual(["Ramone", "Theo", "Solomon"]);
    act(() => statusHandler({ slug: "solomon", status: "away" }));
    expect(names()).toEqual(["Ramone", "Solomon", "Theo"]); // online, away, busy
    act(() => statusHandler({ slug: "ramone", status: "offline" }));
    expect(names()).toEqual(["Solomon", "Theo", "Ramone"]);
    // The change also refetches the selection (the server order may differ).
    await waitFor(() => expect(featured.mock.calls.length).toBeGreaterThan(1));
  });

  it("refreshes every 30 seconds while the page is visible", async () => {
    featured.mockResolvedValue(three());
    vi.useFakeTimers();
    try {
      renderWithI18n(<FeaturedAdvisors />);
      await act(async () => {});
      expect(featured).toHaveBeenCalledOnce();
      await act(async () => vi.advanceTimersByTime(30_000));
      expect(featured).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
    }
  });

  it("explains a failed load and retries", async () => {
    featured
      .mockRejectedValueOnce(new ApiError({ code: "network_error", message: "offline", status: 0 }))
      .mockResolvedValueOnce(three());
    renderWithI18n(<FeaturedAdvisors />);
    expect((await screen.findByRole("alert")).textContent).toContain("We couldn't load the advisors.");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("heading", { name: "Ramone" })).toBeTruthy();
  });
});
