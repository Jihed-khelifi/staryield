// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { renderHook } from "@testing-library/react";

import { KEYBOARD_CLOSE_PX, KEYBOARD_OPEN_PX, keyboardState, useVisualViewport } from "@/lib/use-visual-viewport";

describe("keyboardState (hysteresis, §22.5)", () => {
  it("opens above 120 px and closes below 80 px; in between it keeps its state", () => {
    expect(KEYBOARD_OPEN_PX).toBe(120);
    expect(KEYBOARD_CLOSE_PX).toBe(80);
    expect(keyboardState(121, false)).toBe(true);
    expect(keyboardState(120, false)).toBe(false);
    expect(keyboardState(79, true)).toBe(false);
    expect(keyboardState(80, true)).toBe(true);
    for (const inset of [80, 100, 120]) {
      expect(keyboardState(inset, true)).toBe(true);
      expect(keyboardState(inset, false)).toBe(false);
    }
  });
});

describe("useVisualViewport", () => {
  afterEach(() => {
    delete (window as { visualViewport?: unknown }).visualViewport;
    delete document.documentElement.dataset.keyboard;
  });

  it("does not flap between 80 and 120 px of covered area", async () => {
    const listeners = new Set<() => void>();
    const vv = { height: 800, offsetTop: 0, addEventListener: (_: string, l: () => void) => listeners.add(l), removeEventListener: (_: string, l: () => void) => listeners.delete(l) };
    Object.defineProperty(window, "visualViewport", { configurable: true, value: vv });
    Object.defineProperty(window, "innerHeight", { configurable: true, value: 800 });
    const frames: FrameRequestCallback[] = [];
    window.requestAnimationFrame = ((cb: FrameRequestCallback) => frames.push(cb)) as typeof window.requestAnimationFrame;
    const settle = (height: number) => {
      vv.height = height;
      listeners.forEach((l) => l());
      frames.splice(0).forEach((cb) => cb(0));
      return document.documentElement.dataset.keyboard;
    };
    const { unmount } = renderHook(() => useVisualViewport());
    expect(document.documentElement.dataset.keyboard).toBeUndefined();
    expect(settle(500)).toBe("open"); // 300 px covered
    expect(settle(690)).toBe("open"); // 110 px: still open
    expect(settle(710)).toBe("open"); // 90 px
    expect(settle(730)).toBeUndefined(); // 70 px: closed
    expect(settle(690)).toBeUndefined(); // 110 px: stays closed
    expect(settle(670)).toBe("open"); // 130 px
    unmount();
  });
});
