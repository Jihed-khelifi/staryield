"use client";

import * as React from "react";

import { chatApi } from "@/lib/api/endpoints";
import { toApiError, type ApiError } from "@/lib/api/errors";
import type { OffSessionAllowance } from "@/lib/api/types";
import { useRealtimeEvent, useRealtimeEvents } from "@/lib/auth/session-provider";

export type AllowanceState = {
  /** What the viewer may still send outside a live chat (null until known). */
  allowance: OffSessionAllowance | null;
  loading: boolean;
  error: ApiError | null;
  /** Fetch again (the retry action of the error state). */
  reload: () => void;
  /** Adopt the allowance a send response carried (no request needed). */
  update: (allowance: OffSessionAllowance | undefined) => void;
};

type Result = { conv: string; allowance: OffSessionAllowance | null; error: ApiError | null };

/** Session events after which the allowance may have changed (§22.3). */
const REFRESH_ON = ["session.started", "session.ended"] as const;

/**
 * The viewer's off-session allowance for the conversation on screen. It is
 * fetched while there is no live chat, refreshed from every send response
 * (`update`), and fetched again after a live chat starts or ends and when the
 * realtime connection comes back. `error` carries a failed fetch (`reload`
 * retries); while the last known value is kept so the composer does not flip
 * to "disabled" on a hiccup.
 */
export function useOffSessionAllowance(conversationId: string | null, live: boolean): AllowanceState {
  const [result, setResult] = React.useState<Result | null>(null);
  const [nonce, setNonce] = React.useState(0);
  const enabled = Boolean(conversationId) && !live;

  React.useEffect(() => {
    if (!enabled || !conversationId) return;
    const ctrl = new AbortController();
    chatApi.allowance(conversationId, ctrl.signal).then(
      (allowance) => {
        if (!ctrl.signal.aborted) setResult({ conv: conversationId, allowance, error: null });
      },
      (err) => {
        const error = toApiError(err);
        if (ctrl.signal.aborted || error.code === "aborted") return;
        setResult((r) => ({ conv: conversationId, allowance: r?.conv === conversationId ? r.allowance : null, error }));
      },
    );
    return () => ctrl.abort();
  }, [enabled, conversationId, nonce]);

  const reload = React.useCallback(() => setNonce((n) => n + 1), []);
  useRealtimeEvents(REFRESH_ON, reload);
  useRealtimeEvent<{ reconnected?: boolean }>("hello", (d) => {
    if (d?.reconnected) reload();
  });

  const update = React.useCallback(
    (allowance: OffSessionAllowance | undefined) => {
      if (allowance && conversationId) setResult({ conv: conversationId, allowance, error: null });
    },
    [conversationId],
  );

  const mine = result && result.conv === conversationId ? result : null;
  return {
    allowance: mine?.allowance ?? null,
    error: enabled ? (mine?.error ?? null) : null,
    // Loading until the first answer for this conversation (a failed fetch is an error, not loading).
    loading: enabled && !mine,
    reload,
    update,
  };
}
