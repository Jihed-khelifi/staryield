"use client";

import * as React from "react";

/**
 * Whether scrollbars take layout space (classic: Windows, Linux, macOS with a
 * mouse and "always show") or float over the content (overlay: macOS with a
 * trackpad, phones). A probe box with a forced scrollbar tells: an overlay
 * scrollbar leaves no gutter.
 */
export function hasClassicScrollbars(doc: Document = document): boolean {
  const probe = doc.createElement("div");
  probe.style.cssText = "position:absolute;top:-9999px;width:60px;height:60px;overflow:scroll;visibility:hidden;";
  doc.body.appendChild(probe);
  const gutter = probe.offsetWidth - probe.clientWidth;
  probe.remove();
  return gutter > 0;
}

/**
 * revisions 5.0 (§24.9): marks `<html data-scrollbars="classic">` so the
 * chatroom can draw its thin, low-contrast scrollbars with the WebKit
 * pseudo-elements only where scrollbars are classic. Overlay scrollbars keep
 * the platform's own look (the pseudo-elements would turn them into classic
 * ones). Renders nothing.
 */
export function ScrollbarKind() {
  React.useEffect(() => {
    const root = document.documentElement;
    if (hasClassicScrollbars()) root.dataset.scrollbars = "classic";
    return () => {
      delete root.dataset.scrollbars;
    };
  }, []);
  return null;
}
