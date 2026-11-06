// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, within } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import { AsyncContent } from "@/components/feedback/async-content";
import { EmptyState } from "@/components/feedback/empty-state";
import { announce } from "@/components/feedback/live-region";
import { SaveStatus } from "@/components/feedback/save-status";
import { axeViolations, renderWithI18n } from "@/test/render";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("announce", () => {
  it("writes to one shared polite live region and re-announces repeats", () => {
    vi.useFakeTimers();
    announce("Note saved.");
    act(() => {
      vi.advanceTimersByTime(60);
    });
    const regions = document.querySelectorAll('[data-slot="live-region"][data-politeness="polite"]');
    expect(regions).toHaveLength(1);
    expect(regions[0].getAttribute("aria-live")).toBe("polite");
    expect(regions[0].textContent).toBe("Note saved.");
    announce("Note saved.");
    expect(regions[0].textContent).toBe("");
    act(() => {
      vi.advanceTimersByTime(60);
    });
    expect(regions[0].textContent).toBe("Note saved.");
    announce("Payment failed.", "assertive");
    act(() => {
      vi.advanceTimersByTime(60);
    });
    expect(document.querySelector('[data-politeness="assertive"]')?.getAttribute("role")).toBe("alert");
  });
});

describe("AsyncContent", () => {
  const base = { reload: vi.fn() };

  it("renders loading, error, empty and data states", async () => {
    const view = (state: Parameters<typeof AsyncContent<string[]>>[0]["state"]) => (
      <AsyncContent state={state} isEmpty={(d) => d.length === 0} empty={<EmptyState title="No devices" />}>
        {(data) => <ul>{data.map((d) => <li key={d}>{d}</li>)}</ul>}
      </AsyncContent>
    );
    const { rerender, container } = renderWithI18n(view({ ...base, data: null, error: null, loading: true }));
    expect(within(container).getByRole("status").textContent).toBe("Loading…");

    const reload = vi.fn();
    rerender(view({ data: null, error: new ApiError({ code: "network_error", message: "x" }), loading: false, reload }));
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(reload).toHaveBeenCalledOnce();

    rerender(view({ ...base, data: [], error: null, loading: false }));
    expect(screen.getByText("No devices")).toBeTruthy();

    rerender(view({ ...base, data: ["MacBook Pro", "iPhone"], error: null, loading: false }));
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
    expect(await axeViolations()).toEqual([]);
  });
});

describe("SaveStatus", () => {
  it("announces each state with text", () => {
    const { rerender, container } = renderWithI18n(<SaveStatus state="saving" />);
    const status = () => within(container).getByRole("status").textContent;
    expect(status()).toBe("Saving…");
    rerender(<SaveStatus state="saved" />);
    expect(status()).toBe("All changes saved");
    rerender(<SaveStatus state="error" />);
    expect(status()).toBe("Not saved");
    rerender(<SaveStatus state="dirty" />);
    expect(status()).toBe("Unsaved changes");
    rerender(<SaveStatus state="idle" />);
    expect(status()).toBe("");
  });
});
