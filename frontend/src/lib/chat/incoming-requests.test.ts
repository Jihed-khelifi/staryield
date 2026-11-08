import { describe, expect, it } from "vitest";

import type { ChatSession } from "@/lib/api/types";
import {
  EMPTY_REQUESTS,
  applyRequestEvent,
  isLiveRequest,
  newestRequest,
  nextExpiryMs,
  pruneExpired,
  replaceRequests,
  requestSecondsLeft,
} from "@/lib/chat/incoming-requests";

const NOW = Date.parse("2026-01-01T12:00:00Z");
const iso = (offsetSec: number) => new Date(NOW + offsetSec * 1000).toISOString();

function session(id: string, over: Partial<ChatSession> = {}): ChatSession {
  return {
    id,
    conversation_id: `c-${id}`,
    status: "requested",
    client: { id: `u-${id}`, name: `Client ${id}` },
    requested_at: iso(-5),
    expires_at: iso(60),
    server_time: iso(0),
    ...over,
  } as ChatSession;
}

describe("applyRequestEvent", () => {
  it("adds on session.requested", () => {
    const next = applyRequestEvent(EMPTY_REQUESTS, "session.requested", session("a"), NOW);
    expect([...next.keys()]).toEqual(["a"]);
  });

  it("ignores requests that are not in requested status or already expired", () => {
    expect(applyRequestEvent(EMPTY_REQUESTS, "session.requested", session("a", { status: "active" }), NOW).size).toBe(0);
    expect(applyRequestEvent(EMPTY_REQUESTS, "session.requested", session("a", { expires_at: iso(-1) }), NOW).size).toBe(0);
  });

  it.each(["accepted", "started", "declined", "expired", "cancelled", "ended"])("removes on session.%s", (name) => {
    const set = applyRequestEvent(EMPTY_REQUESTS, "session.requested", session("a"), NOW);
    expect(applyRequestEvent(set, `session.${name}`, session("a"), NOW).size).toBe(0);
  });

  it("keeps the same map for unrelated events and unknown sessions", () => {
    const set = applyRequestEvent(EMPTY_REQUESTS, "session.requested", session("a"), NOW);
    expect(applyRequestEvent(set, "session.disconnected", session("a"), NOW)).toBe(set);
    expect(applyRequestEvent(set, "session.cancelled", session("zzz"), NOW)).toBe(set);
    expect(applyRequestEvent(set, "session.requested", undefined, NOW)).toBe(set);
  });

  it("does not mutate the previous map", () => {
    const a = applyRequestEvent(EMPTY_REQUESTS, "session.requested", session("a"), NOW);
    applyRequestEvent(a, "session.requested", session("b"), NOW);
    expect(a.size).toBe(1);
  });
});

describe("replaceRequests", () => {
  it("replaces the whole set and drops non-live entries", () => {
    const next = replaceRequests([session("a"), session("b", { status: "active" }), session("c", { expires_at: iso(-3) })], NOW);
    expect([...next.keys()]).toEqual(["a"]);
  });
  it("accepts an empty snapshot", () => {
    expect(replaceRequests([], NOW).size).toBe(0);
  });
});

describe("expiry and server clock", () => {
  it("corrects expires_at with server_time (local clock 30 s ahead)", () => {
    // server says it is 12:00:00, expiry 12:01:00, but the browser clock reads 12:00:30
    const local = NOW + 30_000;
    const set = replaceRequests([session("a")], local);
    expect(requestSecondsLeft(set.get("a")!, local)).toBe(60);
    expect(requestSecondsLeft(set.get("a")!, local + 59_000)).toBe(1);
    expect(isLiveRequest(set.get("a")!, local + 61_000)).toBe(false);
  });

  it("prunes expired entries and keeps the same map otherwise", () => {
    const set = replaceRequests([session("a", { expires_at: iso(10) }), session("b", { expires_at: iso(60) })], NOW);
    expect(pruneExpired(set, NOW + 5_000)).toBe(set);
    expect([...pruneExpired(set, NOW + 11_000).keys()]).toEqual(["b"]);
  });

  it("reports the next expiry", () => {
    const set = replaceRequests([session("a", { expires_at: iso(10) }), session("b", { expires_at: iso(60) })], NOW);
    expect(nextExpiryMs(set, NOW)).toBe(10_000);
    expect(nextExpiryMs(EMPTY_REQUESTS, NOW)).toBeNull();
  });
});

describe("newestRequest", () => {
  it("picks the latest requested_at", () => {
    const set = replaceRequests([session("a", { requested_at: iso(-30) }), session("b", { requested_at: iso(-2) }), session("c", { requested_at: iso(-10) })], NOW);
    expect(newestRequest(set)?.session.id).toBe("b");
    expect(newestRequest(EMPTY_REQUESTS)).toBeNull();
  });
});
