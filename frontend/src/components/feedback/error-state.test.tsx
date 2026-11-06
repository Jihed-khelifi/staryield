// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import { ErrorState, errorAction } from "@/components/feedback/error-state";
import { axeViolations, renderWithI18n } from "@/test/render";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

const err = (code: string, status = 400, extra: Partial<ConstructorParameters<typeof ApiError>[0]> = {}) =>
  new ApiError({ code, message: code, status, ...extra });

describe("errorAction", () => {
  it("maps each error to its recovery action", () => {
    expect(errorAction(err("network_error", 0), true)).toEqual({ kind: "retry" });
    expect(errorAction(err("timeout", 0), false)).toEqual({ kind: "none" });
    expect(errorAction(err("rate_limited", 429, { retryAfter: 4.2 }), true)).toEqual({ kind: "retry-after", seconds: 5 });
    expect(errorAction(err("rate_limited", 429), true)).toEqual({ kind: "retry" });
    expect(errorAction(err("session_expired", 401), true)).toEqual({ kind: "sign-in" });
    expect(errorAction(err("unauthenticated", 401), false)).toEqual({ kind: "sign-in" });
    expect(errorAction(err("insufficient_credits", 402), true)).toEqual({ kind: "top-up" });
    expect(errorAction(err("feature_unavailable", 503), true)).toEqual({ kind: "none" });
    expect(errorAction(err("not_found", 404), true)).toEqual({ kind: "home" });
    expect(errorAction(err("internal_error", 500), false)).toEqual({ kind: "support" });
    expect(errorAction(err("internal_error", 500), true)).toEqual({ kind: "retry" });
    expect(errorAction(err("conflict", 409), true)).toEqual({ kind: "retry" });
  });
});

describe("ErrorState", () => {
  it("shows the localized message and retries", async () => {
    const onRetry = vi.fn();
    renderWithI18n(<ErrorState error={err("service_unavailable", 503)} onRetry={onRetry} />);
    expect(screen.getByRole("alert").textContent).toContain("The service is temporarily unavailable");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(onRetry).toHaveBeenCalledOnce();
    expect(await axeViolations()).toEqual([]);
  });

  it("counts down Retry-After before allowing a retry", async () => {
    vi.useFakeTimers();
    renderWithI18n(<ErrorState error={err("rate_limited", 429, { retryAfter: 2 })} onRetry={vi.fn()} />);
    const button = screen.getByRole("button") as HTMLButtonElement;
    expect(button.textContent).toBe("Try again in 2s");
    expect(button.disabled).toBe(true);
    await act(async () => {
      vi.advanceTimersByTime(2000);
    });
    expect(button.textContent).toBe("Try again");
    expect(button.disabled).toBe(false);
  });

  it("offers sign-in, top-up and support links with a reference", () => {
    const { unmount } = renderWithI18n(<ErrorState error={err("session_expired", 401)} />);
    expect(screen.getByRole("link", { name: "Sign In" }).getAttribute("href")).toMatch(/^\/en\/login\?next=/);
    unmount();
    const second = renderWithI18n(<ErrorState error={err("insufficient_credits", 402)} />);
    expect(screen.getByRole("link", { name: "Top up credits" }).getAttribute("href")).toMatch(/^\/en\/credits\?next=/);
    second.unmount();
    renderWithI18n(<ErrorState error={err("internal_error", 500, { requestId: "req-123" })} />);
    expect(screen.getByRole("link", { name: "Contact support" }).getAttribute("href")).toBe("/en/contact");
    expect(screen.getByText("Reference: req-123")).toBeTruthy();
  });
});
