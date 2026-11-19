// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";

const router = vi.hoisted(() => ({ refresh: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

import { browser, retryRoot, useRouteRetry } from "@/lib/error-recovery";

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("retryRoot (the root error page)", () => {
  it("reloads the page when the framework gives no retry (vinext passes global-error only `reset`)", () => {
    const reload = vi.spyOn(browser, "reload").mockImplementation(() => {});
    retryRoot(undefined);
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it("uses the framework's retry when it gives one", () => {
    const reload = vi.spyOn(browser, "reload").mockImplementation(() => {});
    const retry = vi.fn();
    retryRoot(retry);
    expect(retry).toHaveBeenCalledTimes(1);
    expect(reload).not.toHaveBeenCalled();
  });
});

describe("useRouteRetry (a route's error boundary)", () => {
  it("with vinext's `reset` only: refreshes the route's data, then renders the segment again (Next's retry, by hand)", () => {
    const order: string[] = [];
    router.refresh.mockImplementation(() => order.push("refresh"));
    const reset = vi.fn(() => order.push("reset"));
    const { result } = renderHook(() => useRouteRetry({ reset }));
    act(() => result.current());
    expect(order).toEqual(["refresh", "reset"]);
  });

  it("prefers the framework's retry when it gives one", () => {
    const retry = vi.fn();
    const reset = vi.fn();
    const { result } = renderHook(() => useRouteRetry({ retry, reset }));
    act(() => result.current());
    expect(retry).toHaveBeenCalledTimes(1);
    expect(reset).not.toHaveBeenCalled();
    expect(router.refresh).not.toHaveBeenCalled();
  });

  it("reloads the page when it is given nothing to re-render with, so Try again is never a dead button", () => {
    const reload = vi.spyOn(browser, "reload").mockImplementation(() => {});
    const { result } = renderHook(() => useRouteRetry({}));
    act(() => result.current());
    expect(reload).toHaveBeenCalledTimes(1);
    expect(router.refresh).not.toHaveBeenCalled();
  });
});
