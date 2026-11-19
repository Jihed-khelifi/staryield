import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FakeEventSource, installFakeEventSource } from "@/test/event-source";

const m = vi.hoisted(() => ({ api: vi.fn(), refresh: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: m.api, refreshSession: m.refresh }));

import { isFinalPurchase, parsePurchaseMessage, purchaseStreamUrl, waitForPurchase } from "@/lib/api/purchase-stream";
import type { Purchase } from "@/lib/api/types";

const purchase = (over: Partial<Purchase> = {}): Purchase => ({
  id: "pu_12345678", pack_id: "p", credits: 600, price_cents: 1000, currency: "USD", status: "pending", provider: "stripe", created_at: "2026-11-19T10:00:00Z", ...over,
});
const updated = (p: Purchase) => ({ type: "purchase.updated", data: p });

beforeEach(() => {
  vi.useFakeTimers();
  installFakeEventSource();
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("parsePurchaseMessage", () => {
  it("reads a purchase.updated envelope", () => {
    const msg = parsePurchaseMessage(JSON.stringify(updated(purchase({ status: "succeeded" }))));
    expect(msg?.type).toBe("purchase.updated");
    expect(msg && "data" in msg && msg.data.status).toBe("succeeded");
  });
  it("reads purchase.timeout", () => {
    expect(parsePurchaseMessage('{"type":"purchase.timeout"}')).toEqual({ type: "purchase.timeout" });
  });
  it.each([
    ["not json", "nope"],
    ["null", "null"],
    ["unknown type", '{"type":"hello"}'],
    ["no data", '{"type":"purchase.updated"}'],
    ["unknown status", '{"type":"purchase.updated","data":{"id":"x","status":"weird"}}'],
    ["no id", '{"type":"purchase.updated","data":{"status":"pending"}}'],
  ])("ignores %s", (_name, raw) => {
    expect(parsePurchaseMessage(raw)).toBeNull();
  });
  it("knows the final states", () => {
    expect(isFinalPurchase({ status: "pending" })).toBe(false);
    for (const status of ["succeeded", "failed", "refunded"] as const) expect(isFinalPurchase({ status })).toBe(true);
  });
  it("targets the same API origin with the id encoded", () => {
    expect(purchaseStreamUrl("a b")).toMatch(/\/api\/v1\/credits\/purchases\/a%20b\/stream$/);
  });
});

describe("waitForPurchase", () => {
  it("opens the stream with cookies and resolves on a final state, closing the EventSource", async () => {
    const onUpdate = vi.fn();
    const outcome = waitForPurchase("pu_12345678", { onUpdate });
    const es = FakeEventSource.last;
    expect(es.url).toContain("/api/v1/credits/purchases/pu_12345678/stream");
    expect(es.init?.withCredentials).toBe(true);
    es.emit(updated(purchase()));
    expect(onUpdate).toHaveBeenCalledOnce();
    expect(es.closed).toBe(false);
    es.emit(updated(purchase({ status: "succeeded", wallet: { balance: 700, held: 0, free_minutes: 0 } })));
    await expect(outcome).resolves.toMatchObject({ kind: "final", purchase: { status: "succeeded" } });
    expect(es.closed).toBe(true);
  });

  it("reports a failed purchase as final with its reason", async () => {
    const outcome = waitForPurchase("pu_12345678");
    FakeEventSource.last.emit(updated(purchase({ status: "failed", failure_reason: "authentication_failed" })));
    await expect(outcome).resolves.toMatchObject({ kind: "final", purchase: { failure_reason: "authentication_failed" } });
  });

  it("closes and resolves timeout on purchase.timeout", async () => {
    const outcome = waitForPurchase("pu_12345678");
    FakeEventSource.last.emit({ type: "purchase.timeout" });
    await expect(outcome).resolves.toEqual({ kind: "timeout" });
    expect(FakeEventSource.last.closed).toBe(true);
  });

  it("ignores malformed frames and keeps listening", async () => {
    const outcome = waitForPurchase("pu_12345678");
    FakeEventSource.last.emit("garbage");
    FakeEventSource.last.emit({ type: "hello" });
    expect(FakeEventSource.last.closed).toBe(false);
    FakeEventSource.last.emit(updated(purchase({ status: "succeeded" })));
    await expect(outcome).resolves.toMatchObject({ kind: "final" });
  });

  it("lets the browser reconnect while it is only reconnecting", async () => {
    const outcome = waitForPurchase("pu_12345678");
    const es = FakeEventSource.last;
    es.fail(false);
    expect(es.closed).toBe(false);
    es.emit(updated(purchase({ status: "succeeded" })));
    await expect(outcome).resolves.toMatchObject({ kind: "final" });
  });

  it("refreshes the session once when refused, then reconnects", async () => {
    m.refresh.mockResolvedValue(true);
    const outcome = waitForPurchase("pu_12345678");
    FakeEventSource.last.fail(true);
    await vi.advanceTimersByTimeAsync(0);
    expect(m.refresh).toHaveBeenCalledOnce();
    expect(FakeEventSource.instances).toHaveLength(2);
    FakeEventSource.last.emit(updated(purchase({ status: "succeeded" })));
    await expect(outcome).resolves.toMatchObject({ kind: "final" });
  });

  it("gives up with error when refused again or the session cannot be refreshed", async () => {
    m.refresh.mockResolvedValue(true);
    const again = waitForPurchase("pu_12345678");
    FakeEventSource.last.fail(true);
    await vi.advanceTimersByTimeAsync(0);
    FakeEventSource.last.fail(true);
    await expect(again).resolves.toEqual({ kind: "error" });
    expect(FakeEventSource.instances.every((es) => es.closed)).toBe(true);

    m.refresh.mockResolvedValue(false);
    const refused = waitForPurchase("pu_12345678");
    FakeEventSource.last.fail(true);
    await expect(refused).resolves.toEqual({ kind: "error" });
  });

  it("resolves timeout after the client-side safety timeout (130 s) and closes", async () => {
    const outcome = waitForPurchase("pu_12345678");
    await vi.advanceTimersByTimeAsync(129_000);
    expect(FakeEventSource.last.closed).toBe(false);
    await vi.advanceTimersByTimeAsync(1_500);
    await expect(outcome).resolves.toEqual({ kind: "timeout" });
    expect(FakeEventSource.last.closed).toBe(true);
  });

  it("closes the EventSource when aborted (unmount) and reports nothing more", async () => {
    const controller = new AbortController();
    const onUpdate = vi.fn();
    const outcome = waitForPurchase("pu_12345678", { signal: controller.signal, onUpdate });
    const es = FakeEventSource.last;
    controller.abort();
    await outcome;
    expect(es.closed).toBe(true);
    es.emit(updated(purchase()));
    expect(onUpdate).not.toHaveBeenCalled();
  });

  it("does not even open the stream when already aborted", async () => {
    const controller = new AbortController();
    controller.abort();
    await expect(waitForPurchase("pu_12345678", { signal: controller.signal })).resolves.toEqual({ kind: "error" });
    expect(FakeEventSource.instances).toHaveLength(0);
  });
});

describe("without EventSource", () => {
  it("polls the purchase list until the purchase settles", async () => {
    vi.stubGlobal("EventSource", undefined);
    m.api
      .mockResolvedValueOnce({ items: [purchase()] })
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce({ items: [purchase({ id: "other", status: "failed" }), purchase({ status: "succeeded" })] });
    const outcome = waitForPurchase("pu_12345678", { pollIntervalMs: 1000 });
    await vi.advanceTimersByTimeAsync(0);
    expect(m.api).toHaveBeenCalledWith("/api/v1/credits/purchases", expect.objectContaining({ query: { limit: 20 } }));
    await vi.advanceTimersByTimeAsync(1000);
    await vi.advanceTimersByTimeAsync(1000);
    await expect(outcome).resolves.toMatchObject({ kind: "final", purchase: { status: "succeeded" } });
    const calls = m.api.mock.calls.length;
    await vi.advanceTimersByTimeAsync(5000);
    expect(m.api).toHaveBeenCalledTimes(calls); // stopped polling
    expect(FakeEventSource.instances).toHaveLength(0);
  });

  it("stops polling at the safety timeout", async () => {
    vi.stubGlobal("EventSource", undefined);
    m.api.mockResolvedValue({ items: [purchase()] });
    const outcome = waitForPurchase("pu_12345678", { pollIntervalMs: 1000, safetyTimeoutMs: 3500 });
    await vi.advanceTimersByTimeAsync(4000);
    await expect(outcome).resolves.toEqual({ kind: "timeout" });
  });
});
