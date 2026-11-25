// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { FeaturedAdvisor, FeaturedList, Page, PresenceStatus, WaitlistEntry } from "@/lib/api/types";
import { axeViolations, renderWithI18n } from "@/test/render";

const featured = vi.fn<(limit?: number, signal?: AbortSignal) => Promise<FeaturedList>>();
const mine = vi.fn<() => Promise<Page<WaitlistEntry>>>();
const track = vi.fn();
let session: { status: string; user: { id: string; role: string } | null; wallet: null } = { status: "anonymous", user: null, wallet: null };
const handlers = new Map<string, (d: { slug: string; status: PresenceStatus }) => void>();
/** Every subscriber (provider, strip) hears the change, like the real realtime client. */
const statusHandler = (d: { slug: string; status: PresenceStatus }) => handlers.forEach((h) => h(d));

vi.mock("@/lib/api/endpoints", () => ({
  psychicsApi: { featured: (limit?: number, signal?: AbortSignal) => featured(limit, signal) },
  waitlistApi: { join: vi.fn(), leave: vi.fn(), mine: () => mine() },
}));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ ...session, realtime: null }),
  useRealtimeEvent: (type: string, handler: (d: { slug: string; status: PresenceStatus }) => void) => {
    // One slot per subscriber (the call site), refreshed on every render.
    if (type === "psychic.status_changed") handlers.set(new Error().stack?.split("\n")[2] ?? "x", handler);
  },
}));
vi.mock("@/lib/site-config/site-config-provider", () => ({ useFreeMode: () => false, useSignupFreeMinutes: () => 10 }));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => ({ show: vi.fn(), showError: vi.fn() }) }));
vi.mock("@/lib/analytics", () => ({ track: (...args: unknown[]) => track(...args) }));

const { AdvisorStrip } = await import("@/components/booking/advisor-strip");
const { WaitlistProvider } = await import("@/lib/booking/waitlist-provider");

function advisor(slug: string, status: PresenceStatus): FeaturedAdvisor {
  return {
    slug,
    name: slug[0].toUpperCase() + slug.slice(1),
    headline: "Intuitive insights",
    image_url: `https://advisors.test/${slug}.jpg`,
    zodiac: "Leo",
    skills: ["Tarot"],
    rating: 4.9,
    reviews_count: 40,
    readings_count: 900,
    years_experience: 7,
    rate_per_minute: 30,
    status,
    featured: false,
    cta: status === "online" ? "chat" : "waitlist",
  };
}
const list = (...items: FeaturedAdvisor[]): FeaturedList => ({ items, generated_at: "2026-11-05T12:00:00Z" });
const rows = () => screen.getAllByTestId("advisor-strip-item");

beforeEach(() => {
  for (const f of [featured, mine, track]) f.mockReset();
  mine.mockResolvedValue({ items: [] });
  session = { status: "anonymous", user: null, wallet: null };
  handlers.clear();
});
afterEach(cleanup);

describe("AdvisorStrip", () => {
  it("shows a heading, loading, then up to three advisors online first, each with the standard button", async () => {
    featured.mockResolvedValue(list(advisor("solomon", "offline"), advisor("theo", "busy"), advisor("ramone", "online"), advisor("extra", "online")));
    renderWithI18n(<AdvisorStrip heading="Talk to an advisor now" placement="calculator_result" />);
    expect(screen.getByRole("status").textContent).toBe("Loading advisors…");
    expect(screen.getByRole("heading", { name: "Talk to an advisor now" })).toBeTruthy();
    await screen.findAllByTestId("advisor-strip-item");
    expect(featured.mock.calls[0][0]).toBe(3);
    expect(rows()).toHaveLength(3);
    expect(rows().map((r) => within(r).getByText(/^(Ramone|Extra|Theo|Solomon)$/).textContent)).toEqual(["Ramone", "Extra", "Theo"]);
    const first = within(rows()[0]);
    expect(first.getByText("Online")).toBeTruthy();
    // Online with the new-member offer on the button: the row still shows the rate, so the price is in view.
    expect(first.getByTestId("advisor-price").textContent).toBe("30 credits/min");
    // Busy: the row says what a reading costs, the button offers the waiting list.
    const third = within(rows()[2]);
    expect(third.getByText("Busy")).toBeTruthy();
    expect(third.getByTestId("advisor-price").textContent).toBe("30 credits/min");
    expect(third.getByRole("link", { name: /Join the waiting list/ }).getAttribute("href")).toBe("/en/signup?next=%2Fen%2Fpsychics%2Ftheo");
    const link = first.getByRole("link", { name: /Chat now/ });
    expect(link.textContent).toContain("10 free minutes for new members");
    expect(link.getAttribute("href")).toBe("/en/signup?next=%2Fen%2Fchatroom%3Freader%3Dramone");
    fireEvent.click(link);
    expect(track).toHaveBeenCalledWith("book_cta_clicked", { slug: "ramone", status: "online", placement: "calculator_result", action: "signup" });
    expect(await axeViolations()).toEqual([]);
  });

  it("follows live status changes and keeps online advisors first", async () => {
    featured.mockResolvedValue(list(advisor("ramone", "online"), advisor("theo", "busy")));
    renderWithI18n(<AdvisorStrip heading="Talk to an advisor now" />);
    await screen.findAllByTestId("advisor-strip-item");
    act(() => statusHandler({ slug: "theo", status: "online" }));
    act(() => statusHandler({ slug: "ramone", status: "away" }));
    await waitFor(() => expect(within(rows()[0]).getByText("Theo")).toBeTruthy());
    expect(within(rows()[0]).getByRole("link", { name: /Chat now/ })).toBeTruthy();
  });

  it("stays current for visitors, who have no realtime: a refresh every 30 seconds while visible", async () => {
    featured
      .mockResolvedValueOnce(list(advisor("ramone", "online"), advisor("theo", "busy")))
      .mockResolvedValue(list(advisor("theo", "online"), advisor("ramone", "offline")));
    vi.useFakeTimers();
    try {
      renderWithI18n(<AdvisorStrip heading="Talk to an advisor now" />);
      await act(async () => {});
      expect(within(rows()[0]).getByText("Ramone")).toBeTruthy();
      // A background tab does not refresh...
      Object.defineProperty(document, "visibilityState", { configurable: true, value: "hidden" });
      await act(async () => vi.advanceTimersByTime(30_000));
      expect(featured).toHaveBeenCalledOnce();
      // ...a visible one does: Ramone went offline, Theo came online and now leads.
      Object.defineProperty(document, "visibilityState", { configurable: true, value: "visible" });
      await act(async () => vi.advanceTimersByTime(30_000));
      expect(featured).toHaveBeenCalledTimes(2);
      expect(within(rows()[0]).getByText("Theo")).toBeTruthy();
      expect(within(rows()[0]).getByRole("link", { name: /Chat now/ }).getAttribute("href")).toBe("/en/signup?next=%2Fen%2Fchatroom%3Freader%3Dtheo");
      expect(within(rows()[1]).getByText("Offline")).toBeTruthy();
    } finally {
      vi.useRealTimers();
      Object.defineProperty(document, "visibilityState", { configurable: true, value: "visible" });
    }
  });

  it("refetches the selection after a live status change (another advisor may now belong in it)", async () => {
    featured.mockResolvedValue(list(advisor("ramone", "online"), advisor("theo", "busy")));
    renderWithI18n(<AdvisorStrip heading="Talk to an advisor now" />);
    await screen.findAllByTestId("advisor-strip-item");
    act(() => statusHandler({ slug: "ramone", status: "offline" }));
    await waitFor(() => expect(featured).toHaveBeenCalledTimes(2));
  });

  it("says so when no advisor is listed", async () => {
    featured.mockResolvedValue(list());
    renderWithI18n(<AdvisorStrip heading="Talk to an advisor now" />);
    expect(await screen.findByText("No advisors are listed right now.")).toBeTruthy();
    expect(screen.queryByTestId("advisor-strip-item")).toBeNull();
  });

  it("explains a failed load and retries", async () => {
    featured
      .mockRejectedValueOnce(new ApiError({ code: "network_error", message: "offline", status: 0 }))
      .mockResolvedValueOnce(list(advisor("ramone", "online")));
    renderWithI18n(<AdvisorStrip heading="Talk to an advisor now" />);
    expect((await screen.findByRole("alert")).textContent).toContain("We couldn't load the advisors.");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("Ramone")).toBeTruthy();
  });

  it("serves its buttons with one waiting-list read, and reuses a page's provider", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client" }, wallet: null };
    featured.mockResolvedValue(list(advisor("theo", "busy"), advisor("solomon", "offline")));
    renderWithI18n(<AdvisorStrip heading="Talk to an advisor now" />);
    await screen.findAllByTestId("advisor-strip-item");
    await waitFor(() => expect(mine).toHaveBeenCalledOnce());
    cleanup();
    mine.mockClear();
    renderWithI18n(
      <WaitlistProvider>
        <AdvisorStrip heading="One" />
        <AdvisorStrip heading="Two" headingLevel={3} />
      </WaitlistProvider>,
    );
    await waitFor(() => expect(screen.getAllByTestId("advisor-strip-item")).toHaveLength(4));
    await waitFor(() => expect(mine).toHaveBeenCalledOnce());
    expect(screen.getByRole("heading", { level: 3, name: "Two" })).toBeTruthy();
  });

  it("never shows the excluded advisor: asks for one more, filters, keeps three online ones", async () => {
    featured.mockResolvedValue(list(advisor("theo", "online"), advisor("ramone", "online"), advisor("solomon", "online"), advisor("extra", "online")));
    renderWithI18n(<AdvisorStrip heading="Other advisors online now" placement="chat_alternatives" excludeSlug="theo" />);
    await screen.findAllByTestId("advisor-strip-item");
    expect(featured.mock.calls[0][0]).toBe(4);
    expect(rows()).toHaveLength(3);
    expect(screen.queryByText("Theo")).toBeNull();
    expect(screen.getByRole("heading", { name: "Other advisors online now" })).toBeTruthy();
  });

  it("with an exclusion shows only advisors who are online", async () => {
    featured.mockResolvedValue(list(advisor("theo", "online"), advisor("ramone", "online"), advisor("solomon", "busy"), advisor("extra", "offline")));
    renderWithI18n(<AdvisorStrip heading="Other advisors online now" excludeSlug="theo" />);
    await screen.findAllByTestId("advisor-strip-item");
    expect(rows().map((r) => within(r).getByText(/^(Ramone|Solomon|Extra)$/).textContent)).toEqual(["Ramone"]);
  });

  it("hides when every advisor is excluded: no orphan heading, no empty message", async () => {
    featured.mockResolvedValue(list(advisor("theo", "online")));
    const { container } = renderWithI18n(<AdvisorStrip heading="Other advisors online now" excludeSlug="theo" />);
    await waitFor(() => expect(featured).toHaveBeenCalledOnce());
    await act(async () => {});
    expect(screen.queryByRole("heading")).toBeNull();
    expect(screen.queryByText("No advisors are listed right now.")).toBeNull();
    expect(screen.queryByTestId("advisor-strip")).toBeNull();
    expect(container.textContent).toBe("");
  });

  it("with an exclusion shows nothing while loading and when the load fails", async () => {
    featured.mockRejectedValue(new ApiError({ code: "network_error", message: "offline", status: 0 }));
    const { container } = renderWithI18n(<AdvisorStrip heading="Other advisors online now" excludeSlug="theo" />);
    expect(container.textContent).toBe("");
    await waitFor(() => expect(featured).toHaveBeenCalledOnce());
    await act(async () => {});
    expect(container.textContent).toBe("");
  });
});
