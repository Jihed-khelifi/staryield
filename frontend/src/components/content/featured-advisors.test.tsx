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
let statusHandler: ((d: { slug: string; status: PresenceStatus }) => void) | null = null;

vi.mock("@/lib/api/endpoints", () => ({
  psychicsApi: { featured: (limit?: number, signal?: AbortSignal) => featured(limit, signal) },
  waitlistApi: { join: (s: string) => join(s), leave: (s: string) => leave(s), mine: () => mine() },
}));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ ...session, realtime: null }),
  useRealtimeEvent: (type: string, handler: (d: { slug: string; status: PresenceStatus }) => void) => {
    if (type === "psychic.status_changed") statusHandler = handler;
  },
}));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));
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

function card(name: string) {
  return screen.getByRole("heading", { name }).closest("article") as HTMLElement;
}

beforeEach(() => {
  for (const f of [featured, join, leave, mine, push, toastShow, toastError]) f.mockReset();
  session = { status: "anonymous", user: null };
  statusHandler = null;
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
    expect(ramone.getByRole("link", { name: /Start 1-on-1 Chat/ }).getAttribute("href")).toBe("/en/chatroom?reader=ramone");

    expect(within(card("Theo")).getByText("Busy")).toBeTruthy();
    expect(within(card("Theo")).getByText("4.9 (7 reviews)")).toBeTruthy();
    expect(within(card("Solomon")).getByText("New advisor")).toBeTruthy();
    expect(within(card("Solomon")).getByRole("button", { name: /Join Waiting List/ })).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
  });

  it("sends visitors to sign in before joining a waiting list", async () => {
    featured.mockResolvedValue(three());
    renderWithI18n(<FeaturedAdvisors />);
    await screen.findByRole("heading", { name: "Theo" });
    fireEvent.click(within(card("Theo")).getByRole("button", { name: /Join Waiting List/ }));
    expect(push).toHaveBeenCalledWith(expect.stringMatching(/^\/en\/login\?next=/));
    expect(join).not.toHaveBeenCalled();
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

    fireEvent.click(within(card("Theo")).getByRole("button", { name: /Join Waiting List/ }));
    await waitFor(() => expect(within(card("Theo")).getByRole("button", { name: /On the waiting list/ })).toBeTruthy());
    expect(join).toHaveBeenCalledWith("theo");
    expect(toastShow).toHaveBeenCalledWith(expect.objectContaining({ tone: "success" }));

    fireEvent.click(within(card("Solomon")).getByRole("button", { name: /On the waiting list/ }));
    await waitFor(() => expect(within(card("Solomon")).getByRole("button", { name: /Join Waiting List/ })).toBeTruthy());
    expect(leave).toHaveBeenCalledWith("solomon");
  });

  it("switches to the chat button when the advisor came online meanwhile", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client" } };
    featured.mockResolvedValue(three());
    join.mockRejectedValue(new ApiError({ code: "advisor_available", message: "online", status: 409 }));
    renderWithI18n(<FeaturedAdvisors />);
    await screen.findByRole("heading", { name: "Theo" });
    fireEvent.click(within(card("Theo")).getByRole("button", { name: /Join Waiting List/ }));
    await waitFor(() => expect(within(card("Theo")).getByRole("link", { name: /Start 1-on-1 Chat/ })).toBeTruthy());
    expect(toastShow).toHaveBeenCalledWith(expect.objectContaining({ tone: "info", message: "Theo is online now. Start your chat." }));
  });

  it("reports other join errors without changing the card", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client" } };
    featured.mockResolvedValue(three());
    const limit = new ApiError({ code: "limit_reached", message: "You can wait for up to 20 advisors at a time.", status: 409 });
    join.mockRejectedValue(limit);
    renderWithI18n(<FeaturedAdvisors />);
    await screen.findByRole("heading", { name: "Theo" });
    fireEvent.click(within(card("Theo")).getByRole("button", { name: /Join Waiting List/ }));
    await waitFor(() => expect(toastError).toHaveBeenCalledWith(limit));
    expect(within(card("Theo")).getByRole("button", { name: /Join Waiting List/ })).toBeTruthy();
  });

  it("follows live status changes", async () => {
    featured.mockResolvedValue(three());
    renderWithI18n(<FeaturedAdvisors />);
    await screen.findByRole("heading", { name: "Ramone" });
    act(() => statusHandler?.({ slug: "ramone", status: "busy" }));
    expect(within(card("Ramone")).getByRole("button", { name: /Join Waiting List/ })).toBeTruthy();
    act(() => statusHandler?.({ slug: "solomon", status: "online" }));
    expect(within(card("Solomon")).getByRole("link", { name: /Start 1-on-1 Chat/ })).toBeTruthy();
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
