// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";

import type { ChatHold } from "@/lib/api/holds";
import type { ChatSession } from "@/lib/api/types";
import { useChatHoldState } from "@/components/chat/use-chat-hold";

afterEach(cleanup);

const hold = (over: Partial<ChatHold> = {}): ChatHold => ({
  id: "h1",
  session_id: "s1",
  conversation_id: "c1",
  reason: "trial_ended",
  status: "open",
  started_at: "2026-11-19T10:00:00Z",
  expires_at: "2026-11-19T10:01:00Z",
  price_credits: 40,
  held_messages: 0,
  ...over,
});
const session = (h?: ChatHold): ChatSession => ({ id: "s1", conversation_id: "c1", status: "held", ...(h ? { hold: h } : {}) }) as ChatSession;

describe("useChatHoldState", () => {
  it("is the session's own hold, and none without a session or hold", () => {
    expect(renderHook(() => useChatHoldState(null)).result.current.hold).toBeNull();
    expect(renderHook(() => useChatHoldState(session())).result.current.hold).toBeNull();
    expect(renderHook(() => useChatHoldState(session(hold()))).result.current.hold?.id).toBe("h1");
  });

  it("session.hold_extended moves the deadline out and re-syncs the countdown source", () => {
    const { result } = renderHook(() => useChatHoldState(session(hold())));
    act(() => result.current.extend("h1", "2026-11-19T10:01:15Z"));
    expect(result.current.hold?.expires_at).toBe("2026-11-19T10:01:15Z");
    // An older deadline never shortens it.
    act(() => result.current.extend("h1", "2026-11-19T10:01:05Z"));
    expect(result.current.hold?.expires_at).toBe("2026-11-19T10:01:15Z");
  });

  it("session.hold_ended (expired, then converted-looking late events) only moves forward", () => {
    const { result } = renderHook(() => useChatHoldState(session(hold())));
    act(() => result.current.apply(hold({ status: "expired" })));
    expect(result.current.hold?.status).toBe("expired");
    act(() => result.current.apply(hold()));
    expect(result.current.hold?.status).toBe("expired");
    act(() => result.current.apply(hold({ status: "revealed", reveal_credits: 40 } as Partial<ChatHold>)));
    expect(result.current.hold?.status).toBe("revealed");
  });

  it("a hold event for the next session's hold does not leak into the one on screen", () => {
    const { result } = renderHook(() => useChatHoldState(session(hold())));
    act(() => result.current.apply(hold({ id: "h2", status: "expired" })));
    expect(result.current.hold?.id).toBe("h1");
    expect(result.current.holdById("h2")?.status).toBe("expired");
  });

  it("holdById knows the session's hold and every hold applied (a hold of an ended reading, after GET)", () => {
    const { result } = renderHook(() => useChatHoldState(session(hold())));
    expect(result.current.holdById("h1")?.status).toBe("open");
    expect(result.current.holdById("nope")).toBeUndefined();
    act(() => result.current.apply(hold({ id: "h9", status: "expired" })));
    expect(result.current.holdById("h9")?.status).toBe("expired");
  });

  it("a new session view with a newer hold replaces the old one", () => {
    const { result, rerender } = renderHook((s: ChatSession | null) => useChatHoldState(s), { initialProps: session(hold()) as ChatSession | null });
    rerender(session(hold({ id: "h2" })));
    expect(result.current.hold?.id).toBe("h2");
    rerender(null);
    expect(result.current.hold).toBeNull();
  });
});
