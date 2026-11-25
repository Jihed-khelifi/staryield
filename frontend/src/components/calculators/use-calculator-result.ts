"use client";

import * as React from "react";

import { logger } from "@/lib/logger";
import { useAsync } from "@/lib/use-async";
import type { CalculatorFormValues, CalculatorKind } from "@/lib/schemas/calculator";
import { cachedResult, calculate, rememberResult, type CalculatorResult } from "@/components/calculators/calculate";

/**
 * The result of a calculator for `values`, cached by `key` (the canonical
 * fragment): going back and forth never repeats a request. While a new key is
 * being calculated the previous result stays available (`pending`), which is
 * how the natal chart stays on screen while its house system is recomputed.
 */
export function useCalculatorResult(kind: CalculatorKind, values: CalculatorFormValues, key: string) {
  const cached = cachedResult(key);
  const state = useAsync(
    async (signal) => {
      const result = await calculate(kind, values, signal);
      try {
        rememberResult(key, result);
      } catch (error) {
        // The in-memory cache is an optimisation: never turn a good API
        // answer into an error screen.
        logger.warn("calculator result cache failed", error);
      }
      return result;
    },
    [key],
    !cached,
  );
  const error = cached ? null : state.error;
  const fresh: CalculatorResult | null = cached ?? (!state.loading && !state.error ? state.data : null);
  // The last result that belonged to this view (state adjusted during render).
  const [previous, setPrevious] = React.useState<CalculatorResult | null>(null);
  if (fresh && fresh !== previous) setPrevious(fresh);
  const result = fresh ?? previous;
  return { result, error, pending: !fresh && !error && Boolean(previous), reload: state.reload };
}
