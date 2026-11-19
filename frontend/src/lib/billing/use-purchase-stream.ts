"use client";

import * as React from "react";

import { waitForPurchase, type PurchaseOutcome } from "@/lib/api/purchase-stream";
import type { Purchase } from "@/lib/api/types";

export type PurchaseStreamState =
  | { status: "idle" }
  /** Listening; `purchase` is the latest pending state, when one arrived. */
  | { status: "waiting"; purchase?: Purchase }
  | { status: "final"; purchase: Purchase }
  | { status: "timeout" }
  | { status: "error" };

/**
 * Follows one purchase over the stream while mounted (revisions 6.0, §26.5).
 * `purchaseId` null is idle. Changing the id or unmounting aborts the stream
 * (and closes its EventSource); a final state, the timeout and an error each
 * end it. `retry()` listens again after an error or timeout.
 */
export function usePurchaseStream(purchaseId: string | null): PurchaseStreamState & { retry: () => void } {
  const [attempt, setAttempt] = React.useState(0);
  const [state, setState] = React.useState<{ key: string; value: PurchaseStreamState }>({ key: "", value: { status: "idle" } });
  const key = purchaseId ? `${purchaseId}|${attempt}` : "";

  React.useEffect(() => {
    if (!purchaseId) return;
    const controller = new AbortController();
    const apply = (value: PurchaseStreamState) => {
      if (!controller.signal.aborted) setState({ key, value });
    };
    apply({ status: "waiting" });
    void waitForPurchase(purchaseId, { signal: controller.signal, onUpdate: (purchase) => apply({ status: "waiting", purchase }) }).then(
      (outcome: PurchaseOutcome) => {
        if (outcome.kind === "final") apply({ status: "final", purchase: outcome.purchase });
        else apply({ status: outcome.kind });
      },
    );
    return () => controller.abort();
  }, [purchaseId, key]);

  const retry = React.useCallback(() => setAttempt((n) => n + 1), []);
  // A result of an older id/attempt is not shown; the effect is about to replace it.
  const value: PurchaseStreamState = !purchaseId ? { status: "idle" } : state.key === key ? state.value : { status: "waiting" };
  return { ...value, retry };
}
