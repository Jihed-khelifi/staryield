// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const api = vi.hoisted(() => vi.fn(async () => ({})));
vi.mock("@/lib/api/client", () => ({ api, newId: () => "7f0c2d1e-aaaa-4bbb-8ccc-123456789abc" }));

import { analyticsAnonymousId, analyticsOptedOut, track } from "@/lib/analytics";

function setNavigator(values: { dnt?: string | null; gpc?: boolean }) {
  Object.defineProperty(navigator, "doNotTrack", { value: values.dnt ?? null, configurable: true });
  Object.defineProperty(navigator, "globalPrivacyControl", { value: values.gpc, configurable: true });
}

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  document.cookie = "sy_consent=; max-age=0; path=/";
  setNavigator({});
  api.mockClear();
});
afterEach(() => vi.useRealTimers());

describe("analyticsOptedOut", () => {
  it("is false by default", () => expect(analyticsOptedOut()).toBe(false));
  it("honours Do Not Track", () => {
    setNavigator({ dnt: "1" });
    expect(analyticsOptedOut()).toBe(true);
  });
  it("honours Global Privacy Control", () => {
    setNavigator({ gpc: true });
    expect(analyticsOptedOut()).toBe(true);
  });
  it("honours a refusal in privacy preferences", () => {
    document.cookie = `sy_consent=${encodeURIComponent("v=2026-11&a=0&m=0&p=1")}; path=/`;
    expect(analyticsOptedOut()).toBe(true);
  });
});

describe("analyticsAnonymousId", () => {
  it("creates and reuses the browser id", () => {
    const id = analyticsAnonymousId();
    expect(id).toMatch(/^[A-Za-z0-9_-]{8,64}$/);
    expect(localStorage.getItem("staryield.aid")).toBe(id);
    expect(analyticsAnonymousId()).toBe(id);
  });
  it("ignores a malformed stored id", () => {
    localStorage.setItem("staryield.aid", "<bad id>");
    expect(analyticsAnonymousId()).toMatch(/^7f0c2d1e/);
  });
  it("returns nothing and stores nothing when opted out", () => {
    setNavigator({ dnt: "1" });
    expect(analyticsAnonymousId()).toBeUndefined();
    expect(localStorage.getItem("staryield.aid")).toBeNull();
  });
});

describe("track on calculator pages", () => {
  it("sends page_view and calculator events with kind and the anonymous id", async () => {
    vi.useFakeTimers();
    window.history.replaceState({}, "", "/fr/calculators/natal-chart");
    track("page_view");
    track("calculator_started", { kind: "natal-chart" });
    track("calculator_completed", { kind: "natal-chart" });
    await vi.advanceTimersByTimeAsync(5_000);
    expect(api).toHaveBeenCalledTimes(1);
    const body = (api.mock.calls[0] as unknown as [string, { body: { events: Array<Record<string, unknown>> } }])[1].body;
    expect(body.events.map((e) => e.name)).toEqual(["page_view", "calculator_started", "calculator_completed"]);
    expect(body.events[0].path).toBe("/fr/calculators/natal-chart");
    expect(body.events[1].properties).toEqual({ kind: "natal-chart" });
    expect(body.events[2].properties).toEqual({ kind: "natal-chart" });
    expect(body.events[0].anonymous_id).toBeTruthy();
  });

  it("sends nothing when opted out", async () => {
    vi.useFakeTimers();
    setNavigator({ gpc: true });
    track("page_view");
    await vi.advanceTimersByTimeAsync(6_000);
    expect(api).not.toHaveBeenCalled();
  });
});
