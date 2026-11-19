// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { renderWithI18n } from "@/test/render";

const router = vi.hoisted(() => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

import RouteError from "@/app/[lang]/error";
import { logger } from "@/lib/logger";

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("route error page", () => {
  it("Try again works with the props vinext passes (error and reset, no retry): the route is refreshed, then rendered again", async () => {
    vi.spyOn(logger, "error").mockImplementation(() => {});
    const reset = vi.fn();
    renderWithI18n(<RouteError error={new Error("boom")} reset={reset} />);
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(router.refresh).toHaveBeenCalledTimes(1);
    expect(reset).toHaveBeenCalledTimes(1);
  });

  it("uses the framework's retry when it gives one", async () => {
    vi.spyOn(logger, "error").mockImplementation(() => {});
    const retry = vi.fn();
    const reset = vi.fn();
    renderWithI18n(<RouteError error={new Error("boom")} retry={retry} reset={reset} />);
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(retry).toHaveBeenCalledTimes(1);
    expect(reset).not.toHaveBeenCalled();
  });

  it("logs the failure with its digest and shows the reference", () => {
    const logError = vi.spyOn(logger, "error").mockImplementation(() => {});
    const error = Object.assign(new Error("boom"), { digest: "d1g3st" });
    renderWithI18n(<RouteError error={error} reset={vi.fn()} />);
    expect(logError).toHaveBeenCalledWith("render failed: boom", error, expect.objectContaining({ code: "render_failed", digest: "d1g3st" }));
    expect(screen.getByText("Reference: d1g3st")).toBeTruthy();
  });
});
