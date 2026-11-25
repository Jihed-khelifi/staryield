// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/analytics", () => ({ track: vi.fn(), analyticsAnonymousId: () => undefined }));

import { attributionForSignup } from "@/lib/attribution";
import {
  REFERRAL_KEY,
  REFERRAL_STORED_EVENT,
  REFERRAL_TTL_MS,
  captureReferral,
  forgetReferral,
  hasStoredReferral,
  loadReferral,
  normalizeReferralCode,
  referralCodeForSignup,
  referralProgramOf,
  stripReferralParam,
  type ReferralEnv,
} from "@/lib/referral";
import type { SiteConfig } from "@/lib/api/types";

const NOW = Date.parse("2026-11-20T12:00:00Z");

function env(href: string, extra: Partial<ReferralEnv> = {}) {
  return { href, storage: localStorage, now: NOW, replaceUrl: vi.fn(), track: vi.fn(), ...extra } as ReferralEnv & {
    replaceUrl: ReturnType<typeof vi.fn>;
    track: ReturnType<typeof vi.fn>;
  };
}

beforeEach(() => {
  localStorage.clear();
});

describe("normalizeReferralCode", () => {
  it("accepts the unambiguous alphabet, any case, trimmed", () => {
    expect(normalizeReferralCode(" abcd2345 ")).toBe("ABCD2345");
    expect(normalizeReferralCode("XYZ23456")).toBe("XYZ23456");
  });
  it.each([["ABCD234"], ["ABCD23456"], ["ABCD2340"], ["OBCD2345"], ["IBCD2345"], ["LBCD2345"], ["UBCD2345"], ["ABCD-234"], [""], [null], [undefined]])(
    "rejects %s",
    (raw) => {
      expect(normalizeReferralCode(raw as string)).toBeNull();
    },
  );
});

describe("captureReferral", () => {
  it("keeps a valid code, strips ?ref= and records the landing without the code", () => {
    const e = env("https://staryield.net/en/calculators/natal-chart?utm_source=ig&ref=abcd2345#top");
    const onStored = vi.fn();
    window.addEventListener(REFERRAL_STORED_EVENT, onStored);
    expect(captureReferral(e)).toEqual({ landed: true, stored: true });
    window.removeEventListener(REFERRAL_STORED_EVENT, onStored);
    expect(e.replaceUrl).toHaveBeenCalledWith("/en/calculators/natal-chart?utm_source=ig#top");
    expect(JSON.parse(localStorage.getItem(REFERRAL_KEY)!)).toEqual({ code: "ABCD2345", first_seen_at: new Date(NOW).toISOString() });
    expect(e.track).toHaveBeenCalledWith("referral_landing", { stored: true });
    expect(JSON.stringify(e.track.mock.calls)).not.toContain("ABCD2345");
    expect(onStored).toHaveBeenCalledTimes(1);
  });

  it("does nothing without the parameter", () => {
    const e = env("https://staryield.net/en?utm_source=ig");
    expect(captureReferral(e)).toEqual({ landed: false, stored: false });
    expect(e.replaceUrl).not.toHaveBeenCalled();
    expect(e.track).not.toHaveBeenCalled();
  });

  it("strips a malformed code without keeping or recording it", () => {
    const e = env("https://staryield.net/en?ref=%3Cscript%3E");
    expect(captureReferral(e)).toEqual({ landed: false, stored: false });
    expect(e.replaceUrl).toHaveBeenCalledWith("/en");
    expect(localStorage.getItem(REFERRAL_KEY)).toBeNull();
    expect(e.track).not.toHaveBeenCalled();
  });

  it("keeps the first valid code for 30 days, then accepts a new one", () => {
    captureReferral(env("https://staryield.net/en?ref=AAAA2345"));
    const second = env("https://staryield.net/en?ref=BBBB2345", { now: NOW + 1000 });
    expect(captureReferral(second)).toEqual({ landed: true, stored: false });
    expect(second.track).toHaveBeenCalledWith("referral_landing", { stored: false });
    expect(loadReferral(localStorage, NOW + 1000)?.code).toBe("AAAA2345");
    const later = NOW + REFERRAL_TTL_MS + 1;
    expect(captureReferral(env("https://staryield.net/en?ref=CCCC2345", { now: later }))).toEqual({ landed: true, stored: true });
    expect(loadReferral(localStorage, later)?.code).toBe("CCCC2345");
  });

  it("survives blocked storage and a broken URL", () => {
    const blocked = {
      getItem: () => {
        throw new Error("blocked");
      },
      setItem: () => {
        throw new Error("blocked");
      },
      removeItem: () => {
        throw new Error("blocked");
      },
    };
    expect(captureReferral(env("https://staryield.net/en?ref=ABCD2345", { storage: blocked }))).toEqual({ landed: true, stored: false });
    expect(captureReferral(env("https://staryield.net/en?ref=ABCD2345", { storage: null }))).toEqual({ landed: true, stored: false });
    expect(captureReferral(env("not a url"))).toEqual({ landed: false, stored: false });
    expect(captureReferral(null)).toEqual({ landed: false, stored: false });
  });
});

describe("loadReferral", () => {
  it.each([
    ["not json", "{"],
    ["a bad code", JSON.stringify({ code: "nope", first_seen_at: new Date(NOW).toISOString() })],
    ["no date", JSON.stringify({ code: "ABCD2345" })],
    ["a future date", JSON.stringify({ code: "ABCD2345", first_seen_at: new Date(NOW + 3_600_000).toISOString() })],
    ["an expired entry", JSON.stringify({ code: "ABCD2345", first_seen_at: new Date(NOW - REFERRAL_TTL_MS - 1).toISOString() })],
  ])("drops %s", (_, raw) => {
    localStorage.setItem(REFERRAL_KEY, raw);
    expect(loadReferral(localStorage, NOW)).toBeNull();
    expect(localStorage.getItem(REFERRAL_KEY)).toBeNull();
  });
});

describe("sign-up", () => {
  it("sends the kept code with the attribution and forgets it after the sign-up", () => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    expect(attributionForSignup()).toBeUndefined();
    captureReferral(env("https://staryield.net/fr?ref=abcd2345"));
    expect(hasStoredReferral()).toBe(true);
    expect(referralCodeForSignup()).toBe("ABCD2345");
    expect(attributionForSignup()).toEqual({ referral_code: "ABCD2345" });
    forgetReferral();
    expect(hasStoredReferral()).toBe(false);
    expect(attributionForSignup()).toBeUndefined();
    vi.useRealTimers();
  });

  it("strips only the ref parameter", () => {
    expect(stripReferralParam("https://staryield.net/es/pricing?a=1&ref=X&b=2")).toBe("/es/pricing?a=1&b=2");
    expect(stripReferralParam("https://staryield.net/es")).toBeNull();
  });
});

describe("referralProgramOf", () => {
  const base = { banners: { rotation_seconds: 6, items: [] }, signup_offer: { enabled: true, free_minutes: 5 }, free_mode: { active: false }, generated_at: "" };
  it("is off when the configuration is missing, older or malformed", () => {
    expect(referralProgramOf(null)).toEqual({ enabled: false, reward_minutes: 0, launched: false });
    expect(referralProgramOf(base as SiteConfig).enabled).toBe(false);
    expect(referralProgramOf({ ...base, referrals: { enabled: true, reward_minutes: "5", launched: false } } as unknown as SiteConfig).enabled).toBe(false);
    expect(referralProgramOf({ ...base, referrals: { enabled: true, reward_minutes: 0, launched: false } } as SiteConfig).enabled).toBe(false);
  });
  it("reads the announced program", () => {
    expect(referralProgramOf({ ...base, referrals: { enabled: true, reward_minutes: 5, launched: true } } as SiteConfig)).toEqual({
      enabled: true,
      reward_minutes: 5,
      launched: true,
    });
  });
});
