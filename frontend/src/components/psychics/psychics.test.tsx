// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { AdvisorCard, DirectoryPage, DirectoryQuery, Page, PresenceStatus, WaitlistEntry } from "@/lib/api/types";
import { axeViolations, renderWithI18n } from "@/test/render";

const browse = vi.fn<(query: DirectoryQuery, signal?: AbortSignal) => Promise<DirectoryPage>>();
const mine = vi.fn<() => Promise<Page<WaitlistEntry>>>();
const join = vi.fn<(slug: string) => Promise<WaitlistEntry>>();
const session = vi.hoisted(() => ({ status: "authenticated" as "authenticated" | "anonymous" | "loading" }));
const replace = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace, push: vi.fn() }), usePathname: () => "/en/psychics" }));
// Every subscriber (the page and the waiting-list provider) hears a change, one slot per call site.
const handlers = new Map<string, (data: { slug: string; status: PresenceStatus }) => void>();
const statusHandler = (data: { slug: string; status: PresenceStatus }) => handlers.forEach((h) => h(data));

vi.mock("@/lib/api/endpoints", () => ({
  psychicsApi: { browse: (q: DirectoryQuery, s?: AbortSignal) => browse(q, s) },
  requestsApi: {},
  waitlistApi: { join: (s: string) => join(s), leave: vi.fn(), mine: () => mine() },
}));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => ({ show: vi.fn(), showError: vi.fn() }) }));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({
    realtime: null,
    status: session.status,
    user: session.status === "authenticated" ? { id: "u1", role: "client", age_status: "adult" } : null,
    wallet: null,
    setWallet: () => {},
  }),
  useRealtimeEvent: (type: string, handler: (data: { slug: string; status: PresenceStatus }) => void) => {
    if (type === "psychic.status_changed") handlers.set(new Error().stack?.split("\n")[2] ?? "x", handler);
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
  mine.mockReset();
  join.mockReset();
  mine.mockResolvedValue({ items: [] });
  replace.mockReset();
  session.status = "authenticated";
  handlers.clear();
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

  it("puts the standard book button on every card, with one waiting-list read", async () => {
    browse.mockResolvedValue(page(1, [advisor("ada", "online"), advisor("bea", "busy"), advisor("cyd", "offline")], 3));
    mine.mockResolvedValue({ items: [{ slug: "cyd", name: "Cyd", image_url: "", status: "offline", joined_at: "2026-11-05T10:00:00Z" }] });
    renderWithI18n(<PsychicsBrowse />);
    await screen.findByRole("heading", { name: /Ada/ });
    const buttons = screen.getAllByTestId("book-button");
    expect(buttons.map((b) => b.getAttribute("data-action"))).toEqual(["chat", "join_waitlist", "join_waitlist"]);
    expect(screen.getByRole("link", { name: /Chat now/ }).getAttribute("href")).toBe("/en/chatroom?reader=ada");
    await waitFor(() => expect(buttons[2].getAttribute("data-action")).toBe("leave_waitlist"));
    expect(mine).toHaveBeenCalledOnce();
    // Each button is described by its advisor's heading.
    const describedBy = buttons[1].getAttribute("aria-describedby") ?? "";
    expect(document.getElementById(describedBy)?.textContent).toContain("Bea");
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

  it("shows a card online, dot and pill too, when its advisor turns out to be online (409)", async () => {
    browse.mockResolvedValue(page(1, [advisor("ada", "offline"), advisor("bea", "busy")], 2));
    join.mockRejectedValue(new ApiError({ code: "advisor_available", message: "online", status: 409 }));
    renderWithI18n(<PsychicsBrowse />);
    const bea = (await screen.findByRole("heading", { name: /Bea/ })).closest("article") as HTMLElement;
    const button = bea.querySelector<HTMLButtonElement>("[data-testid=book-button]")!;
    await waitFor(() => expect(button.disabled).toBe(false));
    fireEvent.click(button);
    await waitFor(() => expect(bea.getAttribute("data-status")).toBe("online"));
    expect(bea.querySelector(".status-pill")?.textContent).toBe("● Online");
    expect(bea.querySelector(".advisor-status-dot")?.className).toContain("status-dot online");
    expect(bea.querySelector("[data-testid=book-button]")?.getAttribute("data-action")).toBe("chat");
    // Online first: the card moved up.
    expect(screen.getAllByRole("heading", { level: 2 })[0].textContent).toContain("Bea");
  });

  it("lets the desktop card grow to hold the book button instead of cutting it off (globals.css)", () => {
    const css = readFileSync(`${process.cwd()}/src/app/globals.css`, "utf8");
    const card = css.match(/\n\.advisor-card \{([^}]*)\}/)?.[1] ?? "";
    expect(card).toMatch(/overflow: hidden;/);
    // A minimum, never a fixed height, so the button under the profile bar stays inside the card.
    expect(card).toMatch(/min-height: 498px;/);
    expect(card).not.toMatch(/(^|[^-])height:/);
    expect(css).toMatch(/\n\.advisor-profile-link \{[^}]*margin: auto -16px 0 -17px;/);
    expect(css).toMatch(/\n\.advisor-card-book \{\s*margin: 12px 0 20px;\s*\}/);
  });

  it("applies live status changes to the cards on screen", async () => {
    // The quiet refresh that follows a status change reads the server's state.
    const server = new Map<string, PresenceStatus>([["ada", "offline"], ["bea", "offline"]]);
    browse.mockImplementation(async () => page(1, [...server].map(([slug, st]) => advisor(slug, st)), 2));
    renderWithI18n(<PsychicsBrowse />);
    await screen.findByRole("heading", { name: /Ada/ });
    expect(screen.queryByText("● Online")).toBeNull();
    server.set("bea", "online");
    act(() => statusHandler({ slug: "bea", status: "online" }));
    await waitFor(() => expect(screen.getAllByText("● Online")).toHaveLength(1));
    expect(screen.getAllByText("● Offline")).toHaveLength(1);
  });

  const names = () => screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent?.split(",")[0]);

  it("re-sorts the cards by status tier when a status changes, keeping the server order inside a tier", async () => {
    // A server that answers with its own state, ordered like the API
    // (here: by tier, then by name, the advisors being otherwise equal).
    const tier = { online: 0, away: 1, busy: 2, offline: 3 };
    const server = new Map<string, PresenceStatus>([["ada", "online"], ["bea", "offline"], ["cyd", "offline"], ["dax", "offline"]]);
    browse.mockImplementation(async () =>
      page(1, [...server].sort((a, b) => tier[a[1]] - tier[b[1]] || a[0].localeCompare(b[0])).map(([slug, st]) => advisor(slug, st)), 4),
    );
    const change = (slug: string, status: PresenceStatus) => {
      server.set(slug, status);
      act(() => statusHandler({ slug, status }));
    };
    renderWithI18n(<PsychicsBrowse />);
    await screen.findByRole("heading", { name: /Ada/ });
    expect(names()).toEqual(["Ada", "Bea", "Cyd", "Dax"]);
    change("dax", "online");
    expect(names()).toEqual(["Ada", "Dax", "Bea", "Cyd"]); // at once, before any refetch answers
    change("cyd", "busy");
    change("bea", "away");
    expect(names()).toEqual(["Ada", "Dax", "Bea", "Cyd"]);
    change("ada", "offline");
    expect(names()).toEqual(["Dax", "Bea", "Cyd", "Ada"]);
  });

  it("keeps the local order between status changes and the refetch (no server answer yet)", async () => {
    browse.mockResolvedValueOnce(page(1, [advisor("ada", "online"), advisor("bea", "offline"), advisor("cyd", "offline")], 3));
    let answer: (p: DirectoryPage) => void = () => {};
    browse.mockImplementation(() => new Promise<DirectoryPage>((resolve) => (answer = resolve)));
    renderWithI18n(<PsychicsBrowse />);
    await screen.findByRole("heading", { name: /Ada/ });
    act(() => statusHandler({ slug: "cyd", status: "online" }));
    expect(names()).toEqual(["Ada", "Cyd", "Bea"]);
    await waitFor(() => expect(browse).toHaveBeenCalledTimes(2));
    expect(names()).toEqual(["Ada", "Cyd", "Bea"]);
    await act(async () => answer(page(1, [advisor("cyd", "online"), advisor("ada", "online"), advisor("bea")], 3)));
    expect(names()).toEqual(["Cyd", "Ada", "Bea"]); // the server's order wins once it answers
  });

  it("refreshes the page quietly after status changes, at most every 5 seconds", async () => {
    browse.mockResolvedValue(page(1, [advisor("ada"), advisor("bea")], 2));
    renderWithI18n(<PsychicsBrowse />);
    await screen.findByRole("heading", { name: /Ada/ });
    expect(browse).toHaveBeenCalledTimes(1);
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "Date"] });
    try {
      // The server now ranks bea first: the quiet refresh brings that order.
      browse.mockResolvedValue(page(1, [advisor("bea", "online"), advisor("ada")], 2));
      act(() => statusHandler({ slug: "bea", status: "online" }));
      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });
      expect(browse).toHaveBeenCalledTimes(2);
      expect(browse.mock.calls[1][0]).toEqual({ page: 1, page_size: DIRECTORY_PAGE_SIZE });
      for (let i = 0; i < 5; i++) act(() => statusHandler({ slug: "ada", status: i % 2 ? "online" : "busy" }));
      await act(async () => {
        await vi.advanceTimersByTimeAsync(4_900);
      });
      expect(browse).toHaveBeenCalledTimes(2);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(200);
      });
      expect(browse).toHaveBeenCalledTimes(3);
    } finally {
      vi.useRealTimers();
    }
    // No loading state while refreshing.
    expect(screen.queryByTestId("advisor-grid-skeleton")).toBeNull();
  });

  it("ignores a failed or empty quiet refresh and keeps the cards on screen", async () => {
    browse.mockResolvedValueOnce(page(1, [advisor("ada"), advisor("bea")], 2));
    renderWithI18n(<PsychicsBrowse />);
    await screen.findByRole("heading", { name: /Ada/ });
    browse.mockRejectedValueOnce(new ApiError({ code: "service_unavailable", message: "down", status: 503 }));
    act(() => statusHandler({ slug: "bea", status: "online" }));
    await waitFor(() => expect(browse).toHaveBeenCalledTimes(2));
    expect(names()).toEqual(["Bea", "Ada"]);
    expect(screen.queryByRole("alert")).toBeNull();
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
