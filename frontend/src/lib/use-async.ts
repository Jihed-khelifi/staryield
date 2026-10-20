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

type Result<T> = { key: string; data: T | null; error: ApiError | null };

/**
 * Load data with cancellation, error capture and manual reload. Aborted
 * requests (unmount, dependency change) never update state. `loading` is
 * derived (the latest result belongs to an older key), so the effect only
 * sets state from async callbacks.
 */
export function useAsync<T>(load: (signal: AbortSignal) => Promise<T>, deps: unknown[], enabled = true): AsyncState<T> {
  const [nonce, setNonce] = useState(0);
  const [result, setResult] = useState<Result<T>>({ key: "", data: null, error: null });
  const loader = useRef(load);
  useEffect(() => {
    loader.current = load;
  });
  const key = `${deps.map(String).join("\u0001")}|${nonce}`;

  useEffect(() => {
    if (!enabled) return;
    const ctrl = new AbortController();
    loader.current(ctrl.signal).then(
      (data) => {
        if (!ctrl.signal.aborted) setResult({ key, data, error: null });
      },
      (err) => {
        const e = toApiError(err);
        if (!ctrl.signal.aborted && e.code !== "aborted") setResult((r) => ({ key, data: r.data, error: e }));
      },
    );
    return () => ctrl.abort();
  }, [key, enabled]);

  const reload = useCallback(() => setNonce((n) => n + 1), []);
  const setData = useCallback((data: T | null) => setResult((r) => ({ ...r, data })), []);
  const current = result.key === key;
  return {
    data: result.data,
    error: current ? result.error : null,
    loading: enabled && !current,
    reload,
    setData,
  };
}
