// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TRUSTPILOT_SCRIPT_URL } from "@/config/trustpilot";
import { TRUSTPILOT_TIMEOUT_MS, loadTrustpilot, resetTrustpilotLoader } from "@/lib/trustpilot";

const scripts = () => [...document.querySelectorAll<HTMLScriptElement>("script[data-trustpilot-bootstrap]")];
const stub = () => ({ loadFromElement: vi.fn() });

beforeEach(() => {
  vi.useFakeTimers();
  resetTrustpilotLoader();
  delete window.Trustpilot;
});

afterEach(() => {
  vi.unstubAllGlobals();
  resetTrustpilotLoader();
  delete window.Trustpilot;
  vi.useRealTimers();
});

describe("loadTrustpilot", () => {
  it("injects one async https script and resolves once window.Trustpilot exists", async () => {
    const promise = loadTrustpilot();
    const [script] = scripts();
    expect(scripts()).toHaveLength(1);
    expect(script.src).toBe(TRUSTPILOT_SCRIPT_URL);
    expect(script.src.startsWith("https://")).toBe(true);
    expect(script.async).toBe(true);
    const api = stub();
    window.Trustpilot = api;
    script.dispatchEvent(new Event("load"));
    await expect(promise).resolves.toBe(api);
  });

  it("is single flight: many callers share one script and one promise", async () => {
    const [a, b, c] = [loadTrustpilot(), loadTrustpilot(), loadTrustpilot()];
    expect(a).toBe(b);
    expect(b).toBe(c);
    expect(scripts()).toHaveLength(1);
    window.Trustpilot = stub();
    scripts()[0].dispatchEvent(new Event("load"));
    await expect(Promise.all([a, b, c])).resolves.toHaveLength(3);
  });

  it("resolves at once, without a script, when Trustpilot is already there", async () => {
    const api = stub();
    window.Trustpilot = api;
    await expect(loadTrustpilot()).resolves.toBe(api);
    expect(scripts()).toHaveLength(0);
  });

  it("waits for the global when the script loads a tick before defining it", async () => {
    const promise = loadTrustpilot();
    scripts()[0].dispatchEvent(new Event("load"));
    await vi.advanceTimersByTimeAsync(120);
    const api = stub();
    window.Trustpilot = api;
    await vi.advanceTimersByTimeAsync(60);
    await expect(promise).resolves.toBe(api);
  });

  it("rejects on a script error, removes the tag and allows a retry", async () => {
    const promise = loadTrustpilot();
    const failure = expect(promise).rejects.toThrow(/failed to load/);
    scripts()[0].dispatchEvent(new Event("error"));
    await failure;
    expect(scripts()).toHaveLength(0);
    const retry = loadTrustpilot();
    expect(retry).not.toBe(promise);
    expect(scripts()).toHaveLength(1);
    window.Trustpilot = stub();
    scripts()[0].dispatchEvent(new Event("load"));
    await expect(retry).resolves.toBeTruthy();
  });

  it("rejects after 8 seconds", async () => {
    expect(TRUSTPILOT_TIMEOUT_MS).toBe(8000);
    const promise = loadTrustpilot();
    const failure = expect(promise).rejects.toThrow(/timed out/);
    await vi.advanceTimersByTimeAsync(7999);
    expect(scripts()).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(2);
    await failure;
    expect(scripts()).toHaveLength(0);
  });

  it("can be reset for tests", () => {
    const first = loadTrustpilot();
    resetTrustpilotLoader();
    expect(scripts()).toHaveLength(0);
    expect(loadTrustpilot()).not.toBe(first);
    expect(scripts()).toHaveLength(1);
  });

  it("never touches the document during server rendering", async () => {
    vi.stubGlobal("window", undefined);
    await expect(loadTrustpilot()).rejects.toThrow(/no browser/);
  });
});
