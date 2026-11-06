// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen } from "@testing-library/react";
import * as React from "react";

import { ApiError } from "@/lib/api/errors";
import { Combobox } from "@/components/ui/combobox";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

type Place = { id: string; label: string };
const PLACES: Place[] = [
  { id: "1", label: "Paris, France" },
  { id: "2", label: "Parma, Italy" },
];

beforeAll(stubResizeObserver);
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

function Harness({
  loadOptions,
  onChange,
}: {
  loadOptions: (query: string, signal: AbortSignal, cursor?: string) => Promise<Place[] | { items: Place[]; nextCursor?: string }>;
  onChange?: (value: Place | null) => void;
}) {
  const [value, setValue] = React.useState<Place | null>(null);
  return (
    <>
      <label htmlFor="place">Birthplace</label>
      <Combobox
        id="place"
        value={value}
        onChange={(v) => {
          setValue(v);
          onChange?.(v);
        }}
        loadOptions={loadOptions}
        getOptionLabel={(p) => p.label}
        getOptionKey={(p) => p.id}
        debounceMs={200}
      />
      <span data-testid="value">{value?.id ?? "none"}</span>
    </>
  );
}

async function type(input: HTMLElement, value: string) {
  fireEvent.change(input, { target: { value } });
  await act(async () => {
    vi.advanceTimersByTime(250);
  });
  await act(async () => {});
}

describe("Combobox", () => {
  it("searches after the debounce, only from the minimum length, and picks with the keyboard", async () => {
    vi.useFakeTimers();
    const loadOptions = vi.fn(async (query: string) => PLACES.filter((p) => p.label.toLowerCase().startsWith(query.toLowerCase())));
    const onChange = vi.fn();
    renderWithI18n(<Harness loadOptions={loadOptions} onChange={onChange} />);
    const input = screen.getByRole("combobox", { name: "Birthplace" });

    await type(input, "P");
    expect(loadOptions).not.toHaveBeenCalled();

    await type(input, "Par");
    expect(loadOptions).toHaveBeenCalledTimes(1);
    expect(loadOptions.mock.calls[0][0]).toBe("Par");
    expect(input.getAttribute("aria-expanded")).toBe("true");
    expect(screen.getAllByRole("option")).toHaveLength(2);

    fireEvent.keyDown(input, { key: "ArrowDown" });
    expect(input.getAttribute("aria-activedescendant")).toMatch(/-1$/);
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onChange).toHaveBeenLastCalledWith(PLACES[1]);
    expect(screen.getByTestId("value").textContent).toBe("2");
    expect((input as HTMLInputElement).value).toBe("Parma, Italy");
    expect(input.getAttribute("aria-expanded")).toBe("false");
  });

  it("free text never counts as a value: editing clears the pick", async () => {
    vi.useFakeTimers();
    const loadOptions = vi.fn(async () => PLACES);
    const onChange = vi.fn();
    renderWithI18n(<Harness loadOptions={loadOptions} onChange={onChange} />);
    const input = screen.getByRole("combobox");
    await type(input, "Paris");
    fireEvent.click(screen.getByRole("option", { name: "Paris, France" }));
    expect(screen.getByTestId("value").textContent).toBe("1");
    await type(input, "Paris, Texas");
    expect(onChange).toHaveBeenLastCalledWith(null);
    expect(screen.getByTestId("value").textContent).toBe("none");
  });

  it("aborts the previous search when the query changes", async () => {
    vi.useFakeTimers();
    const signals: AbortSignal[] = [];
    const loadOptions = vi.fn((_q: string, signal: AbortSignal) => {
      signals.push(signal);
      return new Promise<Place[]>(() => {});
    });
    renderWithI18n(<Harness loadOptions={loadOptions} />);
    const input = screen.getByRole("combobox");
    await type(input, "Pa");
    await type(input, "Par");
    expect(signals).toHaveLength(2);
    expect(signals[0].aborted).toBe(true);
    expect(signals[1].aborted).toBe(false);
  });

  it("announces empty results and errors, and retries", async () => {
    vi.useFakeTimers();
    let fail = true;
    const loadOptions = vi.fn(async () => {
      if (fail) throw new ApiError({ code: "service_unavailable", message: "down", status: 503 });
      return [];
    });
    renderWithI18n(<Harness loadOptions={loadOptions} />);
    const input = screen.getByRole("combobox");
    await type(input, "Zzz");
    expect(screen.getByRole("status").textContent).toContain("The service is temporarily unavailable");
    fail = false;
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    await act(async () => {
      vi.advanceTimersByTime(250);
    });
    await act(async () => {});
    expect(screen.getByRole("status").textContent).toContain("No matches found.");
  });

  it("clears the selection with the clear button and has no a11y violations", async () => {
    vi.useFakeTimers();
    renderWithI18n(<Harness loadOptions={async () => PLACES} />);
    const input = screen.getByRole("combobox");
    await type(input, "Par");
    fireEvent.click(screen.getByRole("option", { name: "Paris, France" }));
    fireEvent.click(screen.getByRole("button", { name: "Clear selection" }));
    expect((input as HTMLInputElement).value).toBe("");
    expect(screen.getByTestId("value").textContent).toBe("none");
    vi.useRealTimers();
    expect(await axeViolations()).toEqual([]);
  });

  it("paginates: 'Show more results' loads the next page with the cursor, keyboard accessible", async () => {
    vi.useFakeTimers();
    const pages: Record<string, { items: Place[]; nextCursor?: string }> = {
      first: { items: [PLACES[0]], nextCursor: "c2" },
      c2: { items: [PLACES[1], { id: "3", label: "Paros, Greece" }] },
    };
    const loadOptions = vi.fn(async (_q: string, _s: AbortSignal, cursor?: string) => pages[cursor ?? "first"]);
    const onChange = vi.fn();
    renderWithI18n(<Harness loadOptions={loadOptions} onChange={onChange} />);
    const input = screen.getByRole("combobox");
    await type(input, "Par");
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual(["Paris, France", "Show more results"]);
    fireEvent.keyDown(input, { key: "End" });
    fireEvent.keyDown(input, { key: "Enter" });
    await act(async () => {});
    expect(loadOptions).toHaveBeenLastCalledWith("Par", expect.any(AbortSignal), "c2");
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual(["Paris, France", "Parma, Italy", "Paros, Greece"]);
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onChange).toHaveBeenLastCalledWith(PLACES[1]);
  });
});
