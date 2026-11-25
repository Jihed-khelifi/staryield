import { describe, expect, it } from "vitest";

import { ApiError } from "@/lib/api/errors";
import { availableAtFrom, COOLDOWN_CAP_SECONDS, secondsUntil, stateFromError, stateFromSent, waitingUntil } from "@/lib/verification/resend";

const NOW = Date.parse("2026-11-05T10:00:00Z");
const err = (code: string, status: number, retryAfter?: number) => new ApiError({ code, message: code, status, retryAfter });

describe("cooldown arithmetic", () => {
  it("rounds the remaining seconds up and never goes negative", () => {
    expect(secondsUntil(NOW + 59_001, NOW)).toBe(60);
    expect(secondsUntil(NOW + 1, NOW)).toBe(1);
    expect(secondsUntil(NOW, NOW)).toBe(0);
    expect(secondsUntil(NOW - 5000, NOW)).toBe(0);
  });

  it("uses the server's time, capped so a wrong clock cannot lock the button", () => {
    expect(availableAtFrom({ resend_available_at: "2026-11-05T10:01:00Z" }, NOW)).toBe(NOW + 60_000);
    expect(availableAtFrom({ resend_available_at: "2026-11-05T09:00:00Z" }, NOW)).toBe(NOW);
    expect(availableAtFrom({ resend_available_at: "2027-01-01T00:00:00Z" }, NOW)).toBe(NOW + COOLDOWN_CAP_SECONDS * 1000);
    expect(availableAtFrom({ resend_available_at: "nonsense" }, NOW)).toBe(NOW + 60_000);
  });

  it("keeps the masked address of a sent link", () => {
    expect(stateFromSent({ status: "sent", email: "m***@example.com", resend_available_at: "2026-11-05T10:01:00Z" }, NOW)).toEqual({
      phase: "sent",
      email: "m***@example.com",
      availableAt: NOW + 60_000,
    });
  });
});

describe("error mapping", () => {
  it("treats 409 already_verified as verified", () => {
    expect(stateFromError(err("already_verified", 409), NOW)).toEqual({ phase: "verified" });
  });

  it("maps a 429 inside the cooldown to a countdown from Retry-After", () => {
    expect(stateFromError(err("rate_limited", 429, 42), NOW)).toEqual({ phase: "cooldown", availableAt: NOW + 42_000 });
    expect(stateFromError(err("rate_limited", 429), NOW)).toEqual({ phase: "cooldown", availableAt: NOW + 60_000 });
  });

  it("maps an exhausted hourly budget to the limited state", () => {
    expect(stateFromError(err("rate_limited", 429, 3000), NOW)).toEqual({ phase: "limited", availableAt: NOW + 3_000_000 });
  });

  it("keeps any other failure retryable with its error", () => {
    const state = stateFromError(err("network_error", 0), NOW);
    expect(state.phase).toBe("error");
    expect(stateFromError(err("internal_error", 500), NOW).phase).toBe("error");
    expect(stateFromError(err("unauthenticated", 401), NOW).phase).toBe("error");
  });

  it("knows which states are waiting", () => {
    expect(waitingUntil({ phase: "cooldown", availableAt: 5 })).toBe(5);
    expect(waitingUntil({ phase: "idle" })).toBeNull();
    expect(waitingUntil({ phase: "verified" })).toBeNull();
  });
});
