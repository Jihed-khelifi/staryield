"use client";
import { useCallback, useEffect, useRef, useState } from "react";

import { toApiError, type ApiError } from "@/lib/api/errors";

/** The state returned by useAsync. */
export type AsyncState<T> = {
  data: T | null;
  error: ApiError | null;
  loading: boolean;
  reload: () => void;
  setData: (value: T | null) => void;
};

/**
 * Load data with cancellation, error capture and manual reload. Aborted
 * requests (unmount, dependency change) never update state.
 */
export function useAsync<T>(load: (signal: AbortSignal) => Promise<T>, deps: unknown[], enabled = true): AsyncState<T> {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [loading, setLoading] = useState(enabled);
  const [nonce, setNonce] = useState(0);
  const loader = useRef(load);
  useEffect(() => {
    loader.current = load;
  });

  useEffect(() => {
    if (!enabled) {
      setLoading(false);
      return;
    }
    const ctrl = new AbortController();
    setLoading(true);
    setError(null);
    loader.current(ctrl.signal).then(
      (value) => {
        if (!ctrl.signal.aborted) {
          setData(value);
          setLoading(false);
        }
      },
      (err) => {
        const e = toApiError(err);
        if (!ctrl.signal.aborted && e.code !== "aborted") {
          setError(e);
          setLoading(false);
        }
      },
    );
    return () => ctrl.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, nonce, enabled]);

  const reload = useCallback(() => setNonce((n) => n + 1), []);
  return { data, error, loading, reload, setData };
}
