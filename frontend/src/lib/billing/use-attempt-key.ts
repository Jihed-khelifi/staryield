"use client";

import * as React from "react";

import { newId } from "@/lib/api/client";

/**
 * One idempotency key per purchase attempt (§12): double clicks and retries
 * after an unknown outcome replay the same charge instead of making a second
 * one. The key survives going back to the packs and returning to pay for the
 * same pack and card; it renews when the pack or the card changes and after a
 * decline (the next try is a new purchase).
 */
export function useAttemptKey() {
  const ref = React.useRef({ key: newId(), scope: "" });
  return React.useMemo(
    () => ({
      /** The key for this pack and card (a new key when the scope changed). */
      get(scope: string): string {
        if (ref.current.scope !== scope) ref.current = { key: newId(), scope };
        return ref.current.key;
      },
      /** The attempt is over (success or a final failure): the next one starts fresh. */
      renew() {
        ref.current = { key: newId(), scope: "" };
      },
    }),
    [],
  );
}

export type AttemptKey = ReturnType<typeof useAttemptKey>;
