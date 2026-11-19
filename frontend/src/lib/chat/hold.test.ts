import { describe, expect, it } from "vitest";

import type { ChatHold } from "@/lib/api/holds";
import type { ChatMessage, Conversation } from "@/lib/api/types";
import {
  EMPTY_HOLD_STORE,
  currentHold,
  formatCountdown,
  heldMarker,
  holdReasonCopy,
  holdSecondsLeft,
  holdStoreReducer,
  lockedPlaceholder,
  lockedSummary,
  mergeHold,
  revealInbox,
  toTopUpHold,
  type HoldStore,
} from "@/lib/chat/hold";

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

describe("countdown", () => {
  it("counts whole seconds to the server's deadline, never below zero", () => {
    const at = Date.parse("2026-11-19T10:00:00Z");
    expect(holdSecondsLeft("2026-11-19T10:01:00Z", at)).toBe(60);
    expect(holdSecondsLeft("2026-11-19T10:01:00Z", at + 59_001)).toBe(1);
    expect(holdSecondsLeft("2026-11-19T10:01:00Z", at + 61_000)).toBe(0);
  });

  it("follows the server clock, not a wrong device clock", () => {
    const deviceNow = Date.parse("2026-11-19T09:00:00Z"); // device is an hour behind
    const offset = 3_600_000;
    expect(holdSecondsLeft("2026-11-19T10:01:00Z", deviceNow, offset)).toBe(60);
    expect(holdSecondsLeft("2026-11-19T10:01:00Z", deviceNow)).toBe(3660);
  });

  it("is 0 without a usable deadline", () => {
    expect(holdSecondsLeft(undefined)).toBe(0);
    expect(holdSecondsLeft("not a date")).toBe(0);
  });

  it("formats minutes and padded seconds", () => {
    expect(formatCountdown(59)).toBe("0:59");
    expect(formatCountdown(60)).toBe("1:00");
    expect(formatCountdown(65)).toBe("1:05");
    expect(formatCountdown(-3)).toBe("0:00");
    expect(formatCountdown(Number.NaN)).toBe("0:00");
  });
});

describe("mergeHold", () => {
  it("only moves forward: open → expired → revealed", () => {
    const open = hold();
    const expired = hold({ status: "expired", resolved_at: "2026-11-19T10:01:00Z" });
    const revealed = hold({ status: "revealed", revealed_at: "2026-11-19T10:05:00Z" });
    expect(mergeHold(open, expired).status).toBe("expired");
    expect(mergeHold(expired, open).status).toBe("expired");
    expect(mergeHold(expired, revealed).status).toBe("revealed");
    expect(mergeHold(revealed, expired).status).toBe("revealed");
    expect(mergeHold(open, hold({ status: "converted" })).status).toBe("converted");
  });

  it("only moves the deadline out", () => {
    const merged = mergeHold(hold({ expires_at: "2026-11-19T10:01:15Z" }), hold({ expires_at: "2026-11-19T10:01:00Z" }));
    expect(merged.expires_at).toBe("2026-11-19T10:01:15Z");
  });

  it("another hold replaces the current one", () => {
    expect(mergeHold(hold(), hold({ id: "h2" })).id).toBe("h2");
    expect(mergeHold(null, hold()).id).toBe("h1");
  });
});

describe("hold store", () => {
  const apply = (s: HoldStore, h: ChatHold) => holdStoreReducer(s, { type: "hold", hold: h });
  const extend = (s: HoldStore, expiresAt: string) => holdStoreReducer(s, { type: "extended", holdId: "h1", expiresAt });

  it("session.held then session.hold_extended then session.hold_ended", () => {
    let s = apply(EMPTY_HOLD_STORE, hold());
    s = extend(s, "2026-11-19T10:01:15Z");
    expect(s.holds.h1.expires_at).toBe("2026-11-19T10:01:15Z");
    s = apply(s, hold({ status: "expired" }));
    expect(s.holds.h1.status).toBe("expired");
    // A late extension after the end changes nothing.
    expect(extend(s, "2026-11-19T10:02:00Z")).toBe(s);
  });

  it("an extension that arrives before its hold applies to it", () => {
    let s = extend(EMPTY_HOLD_STORE, "2026-11-19T10:01:15Z");
    expect(s.extended.h1).toBe("2026-11-19T10:01:15Z");
    s = apply(s, hold());
    expect(s.holds.h1.expires_at).toBe("2026-11-19T10:01:15Z");
  });

  it("ignores an older copy and keeps the same state object", () => {
    const s = apply(EMPTY_HOLD_STORE, hold({ status: "expired" }));
    expect(apply(s, hold())).toBe(s);
  });

  it("currentHold prefers the newer of the session's copy and the store's", () => {
    const store = apply(EMPTY_HOLD_STORE, hold({ status: "expired" }));
    expect(currentHold(hold(), store)?.status).toBe("expired");
    expect(currentHold(undefined, store)).toBeNull();
    const extended = extend(EMPTY_HOLD_STORE, "2026-11-19T10:01:30Z");
    expect(currentHold(hold(), extended)?.expires_at).toBe("2026-11-19T10:01:30Z");
    expect(currentHold(hold({ status: "expired" }), extended)?.expires_at).toBe("2026-11-19T10:01:00Z");
  });
});

describe("lockedPlaceholder", () => {
  it("has exactly body_length characters (clamped to 1..2000) and is stable per message", () => {
    for (const n of [1, 5, 37, 140, 2000]) expect(lockedPlaceholder(n, "m1")).toHaveLength(n);
    expect(lockedPlaceholder(5000, "m1")).toHaveLength(2000);
    expect(lockedPlaceholder(37, "m1")).toBe(lockedPlaceholder(37, "m1"));
    expect(lockedPlaceholder(37, "m1")).not.toBe(lockedPlaceholder(37, "m2"));
  });

  it("falls back to a short placeholder without a length and never has trailing space", () => {
    expect(lockedPlaceholder(undefined, "m1")).toHaveLength(24);
    expect(lockedPlaceholder(0, "m1")).toHaveLength(24);
    for (const n of [7, 11, 50, 99]) expect(lockedPlaceholder(n, "x")).toBe(lockedPlaceholder(n, "x").trimEnd());
  });

  it("is letters and spaces only", () => {
    expect(lockedPlaceholder(300, "m3")).toMatch(/^[a-z ]+$/);
  });
});

describe("heldMarker (advisor)", () => {
  it("says hidden while the hold is open, not revealed once expired, nothing after", () => {
    const m = { held: true, hold_id: "h1" };
    expect(heldMarker(m, hold())).toBe("hidden");
    expect(heldMarker(m, hold({ status: "expired" }))).toBe("not_revealed");
    expect(heldMarker(m, hold({ status: "revealed" }))).toBeNull();
    expect(heldMarker(m, hold({ status: "converted" }))).toBeNull();
  });

  it("shows nothing on a message that is not held and treats an unknown hold as settled", () => {
    expect(heldMarker({ held: false, hold_id: "h1" }, hold())).toBeNull();
    expect(heldMarker({}, hold())).toBeNull();
    expect(heldMarker({ held: true, hold_id: "old" }, hold())).toBe("not_revealed");
    expect(heldMarker({ held: true, hold_id: "h1" }, null)).toBe("not_revealed");
  });
});

describe("lockedSummary", () => {
  it("names the newest hold that still hides messages and counts them", () => {
    expect(lockedSummary([])).toBeNull();
    expect(lockedSummary([{ locked: false, hold_id: "h1", seq: 1 }, { seq: 2 }])).toBeNull();
    expect(
      lockedSummary([
        { locked: true, hold_id: "h1", seq: 2 },
        { locked: true, hold_id: "h2", seq: 9 },
        { locked: true, hold_id: "h2", seq: 10 },
      ]),
    ).toEqual({ holdId: "h2", loaded: 2 });
  });
});

describe("revealInbox", () => {
  const row = (last: Partial<ChatMessage>): Conversation => ({
    id: "c1",
    counterpart: { id: "p", role: "psychic", name: "Theo" },
    unread: 0,
    created_at: "2026-11-19T10:00:00Z",
    last_message: { id: "m1", conversation_id: "c1", kind: "text", body: "", created_at: "2026-11-19T10:00:00Z", locked: true, ...last },
  });
  const revealed: ChatMessage = { id: "m1", conversation_id: "c1", kind: "text", body: "The stars are kind", created_at: "2026-11-19T10:00:00Z", locked: false };

  it("replaces a locked preview by the revealed message", () => {
    const next = revealInbox([row({})], [revealed]);
    expect(next[0].last_message?.body).toBe("The stars are kind");
    expect(next[0].last_message?.locked).toBeUndefined();
  });

  it("leaves other rows and an empty list untouched", () => {
    const items = [row({ id: "other" })];
    expect(revealInbox(items, [revealed])).toBe(items);
    expect(revealInbox(items, [])).toBe(items);
  });
});

describe("copy and the top-up context", () => {
  it("has a line per reason", () => {
    expect(holdReasonCopy("trial_ended")).toBe("Your free minutes are over.");
    expect(holdReasonCopy("credits_exhausted")).toBe("Your credits ran out.");
    expect(holdReasonCopy(undefined)).toBe("Your free minutes are over.");
  });

  it("builds the dialog's hold only from an open hold", () => {
    expect(toTopUpHold(hold(), 120)).toEqual({ reason: "trial_ended", expiresAt: "2026-11-19T10:01:00Z", priceCredits: 40, clockOffsetMs: 120 });
    expect(toTopUpHold(hold({ status: "expired" }), 0)).toBeUndefined();
    expect(toTopUpHold(null, 0)).toBeUndefined();
  });
});
