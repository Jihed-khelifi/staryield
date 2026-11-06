import { describe, expect, it, vi } from "vitest";

import type { ChatMessage, ChatSession, SincePage } from "@/lib/api/types";
import {
  Backfill,
  EMPTY_TIMELINE,
  elapsedSeconds,
  formatMoney,
  hasGap,
  isGapAfter,
  isOpen,
  mergeMessages,
  newClientMsgId,
  newestIncomingSeq,
  secondsUntil,
  sessionEarnings,
  sessionRate,
  systemCopy,
  withMessages,
  withNewestPage,
  withOlderPage,
  type Timeline,
  type UiMessage,
} from "@/components/chat/chat-model";

const msg = (id: string, at: string, extra: Partial<UiMessage> = {}): UiMessage => ({
  id,
  conversation_id: "c",
  session_id: "s",
  sender_id: "u1",
  kind: "text",
  body: id,
  created_at: at,
  ...extra,
});

/** A confirmed message with a seq; clocks deliberately disagree with seq order. */
const seqMsg = (seq: number, extra: Partial<UiMessage> = {}): ChatMessage =>
  msg(`m${seq}`, `2026-01-01T00:00:${String(60 - seq).padStart(2, "0")}Z`, { seq, ...extra });

const seqs = (items: { seq?: number }[]) => items.map((m) => m.seq);

describe("mergeMessages", () => {
  it("dedupes by id and keeps chronological order", () => {
    const out = mergeMessages([msg("b", "2026-01-01T00:00:02Z")], [msg("a", "2026-01-01T00:00:01Z"), msg("b", "2026-01-01T00:00:02Z")]);
    expect(out.map((m) => m.id)).toEqual(["a", "b"]);
  });

  it("orders confirmed messages strictly by seq, whatever their clocks say", () => {
    const out = mergeMessages([], [seqMsg(3), seqMsg(1), seqMsg(2)]);
    expect(seqs(out)).toEqual([1, 2, 3]);
    // Created-at runs backwards here: seq wins.
    expect(out[0].created_at > out[2].created_at).toBe(true);
  });

  it("keeps local messages after confirmed ones, in the order they were typed", () => {
    const local1 = msg("l1", "2026-01-01T00:00:00Z", { client_msg_id: "l1", pending: "queued" });
    const local2 = msg("l2", "2026-01-01T00:00:01Z", { client_msg_id: "l2", pending: "sending" });
    const out = mergeMessages([local2, local1], [seqMsg(7), seqMsg(6)]);
    expect(out.map((m) => m.id)).toEqual(["m6", "m7", "l1", "l2"]);
  });

  it("replaces an optimistic message with the server copy (same client id)", () => {
    const pending = msg("m-local", "2026-01-01T00:00:05Z", { client_msg_id: "m-local", pending: "sending" });
    const server = msg("srv-1", "2026-01-01T00:00:04Z", { client_msg_id: "m-local", seq: 4 }) as ChatMessage;
    const out = mergeMessages([pending], [server]);
    expect(out).toHaveLength(1);
    expect(out[0].id).toBe("srv-1");
    expect(out[0].pending).toBeUndefined();
  });

  it("never duplicates: a late optimistic copy does not shadow its confirmed message", () => {
    const server = msg("srv-1", "2026-01-01T00:00:04Z", { client_msg_id: "cm-1", seq: 4 });
    const lateLocal = msg("cm-1", "2026-01-01T00:00:05Z", { client_msg_id: "cm-1", pending: "queued" });
    const out = mergeMessages([server], [lateLocal]);
    expect(out).toHaveLength(1);
    expect(out[0].pending).toBeUndefined();
  });

  it("keeps the bookmark flag when an event copy (without it) arrives", () => {
    const out = mergeMessages([seqMsg(1, { saved: true })], [seqMsg(1)]);
    expect(out[0].saved).toBe(true);
  });

  it("does not merge client ids across senders", () => {
    const mine = msg("a", "2026-01-01T00:00:01Z", { client_msg_id: "x", sender_id: "u1" });
    const theirs = msg("b", "2026-01-01T00:00:02Z", { client_msg_id: "x", sender_id: "u2" });
    expect(mergeMessages([mine], [theirs])).toHaveLength(2);
  });
});

describe("timeline: seq ordering and gaps", () => {
  const page = (from: number, to: number, hasMore = false) => ({ items: Array.from({ length: to - from + 1 }, (_, i) => seqMsg(from + i)), has_more: hasMore });

  it("starts from the newest page with a contiguous head", () => {
    const t = withNewestPage(EMPTY_TIMELINE, page(51, 100, true));
    expect(t.loaded).toBe(true);
    expect(t.headSeq).toBe(100);
    expect(t.hasOlder).toBe(true);
    expect(hasGap(t)).toBe(false);
  });

  it("keeps events that arrived while the first page was loading", () => {
    const early = withMessages(EMPTY_TIMELINE, [seqMsg(11)]);
    expect(early.loaded).toBe(false);
    expect(isGapAfter(early, 99)).toBe(false); // not checked before the page arrives
    const t = withNewestPage(early, page(1, 10));
    expect(seqs(t.messages)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11]);
    expect(t.headSeq).toBe(11);
  });

  it("detects a gap when an event skips a seq, and closes it when the fill arrives", () => {
    let t: Timeline = withNewestPage(EMPTY_TIMELINE, page(1, 5));
    expect(isGapAfter(t, 6)).toBe(false);
    expect(isGapAfter(t, 8)).toBe(true);
    t = withMessages(t, [seqMsg(8)]);
    expect(hasGap(t)).toBe(true);
    expect(t.headSeq).toBe(5);
    // The REST fill (after_seq=5) returns 6..8; 8 is deduplicated.
    t = withMessages(t, [seqMsg(6), seqMsg(7), seqMsg(8)]);
    expect(hasGap(t)).toBe(false);
    expect(t.headSeq).toBe(8);
    expect(seqs(t.messages)).toEqual([1, 2, 3, 4, 5, 6, 7, 8]);
  });

  it("merges older pages (before_seq) without touching the head", () => {
    let t = withNewestPage(EMPTY_TIMELINE, page(51, 100, true));
    t = withOlderPage(t, page(1, 50, false));
    expect(t.messages).toHaveLength(100);
    expect(seqs(t.messages).slice(0, 2)).toEqual([1, 2]);
    expect(t.hasOlder).toBe(false);
    expect(t.headSeq).toBe(100);
  });

  it("ignores duplicates from replays and backfills", () => {
    let t = withNewestPage(EMPTY_TIMELINE, page(1, 3));
    t = withMessages(t, [seqMsg(2), seqMsg(3), seqMsg(3)]);
    expect(seqs(t.messages)).toEqual([1, 2, 3]);
  });

  it("finds the newest message from the other side", () => {
    const t = withNewestPage(EMPTY_TIMELINE, { items: [seqMsg(1, { sender_id: "them" }), seqMsg(2, { sender_id: "me" }), seqMsg(3, { sender_id: "them" }), seqMsg(4, { sender_id: "me" })], has_more: false });
    expect(newestIncomingSeq(t.messages, "me")).toBe(3);
    expect(newestIncomingSeq([], "me")).toBe(0);
  });
});

describe("Backfill (single-flight gap filling)", () => {
  const since = (items: ChatMessage[], hasMore = false): SincePage => ({ items, has_more: hasMore, last_seq: items.at(-1)?.seq ?? 0 });

  it("walks has_more pages from the head and applies each page", async () => {
    let head = 5;
    const applied: number[][] = [];
    const fetchSince = vi.fn(async (after: number) => (after === 5 ? since([seqMsg(6), seqMsg(7)], true) : since([seqMsg(8)])));
    const filler = new Backfill({ fetchSince, head: () => head, apply: (items) => applied.push(seqs(items) as number[]) });
    await filler.request();
    expect(fetchSince.mock.calls.map((c) => c[0])).toEqual([5, 7]);
    expect(applied).toEqual([[6, 7], [8]]);
    head = 8;
  });

  it("coalesces requests made while a walk runs into one follow-up walk", async () => {
    let release!: () => void;
    const gate = new Promise<void>((r) => (release = r));
    let head = 1;
    const fetchSince = vi.fn(async (after: number) => {
      if (after === 1) await gate;
      return since([]);
    });
    const filler = new Backfill({ fetchSince, head: () => head, apply: () => undefined });
    const first = filler.request();
    // A burst of gap events while the first walk is in flight.
    void filler.request();
    void filler.request();
    void filler.request();
    expect(filler.busy).toBe(true);
    head = 4;
    release();
    await first;
    // One walk plus exactly one coalesced follow-up from the new head.
    expect(fetchSince.mock.calls.map((c) => c[0])).toEqual([1, 4]);
    expect(filler.busy).toBe(false);
  });

  it("skips the follow-up of gap-only requests when the walk closed the gap", async () => {
    let release!: () => void;
    const gate = new Promise<void>((r) => (release = r));
    let gap = true;
    const fetchSince = vi.fn(async () => {
      await gate;
      gap = false;
      return since([]);
    });
    const filler = new Backfill({ fetchSince, head: () => 1, apply: () => undefined, needed: () => gap });
    const first = filler.request(false);
    void filler.request(false);
    release();
    await first;
    expect(fetchSince).toHaveBeenCalledOnce();
  });

  it("gives up after too many pages and asks for a reload", async () => {
    const onOverflow = vi.fn();
    let n = 0;
    const fetchSince = vi.fn(async () => since([seqMsg(++n)], true));
    const filler = new Backfill({ fetchSince, head: () => 0, apply: () => undefined, onOverflow, maxPages: 3 });
    await filler.request();
    expect(fetchSince).toHaveBeenCalledTimes(3);
    expect(onOverflow).toHaveBeenCalledOnce();
  });

  it("reports a failed walk and accepts a new request afterwards", async () => {
    const fetchSince = vi.fn().mockRejectedValueOnce(new Error("offline")).mockResolvedValue(since([]));
    const filler = new Backfill({ fetchSince, head: () => 0, apply: () => undefined });
    await expect(filler.request()).rejects.toThrow("offline");
    await filler.request();
    expect(fetchSince).toHaveBeenCalledTimes(2);
  });

  it("does nothing once stopped", async () => {
    const fetchSince = vi.fn(async () => since([]));
    const filler = new Backfill({ fetchSince, head: () => 0, apply: () => undefined });
    filler.stop();
    await filler.request();
    expect(fetchSince).not.toHaveBeenCalled();
  });
});

describe("rates and earnings", () => {
  it("shows a member price below the list rate", () => {
    expect(sessionRate({ rate_per_minute: 32, list_rate_per_minute: 40, discount_pct: 20 } as ChatSession)).toEqual({ rate: 32, listRate: 40, discountPct: 20, discounted: true });
  });

  it("has no discount when the rates match or the fields are absent", () => {
    expect(sessionRate({ rate_per_minute: 40, list_rate_per_minute: 40 } as ChatSession).discounted).toBe(false);
    expect(sessionRate({ rate_per_minute: 40 } as ChatSession)).toEqual({ rate: 40, listRate: 40, discountPct: 0, discounted: false });
    expect(sessionRate(null, 25)).toEqual({ rate: 25, listRate: 25, discountPct: 0, discounted: false });
  });

  it("formats money with Intl currency in the viewer's locale", () => {
    expect(formatMoney(4033, "USD", "en")).toBe("$40.33");
    expect(formatMoney(4033, "USD", "fr").replace(/\s/g, " ")).toBe("40,33 $US");
    expect(formatMoney(4033, "EUR", "es").replace(/\s/g, " ")).toBe("40,33 €");
    expect(formatMoney(0, undefined, "en")).toBe("$0.00");
    // An unknown currency code never throws.
    expect(formatMoney(150, "NOT-A-CODE", "en")).toBe("1.50 NOT-A-CODE");
  });

  it("reads the advisor's live earnings from the session view", () => {
    expect(sessionEarnings({ psychic_earnings: 24, psychic_earnings_cents: 48, currency: "USD" } as ChatSession)).toEqual({ credits: 24, cents: 48, currency: "USD" });
    expect(sessionEarnings({ psychic_earnings: 24 } as ChatSession)).toEqual({ credits: 24, cents: undefined, currency: undefined });
    expect(sessionEarnings({ credits_charged: 40 } as ChatSession)).toBeNull();
    expect(sessionEarnings(null)).toBeNull();
  });
});

describe("server-authoritative timers", () => {
  const session = (status: ChatSession["status"], extra: Partial<ChatSession> = {}) =>
    ({ status, active_seconds: 90, server_time: "2026-01-01T12:00:00Z", ...extra }) as ChatSession;

  it("extrapolates active time from the snapshot while active", () => {
    const snap = { session: session("active"), receivedAt: 1_000_000 };
    expect(elapsedSeconds(snap, 1_000_000 + 15_500)).toBe(105);
  });

  it("freezes time when not active", () => {
    const snap = { session: session("disconnected"), receivedAt: 0 };
    expect(elapsedSeconds(snap, 999_999)).toBe(90);
  });

  it("counts down deadlines using the server clock offset", () => {
    // Local clock is 10s behind the server.
    const receivedAt = new Date("2026-01-01T11:59:50Z").getTime();
    const snap = { session: session("requested", { expires_at: "2026-01-01T12:01:00Z" }), receivedAt };
    expect(secondsUntil(snap.session.expires_at, snap, receivedAt)).toBe(60);
    expect(secondsUntil(snap.session.expires_at, snap, receivedAt + 120_000)).toBe(0);
  });

  it("classifies open sessions", () => {
    expect(isOpen(session("requested"))).toBe(true);
    expect(isOpen(session("disconnected"))).toBe(true);
    expect(isOpen(session("settled"))).toBe(false);
    expect(isOpen(null)).toBe(false);
  });
});

describe("copy helpers", () => {
  it("maps system message codes", () => {
    expect(systemCopy("session.started")).toBe("Live chat started");
    expect(systemCopy("session.ended:credits_exhausted")).toBe("Live chat ended");
  });

  it("generates server-acceptable client message ids", () => {
    const id = newClientMsgId();
    expect(id).toMatch(/^[A-Za-z0-9_\-:.]{6,64}$/);
    expect(newClientMsgId()).not.toBe(id);
  });
});
