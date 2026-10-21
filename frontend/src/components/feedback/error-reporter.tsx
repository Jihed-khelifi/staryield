"use client";

import * as React from "react";

import { logger } from "@/lib/logger";

/**
 * Browser notices that are not application errors: ResizeObserver reports
 * a skipped frame when an observed element (auto-growing composer, charts)
 * resizes during layout. Reporting them would only add noise.
 */
export function isBenignBrowserError(message: string | undefined): boolean {
  return /^ResizeObserver loop (limit exceeded|completed with undelivered notifications)/.test(message ?? "");
}

/** Reports uncaught errors and unhandled promise rejections. */
export function ErrorReporter() {
  React.useEffect(() => {
    const onError = (event: ErrorEvent) => {
      if (isBenignBrowserError(event.message)) return;
      logger.error(`uncaught: ${event.message}`, event.error, { code: "uncaught_error", source: event.filename ?? "", line: event.lineno ?? 0 });
    };
    const onRejection = (event: PromiseRejectionEvent) => {
      const reason = event.reason as { code?: string; message?: string } | undefined;
      // Cancelled requests are expected during navigation.
      if (reason?.code === "aborted") return;
      logger.error(`unhandled rejection: ${reason?.message ?? String(event.reason)}`, event.reason, { code: reason?.code ?? "unhandled_rejection" });
    };
    window.addEventListener("error", onError);
    window.addEventListener("unhandledrejection", onRejection);
    return () => {
      window.removeEventListener("error", onError);
      window.removeEventListener("unhandledrejection", onRejection);
    };
  }, []);
  return null;
}
