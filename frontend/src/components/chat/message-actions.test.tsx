// @vitest-environment jsdom
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { axeViolations, renderWithI18n } from "@/test/render";
import { MessageMenu, MessageMenuTrigger, menuPosition, type MenuAnchor } from "@/components/chat/message-actions";

afterEach(cleanup);

const VIEWPORT = { width: 400, height: 800 };
const rectOf = (left: number, top: number, width: number, height: number) => ({ left, top, width, height, right: left + width, bottom: top + height, x: left, y: top, toJSON: () => ({}) }) as DOMRect;
const elementAt = (left: number, top: number, width = 20, height = 20) => {
  const el = document.createElement("button");
  el.getBoundingClientRect = () => rectOf(left, top, width, height);
  return el;
};

describe("menuPosition", () => {
  it("opens under the button, starting at its inline start", () => {
    const anchor: MenuAnchor = { element: elementAt(100, 200), align: "start" };
    expect(menuPosition(anchor, 168, 150, VIEWPORT)).toEqual({ left: 100, top: 224 });
  });

  it("opens under the button, ending at its inline end", () => {
    const anchor: MenuAnchor = { element: elementAt(300, 200), align: "end" };
    // right edge 320 - width 168
    expect(menuPosition(anchor, 168, 150, VIEWPORT)).toEqual({ left: 152, top: 224 });
  });

  it("stays inside the screen sideways", () => {
    expect(menuPosition({ element: elementAt(390, 200), align: "start" }, 168, 150, VIEWPORT).left).toBe(VIEWPORT.width - 168 - 8);
    expect(menuPosition({ element: elementAt(2, 200), align: "end" }, 168, 150, VIEWPORT).left).toBe(8);
  });

  it("flips above the button near the bottom of the screen", () => {
    const { top } = menuPosition({ element: elementAt(100, 740), align: "start" }, 168, 150, VIEWPORT);
    expect(top).toBe(740 - 150 - 4);
  });

  it("at a touch point, opens above the finger, centred and inside the screen", () => {
    expect(menuPosition({ x: 200, y: 400 }, 168, 150, VIEWPORT)).toEqual({ left: 116, top: 238 });
    expect(menuPosition({ x: 5, y: 10 }, 168, 150, VIEWPORT)).toEqual({ left: 8, top: 8 });
    expect(menuPosition({ x: 399, y: 790 }, 168, 150, VIEWPORT).left).toBe(VIEWPORT.width - 168 - 8);
  });
});

function Harness({
  mine = false,
  saved = false,
  liked = false,
  hasText = true,
  handlers = {},
}: {
  mine?: boolean;
  saved?: boolean;
  liked?: boolean;
  hasText?: boolean;
  handlers?: { reply?: () => void; like?: () => void; bookmark?: () => void };
}) {
  const triggerRef = React.useRef<HTMLButtonElement>(null);
  const [anchor, setAnchor] = React.useState<MenuAnchor | null>(null);
  const close = React.useCallback(() => setAnchor(null), []);
  return (
    <div>
      <MessageMenuTrigger ref={triggerRef} mine={mine} open={Boolean(anchor)} onOpen={() => triggerRef.current && setAnchor({ element: triggerRef.current, align: mine ? "start" : "end" })} />
      <input aria-label="Elsewhere" />
      {anchor ? (
        <MessageMenu
          anchor={anchor}
          liked={liked}
          saved={saved}
          hasText={hasText}
          copyText="hello"
          onReply={handlers.reply ?? vi.fn()}
          onLike={handlers.like ?? vi.fn()}
          onBookmark={handlers.bookmark ?? vi.fn()}
          onClose={close}
          returnFocusTo={"element" in anchor ? anchor.element : null}
        />
      ) : null}
    </div>
  );
}

const trigger = () => screen.getByRole("button", { name: "Message actions" });

describe("three-dot button", () => {
  it("is an icon-only button that announces the menu", () => {
    renderWithI18n(<Harness />);
    expect(trigger().textContent).toBe("");
    expect(trigger().querySelector("svg")?.getAttribute("aria-hidden")).toBe("true");
    expect(trigger().getAttribute("aria-haspopup")).toBe("menu");
    expect(trigger().getAttribute("aria-expanded")).toBe("false");
  });

  it("says which side of the bubble it belongs to through data attributes", () => {
    const { unmount } = renderWithI18n(<Harness mine />);
    expect(trigger().getAttribute("data-mine")).toBe("true");
    unmount();
    renderWithI18n(<Harness />);
    expect(trigger().hasAttribute("data-mine")).toBe(false);
  });

  it("opens with Enter, Space, a click and the arrow keys", async () => {
    const user = userEvent.setup();
    renderWithI18n(<Harness />);
    for (const key of ["{Enter}", " ", "{ArrowDown}", "{ArrowUp}"]) {
      trigger().focus();
      await user.keyboard(key);
      expect(screen.getByRole("menu", { name: "Message actions" })).toBeTruthy();
      expect(trigger().getAttribute("aria-expanded")).toBe("true");
      await user.keyboard("{Escape}");
      expect(screen.queryByRole("menu")).toBeNull();
    }
    await user.click(trigger());
    expect(screen.getByRole("menu")).toBeTruthy();
  });
});

describe("message menu", () => {
  it("lists Reply, Bookmark, Like and Copy text and focuses the first item", async () => {
    renderWithI18n(<Harness />);
    fireEvent.click(trigger());
    const menu = screen.getByRole("menu");
    expect(menu.getAttribute("data-anchor")).toBe("button");
    expect(within(menu).getAllByRole("menuitem").map((i) => i.textContent)).toEqual(["Reply", "Bookmark", "Like", "Copy text"]);
    expect(document.activeElement).toBe(within(menu).getByRole("menuitem", { name: "Reply" }));
  });

  it("names the state: Remove bookmark, Unlike; no Copy text without text", () => {
    renderWithI18n(<Harness saved liked hasText={false} />);
    fireEvent.click(trigger());
    expect(within(screen.getByRole("menu")).getAllByRole("menuitem").map((i) => i.textContent)).toEqual(["Reply", "Remove bookmark", "Unlike"]);
  });

  it("the arrow keys, Home and End move between the items and wrap", async () => {
    const user = userEvent.setup();
    renderWithI18n(<Harness />);
    fireEvent.click(trigger());
    const names = () => (document.activeElement as HTMLElement).textContent;
    expect(names()).toBe("Reply");
    await user.keyboard("{ArrowDown}");
    expect(names()).toBe("Bookmark");
    await user.keyboard("{ArrowDown}{ArrowDown}");
    expect(names()).toBe("Copy text");
    await user.keyboard("{ArrowDown}");
    expect(names()).toBe("Reply");
    await user.keyboard("{ArrowUp}");
    expect(names()).toBe("Copy text");
    await user.keyboard("{Home}");
    expect(names()).toBe("Reply");
    await user.keyboard("{End}");
    expect(names()).toBe("Copy text");
  });

  it("Escape closes it and gives the focus back to the button", async () => {
    const user = userEvent.setup();
    renderWithI18n(<Harness />);
    trigger().focus();
    await user.keyboard("{Enter}");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(trigger());
  });

  it("Tab closes it too and carries on from the button (the next stop is the one after it)", async () => {
    const user = userEvent.setup();
    renderWithI18n(<Harness />);
    trigger().focus();
    await user.keyboard("{Enter}");
    await user.keyboard("{Tab}");
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(screen.getByLabelText("Elsewhere"));
  });

  it("a press outside closes it", async () => {
    renderWithI18n(<Harness />);
    fireEvent.click(trigger());
    await act(async () => void (await new Promise((r) => setTimeout(r, 80))));
    fireEvent.pointerDown(screen.getByLabelText("Elsewhere"));
    expect(screen.queryByRole("menu")).toBeNull();
  });

  // revisions 5.0 (integration): a menu opened from the button listens at once, so a quick click elsewhere closes it.
  it("a press outside right after a button or keyboard opening closes it (no grace period)", () => {
    renderWithI18n(<Harness />);
    fireEvent.click(trigger());
    expect(screen.getByRole("menu")).toBeTruthy();
    fireEvent.pointerDown(screen.getByLabelText("Elsewhere"));
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("a press on the button itself is not \"outside\" (the button toggles the menu)", () => {
    renderWithI18n(<Harness />);
    fireEvent.click(trigger());
    fireEvent.pointerDown(trigger());
    expect(screen.getByRole("menu")).toBeTruthy();
  });

  it("a menu opened at a touch point ignores the press that released the long-press", async () => {
    function TouchHarness() {
      const [open, setOpen] = React.useState(true);
      return open ? (
        <>
          <input aria-label="Under the finger" />
          <MessageMenu anchor={{ x: 100, y: 100 }} liked={false} saved={false} hasText copyText="hi" onReply={vi.fn()} onLike={vi.fn()} onBookmark={vi.fn()} onClose={() => setOpen(false)} />
        </>
      ) : null;
    }
    renderWithI18n(<TouchHarness />);
    fireEvent.touchStart(screen.getByLabelText("Under the finger"));
    expect(screen.getByRole("menu")).toBeTruthy();
    await act(async () => void (await new Promise((r) => setTimeout(r, 80))));
    fireEvent.touchStart(screen.getByLabelText("Under the finger"));
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("each item calls its action, closes the menu and returns the focus to the button", async () => {
    const handlers = { reply: vi.fn(), like: vi.fn(), bookmark: vi.fn() };
    renderWithI18n(<Harness handlers={handlers} />);
    for (const [name, fn] of [
      ["Bookmark", handlers.bookmark],
      ["Like", handlers.like],
    ] as const) {
      fireEvent.click(trigger());
      fireEvent.click(within(screen.getByRole("menu")).getByRole("menuitem", { name }));
      expect(fn).toHaveBeenCalledTimes(1);
      expect(screen.queryByRole("menu")).toBeNull();
      expect(document.activeElement).toBe(trigger());
    }
    fireEvent.click(trigger());
    fireEvent.click(within(screen.getByRole("menu")).getByRole("menuitem", { name: "Reply" }));
    expect(handlers.reply).toHaveBeenCalledTimes(1);
  });

  it("an action that moves the focus elsewhere (Reply goes to the composer) keeps it there", () => {
    const reply = vi.fn(() => (screen.getByLabelText("Elsewhere") as HTMLElement).focus());
    renderWithI18n(<Harness handlers={{ reply }} />);
    fireEvent.click(trigger());
    fireEvent.click(within(screen.getByRole("menu")).getByRole("menuitem", { name: "Reply" }));
    expect(document.activeElement).toBe(screen.getByLabelText("Elsewhere"));
  });

  it("copies the text", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    renderWithI18n(<Harness />);
    fireEvent.click(trigger());
    fireEvent.click(within(screen.getByRole("menu")).getByRole("menuitem", { name: "Copy text" }));
    expect(writeText).toHaveBeenCalledWith("hello");
  });

  it("has no serious accessibility violations, closed or open", async () => {
    renderWithI18n(<Harness />);
    expect(await axeViolations()).toEqual([]);
    fireEvent.click(trigger());
    expect(await axeViolations()).toEqual([]);
  });
});
