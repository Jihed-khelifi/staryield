"use client";
import * as React from "react";

/**
 * Publish `window.visualViewport.height` as the CSS variable `--vvh` (plus
 * `--vv-top`, the covered bottom area `--kb-inset` and `data-keyboard=open`) on <html>, so layouts follow the
 * visible area when a phone keyboard opens (iOS Safari does not resize the
 * layout viewport). Browsers without visualViewport keep the CSS fallbacks.
 * `onChange` receives the keyboard inset in px.
 */
export function useVisualViewport(enabled = true, onChange?: (keyboardInset: number) => void) {
  const cb = React.useRef(onChange);
  React.useEffect(() => {
    cb.current = onChange;
  });
  React.useEffect(() => {
    const vv = typeof window !== "undefined" ? window.visualViewport : null;
    if (!enabled || !vv) return;
    const root = document.documentElement;
    let frame = 0;
    const apply = () => {
      frame = 0;
      const inset = Math.max(0, Math.round(window.innerHeight - vv.height - vv.offsetTop));
      root.style.setProperty("--vvh", `${Math.round(vv.height)}px`);
      root.style.setProperty("--kb-inset", `${inset}px`);
      root.style.setProperty("--vv-top", `${Math.round(vv.offsetTop)}px`);
      // A keyboard is a large covered area; small insets are browser toolbars.
      if (inset > 120) root.dataset.keyboard = "open";
      else delete root.dataset.keyboard;
      cb.current?.(inset);
    };
    const schedule = () => {
      if (!frame) frame = window.requestAnimationFrame(apply);
    };
    apply();
    vv.addEventListener("resize", schedule);
    vv.addEventListener("scroll", schedule);
    return () => {
      vv.removeEventListener("resize", schedule);
      vv.removeEventListener("scroll", schedule);
      if (frame) window.cancelAnimationFrame(frame);
      root.style.removeProperty("--vvh");
      root.style.removeProperty("--kb-inset");
      root.style.removeProperty("--vv-top");
      delete root.dataset.keyboard;
    };
  }, [enabled]);
}
