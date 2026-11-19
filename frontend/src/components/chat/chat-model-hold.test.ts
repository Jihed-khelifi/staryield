import { describe, expect, it } from "vitest";

import type { ChatMessage, ChatSession } from "@/lib/api/types";
import {
  EMPTY_TIMELINE,
  asRevealed,
  canLikeOrSave,
  endReasonCopy,
  isOpen,
  mergeMessages,
  replyPreviewOf,
  withHeldCleared,
  withMessages,
  type UiMessage,
} from "@/components/chat/chat-model";

const msg = (over: Partial<UiMessage> = {}): UiMessage => ({
  id: "m1",
  seq: 1,
  conversation_id: "c1",
  sender_id: "theo",
  kind: "text",
  body: "Mercury is retrograde",
  created_at: "2026-11-19T10:00:00Z",
  ...over,
});
const locked = (over: Partial<UiMessage> = {}) => msg({ body: "", locked: true, hold_id: "h1", body_length: 21, ...over });

describe("held sessions", () => {
  it("are open and live (messages go into them)", () => {
    expect(isOpen({ status: "held" } as ChatSession)).toBe(true);
  });
});

describe("merging locked messages", () => {
  it("a revealed message replaces the locked one and is marked unlocked", () => {
    const merged = mergeMessages([locked()], asRevealed([msg({ hold_id: undefined })]));
    expect(merged).toHaveLength(1);
    expect(merged[0].body).toBe("Mercury is retrograde");
    expect(merged[0].locked).toBe(false);
    expect(merged[0].body_length).toBeUndefined();
  });

  it("a revealed message without the mark (history after a reload) is unlocked too", () => {
    const merged = mergeMessages([locked()], [msg()]);
    expect(merged[0].locked).toBeUndefined();
    expect(merged[0].body).toBe("Mercury is retrograde");
  });

  it("a stale locked copy never hides a revealed message again", () => {
    const revealed = mergeMessages([locked()], asRevealed([msg()]));
    const again = mergeMessages(revealed, [locked()]);
    expect(again[0].body).toBe("Mercury is retrograde");
    expect(again[0].locked).toBe(false);
  });

  it("a locked copy arriving for a message not known yet is kept locked", () => {
    const merged = mergeMessages([], [locked({ id: "m9", seq: 9 })]);
    expect(merged[0].locked).toBe(true);
    expect(merged[0].body).toBe("");
  });

  it("the server's word on held (advisor) wins over what was known", () => {
    const held = mergeMessages([], [msg({ held: true, hold_id: "h1" })]);
    expect(held[0].held).toBe(true);
    expect(mergeMessages(held, [msg()])[0].held).toBeUndefined();
  });

  it("an optimistic copy keeps no hold flags of its own", () => {
    const confirmed = msg({ id: "m2", seq: 2, sender_id: "me", client_msg_id: "c-1" });
    const optimistic = msg({ id: "c-1", seq: undefined, sender_id: "me", client_msg_id: "c-1", pending: "sending" });
    const merged = mergeMessages([optimistic], [confirmed]);
    expect(merged).toHaveLength(1);
    expect(merged[0].id).toBe("m2");
  });
});

describe("advisor markers", () => {
  const t = withMessages({ ...EMPTY_TIMELINE, loaded: true }, [
    msg({ id: "a", seq: 1, held: true, hold_id: "h1" }),
    msg({ id: "b", seq: 2, held: true, hold_id: "h1" }),
    msg({ id: "c", seq: 3 }),
  ]);

  it("clears held on the revealed ids only", () => {
    const next = withHeldCleared(t, ["a"]);
    expect(next.messages.map((m) => m.held)).toEqual([false, true, undefined]);
  });

  it("returns the same timeline when nothing is held among the ids", () => {
    expect(withHeldCleared(t, ["c"])).toBe(t);
    expect(withHeldCleared(t, [])).toBe(t);
  });
});

describe("what a locked message allows", () => {
  it("can be answered but not liked or saved", () => {
    expect(canLikeOrSave(locked())).toBe(false);
    expect(canLikeOrSave(msg())).toBe(true);
    expect(canLikeOrSave(msg({ locked: false }))).toBe(true);
    expect(canLikeOrSave(msg({ pending: "sending" }))).toBe(false);
  });

  it("its quote carries no content and is flagged locked", () => {
    expect(replyPreviewOf(locked())).toEqual({ id: "m1", seq: 1, sender_id: "theo", kind: "text", excerpt: "", locked: true });
    // Even a body a buggy server sent is never quoted.
    expect(replyPreviewOf(locked({ body: "SECRET" })).excerpt).toBe("");
    expect(replyPreviewOf(locked({ kind: "image" })).kind).toBe("image");
  });

  it("a readable message quotes as before", () => {
    expect(replyPreviewOf(msg())).toEqual({ id: "m1", seq: 1, sender_id: "theo", kind: "text", excerpt: "Mercury is retrograde" });
  });
});

describe("hold_expired", () => {
  it("has end-reason copy", () => {
    expect(endReasonCopy.hold_expired).toMatch(/no top-up arrived in time/);
  });
});

describe("messages typed as ChatMessage keep compiling", () => {
  it("accepts the hold fields", () => {
    const m: ChatMessage = { ...msg(), held: true, locked: false, hold_id: "h1", body_length: 3 };
    expect(m.hold_id).toBe("h1");
  });
});
