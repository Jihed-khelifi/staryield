// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { renderHook } from "@testing-library/react";

import { useAttemptKey } from "@/lib/billing/use-attempt-key";

describe("useAttemptKey", () => {
  it("replays the same key for the same pack and card (retry after an unknown outcome)", () => {
    const { result } = renderHook(() => useAttemptKey());
    expect(result.current.get("p1:new")).toBe(result.current.get("p1:new"));
  });

  it("starts a new attempt when the pack or the card changes", () => {
    const { result } = renderHook(() => useAttemptKey());
    const first = result.current.get("p1:new");
    expect(result.current.get("p2:new")).not.toBe(first);
    const second = result.current.get("p2:new");
    expect(result.current.get("p2:card_1")).not.toBe(second);
  });

  it("renews after a final outcome", () => {
    const { result } = renderHook(() => useAttemptKey());
    const first = result.current.get("p1:new");
    result.current.renew();
    expect(result.current.get("p1:new")).not.toBe(first);
  });

  it("is stable across renders", () => {
    const { result, rerender } = renderHook(() => useAttemptKey());
    const first = result.current;
    rerender();
    expect(result.current).toBe(first);
  });
});
