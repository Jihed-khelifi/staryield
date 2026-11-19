// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import type { ChatHold, MessagesRevealedEvent } from "@/lib/api/holds";
import type { ChatMessage } from "@/lib/api/types";

const handlers = vi.hoisted(() => new Map<string, (data: unknown) => void>());
const toast = vi.hoisted(() => ({ show: vi.fn(), showError: vi.fn() }));
vi.mock("@/lib/auth/session-provider", async () => {
  const React = await import("react");
  return {
    useRealtimeEvent: (type: string, handler: (data: unknown) => void) => {
      React.useEffect(() => {
        handlers.set(type, handler);
      });
    },
  };
});
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => toast }));

import { useHoldEvents } from "@/components/chat/use-hold-events";

const dictionary = { ...en, copy } as Dictionary;
const wrapper = ({ children }: { children: React.ReactNode }) => (
  <I18nProvider locale="en" dictionary={dictionary}>
    {children}
  </I18nProvider>
);
const hold = (over: Partial<ChatHold> = {}): ChatHold => ({
  id: "h1",
  session_id: "s1",
  conversation_id: "c1",
  reason: "trial_ended",
  status: "open",
  started_at: "2026-11-19T10:00:00Z",
  expires_at: new Date(Date.now() + 60_000).toISOString(),
  price_credits: 40,
  held_messages: 0,
  ...over,
});
const fire = (type: string, data: unknown) => act(() => handlers.get(type)?.(data));

function setup(role: "client" | "psychic" = "client", onScreen = true) {
  const calls = {
    apply: vi.fn(),
    extend: vi.fn(),
    openTopUp: vi.fn(),
    announce: vi.fn(),
    onRevealed: vi.fn(),
    onAdvisorRevealed: vi.fn(),
  };
  renderHook(
    () =>
      useHoldEvents({
        role,
        counterpartName: "Theo",
        isOnScreen: () => onScreen,
        apply: calls.apply,
        extend: calls.extend,
        openTopUp: calls.openTopUp,
        announce: calls.announce,
        onRevealed: calls.onRevealed,
        onAdvisorRevealed: calls.onAdvisorRevealed,
      }),
    { wrapper },
  );
  return calls;
}
const lines = (announce: ReturnType<typeof vi.fn>) => announce.mock.calls.flatMap((c) => c[0] as { id: string; text: string }[]).map((l) => l.text);

beforeEach(() => {
  handlers.clear();
  toast.show.mockReset();
});
afterEach(cleanup);

describe("session.held", () => {
  it("client: stores the hold, opens the top-up once and announces it", () => {
    const calls = setup();
    const h = hold();
    fire("session.held", { session: { server_time: new Date().toISOString() }, hold: h });
    expect(calls.apply).toHaveBeenCalledWith(h);
    expect(calls.openTopUp).toHaveBeenCalledTimes(1);
    expect(lines(calls.announce)[0]).toMatch(/^Your reading is on hold\. Top up within (59|60) seconds to keep chatting with Theo\.$/);
    // A replay of the same event (reconnect, second tab) never opens it again.
    fire("session.held", { session: { server_time: new Date().toISOString() }, hold: h });
    expect(calls.openTopUp).toHaveBeenCalledTimes(1);
    expect(calls.announce).toHaveBeenCalledTimes(1);
  });

  it("another hold opens the dialog again", () => {
    const calls = setup();
    fire("session.held", { session: {}, hold: hold() });
    fire("session.held", { session: {}, hold: hold({ id: "h2" }) });
    expect(calls.openTopUp).toHaveBeenCalledTimes(2);
  });

  it("advisor: announced, no dialog", () => {
    const calls = setup("psychic");
    fire("session.held", { session: {}, hold: hold() });
    expect(calls.openTopUp).not.toHaveBeenCalled();
    expect(lines(calls.announce)[0]).toBe("Theo's reading is on hold. Your replies are hidden from them until they top up.");
  });

  it("a conversation that is not on screen is stored but neither opens the dialog nor speaks", () => {
    const calls = setup("client", false);
    fire("session.held", { session: {}, hold: hold() });
    expect(calls.apply).toHaveBeenCalled();
    expect(calls.openTopUp).not.toHaveBeenCalled();
    expect(calls.announce).not.toHaveBeenCalled();
  });

  it("ignores an event without a hold", () => {
    const calls = setup();
    fire("session.held", { session: {} });
    expect(calls.apply).not.toHaveBeenCalled();
  });
});

describe("session.hold_extended and session.hold_ended", () => {
  it("extends the deadline", () => {
    const calls = setup();
    fire("session.hold_extended", { session_id: "s1", hold_id: "h1", expires_at: "2026-11-19T10:01:15Z" });
    expect(calls.extend).toHaveBeenCalledWith("h1", "2026-11-19T10:01:15Z");
    fire("session.hold_extended", { session_id: "s1" });
    expect(calls.extend).toHaveBeenCalledTimes(1);
  });

  it("converted: the client gets the toast and the announcement", () => {
    const calls = setup();
    fire("session.hold_ended", { session_id: "s1", hold: hold({ status: "converted" }) });
    expect(calls.apply).toHaveBeenCalledWith(expect.objectContaining({ status: "converted" }));
    expect(toast.show).toHaveBeenCalledWith({ tone: "success", message: "Your reading with Theo continues." });
    expect(lines(calls.announce)).toEqual(["Your reading with Theo continues."]);
    // Once only.
    fire("session.hold_ended", { session_id: "s1", hold: hold({ status: "converted" }) });
    expect(toast.show).toHaveBeenCalledTimes(1);
  });

  it("converted: the advisor is told, without a toast", () => {
    const calls = setup("psychic");
    fire("session.hold_ended", { session_id: "s1", hold: hold({ status: "converted" }) });
    expect(toast.show).not.toHaveBeenCalled();
    expect(lines(calls.announce)).toEqual(["Theo topped up. Your replies are visible to them now."]);
  });

  it("expired: each side hears that the reading ended", () => {
    const client = setup();
    fire("session.hold_ended", { session_id: "s1", hold: hold({ status: "expired" }) });
    expect(lines(client.announce)).toEqual(["The reading with Theo ended before a top-up arrived."]);
    cleanup();
    handlers.clear();
    const advisor = setup("psychic");
    fire("session.hold_ended", { session_id: "s1", hold: hold({ id: "h3", status: "expired" }) });
    expect(lines(advisor.announce)).toEqual(["The reading with Theo ended before they topped up. Your replies were not revealed."]);
  });
});

describe("messages.revealed", () => {
  const message: ChatMessage = { id: "m1", conversation_id: "c1", kind: "text", body: "Hello", created_at: "2026-11-19T10:00:00Z" };

  it("client: the full messages", () => {
    const calls = setup();
    const event: MessagesRevealedEvent = { conversation_id: "c1", session_id: "s1", hold_id: "h1", messages: [message] };
    fire("messages.revealed", event);
    expect(calls.onRevealed).toHaveBeenCalledWith(event);
    expect(calls.onAdvisorRevealed).not.toHaveBeenCalled();
  });

  it("advisor: the ids", () => {
    const calls = setup("psychic");
    const event: MessagesRevealedEvent = { conversation_id: "c1", session_id: "s1", hold_id: "h1", message_ids: ["m1", "m2"] };
    fire("messages.revealed", event);
    expect(calls.onAdvisorRevealed).toHaveBeenCalledWith(event);
    expect(calls.onRevealed).not.toHaveBeenCalled();
  });
});
