"use client";

/**
 * Touch gestures on a chat bubble (§23.2, §23.3): swipe right to reply,
 * double-tap to like, long-press for the action menu. The decisions are pure
 * functions (unit-tested); `useBubbleGestures` binds them to touch events.
 *
 * Vertical scrolling is never taken over: the bubble sets `touch-action:
 * pan-y` (the browser keeps vertical pans and double-tap zoom is off), and the
 * direction is locked after `SWIPE_LOCK_PX` — a gesture that starts vertical
 * stays the browser's.
 */
import * as React from "react";

/** Distance the bubble must travel to the right to count as "reply". */
export const SWIPE_THRESHOLD_PX = 56;
/** Movement after which the direction (horizontal or vertical) is decided. */
export const SWIPE_LOCK_PX = 10;
/** The bubble never follows the finger further than this. */
export const SWIPE_MAX_PX = 84;
/** Two taps this close in time are a double-tap. */
export const DOUBLE_TAP_MS = 300;
/** … and this close together on screen. */
export const DOUBLE_TAP_SLOP_PX = 32;
/** Pressing this long opens the menu. */
export const LONG_PRESS_MS = 450;
/** Moving more than this cancels the long-press. */
export const LONG_PRESS_SLOP_PX = 10;
/** Browsers may emit a `dblclick` or `contextmenu` after a touch: ignore those for this long. */
const SYNTHETIC_MOUSE_MS = 800;

export type SwipeAxis = "undecided" | "horizontal" | "vertical";

export type SwipeState = {
  startX: number;
  startY: number;
  axis: SwipeAxis;
  /** Raw horizontal distance (to the right is positive). */
  dx: number;
  /** The reply threshold is currently crossed. */
  crossed: boolean;
};

export function swipeStart(x: number, y: number): SwipeState {
  return { startX: x, startY: y, axis: "undecided", dx: 0, crossed: false };
}

/**
 * Advance a swipe with a new touch position. The first `lock` pixels of
 * movement decide the axis: mostly rightwards is a swipe, anything else
 * (vertical, or to the left) belongs to the browser and stays that way.
 */
export function swipeMove(state: SwipeState, x: number, y: number, opts: { lock?: number; threshold?: number } = {}): SwipeState {
  const lock = opts.lock ?? SWIPE_LOCK_PX;
  const threshold = opts.threshold ?? SWIPE_THRESHOLD_PX;
  const dx = x - state.startX;
  const dy = y - state.startY;
  let axis = state.axis;
  if (axis === "undecided" && Math.max(Math.abs(dx), Math.abs(dy)) >= lock) {
    axis = Math.abs(dx) > Math.abs(dy) && dx > 0 ? "horizontal" : "vertical";
  }
  if (axis !== "horizontal") return { ...state, axis, dx: 0, crossed: false };
  return { ...state, axis, dx, crossed: dx >= threshold };
}

/** How far the bubble is drawn for a raw distance: follows the finger, then gives (rubber band), never past the maximum. */
export function swipeOffset(dx: number, threshold = SWIPE_THRESHOLD_PX, max = SWIPE_MAX_PX): number {
  if (dx <= 0) return 0;
  if (dx <= threshold) return dx;
  return Math.min(max, threshold + (dx - threshold) * 0.35);
}

/** The swipe ended past the threshold while still horizontal. */
export function isSwipeReply(state: SwipeState): boolean {
  return state.axis === "horizontal" && state.crossed;
}

export type Tap = { t: number; x: number; y: number };

/** Whether `next` completes a double-tap that began with `prev`. */
export function isDoubleTap(prev: Tap | null, next: Tap, windowMs = DOUBLE_TAP_MS, slop = DOUBLE_TAP_SLOP_PX): boolean {
  if (!prev) return false;
  const dt = next.t - prev.t;
  return dt >= 0 && dt <= windowMs && Math.hypot(next.x - prev.x, next.y - prev.y) <= slop;
}

/** The pointer moved far enough from where it went down to cancel a long-press. */
export function movedTooFar(startX: number, startY: number, x: number, y: number, slop = LONG_PRESS_SLOP_PX): boolean {
  return Math.hypot(x - startX, y - startY) > slop;
}

/** Links, fields and anything marked `data-no-gesture` (the quote) keep their own behaviour entirely. */
export function isGestureTarget(target: EventTarget | null): boolean {
  return !(target instanceof Element && target.closest("a, input, textarea, [data-no-gesture]"));
}

/** The image opens its viewer on a tap: swipe and long-press work over it, double-tap/click does not like (`data-no-tap`). */
export function isTapTarget(target: EventTarget | null): boolean {
  return isGestureTarget(target) && !(target instanceof Element && target.closest("[data-no-tap]"));
}

function prefersReducedMotion(): boolean {
  return typeof window !== "undefined" && Boolean(window.matchMedia?.("(prefers-reduced-motion: reduce)").matches);
}

export type BubbleGestureOptions = {
  /** Pending, failed and system messages have no actions. */
  enabled: boolean;
  onReply: () => void;
  onLike: () => void;
  /** Long-press: where the menu should open (viewport coordinates). */
  onMenu: (at: { x: number; y: number }) => void;
};

/**
 * Handlers and the element whose position follows the finger. Attach
 * `handlers` to the bubble and `attach` (a callback ref) to the same element; the position is
 * written straight to the element (no re-render per move) and springs back
 * through CSS when the finger lifts.
 */
export function useBubbleGestures({ enabled, onReply, onLike, onMenu }: BubbleGestureOptions) {
  const element = React.useRef<HTMLElement | null>(null);
  const attach = React.useCallback((el: HTMLElement | null) => {
    element.current = el;
  }, []);
  const latest = React.useRef({ enabled, onReply, onLike, onMenu });
  React.useEffect(() => {
    latest.current = { enabled, onReply, onLike, onMenu };
  });

  const swipe = React.useRef<SwipeState | null>(null);
  const lastTap = React.useRef<Tap | null>(null);
  const lastTouchAt = React.useRef(0);
  const pressTimer = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const pressed = React.useRef(false);
  const tapOk = React.useRef(true);
  const reduced = React.useRef(false);

  const clearPress = React.useCallback(() => {
    if (pressTimer.current) clearTimeout(pressTimer.current);
    pressTimer.current = null;
  }, []);
  React.useEffect(() => clearPress, [clearPress]);

  const draw = React.useCallback((px: number, crossed: boolean) => {
    const el = element.current;
    if (!el) return;
    if (px > 0) {
      el.dataset.swiping = "true";
      el.style.setProperty("--swipe-x", `${px}px`);
      el.style.setProperty("--swipe-progress", String(Math.min(1, px / SWIPE_THRESHOLD_PX)));
    } else {
      delete el.dataset.swiping;
      el.style.removeProperty("--swipe-x");
      el.style.removeProperty("--swipe-progress");
    }
    if (crossed) el.dataset.swipeReady = "true";
    else delete el.dataset.swipeReady;
  }, []);

  const finish = React.useCallback(() => {
    swipe.current = null;
    clearPress();
    draw(0, false);
  }, [clearPress, draw]);

  const onTouchStart = (e: React.TouchEvent) => {
    if (!latest.current.enabled || e.touches.length !== 1 || !isGestureTarget(e.target)) {
      swipe.current = null;
      return;
    }
    const touch = e.touches[0];
    lastTouchAt.current = Date.now();
    pressed.current = false;
    tapOk.current = isTapTarget(e.target);
    reduced.current = prefersReducedMotion();
    swipe.current = swipeStart(touch.clientX, touch.clientY);
    clearPress();
    pressTimer.current = setTimeout(() => {
      pressTimer.current = null;
      if (!swipe.current || swipe.current.axis === "horizontal") return;
      pressed.current = true;
      // The finger stays down: the swipe is over, the menu takes the gesture.
      swipe.current = null;
      latest.current.onMenu({ x: touch.clientX, y: touch.clientY });
    }, LONG_PRESS_MS);
  };

  const onTouchMove = (e: React.TouchEvent) => {
    const state = swipe.current;
    if (!state) return;
    const touch = e.touches[0];
    if (!touch) return;
    if (movedTooFar(state.startX, state.startY, touch.clientX, touch.clientY)) clearPress();
    const next = swipeMove(state, touch.clientX, touch.clientY);
    // Crossing the threshold is felt, once per crossing.
    if (next.crossed && !state.crossed) navigator.vibrate?.(10);
    swipe.current = next;
    if (next.axis === "horizontal") draw(reduced.current ? 0 : swipeOffset(next.dx), next.crossed);
  };

  const onTouchEnd = (e: React.TouchEvent) => {
    lastTouchAt.current = Date.now();
    clearPress();
    if (pressed.current) {
      // The menu opened under the finger: lifting it must not tap anything.
      pressed.current = false;
      e.preventDefault();
      finish();
      return;
    }
    const state = swipe.current;
    finish();
    if (!state) return;
    if (isSwipeReply(state)) {
      latest.current.onReply();
      lastTap.current = null;
      return;
    }
    if (state.axis !== "undecided" || !tapOk.current) {
      lastTap.current = null;
      return; // a scroll or a short drag, or the image: not a tap that likes
    }
    const touch = e.changedTouches[0];
    const tap: Tap = { t: Date.now(), x: touch?.clientX ?? state.startX, y: touch?.clientY ?? state.startY };
    if (isDoubleTap(lastTap.current, tap)) {
      lastTap.current = null;
      latest.current.onLike();
    } else {
      lastTap.current = tap;
    }
  };

  const onTouchCancel = () => {
    finish();
  };

  /** Mouse: double-click likes (touch double-taps are handled above; some browsers also emit a dblclick for them). */
  const onDoubleClick = (e: React.MouseEvent) => {
    if (!latest.current.enabled || !isTapTarget(e.target)) return;
    if (Date.now() - lastTouchAt.current < SYNTHETIC_MOUSE_MS) return;
    latest.current.onLike();
  };

  /** Android emits `contextmenu` for a long-press: our menu replaces the browser's. */
  const onContextMenu = (e: React.MouseEvent) => {
    if (Date.now() - lastTouchAt.current < SYNTHETIC_MOUSE_MS * 4 || pressed.current) e.preventDefault();
  };

  return { attach, handlers: { onTouchStart, onTouchMove, onTouchEnd, onTouchCancel, onDoubleClick, onContextMenu } };
}
