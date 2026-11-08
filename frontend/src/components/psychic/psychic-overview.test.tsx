// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { CustomRequest, Page, Review, StatsKPIs, StatsOverview } from "@/lib/api/types";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const mocks = vi.hoisted(() => ({
  psychicsApi: { stats: vi.fn() },
  reviewsApi: { mine: vi.fn() },
  requestsApi: { list: vi.fn(), accept: vi.fn(), decline: vi.fn(), deliver: vi.fn(), dispute: vi.fn(), cancel: vi.fn() },
}));
vi.mock("@/lib/api/endpoints", () => mocks);
const push = vi.hoisted(() => ({ supported: { current: true }, user: { current: { id: "p1", role: "psychic" } as { id: string; role: string } | null }, config: vi.fn() }));
vi.mock("@/lib/auth/session-provider", () => ({ useRealtimeEvent: () => undefined, useSession: () => ({ realtime: null, user: push.user.current }) }));
vi.mock("@/lib/api/account", () => ({ pushApi: { config: push.config }, settingsApi: { update: vi.fn() } }));
vi.mock("@/lib/push", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/push")>()),
  pushSupported: () => push.supported.current,
  pushPermission: () => "default",
}));

import { PsychicOverview } from "@/components/psychic/psychic-overview";

beforeAll(stubResizeObserver);
afterEach(cleanup);
beforeEach(() => {
  for (const group of Object.values(mocks)) for (const fn of Object.values(group)) fn.mockReset();
  mocks.reviewsApi.mine.mockResolvedValue({ items: [] });
  mocks.requestsApi.list.mockResolvedValue({ items: [] });
  push.supported.current = true;
  push.user.current = { id: "p1", role: "psychic" };
  push.config.mockReset().mockResolvedValue({ enabled: true, public_key: "KEY" });
  window.localStorage.clear();
});

const kpis = (over: Partial<StatsKPIs> = {}): StatsKPIs => ({
  free_to_paid: { pct: 18, numerator: 9, denominator: 50 },
  paid_to_retained: { pct: null, numerator: 0, denominator: 0 },
  paid_minutes: 260,
  retained_repeat_days: { median_days: 4, sample: 3 },
  active_hours: { hours: 42, in_session_hours: 12 },
  ...over,
});
const overview = (over: Partial<StatsOverview> = {}): StatsOverview => ({
  range: { from: "2026-10-07T00:00:00Z", to: "2026-11-06T00:00:00Z", tz: "UTC" },
  kpis: kpis(),
  previous: kpis({ free_to_paid: { pct: 14, numerator: 7, denominator: 50 }, paid_minutes: 260, active_hours: { hours: 50, in_session_hours: 0 } }),
  ...over,
});
const review = (n: number): Review => ({
  id: `r${n}`,
  rating: (n % 5) + 1,
  comment: `Review ${n}`,
  author: { name: `Client ${n}` },
  created_at: "2026-04-21T10:00:00Z",
});
const request = (over: Partial<CustomRequest> = {}): CustomRequest =>
  ({
    id: "req-1",
    client: { id: "c1", name: "Paul" },
    psychic: { id: "p1", name: "Ramone" },
    service_id: "s1",
    service_name: "Love reading",
    price_credits: 500,
    status: "accepted",
    client_note: "About my relationship",
    accept_by: "2026-11-06T00:00:00Z",
    created_at: "2026-11-01T00:00:00Z",
    ...over,
  }) as CustomRequest;

function renderOverview() {
  return renderWithI18n(
    <ToastProvider>
      <PsychicOverview />
    </ToastProvider>,
  );
}

describe("PsychicOverview", () => {
  it("offers the advisor push prompt when push is available and this browser isn't subscribed", async () => {
    mocks.psychicsApi.stats.mockResolvedValue(overview());
    renderOverview();
    expect(await screen.findByText("Never miss a chat request")).toBeTruthy();
  });

  it("hides the push prompt when dismissed, unsupported or for no user", async () => {
    mocks.psychicsApi.stats.mockResolvedValue(overview());
    window.localStorage.setItem("staryield.chat.push-prompt.v1:p1", "1");
    renderOverview();
    await screen.findByTestId("psychic-overview");
    expect(screen.queryByText("Never miss a chat request")).toBeNull();
    cleanup();
    window.localStorage.clear();
    push.supported.current = false;
    renderOverview();
    await screen.findByTestId("psychic-overview");
    expect(screen.queryByText("Never miss a chat request")).toBeNull();
    cleanup();
    push.supported.current = true;
    push.user.current = null;
    renderOverview();
    await screen.findByTestId("psychic-overview");
    expect(screen.queryByText("Never miss a chat request")).toBeNull();
  });

  it("shows the five KPI cards with units, trends and no-data states", async () => {
    mocks.psychicsApi.stats.mockResolvedValue(overview());
    renderOverview();
    expect(screen.getByRole("heading", { level: 1, name: "Advisor stats overview" })).toBeTruthy();
    expect(screen.getByRole("status", { name: "Loading your stats…" })).toBeTruthy();

    const free = await screen.findByTestId("kpi-free_to_paid");
    expect(free.textContent).toContain("18%");
    expect(free.textContent).toContain("Up 4 pts on the previous period");
    const retained = screen.getByTestId("kpi-paid_to_retained");
    expect(retained.textContent).toContain("No data yet");
    expect(retained.textContent).not.toContain("previous period");
    expect(screen.getByTestId("kpi-paid_minutes").textContent).toContain("260 min");
    expect(screen.getByTestId("kpi-paid_minutes").textContent).toContain("Same as the previous period");
    expect(screen.getByTestId("kpi-retained_repeat_days").textContent).toContain("4 days");
    const hours = screen.getByTestId("kpi-active_hours");
    expect(hours.textContent).toContain("42 hr");
    expect(hours.textContent).toContain("Down 8 hr on the previous period");

    // Definitions open on click/tap.
    fireEvent.click(within(free).getByRole("button", { name: "What does Free to paid mean?" }));
    expect(await screen.findByText(/who then paid for a session/)).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
  });

  it("asks for calendar days in the viewer's time zone and refetches per range", async () => {
    mocks.psychicsApi.stats.mockResolvedValue(overview());
    renderOverview();
    await screen.findByTestId("kpi-free_to_paid");
    const first = mocks.psychicsApi.stats.mock.calls[0][0];
    expect(first.from).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(first.to).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(first.tz).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone);
    const days = (q: { from: string; to: string }) => Math.round((Date.parse(q.to) - Date.parse(q.from)) / 86_400_000) + 1;
    expect(days(first)).toBe(30);

    fireEvent.click(screen.getByLabelText("7 days"));
    await waitFor(() => expect(mocks.psychicsApi.stats).toHaveBeenCalledTimes(2));
    expect(days(mocks.psychicsApi.stats.mock.calls[1][0])).toBe(7);
  });

  it("offers a retry when the stats fail", async () => {
    mocks.psychicsApi.stats.mockRejectedValueOnce(new ApiError({ code: "service_unavailable", message: "x", status: 503 })).mockResolvedValue(overview());
    renderOverview();
    fireEvent.click(await screen.findByRole("button", { name: "Try again" }));
    expect(await screen.findByTestId("kpi-cards")).toBeTruthy();
  });

  it("pages recent reviews five at a time", async () => {
    mocks.psychicsApi.stats.mockResolvedValue(overview());
    mocks.reviewsApi.mine.mockImplementation(async (cursor?: string): Promise<Page<Review>> =>
      cursor ? { items: [review(6)] } : { items: [1, 2, 3, 4, 5].map(review), next_cursor: "r2" },
    );
    renderOverview();
    const reviews = await screen.findByTestId("overview-reviews");
    await waitFor(() => expect(within(reviews).getAllByTestId("overview-review")).toHaveLength(5));
    expect(mocks.reviewsApi.mine).toHaveBeenCalledWith(undefined, expect.any(AbortSignal), 5);
    expect(within(reviews).getByText("Client 1")).toBeTruthy();
    expect(within(reviews).getAllByText("Apr 21, 2026")).toHaveLength(5);

    fireEvent.click(within(reviews).getByRole("button", { name: /Next/ }));
    await waitFor(() => expect(within(reviews).getAllByTestId("overview-review")).toHaveLength(1));
    expect(mocks.reviewsApi.mine).toHaveBeenLastCalledWith("r2", expect.any(AbortSignal), 5);
  });

  it("has empty states for reviews and requests", async () => {
    mocks.psychicsApi.stats.mockResolvedValue(overview());
    renderOverview();
    expect(await screen.findByText("No reviews yet")).toBeTruthy();
    expect(await screen.findByText("No requests yet.")).toBeTruthy();
  });

  it("delivers a reading from a validated dialog", async () => {
    mocks.psychicsApi.stats.mockResolvedValue(overview());
    mocks.requestsApi.list.mockResolvedValue({ items: [request()] });
    mocks.requestsApi.deliver.mockResolvedValue(request({ status: "delivered", delivery: "Your reading" }));
    renderOverview();
    const row = await screen.findByTestId("advisor-request");
    fireEvent.click(within(row).getByRole("button", { name: "Deliver" }));
    const dialog = await screen.findByRole("dialog", { name: "Deliver your reading" });

    fireEvent.click(within(dialog).getByRole("button", { name: "Deliver" }));
    expect(await within(dialog).findByText("Write the reading before you deliver it.")).toBeTruthy();
    expect(mocks.requestsApi.deliver).not.toHaveBeenCalled();

    fireEvent.change(within(dialog).getByRole("textbox", { name: "Your reading" }), { target: { value: "  The cards say yes.  " } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Deliver" }));
    await waitFor(() => expect(mocks.requestsApi.deliver).toHaveBeenCalledWith("req-1", "The cards say yes."));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByTestId("advisor-request").getAttribute("data-status")).toBe("delivered");
  });

  it("declines with an optional reason and shows server field errors", async () => {
    mocks.psychicsApi.stats.mockResolvedValue(overview());
    mocks.requestsApi.list.mockResolvedValue({ items: [request({ status: "pending" })] });
    mocks.requestsApi.decline.mockRejectedValueOnce(
      new ApiError({ code: "validation_failed", message: "x", status: 422, details: { fields: { reason: "Use at most 500 characters." } } }),
    );
    renderOverview();
    const row = await screen.findByTestId("advisor-request");
    fireEvent.click(within(row).getByRole("button", { name: "Decline" }));
    const dialog = await screen.findByRole("dialog", { name: "Decline this request?" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Decline request" }));
    await waitFor(() => expect(mocks.requestsApi.decline).toHaveBeenCalledWith("req-1", undefined));
    expect(await within(dialog).findByText("Use at most 500 characters.")).toBeTruthy();
  });
});
