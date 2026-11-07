// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { AdvisorCard, DirectoryPage, DirectoryQuery, PresenceStatus } from "@/lib/api/types";
import { axeViolations, renderWithI18n } from "@/test/render";

const browse = vi.fn<(query: DirectoryQuery, signal?: AbortSignal) => Promise<DirectoryPage>>();
const session = vi.hoisted(() => ({ status: "authenticated" as "authenticated" | "anonymous" | "loading" }));
const replace = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace, push: vi.fn() }), usePathname: () => "/en/psychics" }));
let statusHandler: ((data: { slug: string; status: PresenceStatus }) => void) | null = null;

vi.mock("@/lib/api/endpoints", () => ({
  psychicsApi: { browse: (q: DirectoryQuery, s?: AbortSignal) => browse(q, s) },
  requestsApi: {},
}));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ realtime: null, status: session.status, user: null, wallet: null, setWallet: () => {} }),
  useRealtimeEvent: (type: string, handler: (data: { slug: string; status: PresenceStatus }) => void) => {
    if (type === "psychic.status_changed") statusHandler = handler;
  },
}));
vi.mock("@/components/layout/site-header", () => ({ SiteHeader: () => null, MobileNavigation: () => null }));
vi.mock("@/lib/analytics", () => ({ track: vi.fn() }));

const { PsychicsBrowse, readDirectoryPage, DIRECTORY_PAGE_SIZE } = await import("@/components/psychics/psychics");

function advisor(slug: string, status: PresenceStatus = "offline"): AdvisorCard {
  return {
    slug,
    name: slug[0].toUpperCase() + slug.slice(1),
    headline: `${slug} reads tarot`,
    image_url: `https://advisors.test/${slug}.jpg`,
    zodiac: "Leo",
    skills: ["Tarot"],
    rating: 4.8,
    reviews_count: 120,
    readings_count: 900,
    years_experience: 7,
    rate_per_minute: 30,
    status,
    featured: false,
  };
}

function page(n: number, items: AdvisorCard[], total: number): DirectoryPage {
  return { items, page: n, page_size: DIRECTORY_PAGE_SIZE, total, total_pages: Math.ceil(total / DIRECTORY_PAGE_SIZE) };
}

beforeEach(() => {
  browse.mockReset();
  replace.mockReset();
  session.status = "authenticated";
  statusHandler = null;
  window.history.replaceState(null, "", "/en/psychics");
  Element.prototype.scrollIntoView = vi.fn();
});
afterEach(cleanup);

describe("readDirectoryPage", () => {
  it("accepts page numbers from 1 to 10000 only", () => {
    expect(readDirectoryPage("")).toBe(1);
    expect(readDirectoryPage("?page=3")).toBe(3);
    expect(readDirectoryPage("?page=10000")).toBe(10000);
    for (const bad of ["?page=0", "?page=-2", "?page=2.5", "?page=abc", "?page=10001", "?page=1e3", "?page=%203"]) {
      expect(readDirectoryPage(bad), bad).toBe(1);
    }
  });
});

describe("PsychicsBrowse", () => {
  it("opens the page from the address bar and renders cards with a pager", async () => {
    window.history.replaceState(null, "", "/en/psychics?page=2");
    browse.mockResolvedValue(page(2, [advisor("ada"), advisor("bea", "online")], 14));
    renderWithI18n(<PsychicsBrowse />);
    expect(screen.getByRole("status").textContent).toBe("Loading advisors…");
    expect(await screen.findByRole("heading", { name: /Ada/ })).toBeTruthy();
    expect(browse.mock.calls[0][0]).toEqual({ page: 2, page_size: DIRECTORY_PAGE_SIZE });
    expect(screen.getAllByRole("link", { name: /View full profile/ }).map((a) => a.getAttribute("href"))).toEqual(["/en/psychics/ada", "/en/psychics/bea"]);
    expect(screen.getByRole("button", { name: "Go to page 1" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Go to page 2" }).getAttribute("aria-current")).toBe("page");
    expect(await axeViolations()).toEqual([]);
  });

  it("changes page, keeps the address bar in sync and announces it", async () => {
    browse.mockImplementation(async (q) => page(q.page ?? 1, [advisor(`seer${q.page}`)], 30));
    renderWithI18n(<PsychicsBrowse />);
    await screen.findByRole("heading", { name: /Seer1/ });
    expect(window.location.search).toBe("");
    fireEvent.click(screen.getByRole("button", { name: "Go to page 2" }));
    await screen.findByRole("heading", { name: /Seer2/ });
    expect(window.location.search).toBe("?page=2");
    expect(Element.prototype.scrollIntoView).toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Go to page 1" }));
    await screen.findByRole("heading", { name: /Seer1/ });
    expect(window.location.search).toBe("");
  });

  it("falls back to the last page for a link past the end", async () => {
    window.history.replaceState(null, "", "/en/psychics?page=9");
    browse.mockImplementation(async (q) => (q.page === 9 ? page(9, [], 3) : page(1, [advisor("ada")], 3)));
    renderWithI18n(<PsychicsBrowse />);
    await screen.findByRole("heading", { name: /Ada/ });
    expect(browse.mock.calls.map((c) => c[0].page)).toEqual([9, 1]);
    expect(window.location.search).toBe("");
  });

  it("explains a failed load and retries", async () => {
    browse
      .mockRejectedValueOnce(new ApiError({ code: "service_unavailable", message: "down", status: 503 }))
      .mockResolvedValueOnce(page(1, [advisor("ada")], 1));
    renderWithI18n(<PsychicsBrowse />);
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("We couldn't load the advisors.");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("heading", { name: /Ada/ })).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows an empty state without a pager", async () => {
    browse.mockResolvedValue(page(1, [], 0));
    renderWithI18n(<PsychicsBrowse />);
    expect(await screen.findByText("No advisors are listed right now.")).toBeTruthy();
    expect(screen.queryByRole("navigation")).toBeNull();
  });

  it("applies live status changes to the cards on screen", async () => {
    browse.mockResolvedValue(page(1, [advisor("ada"), advisor("bea")], 2));
    renderWithI18n(<PsychicsBrowse />);
    await screen.findByRole("heading", { name: /Ada/ });
    expect(screen.queryByText("● Online")).toBeNull();
    act(() => statusHandler?.({ slug: "bea", status: "online" }));
    await waitFor(() => expect(screen.getAllByText("● Online")).toHaveLength(1));
    expect(screen.getAllByText("● Offline")).toHaveLength(1);
  });
});

describe("PsychicsBrowse gate (signed-in customers only)", () => {
  it("shows a neutral loading state, never the directory, while the session loads", async () => {
    session.status = "loading";
    browse.mockResolvedValue(page(1, [advisor("ada")], 1));
    renderWithI18n(<PsychicsBrowse />);
    expect(screen.getByTestId("psychics-gate")).toBeTruthy();
    expect(screen.getByText("Loading advisors…")).toBeTruthy();
    await act(async () => {});
    expect(browse).not.toHaveBeenCalled();
    expect(replace).not.toHaveBeenCalled();
    expect(screen.queryByRole("heading", { name: /Ada/ })).toBeNull();
  });

  it("redirects anonymous visitors to the home section and never loads the directory", async () => {
    session.status = "anonymous";
    renderWithI18n(<PsychicsBrowse />);
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/en#psychics"));
    expect(replace).toHaveBeenCalledTimes(1);
    expect(browse).not.toHaveBeenCalled();
    expect(screen.queryByRole("heading", { name: /Our advisors/ })).toBeNull();
  });

  it("shows the directory to signed-in customers without redirecting", async () => {
    browse.mockResolvedValue(page(1, [advisor("ada")], 1));
    renderWithI18n(<PsychicsBrowse />);
    expect(await screen.findByRole("heading", { name: /Ada/ })).toBeTruthy();
    expect(replace).not.toHaveBeenCalled();
  });
});
