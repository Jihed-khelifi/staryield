// @vitest-environment jsdom
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, within } from "@testing-library/react";

import { axeViolations, renderWithI18n } from "@/test/render";
import { MessageActionsProvider, type MessageActions } from "@/components/chat/message-actions-context";
import { MessageBubble } from "@/components/chat/message-bubble";
import type { UiMessage } from "@/components/chat/chat-model";

afterEach(cleanup);

const ME = "me";
const THEO = "theo";
const msg = (extra: Partial<UiMessage> = {}): UiMessage => ({
  id: "m5",
  seq: 5,
  conversation_id: "c",
  session_id: "s",
  sender_id: THEO,
  kind: "text",
  body: "Mercury is retrograde",
  created_at: "2026-11-05T10:00:00Z",
  ...extra,
});

function actions(over: Partial<MessageActions> = {}): MessageActions {
  return {
    enabled: true,
    viewerId: ME,
    replyTarget: null,
    reply: vi.fn(),
    cancelReply: vi.fn(),
    toggleLike: vi.fn(),
    jumpTo: vi.fn(),
    jumpingTo: null,
    nameOf: (id) => (id === ME ? "You" : "Theo"),
    ...over,
  };
}

function bubble(message: UiMessage, props: Partial<React.ComponentProps<typeof MessageBubble>> = {}, a: MessageActions = actions()) {
  const ui = renderWithI18n(
    <MessageActionsProvider value={a}>
      <ol>
        <MessageBubble message={message} mine={message.sender_id === ME} authorName="Theo" onBookmark={vi.fn()} {...props} />
      </ol>
    </MessageActionsProvider>,
  );
  return { ...ui, a };
}

const QUOTE = { id: "m2", seq: 2, sender_id: ME, kind: "text" as const, excerpt: "What does it mean for me?" };

describe("quote of a reply (§23.2)", () => {
  it("shows the author and the excerpt above the text", () => {
    bubble(msg({ reply_to: { ...QUOTE, sender_id: THEO } }));
    const quote = screen.getByTestId("message-quote");
    expect(within(quote).getByText("Theo")).toBeTruthy();
    expect(within(quote).getByText("What does it mean for me?")).toBeTruthy();
    // Above the message text.
    const text = screen.getByText("Mercury is retrograde");
    expect(quote.compareDocumentPosition(text) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("names the viewer 'You' and says 'Photo' for an image without a caption", () => {
    bubble(msg({ reply_to: { id: "m1", seq: 1, sender_id: ME, kind: "image", excerpt: "" } }));
    const quote = screen.getByTestId("message-quote");
    expect(within(quote).getByText("You")).toBeTruthy();
    expect(within(quote).getByText("Photo")).toBeTruthy();
  });

  it("is a focusable button that jumps to the original", () => {
    const { a } = bubble(msg({ reply_to: QUOTE }));
    const quote = screen.getByRole("button", { name: /^You\s*What does it mean for me\?\s*Go to the original message$/ });
    quote.focus();
    expect(document.activeElement).toBe(quote);
    fireEvent.click(quote);
    expect(a.jumpTo).toHaveBeenCalledWith(QUOTE);
  });

  it("shows a loading state on the quote being looked for", () => {
    bubble(msg({ reply_to: QUOTE }), {}, actions({ jumpingTo: QUOTE.id }));
    expect(screen.getByTestId("message-quote").getAttribute("aria-busy")).toBe("true");
  });

  it("an optimistic reply that is still sending shows its quote", () => {
    bubble(msg({ id: "cm-1", seq: undefined, sender_id: ME, pending: "sending", reply_to: QUOTE }), { delivery: "sending" });
    expect(screen.getByTestId("message-quote")).toBeTruthy();
  });

  it("explains a reply whose target is gone", () => {
    bubble(msg({ sender_id: ME, pending: "failed", failure: "reply_target_invalid", reply_to: QUOTE }), { delivery: "failed", onRetry: vi.fn() });
    expect(screen.getByTestId("message-failed").textContent).toContain("the message you replied to is no longer available");
  });
});

describe("heart badge (§23.3)", () => {
  it("says who liked it", () => {
    const { unmount } = bubble(msg({ liked_by: [ME] }));
    expect(screen.getByRole("img", { name: "You liked this" })).toBeTruthy();
    unmount();
    const second = bubble(msg({ liked_by: [THEO] }));
    expect(screen.getByRole("img", { name: "Theo liked this" })).toBeTruthy();
    second.unmount();
    bubble(msg({ liked_by: [THEO, ME] }));
    expect(screen.getByRole("img", { name: "You and Theo liked this" })).toBeTruthy();
  });

  it("is absent when nobody liked it", () => {
    bubble(msg({ liked_by: [] }));
    expect(screen.queryByTestId("message-like-badge")).toBeNull();
    cleanup();
    bubble(msg());
    expect(screen.queryByTestId("message-like-badge")).toBeNull();
  });
});

describe("message actions", () => {
  it("reply and like buttons sit beside the bookmark; like is a toggle", () => {
    const { a } = bubble(msg({ liked_by: [ME] }));
    fireEvent.click(screen.getByRole("button", { name: "Reply" }));
    expect(a.reply).toHaveBeenCalledTimes(1);
    const like = screen.getByRole("button", { name: "Unlike" });
    expect(like.getAttribute("aria-pressed")).toBe("true");
    fireEvent.click(like);
    expect(a.toggleLike).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "Bookmark this message" })).toBeTruthy();
  });

  it("an unliked message offers 'Like' (aria-pressed false)", () => {
    bubble(msg());
    expect(screen.getByRole("button", { name: "Like" }).getAttribute("aria-pressed")).toBe("false");
  });

  it("images can be answered and liked too", () => {
    bubble(msg({ kind: "image", body: "", attachments: [{ id: "a1", content_type: "image/png", width: 10, height: 10, bytes: 5 }] }));
    expect(screen.getByRole("button", { name: "Reply" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Like" })).toBeTruthy();
  });

  it("double-click toggles the like", () => {
    const { a } = bubble(msg());
    fireEvent.doubleClick(screen.getByText("Mercury is retrograde"));
    expect(a.toggleLike).toHaveBeenCalledTimes(1);
  });

  it("the heart pops when a like is given, not when it is taken back", () => {
    const { unmount } = bubble(msg());
    fireEvent.doubleClick(screen.getByText("Mercury is retrograde"));
    expect(screen.getByTestId("message-like-pop")).toBeTruthy();
    unmount();
    bubble(msg({ liked_by: [ME] }));
    fireEvent.doubleClick(screen.getByText("Mercury is retrograde"));
    expect(screen.queryByTestId("message-like-pop")).toBeNull();
  });

  it("pending and failed messages have no actions and no gestures", () => {
    const { a, unmount } = bubble(msg({ id: "cm", seq: undefined, sender_id: ME, pending: "sending" }), { delivery: "sending" });
    expect(screen.queryByRole("button", { name: "Reply" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Like" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Bookmark this message" })).toBeNull();
    fireEvent.doubleClick(screen.getByText("Mercury is retrograde"));
    expect(a.toggleLike).not.toHaveBeenCalled();
    unmount();
    bubble(msg({ id: "cm2", seq: undefined, sender_id: ME, pending: "failed", failure: "timeout" }), { delivery: "failed", onRetry: vi.fn() });
    expect(screen.queryByRole("button", { name: "Reply" })).toBeNull();
    expect(screen.getByTestId("chat-message").querySelector("[data-actionable]")).toBeNull();
  });

  it("system notices have no actions", () => {
    bubble(msg({ kind: "system", body: "session.started", sender_id: undefined }));
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("outside a chatroom (no actions provider) the bubble shows no reply or like", () => {
    renderWithI18n(
      <ol>
        <MessageBubble message={msg()} mine={false} authorName="Theo" onBookmark={vi.fn()} />
      </ol>,
    );
    expect(screen.queryByRole("button", { name: "Reply" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Like" })).toBeNull();
  });

  it("marks the row with the message id so a quote can scroll to it", () => {
    bubble(msg());
    expect(screen.getByTestId("chat-message").getAttribute("data-message-id")).toBe("m5");
  });
});

describe("long-press menu (touch)", () => {
  const press = async (el: HTMLElement) => {
    fireEvent.touchStart(el, { touches: [{ clientX: 120, clientY: 200 }] });
    await act(async () => void (await new Promise((r) => setTimeout(r, 500))));
  };

  it("opens with Reply, Like, Save and Copy text; each does its thing and closes", async () => {
    const onBookmark = vi.fn();
    const { a } = bubble(msg(), { onBookmark });
    await press(screen.getByText("Mercury is retrograde"));
    const menu = screen.getByRole("menu", { name: "Message actions" });
    expect(within(menu).getAllByRole("menuitem").map((i) => i.textContent)).toEqual(["Reply", "Like", "Save", "Copy text"]);
    fireEvent.click(within(menu).getByRole("menuitem", { name: "Reply" }));
    expect(a.reply).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("menu")).toBeNull();

    await press(screen.getByText("Mercury is retrograde"));
    fireEvent.click(screen.getByRole("menuitem", { name: "Save" }));
    expect(onBookmark).toHaveBeenCalledTimes(1);
  });

  it("offers Unlike and Unsave for a liked, saved message and copies the text", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    bubble(msg({ liked_by: [ME] }), { saved: true });
    await press(screen.getByText("Mercury is retrograde"));
    expect(screen.getByRole("menuitem", { name: "Unlike" })).toBeTruthy();
    expect(screen.getByRole("menuitem", { name: "Unsave" })).toBeTruthy();
    fireEvent.click(screen.getByRole("menuitem", { name: "Copy text" }));
    expect(writeText).toHaveBeenCalledWith("Mercury is retrograde");
  });

  it("closes on Escape, with the first item focused when it opens", async () => {
    bubble(msg());
    await press(screen.getByText("Mercury is retrograde"));
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Reply" }));
    fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("an image without a caption has no 'Copy text'", async () => {
    bubble(msg({ kind: "image", body: "", attachments: [{ id: "a1", content_type: "image/png", width: 10, height: 10, bytes: 5 }] }));
    await press(screen.getByTestId("chat-message").querySelector(".message-bubble") as HTMLElement);
    expect(screen.queryByRole("menuitem", { name: "Copy text" })).toBeNull();
  });
});

describe("accessibility", () => {
  it("a bubble with a quote, a heart and actions has no serious violations", async () => {
    bubble(msg({ reply_to: QUOTE, liked_by: [ME, THEO] }));
    expect(await axeViolations()).toEqual([]);
  });

  it("the long-press menu has no serious violations", async () => {
    bubble(msg());
    fireEvent.touchStart(screen.getByText("Mercury is retrograde"), { touches: [{ clientX: 1, clientY: 1 }] });
    await act(async () => void (await new Promise((r) => setTimeout(r, 500))));
    expect(await axeViolations()).toEqual([]);
  });
});
