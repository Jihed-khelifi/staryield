// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, within } from "@testing-library/react";

import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";
import { ChatFeed, type Announcement } from "@/components/chat/chat-feed";
import type { UiMessage } from "@/components/chat/chat-model";
import { TooltipProvider } from "@/components/ui/tooltip";

beforeAll(() => {
  stubResizeObserver();
  HTMLElement.prototype.scrollTo = function scrollTo(this: HTMLElement, opts?: ScrollToOptions | number) {
    if (typeof opts === "object" && typeof opts.top === "number") this.scrollTop = opts.top;
  } as typeof HTMLElement.prototype.scrollTo;
});
afterEach(cleanup);

const ME = "me";
const THEM = "them";
const m = (seq: number, sender = THEM, extra: Partial<UiMessage> = {}): UiMessage => ({
  id: `m${seq}`,
  seq,
  conversation_id: "c",
  session_id: "s",
  sender_id: sender,
  kind: "text",
  body: `Message ${seq}`,
  created_at: new Date(Date.UTC(2026, 10, 5, 10, 0, seq)).toISOString(),
  ...extra,
});

type Props = Partial<React.ComponentProps<typeof ChatFeed>>;
function feed(props: Props = {}) {
  const base: React.ComponentProps<typeof ChatFeed> = {
    messages: [m(1), m(2, ME)],
    meId: ME,
    authorName: "Theo",
    typing: false,
    savedIds: new Set(),
    hasOlder: false,
    loadingOlder: false,
    onLoadOlder: vi.fn(),
    onBookmark: vi.fn(),
    onRetry: vi.fn(),
    onDiscard: vi.fn(),
  };
  const ui = (p: Props) => (
    <TooltipProvider>
      <ChatFeed {...base} {...p} />
    </TooltipProvider>
  );
  const view = renderWithI18n(ui(props));
  return { ...view, rerender: (next: Props) => view.rerender(ui({ ...props, ...next })) };
}

/** jsdom has no layout: give the scroller a geometry we control. */
function geometry(el: HTMLElement, g: { height: number; client: number; top: number }) {
  Object.defineProperty(el, "scrollHeight", { configurable: true, get: () => g.height });
  Object.defineProperty(el, "clientHeight", { configurable: true, get: () => g.client });
  Object.defineProperty(el, "scrollTop", {
    configurable: true,
    get: () => g.top,
    set: (v: number) => {
      g.top = Math.max(0, Math.min(v, g.height - g.client));
    },
  });
  return g;
}

describe("ChatFeed", () => {
  it("announces incoming messages through a log region, never the message list itself", async () => {
    const log: Announcement[] = [{ id: "m1", text: "Theo: Message 1" }];
    feed({ announcements: log });
    const list = screen.getByRole("list", { name: /Theo/ });
    expect(list.getAttribute("aria-live")).toBeNull();
    const region = screen.getByRole("log", { name: "New messages from Theo" });
    expect(region.getAttribute("aria-live")).toBe("polite");
    expect(region.textContent).toBe("Theo: Message 1");
    // The viewer's own message is on screen but not in the log.
    expect(within(list).getByText("Message 2")).toBeTruthy();
    expect(region.textContent).not.toContain("Message 2");
    expect(await axeViolations()).toEqual([]);
  });

  it("offers 'Jump to latest' when scrolled up and new messages arrive", () => {
    const { rerender } = feed({ messages: [m(1), m(2, ME), m(3)] });
    const scroller = screen.getByTestId("chat-feed");
    const g = geometry(scroller, { height: 1000, client: 400, top: 600 });
    fireEvent.scroll(scroller); // at the bottom
    expect(screen.queryByTestId("jump-to-latest")).toBeNull();
    g.top = 100;
    fireEvent.scroll(scroller); // the reader scrolls up
    rerender({ messages: [m(1), m(2, ME), m(3), m(4)] });
    // Not pulled down while reading history.
    expect(g.top).toBe(100);
    const pill = screen.getByTestId("jump-to-latest");
    expect(pill.textContent).toContain("Jump to latest");
    expect(pill.textContent).toContain("1 new");
    // The viewer's own messages and system notices are not "new".
    rerender({ messages: [m(1), m(2, ME), m(3), m(4), m(5, ME), m(6, THEM, { kind: "system", body: "session.started" })] });
    expect(screen.getByTestId("jump-to-latest").textContent).toContain("1 new");
    fireEvent.click(screen.getByTestId("jump-to-latest"));
    expect(g.top).toBe(600);
    expect(screen.queryByTestId("jump-to-latest")).toBeNull();
    expect(document.activeElement).toBe(scroller);
  });

  it("follows new messages while at the bottom", () => {
    const { rerender } = feed({ messages: [m(1)] });
    const scroller = screen.getByTestId("chat-feed");
    const g = geometry(scroller, { height: 800, client: 400, top: 400 });
    fireEvent.scroll(scroller);
    g.height = 900;
    rerender({ messages: [m(1), m(2)] });
    expect(g.top).toBe(500);
    expect(screen.queryByTestId("jump-to-latest")).toBeNull();
  });

  it("keeps the reader's place when older history is prepended", () => {
    const onLoadOlder = vi.fn();
    const { rerender } = feed({ messages: [m(51), m(52)], hasOlder: true, onLoadOlder });
    const scroller = screen.getByTestId("chat-feed");
    const g = geometry(scroller, { height: 1000, client: 400, top: 300 });
    fireEvent.scroll(scroller); // mid-history
    rerender({ messages: [m(51), m(52)], hasOlder: true, onLoadOlder });
    g.height = 1600; // the older page adds 600px above
    rerender({ messages: [m(49), m(50), m(51), m(52)], hasOlder: true, onLoadOlder });
    expect(g.top).toBe(900);
    // Near the top, the next older page loads by itself.
    g.top = 50;
    fireEvent.scroll(scroller);
    expect(onLoadOlder).toHaveBeenCalled();
    fireEvent.click(screen.getByTestId("load-older"));
    expect(onLoadOlder).toHaveBeenCalledTimes(2);
  });

  it("shows sending, waiting, retrying and failed states with retry and remove", async () => {
    const onRetry = vi.fn();
    const onDiscard = vi.fn();
    const local = (id: string, pending: UiMessage["pending"], failure?: string): UiMessage => ({
      ...m(0, ME),
      id,
      seq: undefined,
      client_msg_id: id,
      body: id,
      pending,
      failure,
    });
    const { rerender } = feed({ messages: [m(1), local("queued-1", "queued")], connected: false, onRetry, onDiscard });
    expect(screen.getByTestId("message-status").textContent).toBe("Waiting for connection…");
    rerender({ messages: [m(1), local("queued-1", "queued", "network_error")], connected: true, onRetry, onDiscard });
    expect(screen.getByTestId("message-status").textContent).toBe("Retrying…");
    rerender({ messages: [m(1), local("queued-1", "sending")], connected: true, onRetry, onDiscard });
    expect(screen.getByTestId("message-status").textContent).toBe("Sending…");
    rerender({ messages: [m(1), local("queued-1", "failed", "session_ended")], connected: true, onRetry, onDiscard });
    const failed = screen.getByTestId("message-failed");
    expect(failed.textContent).toContain("Not delivered: this chat has ended.");
    fireEvent.click(within(failed).getByRole("button", { name: "Retry" }));
    fireEvent.click(within(failed).getByRole("button", { name: "Remove" }));
    expect(onRetry).toHaveBeenCalledOnce();
    expect(onDiscard).toHaveBeenCalledOnce();
    expect(await axeViolations()).toEqual([]);
  });

  it("marks the newest of the viewer's messages the other side has read as Seen", () => {
    const { rerender } = feed({ messages: [m(1, ME), m(2, ME), m(3)], peerReadSeq: 1 });
    const seen = () => screen.getAllByTestId("message-seen").map((el) => el.closest("[data-testid=chat-message]")?.getAttribute("data-seq"));
    expect(seen()).toEqual(["1"]);
    rerender({ messages: [m(1, ME), m(2, ME), m(3)], peerReadSeq: 3 });
    expect(seen()).toEqual(["2"]);
  });

  it("shows a skeleton while the first page loads", () => {
    feed({ messages: [], status: "loading" });
    expect(screen.getByRole("status", { name: "Loading messages…" })).toBeTruthy();
  });

  it("scrolls to the newest message when the viewer sends", () => {
    const { rerender } = feed({ messages: [m(1), m(2)] });
    const scroller = screen.getByTestId("chat-feed");
    const g = geometry(scroller, { height: 1200, client: 400, top: 0 });
    fireEvent.scroll(scroller);
    act(() => rerender({ messages: [m(1), m(2)], scrollToken: 1 }));
    expect(g.top).toBe(800);
  });

  it("does not re-pin on re-renders that do not change the messages (§22.5)", () => {
    const messages = [m(1), m(2)];
    const { rerender } = feed({ messages });
    const scroller = screen.getByTestId("chat-feed");
    const g = geometry(scroller, { height: 800, client: 400, top: 400 });
    fireEvent.scroll(scroller); // at the bottom
    // Content grew without a message change (e.g. the 1 s session timer re-renders the shell).
    g.height = 900;
    rerender({ messages, announcements: [{ id: "x", text: "tick" }] });
    rerender({ messages, savedIds: new Set(["m1"]) });
    expect(g.top).toBe(400);
    // The other person starting or stopping to type never moves the feed (fixed-height slot).
    rerender({ messages, typing: true });
    rerender({ messages, typing: false });
    expect(g.top).toBe(400);
    // A new message does pin.
    rerender({ messages: [m(1), m(2), m(3)] });
    expect(g.top).toBe(500);
  });

  it("follows content that grows later (an image loading) while the reader is at the bottom", () => {
    const observers: { cb: ResizeObserverCallback; targets: Element[] }[] = [];
    class RO {
      targets: Element[] = [];
      constructor(public cb: ResizeObserverCallback) {
        observers.push(this);
      }
      observe(el: Element) {
        this.targets.push(el);
      }
      unobserve() {}
      disconnect() {}
    }
    vi.stubGlobal("ResizeObserver", RO);
    const messages = [m(1)];
    feed({ messages });
    const scroller = screen.getByTestId("chat-feed");
    const g = geometry(scroller, { height: 800, client: 400, top: 400 });
    fireEvent.scroll(scroller);
    // The observer watches both the viewport and the content list.
    const ro = observers.at(-1)!;
    expect(ro.targets).toContain(scroller);
    const content = ro.targets.find((t) => t !== scroller) as HTMLElement;
    expect(content.contains(screen.getByRole("list", { name: /Theo/ }))).toBe(true);
    Object.defineProperty(content, "offsetHeight", { configurable: true, value: 1200 });
    g.height = 1200;
    act(() => ro.cb([], ro as unknown as ResizeObserver));
    expect(g.top).toBe(800);
    // A reader who scrolled up is left where they are.
    g.top = 100;
    fireEvent.scroll(scroller);
    Object.defineProperty(content, "offsetHeight", { configurable: true, value: 1300 });
    g.height = 1300;
    act(() => ro.cb([], ro as unknown as ResizeObserver));
    expect(g.top).toBe(100);
    vi.unstubAllGlobals();
  });

  it("keeps the typing indicator in an always-rendered fixed slot (hidden from assistive tech when idle)", () => {
    const { rerender } = feed({ typing: false });
    const slot = screen.getByTestId("typing-slot");
    expect(slot.className).toBe("chat-typing-slot");
    expect(slot.getAttribute("aria-hidden")).toBe("true");
    expect(within(slot).queryByRole("status")).toBeNull();
    rerender({ typing: true });
    // The same element stays in the list; only its content changes.
    expect(screen.getByTestId("typing-slot")).toBe(slot);
    expect(slot.getAttribute("aria-hidden")).toBeNull();
    expect(within(slot).getByRole("status", { name: "Theo is typing" })).toBeTruthy();
    rerender({ typing: false });
    expect(slot.getAttribute("aria-hidden")).toBe("true");
  });

  it("labels messages written outside a live chat, and explains why one was not delivered", () => {
    const off = { ...m(3, THEM), off_session: true, session_id: undefined };
    const failed: UiMessage = { ...m(0, ME), id: "f1", seq: undefined, client_msg_id: "f1", pending: "failed", failure: "off_session_limit_reached", body: "hello?" };
    feed({ messages: [m(1), off, failed] });
    expect(screen.getAllByTestId("message-off-session")).toHaveLength(1);
    expect(screen.getByTestId("message-off-session").textContent).toBe("Outside a live chat");
    expect(screen.getByTestId("message-failed").textContent).toContain("Not delivered: you've used today's messages outside a live chat.");
  });

  it("explains image failures: too large, the server's own message, and a network problem", () => {
    const image = (failure: string, failureDetail?: string): UiMessage => ({
      ...m(0, ME),
      id: `i-${failure}`,
      seq: undefined,
      client_msg_id: `i-${failure}`,
      kind: "image",
      body: "",
      attachments: [{ id: "local-1", content_type: "image/jpeg", width: 10, height: 10, bytes: 10 }],
      pending: "failed",
      failure,
      failureDetail,
    });
    const { rerender } = feed({ messages: [image("payload_too_large")] });
    expect(screen.getByTestId("message-failed").textContent).toContain("Not delivered: this image is too large.");
    rerender({ messages: [image("validation_failed", "The image is too small.")] });
    expect(screen.getByTestId("message-failed").textContent).toContain("Not delivered: The image is too small.");
    rerender({ messages: [image("network_error")] });
    expect(screen.getByTestId("message-failed").textContent).toContain("Not delivered: check your connection and try again.");
  });
});
