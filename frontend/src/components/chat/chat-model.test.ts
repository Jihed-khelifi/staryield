import { describe, expect, it } from "vitest";

import type { ChatMessage, ChatSession } from "@/lib/api/types";
import { elapsedSeconds, isOpen, mergeMessages, newClientMsgId, secondsUntil, systemCopy, type UiMessage } from "@/components/chat/chat-model";

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

describe("mergeMessages", () => {
  it("dedupes by id and keeps chronological order", () => {
    const out = mergeMessages([msg("b", "2026-01-01T00:00:02Z")], [msg("a", "2026-01-01T00:00:01Z"), msg("b", "2026-01-01T00:00:02Z")]);
    expect(out.map((m) => m.id)).toEqual(["a", "b"]);
  });

  it("replaces an optimistic message with the server copy (same client id)", () => {
    const pending = msg("m-local", "2026-01-01T00:00:05Z", { client_msg_id: "m-local", pending: "sending" });
    const server = msg("srv-1", "2026-01-01T00:00:04Z", { client_msg_id: "m-local" }) as ChatMessage;
    const out = mergeMessages([pending], [server]);
    expect(out).toHaveLength(1);
    expect(out[0].id).toBe("srv-1");
    expect(out[0].pending).toBeUndefined();
  });

  it("does not merge client ids across senders", () => {
    const mine = msg("a", "2026-01-01T00:00:01Z", { client_msg_id: "x", sender_id: "u1" });
    const theirs = msg("b", "2026-01-01T00:00:02Z", { client_msg_id: "x", sender_id: "u2" });
    expect(mergeMessages([mine], [theirs])).toHaveLength(2);
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
