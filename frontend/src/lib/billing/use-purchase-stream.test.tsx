// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";

import { FakeEventSource, installFakeEventSource } from "@/test/event-source";

vi.mock("@/lib/api/client", () => ({ api: vi.fn(), refreshSession: vi.fn().mockResolvedValue(false) }));

import { usePurchaseStream } from "@/lib/billing/use-purchase-stream";
import type { Purchase } from "@/lib/api/types";

const purchase = (over: Partial<Purchase> = {}): Purchase => ({
  id: "pu_12345678", pack_id: "p", credits: 600, price_cents: 1000, currency: "USD", status: "pending", provider: "stripe", created_at: "2026-11-19T10:00:00Z", ...over,
});
const send = (p: Purchase) => act(() => FakeEventSource.last.emit({ type: "purchase.updated", data: p }));

beforeEach(installFakeEventSource);
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("usePurchaseStream", () => {
  it("is idle without an id and opens no stream", () => {
    const { result } = renderHook(() => usePurchaseStream(null));
    expect(result.current.status).toBe("idle");
    expect(FakeEventSource.instances).toHaveLength(0);
  });

  it("waits, shows the pending state, then the final purchase and closes the stream", async () => {
    const { result } = renderHook(() => usePurchaseStream("pu_12345678"));
    expect(result.current.status).toBe("waiting");
    send(purchase());
    expect(result.current).toMatchObject({ status: "waiting", purchase: { status: "pending" } });
    send(purchase({ status: "succeeded" }));
    await vi.waitFor(() => expect(result.current).toMatchObject({ status: "final", purchase: { status: "succeeded" } }));
    expect(FakeEventSource.last.closed).toBe(true);
  });

  it("reports a timeout and can listen again", async () => {
    const { result } = renderHook(() => usePurchaseStream("pu_12345678"));
    act(() => FakeEventSource.last.emit({ type: "purchase.timeout" }));
    await vi.waitFor(() => expect(result.current.status).toBe("timeout"));
    act(() => result.current.retry());
    await vi.waitFor(() => expect(FakeEventSource.instances).toHaveLength(2));
    expect(result.current.status).toBe("waiting");
  });

  it("reports an unreadable stream as error", async () => {
    const { result } = renderHook(() => usePurchaseStream("pu_12345678"));
    act(() => FakeEventSource.last.fail(true));
    await vi.waitFor(() => expect(result.current.status).toBe("error"));
  });

  it("closes the EventSource on unmount", () => {
    const { unmount } = renderHook(() => usePurchaseStream("pu_12345678"));
    const es = FakeEventSource.last;
    unmount();
    expect(es.closed).toBe(true);
  });

  it("switches streams when the id changes, closing the old one", () => {
    const { rerender } = renderHook(({ id }) => usePurchaseStream(id), { initialProps: { id: "pu_aaaaaaaa" } });
    const first = FakeEventSource.last;
    rerender({ id: "pu_bbbbbbbb" });
    expect(first.closed).toBe(true);
    expect(FakeEventSource.last.url).toContain("pu_bbbbbbbb");
  });
});
