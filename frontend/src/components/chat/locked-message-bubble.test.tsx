// @vitest-environment jsdom
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, within } from "@testing-library/react";

import type { ChatHold } from "@/lib/api/holds";
import { axeViolations, renderWithI18n } from "@/test/render";
import { ChatHoldProvider, type ChatHoldValue } from "@/components/chat/chat-hold-context";
import { MessageActionsProvider, type MessageActions } from "@/components/chat/message-actions-context";
import { MessageBubble } from "@/components/chat/message-bubble";
import { LockedMessageBody } from "@/components/chat/locked-message-bubble";
import type { UiMessage } from "@/components/chat/chat-model";

afterEach(cleanup);

const ME = "me";
const SECRET = "The tower card means a great upheaval is coming";
const locked = (over: Partial<UiMessage> = {}): UiMessage => ({
  id: "m5",
  seq: 5,
  conversation_id: "c",
  session_id: "s",
  sender_id: "theo",
  kind: "text",
  body: "",
  locked: true,
  hold_id: "h1",
  body_length: 47,
  created_at: "2026-11-19T10:00:00Z",
  ...over,
});
const hold = (over: Partial<ChatHold> = {}): ChatHold => ({
  id: "h1",
  session_id: "s",
  conversation_id: "c",
  reason: "trial_ended",
  status: "open",
  started_at: "2026-11-19T10:00:00Z",
  expires_at: "2026-11-19T10:01:00Z",
  price_credits: 40,
  held_messages: 2,
  ...over,
});
const chatHold = (over: Partial<ChatHoldValue> = {}): ChatHoldValue => ({
  role: "client",
  hold: hold(),
  counterpartName: "Theo",
  clockOffsetMs: 0,
  hidden: null,
  openLocked: vi.fn(),
  openTopUp: vi.fn(),
  busy: false,
  continueOffered: false,
  continuing: false,
  continueReading: vi.fn(),
  ...over,
});
const actions = (over: Partial<MessageActions> = {}): MessageActions => ({
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
});

function bubble(message: UiMessage, h: ChatHoldValue = chatHold(), a: MessageActions = actions()) {
  const ui = renderWithI18n(
    <ChatHoldProvider value={h}>
      <MessageActionsProvider value={a}>
        <ol>
          <MessageBubble message={message} mine={message.sender_id === ME} authorName="Theo" onBookmark={vi.fn()} />
        </ol>
      </MessageActionsProvider>
    </ChatHoldProvider>,
  );
  return { ...ui, h, a };
}

describe("locked bubble (§26.1)", () => {
  it("never shows body, even when a buggy server sent it", () => {
    const { container } = bubble(locked({ body: SECRET }));
    expect(container.textContent).not.toContain(SECRET);
    expect(container.innerHTML).not.toContain(SECRET);
    expect(container.innerHTML).not.toContain("tower");
  });

  it("makes the placeholder from body_length alone", () => {
    bubble(locked({ body: SECRET, body_length: 12 }));
    expect(screen.getByTestId("locked-message-text").textContent).toHaveLength(12);
    cleanup();
    bubble(locked({ body_length: 300 }));
    expect(screen.getByTestId("locked-message-text").textContent).toHaveLength(300);
  });

  it("is one button named for assistive technology, with the placeholder hidden from it", () => {
    bubble(locked());
    const button = screen.getByRole("button", { name: "Hidden message from Theo. Top up to read it." });
    expect(button.getAttribute("data-testid")).toBe("locked-message");
    expect(button.querySelector('[data-testid="locked-message-text"]')?.getAttribute("aria-hidden")).toBe("true");
    expect(within(button).getByText("Hidden · Top up to read")).toBeTruthy();
  });

  it("opens the hold's top-up or reveal with the message's hold id on click and on keyboard", () => {
    const { h } = bubble(locked({ hold_id: "h-42" }));
    const button = screen.getByTestId("locked-message");
    fireEvent.click(button);
    expect(h.openLocked).toHaveBeenCalledWith("h-42");
    button.focus();
    expect(document.activeElement).toBe(button);
  });

  it("an image message shows a blurred block instead of the picture", () => {
    const { container } = bubble(locked({ kind: "image", body_length: undefined, attachments: [{ id: "a1", content_type: "image/png", width: 10, height: 10, bytes: 1 }] }));
    expect(container.querySelector(".locked-message-image")).toBeTruthy();
    expect(container.querySelector("img")).toBeNull();
    expect(screen.queryByTestId("locked-message-text")).toBeNull();
    expect(screen.getByRole("button", { name: /Hidden message from Theo/ })).toBeTruthy();
  });

  it("reply is allowed from the menu; like, bookmark and copy are not", () => {
    bubble(locked());
    fireEvent.click(screen.getByTestId("message-menu-trigger"));
    const items = within(screen.getByTestId("message-menu")).getAllByRole("menuitem");
    expect(items.map((i) => i.textContent)).toEqual(["Reply"]);
  });

  it("reply works and the double-tap like does nothing", () => {
    const { a } = bubble(locked());
    fireEvent.click(screen.getByTestId("message-menu-trigger"));
    fireEvent.click(screen.getByTestId("message-menu-reply"));
    expect(a.reply).toHaveBeenCalledTimes(1);
    expect(a.toggleLike).not.toHaveBeenCalled();
  });

  it("a locked quote reads 'Hidden message' with a lock, never its excerpt", () => {
    const readable: UiMessage = { ...locked(), id: "m6", locked: undefined, hold_id: undefined, body: "Thanks!", body_length: undefined, sender_id: ME };
    bubble({ ...readable, reply_to: { id: "m5", seq: 5, sender_id: "theo", kind: "text", excerpt: SECRET, locked: true } });
    const quote = screen.getByTestId("message-quote");
    expect(within(quote).getByTestId("hidden-message-label").textContent).toContain("Hidden message");
    expect(quote.textContent).not.toContain(SECRET);
  });

  it("has no serious accessibility violations", async () => {
    const { container } = bubble(locked());
    expect(await axeViolations(container)).toEqual([]);
  });
});

describe("a revealed message", () => {
  it("shows its text with the un-blur class and the normal menu", () => {
    bubble({ ...locked(), body: SECRET, locked: false });
    expect(screen.getByText(SECRET).className).toContain("message-unblur");
    expect(screen.queryByTestId("locked-message")).toBeNull();
    fireEvent.click(screen.getByTestId("message-menu-trigger"));
    expect(within(screen.getByTestId("message-menu")).getAllByRole("menuitem").length).toBeGreaterThan(1);
  });
});

describe("advisor markers (§26.1)", () => {
  const mine = (over: Partial<UiMessage> = {}): UiMessage => ({ ...locked(), id: "m7", sender_id: ME, body: "Your reading is promising", locked: undefined, body_length: undefined, held: true, ...over });

  it("says 'Hidden from client' while the hold is open", () => {
    bubble(mine(), chatHold({ role: "psychic" }));
    expect(screen.getByTestId("message-held").textContent).toContain("Hidden from client");
    expect(screen.getByText("Your reading is promising")).toBeTruthy();
  });

  it("says 'Not revealed' after expiry", () => {
    bubble(mine(), chatHold({ role: "psychic", hold: hold({ status: "expired" }) }));
    expect(screen.getByTestId("message-held").textContent).toContain("Not revealed");
  });

  it("disappears on reveal and on conversion", () => {
    bubble(mine({ held: false }), chatHold({ role: "psychic", hold: hold({ status: "revealed" }) }));
    expect(screen.queryByTestId("message-held")).toBeNull();
    cleanup();
    bubble(mine(), chatHold({ role: "psychic", hold: hold({ status: "converted" }) }));
    expect(screen.queryByTestId("message-held")).toBeNull();
  });

  it("a message that was never held has no marker", () => {
    bubble(mine({ held: undefined }), chatHold({ role: "psychic" }));
    expect(screen.queryByTestId("message-held")).toBeNull();
  });
});

describe("LockedMessageBody alone", () => {
  it("renders inside no provider without throwing (default context)", () => {
    renderWithI18n(<LockedMessageBody message={{ id: "x", kind: "text", body_length: 8, hold_id: "h" }} authorName="Theo" />);
    expect(screen.getByTestId("locked-message")).toBeTruthy();
  });
});
