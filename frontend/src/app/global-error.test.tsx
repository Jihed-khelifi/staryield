// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import GlobalError from "@/app/global-error";
import { browser } from "@/lib/error-recovery";
import { logger } from "@/lib/logger";

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

/** The page renders its own <html> and <body>; React warns about that inside a test container. */
function quiet() {
  vi.spyOn(console, "error").mockImplementation(() => {});
  vi.spyOn(logger, "error").mockImplementation(() => {});
}

describe("root error page", () => {
  it("Try again reloads the page with the props vinext passes (error and reset, no retry) instead of throwing", async () => {
    quiet();
    const reload = vi.spyOn(browser, "reload").mockImplementation(() => {});
    render(<GlobalError error={new Error("root layout failed")} reset={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it("uses the framework's retry when it gives one", async () => {
    quiet();
    const reload = vi.spyOn(browser, "reload").mockImplementation(() => {});
    const retry = vi.fn();
    render(<GlobalError error={new Error("root layout failed")} retry={retry} />);
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(retry).toHaveBeenCalledTimes(1);
    expect(reload).not.toHaveBeenCalled();
  });

  it("logs the failure once with its code and digest", () => {
    quiet();
    const error = Object.assign(new Error("root layout failed"), { digest: "abc" });
    render(<GlobalError error={error} reset={vi.fn()} />);
    expect(logger.error).toHaveBeenCalledWith("root layout failed: root layout failed", error, { code: "root_failed", digest: "abc" });
  });
});
