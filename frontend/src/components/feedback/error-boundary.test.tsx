// @vitest-environment jsdom
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { notFound } from "next/navigation";

import { ErrorBoundary } from "@/components/feedback/error-boundary";
import { logger } from "@/lib/logger";

function Boom({ error }: { error: unknown }): React.ReactNode {
  throw error;
}

/** Stands in for the router's own boundaries above ours. */
class Outer extends React.Component<{ children: React.ReactNode }, { caught: Error | null }> {
  state = { caught: null as Error | null };
  static getDerivedStateFromError(caught: Error) {
    return { caught };
  }
  render() {
    return this.state.caught ? <p data-testid="outer">{this.state.caught.message}</p> : this.props.children;
  }
}

afterEach(cleanup);

describe("ErrorBoundary", () => {
  it("renders its children while they render", () => {
    render(
      <ErrorBoundary code="widget_failed" fallback={<p>fallback</p>}>
        <p>content</p>
      </ErrorBoundary>,
    );
    expect(screen.getByText("content")).toBeTruthy();
    expect(screen.queryByText("fallback")).toBeNull();
  });

  it("shows the fallback in place of children that threw, keeps the rest of the page, and logs the error once with its code", () => {
    const logError = vi.spyOn(logger, "error").mockImplementation(() => {});
    vi.spyOn(console, "error").mockImplementation(() => {}); // React reports the caught error too.
    const error = new Error("widget exploded");
    render(
      <div>
        <p>sibling</p>
        <ErrorBoundary code="widget_failed" fallback={<p>fallback</p>}>
          <Boom error={error} />
        </ErrorBoundary>
      </div>,
    );
    expect(screen.getByText("fallback")).toBeTruthy();
    expect(screen.getByText("sibling")).toBeTruthy();
    expect(logError).toHaveBeenCalledTimes(1);
    expect(logError).toHaveBeenCalledWith("widget_failed: widget exploded", error, expect.objectContaining({ code: "widget_failed" }));
    // Where it failed, under a key the logger ships to the API (snake_case only).
    expect(logError.mock.calls[0][2]?.component_stack).toContain("Boom");
  });

  it("lets the router's own signals (notFound, redirect) through to the boundaries that handle them", () => {
    const logError = vi.spyOn(logger, "error").mockImplementation(() => {});
    vi.spyOn(console, "error").mockImplementation(() => {});
    let signal: unknown;
    try {
      notFound();
    } catch (thrown) {
      signal = thrown;
    }
    expect(signal).toBeInstanceOf(Error);
    render(
      <Outer>
        <ErrorBoundary code="widget_failed" fallback={<p>fallback</p>}>
          <Boom error={signal} />
        </ErrorBoundary>
      </Outer>,
    );
    expect(screen.queryByText("fallback")).toBeNull();
    expect(screen.getByTestId("outer")).toBeTruthy();
    expect(logError).not.toHaveBeenCalled();
  });
});
