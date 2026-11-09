// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { OffSessionAllowance } from "@/lib/api/types";

const allowanceApi = vi.hoisted(() => vi.fn());
const realtime = vi.hoisted(() => ({ session: null as null | ((type: string, data: unknown) => void), single: new Map<string, (data: unknown) => void>() }));
vi.mock("@/lib/api/endpoints", () => ({ chatApi: { allowance: (id: string, signal?: AbortSignal) => allowanceApi(id, signal) } }));
vi.mock("@/lib/auth/session-provider", async () => {
  const React = await import("react");
  return {
    useRealtimeEvents: (_types: readonly string[], handler: (type: string, data: unknown) => void) => {
      React.useEffect(() => {
        realtime.session = handler;
        return () => {
          realtime.session = null;
        };
      });
    },
    useRealtimeEvent: (type: string, handler: (data: unknown) => void) => {
      React.useEffect(() => {
        realtime.single.set(type, handler);
      });
    },
  };
});

import { useOffSessionAllowance } from "@/components/chat/use-off-session-allowance";

const a = (over: Partial<OffSessionAllowance> = {}): OffSessionAllowance => ({ live: false, enabled: true, eligible: true, unlimited: false, limit: 20, used: 2, remaining: 18, ...over });

beforeEach(() => {
  allowanceApi.mockReset();
  realtime.single.clear();
});
afterEach(cleanup);

describe("useOffSessionAllowance", () => {
  it("fetches when there is no live chat, with loading then data", async () => {
    allowanceApi.mockResolvedValue(a());
    const { result } = renderHook(() => useOffSessionAllowance("c1", false));
    expect(result.current.loading).toBe(true);
    expect(result.current.allowance).toBeNull();
    await waitFor(() => expect(result.current.allowance).toEqual(a()));
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeNull();
    expect(allowanceApi).toHaveBeenCalledWith("c1", expect.any(AbortSignal));
  });

  it("does not fetch during a live chat or without a conversation", () => {
    renderHook(() => useOffSessionAllowance("c1", true));
    renderHook(() => useOffSessionAllowance(null, false));
    expect(allowanceApi).not.toHaveBeenCalled();
  });

  it("reports a failed fetch, keeps retrying on demand, and recovers", async () => {
    allowanceApi.mockRejectedValueOnce(new ApiError({ code: "network_error", message: "offline" }));
    const { result } = renderHook(() => useOffSessionAllowance("c1", false));
    await waitFor(() => expect(result.current.error?.code).toBe("network_error"));
    expect(result.current.loading).toBe(false);
    allowanceApi.mockResolvedValue(a({ remaining: 5 }));
    act(() => result.current.reload());
    await waitFor(() => expect(result.current.allowance?.remaining).toBe(5));
    expect(result.current.error).toBeNull();
  });

  it("adopts the allowance a send response carried, without a request", async () => {
    allowanceApi.mockResolvedValue(a());
    const { result } = renderHook(() => useOffSessionAllowance("c1", false));
    await waitFor(() => expect(result.current.allowance).not.toBeNull());
    allowanceApi.mockClear();
    act(() => result.current.update(a({ used: 3, remaining: 17 })));
    expect(result.current.allowance?.remaining).toBe(17);
    act(() => result.current.update(undefined)); // old servers send none
    expect(result.current.allowance?.remaining).toBe(17);
    expect(allowanceApi).not.toHaveBeenCalled();
  });

  it("fetches again after a live chat starts or ends, and after a reconnect", async () => {
    allowanceApi.mockResolvedValue(a());
    const { result } = renderHook(() => useOffSessionAllowance("c1", false));
    await waitFor(() => expect(allowanceApi).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(result.current.allowance).not.toBeNull());
    act(() => realtime.session!("session.ended", {}));
    await waitFor(() => expect(allowanceApi).toHaveBeenCalledTimes(2));
    act(() => realtime.single.get("hello")!({ reconnected: true }));
    await waitFor(() => expect(allowanceApi).toHaveBeenCalledTimes(3));
    act(() => realtime.single.get("hello")!({ reconnected: false }));
    expect(allowanceApi).toHaveBeenCalledTimes(3);
  });

  it("fetches when a live chat ends (live flips to false) and forgets another conversation's answer", async () => {
    allowanceApi.mockResolvedValue(a({ remaining: 7 }));
    const { result, rerender } = renderHook(({ conv, live }) => useOffSessionAllowance(conv, live), { initialProps: { conv: "c1", live: true } });
    expect(allowanceApi).not.toHaveBeenCalled();
    rerender({ conv: "c1", live: false });
    await waitFor(() => expect(result.current.allowance?.remaining).toBe(7));
    allowanceApi.mockResolvedValue(a({ remaining: 1 }));
    rerender({ conv: "c2", live: false });
    // Never shows c1's numbers for c2.
    expect(result.current.allowance).toBeNull();
    await waitFor(() => expect(result.current.allowance?.remaining).toBe(1));
  });
});
