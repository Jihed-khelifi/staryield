// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";

import type { PresenceStatus } from "@/lib/api/types";

let statusHandler: ((d: { slug: string; status: PresenceStatus }) => void) | null = null;
vi.mock("@/lib/auth/session-provider", () => ({
  useRealtimeEvent: (type: string, handler: (d: { slug: string; status: PresenceStatus }) => void) => {
    if (type === "psychic.status_changed") statusHandler = handler;
  },
}));

const { STATUS_REFETCH_MS, applyStatusChange, sortByStatusTier, useStatusChanges, useThrottledRun } = await import("@/lib/psychics-live");

type Card = { slug: string; status: PresenceStatus; cta?: string };
const c = (slug: string, status: PresenceStatus): Card => ({ slug, status });
const slugs = (items: Card[]) => items.map((i) => i.slug);

afterEach(cleanup);

describe("sortByStatusTier", () => {
  it("orders online, away, busy, offline and keeps the given order inside a tier", () => {
    const sorted = sortByStatusTier([c("o1", "offline"), c("b1", "busy"), c("a1", "away"), c("n1", "online"), c("o2", "offline"), c("n2", "online"), c("a2", "away")]);
    expect(slugs(sorted)).toEqual(["n1", "n2", "a1", "a2", "b1", "o1", "o2"]);
  });

  it("does not change its input and handles an empty list", () => {
    const input = [c("o", "offline"), c("n", "online")];
    sortByStatusTier(input);
    expect(slugs(input)).toEqual(["o", "n"]);
    expect(sortByStatusTier([])).toEqual([]);
  });
});

describe("applyStatusChange", () => {
  const list = [c("n1", "online"), c("n2", "online"), c("o1", "offline"), c("o2", "offline")];

  it("moves the advisor to the end of its new tier (stable)", () => {
    expect(slugs(applyStatusChange(list, "o2", "online"))).toEqual(["n1", "n2", "o2", "o1"]);
    expect(slugs(applyStatusChange(list, "n1", "busy"))).toEqual(["n2", "n1", "o1", "o2"]);
    expect(slugs(applyStatusChange(list, "n1", "away"))).toEqual(["n2", "n1", "o1", "o2"]);
    expect(slugs(applyStatusChange(list, "n1", "offline"))).toEqual(["n2", "n1", "o1", "o2"]);
  });

  it("changes the status and applies the patch", () => {
    const out = applyStatusChange(list, "o1", "online", (a) => ({ ...a, cta: "chat" }));
    expect(out.find((a) => a.slug === "o1")).toEqual({ slug: "o1", status: "online", cta: "chat" });
  });

  it("returns the same list for an unknown advisor or an unchanged status", () => {
    expect(applyStatusChange(list, "nobody", "online")).toBe(list);
    expect(applyStatusChange(list, "n1", "online")).toBe(list);
  });
});

describe("useThrottledRun", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("runs the first call at once and at most once per gap afterwards", () => {
    const run = vi.fn();
    const { result } = renderHook(() => useThrottledRun(run));
    act(() => result.current());
    act(() => vi.advanceTimersByTime(0));
    expect(run).toHaveBeenCalledTimes(1);

    // A burst right after: one run, when the gap has passed.
    act(() => vi.advanceTimersByTime(1_000));
    for (let i = 0; i < 10; i++) act(() => result.current());
    act(() => vi.advanceTimersByTime(STATUS_REFETCH_MS - 1_001));
    expect(run).toHaveBeenCalledTimes(1);
    act(() => vi.advanceTimersByTime(1));
    expect(run).toHaveBeenCalledTimes(2);

    // Quiet for longer than the gap: runs at once again.
    act(() => vi.advanceTimersByTime(STATUS_REFETCH_MS * 3));
    act(() => result.current());
    act(() => vi.advanceTimersByTime(0));
    expect(run).toHaveBeenCalledTimes(3);
  });

  it("calls the latest function and cancels a pending run on unmount", () => {
    const first = vi.fn();
    const second = vi.fn();
    const { result, rerender, unmount } = renderHook(({ fn }) => useThrottledRun(fn), { initialProps: { fn: first } });
    act(() => result.current());
    act(() => vi.advanceTimersByTime(0));
    act(() => vi.advanceTimersByTime(100));
    act(() => result.current());
    rerender({ fn: second });
    act(() => vi.advanceTimersByTime(STATUS_REFETCH_MS));
    expect(first).toHaveBeenCalledTimes(1);
    expect(second).toHaveBeenCalledTimes(1);

    act(() => vi.advanceTimersByTime(STATUS_REFETCH_MS));
    act(() => result.current());
    unmount();
    act(() => vi.advanceTimersByTime(STATUS_REFETCH_MS));
    expect(second).toHaveBeenCalledTimes(1);
  });
});

describe("useStatusChanges", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    statusHandler = null;
  });
  afterEach(() => vi.useRealTimers());

  it("applies every change and refetches at most every 5 s", () => {
    const update = vi.fn();
    const refetch = vi.fn();
    renderHook(() => useStatusChanges(update, refetch));
    act(() => statusHandler?.({ slug: "a", status: "online" }));
    act(() => vi.advanceTimersByTime(0));
    expect(refetch).toHaveBeenCalledTimes(1);
    for (const status of ["busy", "away", "offline", "online"] as const) act(() => statusHandler?.({ slug: "b", status }));
    expect(update).toHaveBeenCalledTimes(5);
    expect(update).toHaveBeenLastCalledWith("b", "online");
    act(() => vi.advanceTimersByTime(STATUS_REFETCH_MS));
    expect(refetch).toHaveBeenCalledTimes(2);
  });

  it("does not refetch without a refetch function", () => {
    const update = vi.fn();
    renderHook(() => useStatusChanges(update));
    act(() => statusHandler?.({ slug: "a", status: "online" }));
    act(() => vi.advanceTimersByTime(STATUS_REFETCH_MS * 2));
    expect(update).toHaveBeenCalledTimes(1);
  });
});
