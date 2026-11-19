"use client";

import * as React from "react";
import { useRouter } from "next/navigation";

/**
 * What an error page (`error.tsx`, `global-error.tsx`) is given. vinext 1.0
 * passes `error` and `reset` only (clear the boundary, render it again);
 * Next.js 16.3 also passes `retry` (refresh the route first). Both are
 * optional here so neither is ever called blindly: calling the missing
 * `retry` turned "Try again" into "TypeError: t is not a function".
 */
export type ErrorPageProps = {
  error: Error & { digest?: string };
  reset?: () => void;
  retry?: () => void;
};

/** Wrapper around the one real reload, so tests can observe it. */
export const browser = {
  reload: () => window.location.reload(),
};

/**
 * "Try again" on a route's error page: the framework's `retry` when it gives
 * one, else the same thing by hand (refresh the route's server data, then
 * render the segment again). With neither, a reload: never a dead button.
 */
export function useRouteRetry({ retry, reset }: Pick<ErrorPageProps, "retry" | "reset">): () => void {
  const router = useRouter();
  return React.useCallback(() => {
    if (typeof retry === "function") return retry();
    if (typeof reset !== "function") return browser.reload();
    React.startTransition(() => {
      router.refresh();
      reset();
    });
  }, [retry, reset, router]);
}

/**
 * "Try again" on the root error page. The root layout itself failed, so a
 * full reload (fresh HTML, scripts and data) is the dependable recovery,
 * unless the framework offers `retry`.
 */
export function retryRoot(retry?: () => void) {
  if (typeof retry === "function") retry();
  else browser.reload();
}
