// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";

const api = vi.hoisted(() => ({ config: vi.fn() }));
vi.mock("@/lib/api/calculator-report", () => ({ reportEmailApi: api }));

import { PROMPT_QUIET_MS } from "@/components/calculators/report-email";
import { useReportEmailPrompt } from "@/components/calculators/use-report-email-prompt";

const KEY = "staryield.report-email.natal-chart";

function setScroll(share: number, scrollHeight = 3000, innerHeight = 800) {
  Object.defineProperty(document.documentElement, "scrollHeight", { configurable: true, value: scrollHeight });
  Object.defineProperty(window, "innerHeight", { configurable: true, value: innerHeight });
  Object.defineProperty(window, "scrollY", { configurable: true, value: share * (scrollHeight - innerHeight) });
  window.dispatchEvent(new Event("scroll"));
}
function setVisibility(state: "visible" | "hidden") {
  Object.defineProperty(document, "visibilityState", { configurable: true, value: state });
  document.dispatchEvent(new Event("visibilitychange"));
}
/** Let the config request settle (promises run on the microtask queue, timers are faked). */
async function settle() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}
const advance = (ms: number) => act(async () => void (await vi.advanceTimersByTimeAsync(ms)));

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-11-19T10:00:00Z"));
  window.localStorage.clear();
  api.config.mockReset();
  api.config.mockResolvedValue({ enabled: true, marketing_prechecked: false });
  setVisibility("visible");
  setScroll(0);
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("useReportEmailPrompt: when the dialog opens by itself", () => {
  it("opens after 6 seconds on screen", async () => {
    const start = Date.now();
    const { result } = renderHook(() => useReportEmailPrompt("natal-chart", true));
    await settle();
    expect(result.current.open).toBe(false);
    await advance(5900);
    expect(result.current.open).toBe(false);
    await advance(200);
    expect(result.current.open).toBe(true);
    expect(result.current.manual).toBe(false);
    const shownAt = Number(window.localStorage.getItem(KEY));
    expect(shownAt).toBeGreaterThanOrEqual(start + 6000);
    expect(shownAt).toBeLessThanOrEqual(Date.now());
  });

  it("opens at once when 60 % of the page was scrolled, and only then", async () => {
    const { result } = renderHook(() => useReportEmailPrompt("natal-chart", true));
    await settle();
    act(() => setScroll(0.5));
    expect(result.current.open).toBe(false);
    act(() => setScroll(0.6));
    expect(result.current.open).toBe(true);
    // the timer no longer fires a second time
    await advance(10_000);
    expect(result.current.open).toBe(true);
  });

  it("counts only the time the page is visible", async () => {
    const { result } = renderHook(() => useReportEmailPrompt("natal-chart", true));
    await settle();
    await advance(4000);
    act(() => setVisibility("hidden"));
    await advance(60_000);
    expect(result.current.open).toBe(false);
    act(() => setVisibility("visible"));
    await advance(1900);
    expect(result.current.open).toBe(false);
    await advance(200);
    expect(result.current.open).toBe(true);
  });

  it("does not start counting while the tab is hidden", async () => {
    setVisibility("hidden");
    const { result } = renderHook(() => useReportEmailPrompt("natal-chart", true));
    await settle();
    await advance(30_000);
    expect(result.current.open).toBe(false);
    act(() => setVisibility("visible"));
    await advance(6100);
    expect(result.current.open).toBe(true);
  });

  it("never opens on the sample route (inactive) and does not even ask for the config", async () => {
    const { result } = renderHook(() => useReportEmailPrompt("natal-chart", false));
    await settle();
    await advance(60_000);
    act(() => setScroll(0.9));
    expect(result.current.open).toBe(false);
    expect(api.config).not.toHaveBeenCalled();
  });

  it("waits for the result: it arms when the calculation is done", async () => {
    const { result, rerender } = renderHook(({ active }) => useReportEmailPrompt("natal-chart", active), { initialProps: { active: false } });
    await advance(20_000);
    expect(result.current.open).toBe(false);
    rerender({ active: true });
    await settle();
    await advance(6100);
    expect(result.current.open).toBe(true);
  });

  it("opens once per calculator per browser, and again 30 days after it was shown", async () => {
    const first = renderHook(() => useReportEmailPrompt("natal-chart", true));
    await settle();
    await advance(6100);
    expect(first.result.current.open).toBe(true);
    first.unmount();
    // a new visit: quiet
    const second = renderHook(() => useReportEmailPrompt("natal-chart", true));
    await settle();
    await advance(60_000);
    act(() => setScroll(1));
    expect(second.result.current.open).toBe(false);
    second.unmount();
    // another calculator has its own prompt
    const other = renderHook(() => useReportEmailPrompt("astrology", true));
    await settle();
    await advance(6100);
    expect(other.result.current.open).toBe(true);
    other.unmount();
    // 30 days later
    vi.setSystemTime(Date.now() + PROMPT_QUIET_MS);
    const later = renderHook(() => useReportEmailPrompt("natal-chart", true));
    await settle();
    await advance(6100);
    expect(later.result.current.open).toBe(true);
  });

  it("does not reopen after being closed, even with blocked storage", async () => {
    const getItem = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new DOMException("denied", "SecurityError");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("denied", "SecurityError");
    });
    const { result } = renderHook(() => useReportEmailPrompt("natal-chart", true));
    await settle();
    await advance(6100);
    expect(result.current.open).toBe(true);
    act(() => result.current.setOpen(false));
    await advance(60_000);
    expect(result.current.open).toBe(false);
    expect(getItem).toHaveBeenCalled();
  });
});

describe("useReportEmailPrompt: the config", () => {
  it("stays quiet and hides the button when the report is switched off", async () => {
    api.config.mockResolvedValue({ enabled: false, marketing_prechecked: true });
    const { result } = renderHook(() => useReportEmailPrompt("natal-chart", true));
    await settle();
    await advance(30_000);
    act(() => setScroll(1));
    expect(result.current.available).toBe(false);
    expect(result.current.open).toBe(false);
    expect(result.current.prechecked).toBe(false);
  });

  it("starts the marketing box ticked only where the API says so", async () => {
    api.config.mockResolvedValue({ enabled: true, marketing_prechecked: true });
    const us = renderHook(() => useReportEmailPrompt("natal-chart", true));
    await settle();
    expect(us.result.current.prechecked).toBe(true);
    expect(us.result.current.configFailed).toBe(false);
    us.unmount();
    api.config.mockResolvedValue({ enabled: true, marketing_prechecked: false });
    const fr = renderHook(() => useReportEmailPrompt("natal-chart", true));
    await settle();
    expect(fr.result.current.prechecked).toBe(false);
  });

  it("is unticked while loading and keeps working when the config cannot be read", async () => {
    api.config.mockRejectedValue(new ApiError({ code: "network_error", message: "x" }));
    const { result } = renderHook(() => useReportEmailPrompt("natal-chart", true));
    expect(result.current.prechecked).toBe(false);
    await settle();
    expect(result.current.configFailed).toBe(true);
    expect(result.current.prechecked).toBe(false);
    expect(result.current.available).toBe(true);
    await advance(6100);
    expect(result.current.open).toBe(true);
  });

  it("the button opens it any time, even after it was dismissed", async () => {
    window.localStorage.setItem(KEY, String(Date.now()));
    const { result } = renderHook(() => useReportEmailPrompt("natal-chart", true));
    await settle();
    await advance(60_000);
    expect(result.current.open).toBe(false);
    act(() => result.current.openManually());
    expect(result.current.open).toBe(true);
    expect(result.current.manual).toBe(true);
    act(() => result.current.setOpen(false));
    expect(result.current.open).toBe(false);
  });

  it("remembers a sent report like a shown dialog", async () => {
    const { result } = renderHook(() => useReportEmailPrompt("natal-chart", true));
    await settle();
    act(() => result.current.markSent());
    expect(window.localStorage.getItem(KEY)).toBe(String(Date.now()));
  });
});
