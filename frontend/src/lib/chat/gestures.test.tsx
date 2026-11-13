// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";

import {
  DOUBLE_TAP_MS,
  LONG_PRESS_MS,
  SWIPE_LOCK_PX,
  SWIPE_MAX_PX,
  SWIPE_THRESHOLD_PX,
  isDoubleTap,
  isGestureTarget,
  isSwipeReply,
  isTapTarget,
  movedTooFar,
  swipeMove,
  swipeOffset,
  swipeStart,
  useBubbleGestures,
} from "@/lib/chat/gestures";

describe("swipe (pure)", () => {
  it("stays undecided below the lock distance", () => {
    const s = swipeMove(swipeStart(100, 100), 100 + SWIPE_LOCK_PX - 1, 102);
    expect(s.axis).toBe("undecided");
  });

  it("locks to horizontal when the first movement is mostly rightwards", () => {
    const s = swipeMove(swipeStart(100, 100), 100 + SWIPE_LOCK_PX + 2, 104);
    expect(s.axis).toBe("horizontal");
    expect(s.dx).toBe(SWIPE_LOCK_PX + 2);
  });

  it("a vertical start is the browser's scroll and stays so, however far the finger then goes sideways", () => {
    let s = swipeMove(swipeStart(100, 100), 103, 100 + SWIPE_LOCK_PX + 4);
    expect(s.axis).toBe("vertical");
    s = swipeMove(s, 100 + SWIPE_THRESHOLD_PX + 40, 130);
    expect(s.axis).toBe("vertical");
    expect(s.crossed).toBe(false);
    expect(isSwipeReply(s)).toBe(false);
  });

  it("a swipe to the left is not ours", () => {
    expect(swipeMove(swipeStart(100, 100), 100 - 30, 101).axis).toBe("vertical");
  });

  it("crosses the threshold at 56px and un-crosses when the finger returns", () => {
    let s = swipeMove(swipeStart(0, 0), SWIPE_LOCK_PX + 1, 0);
    s = swipeMove(s, SWIPE_THRESHOLD_PX - 1, 0);
    expect(s.crossed).toBe(false);
    s = swipeMove(s, SWIPE_THRESHOLD_PX, 0);
    expect(s.crossed).toBe(true);
    expect(isSwipeReply(s)).toBe(true);
    s = swipeMove(s, 20, 0);
    expect(isSwipeReply(s)).toBe(false);
  });

  it("draws the bubble with the finger, then with resistance, never past the maximum", () => {
    expect(swipeOffset(-10)).toBe(0);
    expect(swipeOffset(30)).toBe(30);
    expect(swipeOffset(SWIPE_THRESHOLD_PX)).toBe(SWIPE_THRESHOLD_PX);
    expect(swipeOffset(SWIPE_THRESHOLD_PX + 20)).toBeGreaterThan(SWIPE_THRESHOLD_PX);
    expect(swipeOffset(SWIPE_THRESHOLD_PX + 20)).toBeLessThan(SWIPE_THRESHOLD_PX + 20);
    expect(swipeOffset(5000)).toBe(SWIPE_MAX_PX);
  });
});

describe("double-tap and long-press (pure)", () => {
  it("two close taps within the window are a double-tap", () => {
    expect(isDoubleTap({ t: 1000, x: 10, y: 10 }, { t: 1000 + DOUBLE_TAP_MS, x: 14, y: 12 })).toBe(true);
  });
  it("a slow second tap, a far one, or no first tap is not", () => {
    expect(isDoubleTap({ t: 1000, x: 10, y: 10 }, { t: 1000 + DOUBLE_TAP_MS + 1, x: 10, y: 10 })).toBe(false);
    expect(isDoubleTap({ t: 1000, x: 10, y: 10 }, { t: 1100, x: 200, y: 10 })).toBe(false);
    expect(isDoubleTap(null, { t: 1100, x: 10, y: 10 })).toBe(false);
  });
  it("movement beyond the slop cancels a long-press", () => {
    expect(movedTooFar(0, 0, 3, 4)).toBe(false);
    expect(movedTooFar(0, 0, 9, 9)).toBe(true);
  });
  it("links and marked elements keep their behaviour; the image only opts out of tapping", () => {
    const wrap = document.createElement("div");
    wrap.innerHTML = '<a href="#x"><span id="a"></span></a><button data-no-gesture id="q"></button><button data-no-tap id="i"></button><p id="p"></p>';
    const get = (id: string) => wrap.querySelector(`#${id}`);
    expect(isGestureTarget(get("a"))).toBe(false);
    expect(isGestureTarget(get("q"))).toBe(false);
    expect(isGestureTarget(get("i"))).toBe(true);
    expect(isTapTarget(get("i"))).toBe(false);
    expect(isTapTarget(get("p"))).toBe(true);
  });
});

function Probe(props: { enabled?: boolean; onReply: () => void; onLike: () => void; onMenu: (at: { x: number; y: number }) => void }) {
  const { attach, handlers } = useBubbleGestures({ enabled: props.enabled ?? true, onReply: props.onReply, onLike: props.onLike, onMenu: props.onMenu });
  return (
    <div ref={attach} data-testid="bubble" {...handlers}>
      <p data-testid="text">hello</p>
      <button type="button" data-no-tap data-testid="image">
        img
      </button>
      {/* The three-dot button of the bubble (§24.9) opts out of every bubble gesture. */}
      <button type="button" data-no-gesture data-testid="dots">
        ...
      </button>
    </div>
  );
}

const touch = (x: number, y: number) => ({ touches: [{ clientX: x, clientY: y }], changedTouches: [{ clientX: x, clientY: y }] });
const end = (x: number, y: number) => ({ touches: [], changedTouches: [{ clientX: x, clientY: y }] });

describe("useBubbleGestures", () => {
  const spies = () => ({ onReply: vi.fn(), onLike: vi.fn(), onMenu: vi.fn() });
  let vibrate: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    vi.useFakeTimers();
    vibrate = vi.fn();
    Object.defineProperty(navigator, "vibrate", { configurable: true, value: vibrate });
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("swipe right past the threshold replies, vibrates once when crossed, and springs back", () => {
    const s = spies();
    render(<Probe {...s} />);
    const bubble = screen.getByTestId("bubble");
    fireEvent.touchStart(bubble, touch(0, 0));
    fireEvent.touchMove(bubble, touch(20, 2));
    expect(bubble.dataset.swiping).toBe("true");
    expect(bubble.style.getPropertyValue("--swipe-x")).toBe("20px");
    fireEvent.touchMove(bubble, touch(SWIPE_THRESHOLD_PX + 4, 3));
    fireEvent.touchMove(bubble, touch(SWIPE_THRESHOLD_PX + 8, 3));
    expect(vibrate).toHaveBeenCalledTimes(1);
    expect(vibrate).toHaveBeenCalledWith(10);
    expect(bubble.dataset.swipeReady).toBe("true");
    fireEvent.touchEnd(bubble, end(SWIPE_THRESHOLD_PX + 8, 3));
    expect(s.onReply).toHaveBeenCalledTimes(1);
    expect(bubble.dataset.swiping).toBeUndefined();
    expect(bubble.style.getPropertyValue("--swipe-x")).toBe("");
  });

  it("a short swipe does nothing and springs back", () => {
    const s = spies();
    render(<Probe {...s} />);
    const bubble = screen.getByTestId("bubble");
    fireEvent.touchStart(bubble, touch(0, 0));
    fireEvent.touchMove(bubble, touch(30, 0));
    fireEvent.touchEnd(bubble, end(30, 0));
    expect(s.onReply).not.toHaveBeenCalled();
    expect(s.onLike).not.toHaveBeenCalled();
    expect(vibrate).not.toHaveBeenCalled();
    expect(bubble.dataset.swiping).toBeUndefined();
  });

  it("vertical scrolling is never hijacked: no transform, no reply", () => {
    const s = spies();
    render(<Probe {...s} />);
    const bubble = screen.getByTestId("bubble");
    fireEvent.touchStart(bubble, touch(0, 0));
    fireEvent.touchMove(bubble, touch(2, SWIPE_LOCK_PX + 5));
    fireEvent.touchMove(bubble, touch(SWIPE_THRESHOLD_PX + 20, SWIPE_LOCK_PX + 8));
    fireEvent.touchEnd(bubble, end(SWIPE_THRESHOLD_PX + 20, SWIPE_LOCK_PX + 8));
    expect(bubble.dataset.swiping).toBeUndefined();
    expect(s.onReply).not.toHaveBeenCalled();
  });

  it("does not follow the finger under reduced motion, but still replies", () => {
    window.matchMedia = ((q: string) => ({ matches: q.includes("reduce"), media: q, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {} })) as unknown as typeof window.matchMedia;
    const s = spies();
    render(<Probe {...s} />);
    const bubble = screen.getByTestId("bubble");
    fireEvent.touchStart(bubble, touch(0, 0));
    fireEvent.touchMove(bubble, touch(30, 0));
    expect(bubble.style.getPropertyValue("--swipe-x")).toBe("");
    fireEvent.touchMove(bubble, touch(SWIPE_THRESHOLD_PX + 5, 0));
    fireEvent.touchEnd(bubble, end(SWIPE_THRESHOLD_PX + 5, 0));
    expect(s.onReply).toHaveBeenCalledTimes(1);
    // @ts-expect-error restore the jsdom default (no matchMedia)
    delete window.matchMedia;
  });

  it("two quick taps like; two slow taps do not", () => {
    const s = spies();
    render(<Probe {...s} />);
    const text = screen.getByTestId("text");
    const tap = () => {
      fireEvent.touchStart(text, touch(50, 50));
      fireEvent.touchEnd(text, end(50, 50));
    };
    tap();
    vi.advanceTimersByTime(DOUBLE_TAP_MS - 100);
    tap();
    expect(s.onLike).toHaveBeenCalledTimes(1);
    // A third tap right after starts over.
    vi.advanceTimersByTime(50);
    tap();
    expect(s.onLike).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(DOUBLE_TAP_MS + 100);
    tap();
    expect(s.onLike).toHaveBeenCalledTimes(1);
  });

  it("double-tapping the image does not like", () => {
    const s = spies();
    render(<Probe {...s} />);
    const image = screen.getByTestId("image");
    for (let i = 0; i < 2; i++) {
      fireEvent.touchStart(image, touch(50, 50));
      fireEvent.touchEnd(image, end(50, 50));
    }
    expect(s.onLike).not.toHaveBeenCalled();
  });

  it("holding opens the menu after 450 ms at the finger", () => {
    const s = spies();
    render(<Probe {...s} />);
    const bubble = screen.getByTestId("bubble");
    fireEvent.touchStart(bubble, touch(40, 80));
    act(() => void vi.advanceTimersByTime(LONG_PRESS_MS - 1));
    expect(s.onMenu).not.toHaveBeenCalled();
    act(() => void vi.advanceTimersByTime(2));
    expect(s.onMenu).toHaveBeenCalledWith({ x: 40, y: 80 });
    // Lifting the finger afterwards is not a tap or a swipe.
    fireEvent.touchEnd(bubble, end(40, 80));
    expect(s.onLike).not.toHaveBeenCalled();
    expect(s.onReply).not.toHaveBeenCalled();
  });

  it("the three-dot button is not part of the gestures: no long-press, swipe or double-tap starts on it", () => {
    const s = spies();
    render(<Probe {...s} />);
    const dots = screen.getByTestId("dots");
    fireEvent.touchStart(dots, touch(40, 80));
    act(() => void vi.advanceTimersByTime(LONG_PRESS_MS + 50));
    expect(s.onMenu).not.toHaveBeenCalled();
    fireEvent.touchEnd(dots, end(40, 80));
    fireEvent.touchStart(dots, touch(40, 80));
    fireEvent.touchEnd(dots, end(40, 80));
    expect(s.onLike).not.toHaveBeenCalled();
    fireEvent.doubleClick(dots);
    expect(s.onLike).not.toHaveBeenCalled();
  });

  it("moving cancels the long-press; lifting early does too", () => {
    const s = spies();
    render(<Probe {...s} />);
    const bubble = screen.getByTestId("bubble");
    fireEvent.touchStart(bubble, touch(40, 80));
    fireEvent.touchMove(bubble, touch(40, 80 + 14));
    act(() => void vi.advanceTimersByTime(LONG_PRESS_MS + 100));
    expect(s.onMenu).not.toHaveBeenCalled();
    fireEvent.touchEnd(bubble, end(40, 94));

    fireEvent.touchStart(bubble, touch(40, 80));
    act(() => void vi.advanceTimersByTime(200));
    fireEvent.touchEnd(bubble, end(40, 80));
    act(() => void vi.advanceTimersByTime(LONG_PRESS_MS));
    expect(s.onMenu).not.toHaveBeenCalled();
  });

  it("double-click likes on desktop, but not right after a touch (browsers synthesise one)", () => {
    const s = spies();
    render(<Probe {...s} />);
    const text = screen.getByTestId("text");
    fireEvent.doubleClick(text);
    expect(s.onLike).toHaveBeenCalledTimes(1);
    fireEvent.doubleClick(screen.getByTestId("image"));
    expect(s.onLike).toHaveBeenCalledTimes(1);
    fireEvent.touchStart(text, touch(1, 1));
    fireEvent.touchEnd(text, end(1, 1));
    fireEvent.doubleClick(text);
    expect(s.onLike).toHaveBeenCalledTimes(1);
  });

  it("does nothing when disabled (pending, failed, system messages)", () => {
    const s = spies();
    render(<Probe {...s} enabled={false} />);
    const bubble = screen.getByTestId("bubble");
    fireEvent.touchStart(bubble, touch(0, 0));
    fireEvent.touchMove(bubble, touch(SWIPE_THRESHOLD_PX + 10, 0));
    fireEvent.touchEnd(bubble, end(SWIPE_THRESHOLD_PX + 10, 0));
    fireEvent.doubleClick(bubble);
    act(() => void vi.advanceTimersByTime(LONG_PRESS_MS + 50));
    expect(s.onReply).not.toHaveBeenCalled();
    expect(s.onLike).not.toHaveBeenCalled();
    expect(s.onMenu).not.toHaveBeenCalled();
  });
});
