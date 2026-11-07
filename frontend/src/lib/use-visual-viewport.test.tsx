// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render } from "@testing-library/react";

import { useVisualViewport } from "@/lib/use-visual-viewport";

type VV = EventTarget & { height: number; offsetTop: number };
function fakeViewport(height: number): VV {
  const t = new EventTarget() as VV;
  t.height = height;
  t.offsetTop = 0;
  return t;
}
function Probe({ enabled = true, onChange }: { enabled?: boolean; onChange?: (n: number) => void }) {
  useVisualViewport(enabled, onChange);
  return null;
}
const root = () => document.documentElement;
let queue: FrameRequestCallback[] = [];
const flush = () => {
  const run = queue;
  queue = [];
  run.forEach((cb) => cb(0));
};

beforeEach(() => {
  queue = [];
  vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => queue.push(cb));
  vi.stubGlobal("cancelAnimationFrame", () => undefined);
  Object.defineProperty(window, "innerHeight", { configurable: true, value: 800 });
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  Object.defineProperty(window, "visualViewport", { configurable: true, value: undefined });
});

describe("useVisualViewport", () => {
  it("publishes the visible height and follows the keyboard", () => {
    const vv = fakeViewport(800);
    Object.defineProperty(window, "visualViewport", { configurable: true, value: vv });
    const onChange = vi.fn();
    render(<Probe onChange={onChange} />);
    expect(root().style.getPropertyValue("--vvh")).toBe("800px");
    expect(root().dataset.keyboard).toBeUndefined();
    act(() => {
      vv.height = 480;
      vv.dispatchEvent(new Event("resize"));
      flush();
    });
    expect(root().style.getPropertyValue("--vvh")).toBe("480px");
    expect(root().style.getPropertyValue("--kb-inset")).toBe("320px");
    expect(root().dataset.keyboard).toBe("open");
    expect(onChange).toHaveBeenLastCalledWith(320);
    act(() => {
      vv.height = 800;
      vv.dispatchEvent(new Event("resize"));
      flush();
    });
    expect(root().dataset.keyboard).toBeUndefined();
  });

  it("tracks the offset when the page is panned (iOS)", () => {
    const vv = fakeViewport(500);
    vv.offsetTop = 100;
    Object.defineProperty(window, "visualViewport", { configurable: true, value: vv });
    render(<Probe />);
    expect(root().style.getPropertyValue("--vv-top")).toBe("100px");
    expect(root().style.getPropertyValue("--kb-inset")).toBe("200px");
  });

  it("cleans up the variables on unmount", () => {
    Object.defineProperty(window, "visualViewport", { configurable: true, value: fakeViewport(700) });
    const { unmount } = render(<Probe />);
    unmount();
    expect(root().style.getPropertyValue("--vvh")).toBe("");
    expect(root().dataset.keyboard).toBeUndefined();
  });

  it("does nothing when disabled or unsupported", () => {
    Object.defineProperty(window, "visualViewport", { configurable: true, value: fakeViewport(700) });
    render(<Probe enabled={false} />);
    expect(root().style.getPropertyValue("--vvh")).toBe("");
    cleanup();
    Object.defineProperty(window, "visualViewport", { configurable: true, value: undefined });
    expect(() => render(<Probe />)).not.toThrow();
  });
});
