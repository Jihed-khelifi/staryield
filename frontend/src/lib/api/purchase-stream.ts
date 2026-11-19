/**
 * The purchase stream (revisions 6.0, docs/platform-extensions.md §26.5):
 * `GET /credits/purchases/:id/stream` is Server-Sent Events carrying
 * `{"type":"purchase.updated","data":PurchaseView}` and, after 120 s,
 * `{"type":"purchase.timeout"}`. The server checks the processor itself while
 * the purchase is pending, so the browser only listens (no polling) and the
 * dialog never navigates.
 *
 * An EventSource reconnects by itself when the server closes the stream, so it
 * is **closed here** on a final state, on the timeout, on abort and after a
 * client-side safety timeout. Without `EventSource` (very old browsers) the
 * purchase list is polled instead.
 */
import { API_URL } from "@/lib/api/config";
import { api, refreshSession } from "@/lib/api/client";
import type { Page, Purchase } from "@/lib/api/types";

/** The server closes the stream after 120 s; the client gives up a little later. */
export const PURCHASE_STREAM_SAFETY_MS = 130_000;
const POLL_INTERVAL_MS = 3_000;

export type PurchaseStreamMessage = { type: "purchase.updated"; data: Purchase } | { type: "purchase.timeout" };

const STATUSES: readonly string[] = ["pending", "succeeded", "failed", "refunded"];

/** A purchase that will not change any more (`succeeded`, `failed`, `refunded`). */
export function isFinalPurchase(purchase: Pick<Purchase, "status">): boolean {
  return purchase.status !== "pending";
}

/** Decode one SSE `data:` payload; null for anything that is not a known, well-formed envelope. */
export function parsePurchaseMessage(raw: string): PurchaseStreamMessage | null {
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!value || typeof value !== "object") return null;
  const envelope = value as { type?: unknown; data?: unknown };
  if (envelope.type === "purchase.timeout") return { type: "purchase.timeout" };
  if (envelope.type !== "purchase.updated") return null;
  const data = envelope.data as Partial<Purchase> | null | undefined;
  if (!data || typeof data !== "object" || typeof data.id !== "string" || typeof data.status !== "string" || !STATUSES.includes(data.status)) return null;
  return { type: "purchase.updated", data: data as Purchase };
}

export function purchaseStreamUrl(purchaseId: string): string {
  return `${API_URL}/api/v1/credits/purchases/${encodeURIComponent(purchaseId)}/stream`;
}

/** How waiting for a purchase ended. */
export type PurchaseOutcome =
  /** `succeeded`, `failed` or `refunded` (the purchase carries `wallet` / `failure_reason`). */
  | { kind: "final"; purchase: Purchase }
  /** Still pending after the server's (or the safety) timeout: it settles later by webhook. */
  | { kind: "timeout" }
  /** The stream could not be read (offline, refused): the outcome is unknown, not a failure. */
  | { kind: "error" };

export type WatchOptions = {
  /** Stops listening (unmount): nothing is reported afterwards. */
  signal?: AbortSignal;
  /** Each non-final state (pending) as it arrives. */
  onUpdate?: (purchase: Purchase) => void;
  safetyTimeoutMs?: number;
  pollIntervalMs?: number;
};

/** Listen to a purchase until it settles. Resolves once, never rejects; closes the EventSource on every exit. */
export function waitForPurchase(purchaseId: string, options: WatchOptions = {}): Promise<PurchaseOutcome> {
  const { signal, onUpdate, safetyTimeoutMs = PURCHASE_STREAM_SAFETY_MS, pollIntervalMs = POLL_INTERVAL_MS } = options;
  return new Promise<PurchaseOutcome>((resolve) => {
    let source: EventSource | null = null;
    let pollTimer: ReturnType<typeof setTimeout> | null = null;
    let reconnected = false;
    let done = false;

    const safety = setTimeout(() => finish({ kind: "timeout" }), safetyTimeoutMs);

    function cleanup() {
      clearTimeout(safety);
      if (pollTimer) clearTimeout(pollTimer);
      pollTimer = null;
      source?.close();
      source = null;
      signal?.removeEventListener("abort", onAbort);
    }
    function finish(outcome: PurchaseOutcome) {
      if (done) return;
      done = true;
      cleanup();
      resolve(outcome);
    }
    function onAbort() {
      // Unmounted: stop quietly. The promise resolves so no caller hangs.
      finish({ kind: "error" });
    }
    function handle(purchase: Purchase) {
      if (done) return; // a frame that was already in flight when the stream closed
      if (isFinalPurchase(purchase)) finish({ kind: "final", purchase });
      else onUpdate?.(purchase);
    }

    function poll() {
      if (done) return;
      api<Page<Purchase>>("/api/v1/credits/purchases", { query: { limit: 20 }, signal })
        .then((page) => {
          const purchase = page.items.find((item) => item.id === purchaseId);
          if (purchase) handle(purchase);
        })
        .catch(() => undefined) // a missed poll is retried until the safety timeout
        .finally(() => {
          if (!done) pollTimer = setTimeout(poll, pollIntervalMs);
        });
    }

    function connect() {
      const es = new EventSource(purchaseStreamUrl(purchaseId), { withCredentials: true });
      source = es;
      es.onmessage = (event: MessageEvent<string>) => {
        const message = parsePurchaseMessage(event.data);
        if (!message) return;
        if (message.type === "purchase.timeout") finish({ kind: "timeout" });
        else handle(message.data);
      };
      es.onerror = () => {
        if (done || source !== es) return;
        // CONNECTING: the browser is reconnecting (Retry: 3000) by itself.
        if (es.readyState !== EventSource.CLOSED) return;
        es.close();
        source = null;
        if (reconnected) {
          finish({ kind: "error" });
          return;
        }
        // Refused: most likely an expired access token. Refresh once and try again.
        reconnected = true;
        void refreshSession().then((refreshed) => {
          if (done) return;
          if (refreshed) connect();
          else finish({ kind: "error" });
        });
      };
    }

    if (signal?.aborted) {
      finish({ kind: "error" });
      return;
    }
    signal?.addEventListener("abort", onAbort, { once: true });
    if (typeof EventSource === "undefined") poll();
    else connect();
  });
}
