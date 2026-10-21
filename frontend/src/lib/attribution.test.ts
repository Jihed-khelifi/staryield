// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/analytics", () => ({ track: vi.fn() }));

import {
  ATTRIBUTION_KEY,
  ATTRIBUTION_TTL_MS,
  attributionForSignup,
  captureAttribution,
  loadAttribution,
  readCampaignParams,
  referrerOrigin,
  stripClickId,
  type AttributionEnv,
} from "@/lib/attribution";

const NOW = Date.parse("2026-10-20T12:00:00Z");

function env(href: string, extra: Partial<AttributionEnv> = {}): AttributionEnv & { replaceUrl: ReturnType<typeof vi.fn>; track: ReturnType<typeof vi.fn> } {
  return {
    href,
    referrer: "https://www.instagram.com/p/abc?igsh=secret",
    storage: localStorage,
    now: NOW,
    replaceUrl: vi.fn(),
    track: vi.fn(),
    ...extra,
  } as AttributionEnv & { replaceUrl: ReturnType<typeof vi.fn>; track: ReturnType<typeof vi.fn> };
}

beforeEach(() => localStorage.clear());

describe("readCampaignParams", () => {
  it("reads utm_* and the click id", () => {
    expect(readCampaignParams("?utm_source=ig&utm_medium=social&utm_campaign=spring-sale&utm_content=story&utm_term=tarot&sy_click=abc_123&x=1")).toEqual({
      utm_source: "ig",
      utm_medium: "social",
      utm_campaign: "spring-sale",
      utm_content: "story",
      utm_term: "tarot",
      click_id: "abc_123",
    });
  });

  it("returns null without campaign parameters and ignores malformed click ids", () => {
    expect(readCampaignParams("?reader=theo")).toBeNull();
    expect(readCampaignParams("?sy_click=%3Cscript%3E")).toBeNull();
  });

  it("drops control characters and truncates long values", () => {
    const v = readCampaignParams(`?utm_source=a%0Ab&utm_campaign=${"x".repeat(300)}`)!;
    expect(v.utm_source).toBe("ab");
    expect(v.utm_campaign).toHaveLength(100);
  });
});

describe("captureAttribution", () => {
  it("stores the first touch, strips only sy_click and tracks the landing", () => {
    const e = env("https://staryield.net/en/psychics?utm_source=ig&utm_campaign=spring&sy_click=c1&reader=theo#top");
    expect(captureAttribution(e)).toEqual({ landed: true, stored: true });
    expect(e.replaceUrl).toHaveBeenCalledWith("/en/psychics?utm_source=ig&utm_campaign=spring&reader=theo#top");
    expect(JSON.parse(localStorage.getItem(ATTRIBUTION_KEY)!)).toEqual({
      utm_source: "ig",
      utm_campaign: "spring",
      click_id: "c1",
      referrer: "https://www.instagram.com",
      landing_path: "/en/psychics",
      first_seen_at: new Date(NOW).toISOString(),
    });
    expect(e.track).toHaveBeenCalledWith("campaign_landing", { has_click: true, utm_source: "ig", utm_campaign: "spring" });
  });

  it("keeps the first touch for 30 days, then accepts a new one", () => {
    captureAttribution(env("https://staryield.net/en?utm_source=first"));
    const second = env("https://staryield.net/en?utm_source=second", { now: NOW + 29 * 86_400_000 });
    expect(captureAttribution(second)).toEqual({ landed: true, stored: false });
    expect(second.track).toHaveBeenCalled();
    expect(loadAttribution(localStorage, NOW + 29 * 86_400_000)?.utm_source).toBe("first");

    const later = NOW + ATTRIBUTION_TTL_MS + 1;
    expect(loadAttribution(localStorage, later)).toBeNull();
    expect(localStorage.getItem(ATTRIBUTION_KEY)).toBeNull();
    captureAttribution(env("https://staryield.net/en?utm_source=third", { now: later }));
    expect(loadAttribution(localStorage, later)?.utm_source).toBe("third");
  });

  it("does nothing on organic landings", () => {
    const e = env("https://staryield.net/en/psychics?reader=theo");
    expect(captureAttribution(e)).toEqual({ landed: false, stored: false });
    expect(e.replaceUrl).not.toHaveBeenCalled();
    expect(e.track).not.toHaveBeenCalled();
    expect(localStorage.getItem(ATTRIBUTION_KEY)).toBeNull();
  });

  it("never throws when storage is unavailable", () => {
    const broken = {
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
    const e = env("https://staryield.net/en?utm_source=ig&sy_click=c9", { storage: broken });
    expect(captureAttribution(e)).toEqual({ landed: true, stored: false });
    expect(e.replaceUrl).toHaveBeenCalledWith("/en?utm_source=ig");
    expect(captureAttribution(env("https://staryield.net/en?utm_source=ig", { storage: null }))).toEqual({ landed: true, stored: false });
  });

  it("removes malformed stored values", () => {
    localStorage.setItem(ATTRIBUTION_KEY, "{not json");
    expect(loadAttribution(localStorage, NOW)).toBeNull();
    expect(localStorage.getItem(ATTRIBUTION_KEY)).toBeNull();
  });
});

describe("helpers", () => {
  it("stripClickId keeps every other parameter", () => {
    expect(stripClickId("https://s.net/en?a=1&sy_click=x&b=2")).toBe("/en?a=1&b=2");
    expect(stripClickId("https://s.net/en?a=1")).toBeNull();
  });

  it("referrerOrigin drops same-site and non-http referrers and never keeps paths", () => {
    expect(referrerOrigin("https://t.co/xyz?q=1", "https://staryield.net")).toBe("https://t.co");
    expect(referrerOrigin("https://staryield.net/en", "https://staryield.net")).toBeUndefined();
    expect(referrerOrigin("android-app://com.google", "https://staryield.net")).toBeUndefined();
    expect(referrerOrigin("", "https://staryield.net")).toBeUndefined();
  });

  it("attributionForSignup returns the stored, unexpired touch", () => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    captureAttribution(env("https://staryield.net/en?utm_source=ig&sy_click=c1"));
    expect(attributionForSignup()).toMatchObject({ utm_source: "ig", click_id: "c1", landing_path: "/en" });
    expect(attributionForSignup(NOW + ATTRIBUTION_TTL_MS + 1)).toBeUndefined();
    vi.useRealTimers();
  });
});
