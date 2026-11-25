// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";

import { BOOKING_INTENT_TTL_MS, rememberBookingIntent, takeBookingIntent } from "@/lib/booking/booking-intent";

const KEY = "staryield.booking.intent";

afterEach(() => {
  sessionStorage.clear();
  vi.restoreAllMocks();
});

describe("booking intent", () => {
  it("is taken once, for the advisor it was made for", () => {
    rememberBookingIntent("solomon", 1_000);
    expect(takeBookingIntent("theo", 2_000)).toBe(false); // another advisor's page leaves it alone
    expect(takeBookingIntent("solomon", 2_000)).toBe(true);
    expect(takeBookingIntent("solomon", 2_000)).toBe(false);
    expect(sessionStorage.getItem(KEY)).toBeNull();
  });

  it("expires, and a stale one is dropped", () => {
    rememberBookingIntent("solomon", 0);
    expect(takeBookingIntent("solomon", BOOKING_INTENT_TTL_MS + 1)).toBe(false);
    expect(sessionStorage.getItem(KEY)).toBeNull();
    rememberBookingIntent("solomon", 0);
    expect(takeBookingIntent("solomon", BOOKING_INTENT_TTL_MS)).toBe(true);
  });

  it("drops anything malformed or from the future", () => {
    for (const raw of ["{", "null", "[]", JSON.stringify({ slug: "solomon", action: "chat", at: 0 }), JSON.stringify({ slug: 7, action: "join_waitlist", at: 0 }), JSON.stringify({ slug: "solomon", action: "join_waitlist", at: 5_000 })]) {
      sessionStorage.setItem(KEY, raw);
      expect(takeBookingIntent("solomon", 1_000)).toBe(false);
      expect(sessionStorage.getItem(KEY)).toBeNull();
    }
  });

  it("only remembers advisor slugs", () => {
    rememberBookingIntent("../../me");
    rememberBookingIntent("");
    expect(sessionStorage.getItem(KEY)).toBeNull();
    rememberBookingIntent("ramone-2");
    expect(takeBookingIntent("ramone-2")).toBe(true);
  });

  it("works without storage (blocked or full)", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("QuotaExceededError");
    });
    expect(() => rememberBookingIntent("solomon")).not.toThrow();
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("SecurityError");
    });
    expect(takeBookingIntent("solomon")).toBe(false);
  });
});
