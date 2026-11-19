// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";

import type { ChatHold } from "@/lib/api/holds";
import { renderWithI18n } from "@/test/render";
import { ChatHoldProvider, type ChatHoldValue } from "@/components/chat/chat-hold-context";
import { HiddenMessagesPrompt } from "@/components/chat/hidden-messages-prompt";

afterEach(cleanup);

const hold = (over: Partial<ChatHold> = {}): ChatHold => ({
  id: "h1",
  session_id: "s",
  conversation_id: "c",
  reason: "trial_ended",
  status: "expired",
  started_at: "2026-11-19T10:00:00Z",
  expires_at: "2026-11-19T10:01:00Z",
  price_credits: 40,
  held_messages: 4,
  ...over,
});
const value = (over: Partial<ChatHoldValue> = {}): ChatHoldValue => ({
  role: "client",
  hold: hold(),
  counterpartName: "Theo",
  clockOffsetMs: 0,
  hidden: { holdId: "h1", count: 2 },
  openLocked: vi.fn(),
  openTopUp: vi.fn(),
  busy: false,
  continueOffered: false,
  continuing: false,
  continueReading: vi.fn(),
  ...over,
});
const prompt = (v: ChatHoldValue) =>
  renderWithI18n(
    <ChatHoldProvider value={v}>
      <HiddenMessagesPrompt />
    </ChatHoldProvider>,
  );

describe("HiddenMessagesPrompt (expired hold)", () => {
  it("says 'Top up to read {n} hidden messages' with the hold's count and asks for the reveal", () => {
    const v = value();
    prompt(v);
    const button = screen.getByTestId("hidden-messages-reveal");
    expect(button.textContent).toBe("Top up to read 4 hidden messages");
    fireEvent.click(button);
    expect(v.openLocked).toHaveBeenCalledWith("h1");
  });

  it("uses the singular and falls back to the loaded count when the hold is not known", () => {
    prompt(value({ hold: hold({ held_messages: 1 }), hidden: { holdId: "h1", count: 1 } }));
    expect(screen.getByTestId("hidden-messages-reveal").textContent).toBe("Top up to read 1 hidden message");
    cleanup();
    prompt(value({ hold: null, hidden: { holdId: "h9", count: 2 } }));
    expect(screen.getByTestId("hidden-messages-reveal").textContent).toBe("Top up to read 2 hidden messages");
  });

  it("is absent when nothing is hidden, while the hold is open (the banner owns that) and for advisors", () => {
    prompt(value({ hidden: null }));
    expect(screen.queryByTestId("hidden-messages-prompt")).toBeNull();
    cleanup();
    prompt(value({ hold: hold({ status: "open" }) }));
    expect(screen.queryByTestId("hidden-messages-prompt")).toBeNull();
    cleanup();
    prompt(value({ role: "psychic" }));
    expect(screen.queryByTestId("hidden-messages-prompt")).toBeNull();
  });

  it("disables the button while a request runs", () => {
    prompt(value({ busy: true }));
    expect((screen.getByTestId("hidden-messages-reveal") as HTMLButtonElement).disabled).toBe(true);
  });
});
