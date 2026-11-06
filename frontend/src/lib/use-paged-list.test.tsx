// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { Page } from "@/lib/api/types";
import { usePagedList } from "@/lib/use-paged-list";

afterEach(cleanup);

type Row = { id: number };

/** Fake cursor API: pages of `size` rows over `total` rows. */
function api(total: number, size = 2) {
  const calls: (string | undefined)[] = [];
  const fetchPage = vi.fn(async (cursor: string | undefined): Promise<Page<Row>> => {
    calls.push(cursor);
    const start = cursor ? Number(cursor) : 0;
    const items = Array.from({ length: Math.max(0, Math.min(size, total - start)) }, (_, i) => ({ id: start + i }));
    // Like the API: a cursor whenever the page is full (even if nothing follows).
    return { items, next_cursor: items.length === size ? String(start + size) : undefined };
  });
  return { fetchPage, calls };
}

describe("usePagedList", () => {
  it("loads the first page, then pages forward and back from the cache", async () => {
    const { fetchPage, calls } = api(5);
    const { result } = renderHook(() => usePagedList(fetchPage, ["list"]));
    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.items).toEqual([{ id: 0 }, { id: 1 }]));
    expect(result.current).toMatchObject({ page: 1, hasPrev: false, hasNext: true, loading: false });

    act(() => result.current.next());
    await waitFor(() => expect(result.current.items).toEqual([{ id: 2 }, { id: 3 }]));
    expect(result.current).toMatchObject({ page: 2, hasPrev: true, hasNext: true });

    act(() => result.current.prev());
    expect(result.current.items).toEqual([{ id: 0 }, { id: 1 }]);
    expect(calls).toEqual([undefined, "2"]);
  });

  it("steps back when the page after a full last page is empty", async () => {
    const { fetchPage } = api(4);
    const { result } = renderHook(() => usePagedList(fetchPage, ["list"]));
    await waitFor(() => expect(result.current.items).toHaveLength(2));
    act(() => result.current.next());
    await waitFor(() => expect(result.current.items).toEqual([{ id: 2 }, { id: 3 }]));
    expect(result.current.hasNext).toBe(true);
    act(() => result.current.next());
    // The empty third page is fetched, then the hook steps back to page 2.
    await waitFor(() => expect(fetchPage).toHaveBeenCalledTimes(3));
    await waitFor(() => expect(result.current.page).toBe(2));
    expect(result.current.hasNext).toBe(false);
    expect(result.current.items).toEqual([{ id: 2 }, { id: 3 }]);
  });

  it("starts over when the deps change", async () => {
    const { fetchPage, calls } = api(10);
    const { result, rerender } = renderHook(({ filter }) => usePagedList(fetchPage, [filter]), {
      initialProps: { filter: "a" },
    });
    await waitFor(() => expect(result.current.items).toHaveLength(2));
    act(() => result.current.next());
    await waitFor(() => expect(result.current.page).toBe(2));
    rerender({ filter: "b" });
    await waitFor(() => expect(result.current.page).toBe(1));
    await waitFor(() => expect(result.current.items).toEqual([{ id: 0 }, { id: 1 }]));
    expect(calls).toEqual([undefined, "2", undefined]);
  });

  it("aborts the in-flight request when leaving a page", async () => {
    const signals: AbortSignal[] = [];
    const fetchPage = vi.fn((_cursor: string | undefined, signal: AbortSignal) => {
      signals.push(signal);
      return new Promise<Page<Row>>(() => {});
    });
    const { unmount } = renderHook(() => usePagedList(fetchPage, ["x"]));
    await waitFor(() => expect(signals).toHaveLength(1));
    unmount();
    expect(signals[0].aborted).toBe(true);
  });

  it("exposes errors and recovers with reload", async () => {
    let fail = true;
    const fetchPage = vi.fn(async (): Promise<Page<Row>> => {
      if (fail) throw new ApiError({ code: "service_unavailable", message: "down", status: 503 });
      return { items: [{ id: 1 }] };
    });
    const { result } = renderHook(() => usePagedList(fetchPage, ["x"]));
    await waitFor(() => expect(result.current.error?.code).toBe("service_unavailable"));
    expect(result.current.loading).toBe(false);
    fail = false;
    act(() => result.current.reload());
    await waitFor(() => expect(result.current.items).toEqual([{ id: 1 }]));
    expect(result.current.error).toBeNull();
  });

  it("patches cached pages for optimistic updates", async () => {
    const { fetchPage } = api(3);
    const { result } = renderHook(() => usePagedList(fetchPage, ["x"]));
    await waitFor(() => expect(result.current.items).toHaveLength(2));
    act(() => result.current.prependItem({ id: 99 }));
    expect(result.current.items.map((r) => r.id)).toEqual([99, 0, 1]);
    act(() => result.current.replaceItem((r) => r.id === 0, { id: 100 }));
    expect(result.current.items.map((r) => r.id)).toEqual([99, 100, 1]);
    act(() => result.current.removeItem((r) => r.id === 99));
    expect(result.current.items.map((r) => r.id)).toEqual([100, 1]);
  });

  it("does nothing while disabled", async () => {
    const { fetchPage } = api(3);
    const { result } = renderHook(() => usePagedList(fetchPage, ["x"], { enabled: false }));
    await new Promise((r) => setTimeout(r, 10));
    expect(fetchPage).not.toHaveBeenCalled();
    expect(result.current.loading).toBe(false);
  });
});
