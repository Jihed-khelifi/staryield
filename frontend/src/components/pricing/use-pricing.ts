"use client";

/**
 * The catalog for the pricing page: the server-rendered copy first, then a
 * client refetch only when the person presses Retry (the server render
 * already had its own timeout and cache). Server state (`summary`) and the
 * request's progress (`status`) are kept apart from presentation.
 */
import { useCallback, useEffect, useRef, useState } from "react";

import { pricingApi, type PricingSummary } from "@/lib/api/pricing";
import { parsePricingSummary } from "@/lib/pricing/parse";

export type PricingStatus = "ready" | "loading" | "failed";

export type PricingState = {
  summary: PricingSummary | null;
  status: PricingStatus;
  retry: () => void;
};

export function usePricing(initial: PricingSummary | null): PricingState {
  const [state, setState] = useState<{ summary: PricingSummary | null; status: PricingStatus }>({
    summary: initial,
    status: initial ? "ready" : "failed",
  });
  const inflight = useRef<AbortController | null>(null);

  useEffect(() => () => inflight.current?.abort(), []);

  const retry = useCallback(() => {
    inflight.current?.abort();
    const ctrl = new AbortController();
    inflight.current = ctrl;
    setState((current) => ({ ...current, status: "loading" }));
    pricingApi.summary(ctrl.signal).then(
      (body) => {
        if (ctrl.signal.aborted) return;
        const summary = parsePricingSummary(body);
        setState((current) => (summary ? { summary, status: "ready" } : { summary: current.summary, status: current.summary ? "ready" : "failed" }));
      },
      () => {
        if (ctrl.signal.aborted) return;
        setState((current) => ({ summary: current.summary, status: current.summary ? "ready" : "failed" }));
      },
    );
  }, []);

  return { ...state, retry };
}
