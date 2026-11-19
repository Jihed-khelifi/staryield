"use client";

import * as React from "react";
import { unstable_rethrow } from "next/navigation";

import { logger } from "@/lib/logger";

type ErrorBoundaryProps = {
  /** Shown in place of the children after they threw. */
  fallback: React.ReactNode;
  /** The log code the failure is reported under (it reaches the API's client logs). */
  code: string;
  children: React.ReactNode;
};

/**
 * Keeps a render failure inside the part of the page that failed. A widget we
 * do not control (Stripe's Elements) must never take the whole site down to
 * the root error page: it is replaced by `fallback` and the rest of the page
 * keeps working. Give it a new `key` to try again. The router's own signals
 * (`notFound()`, `redirect()`) pass through to the boundaries that handle them.
 */
export class ErrorBoundary extends React.Component<ErrorBoundaryProps, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError(error: unknown) {
    unstable_rethrow(error);
    return { failed: true };
  }

  componentDidCatch(error: unknown, info: React.ErrorInfo) {
    const { code } = this.props;
    const message = error instanceof Error ? error.message : String(error);
    // snake_case: the logger only ships such context keys to the API (the top of the stack, truncated).
    logger.error(`${code}: ${message}`, error, { code, component_stack: info.componentStack ?? "" });
  }

  render() {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}
