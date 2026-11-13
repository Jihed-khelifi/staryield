// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";

import { ScrollbarKind, hasClassicScrollbars } from "@/components/layout/scrollbar-kind";

afterEach(() => {
  cleanup();
  delete document.documentElement.dataset.scrollbars;
  Reflect.deleteProperty(HTMLElement.prototype, "offsetWidth");
  Reflect.deleteProperty(HTMLElement.prototype, "clientWidth");
});

/** jsdom has no layout: report the widths of a 60px box with (15px gutter) or without a classic scrollbar. */
function scrollbarGutter(px: number) {
  Object.defineProperty(HTMLElement.prototype, "offsetWidth", { configurable: true, get: () => 60 });
  Object.defineProperty(HTMLElement.prototype, "clientWidth", { configurable: true, get: () => 60 - px });
}

describe("hasClassicScrollbars", () => {
  it("is true when a forced scrollbar takes space, false when it floats over the content", () => {
    scrollbarGutter(15);
    expect(hasClassicScrollbars()).toBe(true);
    scrollbarGutter(0);
    expect(hasClassicScrollbars()).toBe(false);
  });

  it("leaves no probe element behind", () => {
    scrollbarGutter(15);
    hasClassicScrollbars();
    expect(document.body.children).toHaveLength(0);
  });
});

describe("ScrollbarKind", () => {
  it("marks <html> on platforms with classic scrollbars and removes the mark on unmount", () => {
    scrollbarGutter(15);
    const { unmount } = render(<ScrollbarKind />);
    expect(document.documentElement.dataset.scrollbars).toBe("classic");
    unmount();
    expect(document.documentElement.dataset.scrollbars).toBeUndefined();
  });

  it("leaves overlay-scrollbar platforms alone", () => {
    scrollbarGutter(0);
    render(<ScrollbarKind />);
    expect(document.documentElement.dataset.scrollbars).toBeUndefined();
  });
});
