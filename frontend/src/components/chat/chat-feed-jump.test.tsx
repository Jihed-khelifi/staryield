// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, screen } from "@testing-library/react";

import { renderWithI18n, stubResizeObserver } from "@/test/render";
import { ChatFeed, FLASH_MS, type ChatFeedHandle } from "@/components/chat/chat-feed";
import type { UiMessage } from "@/components/chat/chat-model";

const scrolls: ScrollToOptions[] = [];
beforeAll(() => {
  stubResizeObserver();
  HTMLElement.prototype.scrollTo = function scrollTo(this: HTMLElement, opts?: ScrollToOptions | number) {
    if (typeof opts === "object") scrolls.push(opts);
  } as typeof HTMLElement.prototype.scrollTo;
});
beforeEach(() => {
  vi.useFakeTimers();
  scrolls.length = 0;
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

const m = (seq: number): UiMessage => ({ id: `m${seq}`, seq, conversation_id: "c", session_id: "s", sender_id: "them", kind: "text", body: `Message ${seq}`, created_at: new Date(Date.UTC(2026, 10, 5, 10, 0, seq)).toISOString() });

function mount(messages: UiMessage[] = [m(1), m(2), m(3)]) {
  const ref = React.createRef<ChatFeedHandle>();
  renderWithI18n(
    <ChatFeed ref={ref} messages={messages} meId="me" authorName="Theo" typing={false} savedIds={new Set()} hasOlder={false} loadingOlder={false} onLoadOlder={vi.fn()} onBookmark={vi.fn()} onRetry={vi.fn()} />,
  );
  return ref;
}
const row = (id: string) => screen.getAllByTestId("chat-message").find((el) => el.getAttribute("data-message-id") === id)!;

describe("ChatFeed.scrollToMessage (§23.2)", () => {
  it("scrolls to a loaded message and highlights it for a moment", () => {
    const ref = mount();
    let found = false;
    act(() => {
      found = ref.current!.scrollToMessage("m2");
    });
    expect(found).toBe(true);
    expect(scrolls).toHaveLength(1);
    expect(row("m2").getAttribute("data-flash")).toBe("true");
    act(() => void vi.advanceTimersByTime(FLASH_MS + 10));
    expect(row("m2").hasAttribute("data-flash")).toBe(false);
  });

  it("returns false when the message is not loaded (the caller loads older pages)", () => {
    const ref = mount();
    let found = true;
    act(() => {
      found = ref.current!.scrollToMessage("m99");
    });
    expect(found).toBe(false);
    expect(scrolls).toHaveLength(0);
  });

  it("moves the highlight when another message is jumped to", () => {
    const ref = mount();
    act(() => void ref.current!.scrollToMessage("m1"));
    act(() => void ref.current!.scrollToMessage("m3"));
    expect(row("m1").hasAttribute("data-flash")).toBe(false);
    expect(row("m3").getAttribute("data-flash")).toBe("true");
  });
});
