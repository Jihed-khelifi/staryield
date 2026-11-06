// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, renderHook } from "@testing-library/react";

import { useUnsavedChangesGuard } from "@/lib/forms/unsaved-changes";

afterEach(() => {
  cleanup();
  document.body.innerHTML = "";
  vi.restoreAllMocks();
});

function clickLink(href: string, init: MouseEventInit = {}) {
  const event = new MouseEvent("click", { bubbles: true, cancelable: true, button: 0, ...init });
  document.querySelector(`a[href="${href}"]`)!.dispatchEvent(event);
  return event;
}

/** Plain anchors (the guard must work on raw <a> as well as next/link). */
function renderLinks() {
  const nav = document.createElement("nav");
  nav.innerHTML =
    '<a href="/en/profile">Dashboard</a><a href="https://example.com/x">External</a><a href="/en/terms" target="_blank">Terms</a>';
  document.body.appendChild(nav);
}

describe("useUnsavedChangesGuard", () => {
  it("asks before following an in-app link and stays when cancelled", () => {
    const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
    renderLinks();
    renderHook(() => useUnsavedChangesGuard(true, "Leave without saving?"));
    const event = clickLink("/en/profile");
    expect(confirm).toHaveBeenCalledWith("Leave without saving?");
    expect(event.defaultPrevented).toBe(true);
  });

  it("lets the navigation through when confirmed, and ignores external/new-tab links", () => {
    const confirm = vi.spyOn(window, "confirm").mockReturnValue(true);
    renderLinks();
    renderHook(() => useUnsavedChangesGuard(true, "Leave?"));
    expect(clickLink("/en/profile").defaultPrevented).toBe(false);
    expect(confirm).toHaveBeenCalledTimes(1);
    clickLink("https://example.com/x");
    clickLink("/en/terms");
    clickLink("/en/profile", { metaKey: true });
    expect(confirm).toHaveBeenCalledTimes(1);
  });

  it("does nothing when the form is clean and blocks unload when dirty", () => {
    const confirm = vi.spyOn(window, "confirm");
    renderLinks();
    const { rerender } = renderHook(({ dirty }) => useUnsavedChangesGuard(dirty, "Leave?"), {
      initialProps: { dirty: false },
    });
    clickLink("/en/profile");
    expect(confirm).not.toHaveBeenCalled();
    const unload = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(unload);
    expect(unload.defaultPrevented).toBe(false);
    rerender({ dirty: true });
    const dirtyUnload = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(dirtyUnload);
    expect(dirtyUnload.defaultPrevented).toBe(true);
  });
});
