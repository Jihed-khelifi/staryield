// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";

import type { ReplyPreview } from "@/lib/api/types";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";
import { MessageActionsProvider, type MessageActions } from "@/components/chat/message-actions-context";
import { MessageComposer } from "@/components/chat/message-composer";

beforeEach(() => {
  stubResizeObserver();
  vi.stubGlobal(
    "matchMedia",
    vi.fn((query: string) => ({ matches: false, media: query, addEventListener: vi.fn(), removeEventListener: vi.fn() })),
  );
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const ME = "me";
const TARGET: ReplyPreview = { id: "55555555-5555-4555-8555-555555555555", seq: 4, sender_id: "theo", kind: "text", excerpt: "Mercury is retrograde" };

/** The reply target lives in a tiny stand-in for the chatroom, like in the app (per conversation, cleared by cancel). */
function Harness({ initial, onSend, draftId = "conv-1" }: { initial: ReplyPreview | null; onSend: (m: unknown) => void; draftId?: string }) {
  const [target, setTarget] = React.useState<ReplyPreview | null>(initial);
  const actions: MessageActions = {
    enabled: true,
    viewerId: ME,
    replyTarget: target,
    reply: () => undefined,
    cancelReply: () => setTarget(null),
    toggleLike: () => undefined,
    jumpTo: () => undefined,
    jumpingTo: null,
    nameOf: (id) => (id === ME ? "You" : "Theo"),
  };
  return (
    <>
      <button type="button" onClick={() => setTarget(TARGET)}>
        start reply
      </button>
      <MessageActionsProvider value={actions}>
        <MessageComposer onSend={onSend} note="note" userId={ME} draftId={draftId} />
      </MessageActionsProvider>
    </>
  );
}

const box = () => screen.getByRole("textbox", { name: "Message" }) as HTMLTextAreaElement;

describe("reply bar (§23.2)", () => {
  it("shows who and what is being answered, with a cancel button", () => {
    renderWithI18n(<Harness initial={TARGET} onSend={vi.fn()} />);
    const bar = screen.getByTestId("composer-reply");
    expect(bar.textContent).toContain("Replying to Theo");
    expect(bar.textContent).toContain("Mercury is retrograde");
    expect(screen.getByRole("button", { name: "Cancel reply" })).toBeTruthy();
  });

  it("says 'Replying to yourself' for your own message and 'Photo' for a bare image", () => {
    renderWithI18n(<Harness initial={{ ...TARGET, sender_id: ME, kind: "image", excerpt: "" }} onSend={vi.fn()} />);
    const bar = screen.getByTestId("composer-reply");
    expect(bar.textContent).toContain("Replying to yourself");
    expect(bar.textContent).toContain("Photo");
  });

  it("is absent without a reply", () => {
    renderWithI18n(<Harness initial={null} onSend={vi.fn()} />);
    expect(screen.queryByTestId("composer-reply")).toBeNull();
  });

  it("the cancel button clears it and returns to the field", () => {
    renderWithI18n(<Harness initial={TARGET} onSend={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Cancel reply" }));
    expect(screen.queryByTestId("composer-reply")).toBeNull();
    expect(document.activeElement).toBe(box());
  });

  it("Esc in the field cancels the reply, keeping what was typed", () => {
    renderWithI18n(<Harness initial={TARGET} onSend={vi.fn()} />);
    fireEvent.change(box(), { target: { value: "draft text" } });
    fireEvent.keyDown(box(), { key: "Escape" });
    expect(screen.queryByTestId("composer-reply")).toBeNull();
    expect(box().value).toBe("draft text");
  });

  it("Esc on the bar cancels it too", () => {
    renderWithI18n(<Harness initial={TARGET} onSend={vi.fn()} />);
    fireEvent.keyDown(screen.getByRole("button", { name: "Cancel reply" }), { key: "Escape" });
    expect(screen.queryByTestId("composer-reply")).toBeNull();
  });

  it("sending hands the target to the chatroom and clears it", () => {
    const onSend = vi.fn();
    renderWithI18n(<Harness initial={TARGET} onSend={onSend} />);
    fireEvent.change(box(), { target: { value: "It means growth" } });
    fireEvent.click(screen.getByRole("button", { name: "Send message" }));
    expect(onSend).toHaveBeenCalledWith({ body: "It means growth", replyTo: TARGET });
    expect(screen.queryByTestId("composer-reply")).toBeNull();
  });

  it("a message that is not a reply carries no replyTo", () => {
    const onSend = vi.fn();
    renderWithI18n(<Harness initial={null} onSend={onSend} />);
    fireEvent.change(box(), { target: { value: "plain" } });
    fireEvent.click(screen.getByRole("button", { name: "Send message" }));
    expect(onSend).toHaveBeenCalledWith({ body: "plain" });
    expect("replyTo" in onSend.mock.calls[0][0]).toBe(false);
  });

  it("starting a reply moves focus to the field", () => {
    renderWithI18n(<Harness initial={null} onSend={vi.fn()} />);
    const start = screen.getByRole("button", { name: "start reply" });
    start.focus();
    fireEvent.click(start);
    expect(document.activeElement).toBe(box());
  });

  it("has no serious accessibility violations", async () => {
    renderWithI18n(<Harness initial={TARGET} onSend={vi.fn()} />);
    expect(await axeViolations()).toEqual([]);
  });
});
