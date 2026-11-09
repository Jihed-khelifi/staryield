// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import { renderWithI18n, stubResizeObserver } from "@/test/render";

// The API call and the page's own post-processing fail differently (§22.2):
// only the former uses API copy; the latter is a client_error with "Try again".

const mocks = vi.hoisted(() => ({ calculate: vi.fn(), track: vi.fn() }));
vi.mock("@/components/calculators/calculate", () => ({ calculate: mocks.calculate }));
vi.mock("@/lib/analytics", () => ({ track: mocks.track }));
vi.mock("@/lib/api/astrology", () => ({ astroApi: { place: vi.fn() } }));
vi.mock("@/components/astrology/place-combobox", () => ({ PlaceCombobox: () => null }));

import { CalculatorForm } from "@/components/calculators/calculator-form";

const RESULT = { kind: "path-of-life", numbers: {} };
const VALUES = { birth: { day: "17", month: "5", year: "1990", time: "" } };

beforeAll(stubResizeObserver);
beforeEach(() => {
  mocks.calculate.mockReset();
  mocks.track.mockReset();
  vi.spyOn(console, "error").mockImplementation(() => undefined);
  vi.spyOn(console, "warn").mockImplementation(() => undefined);
  vi.spyOn(window.navigator, "onLine", "get").mockReturnValue(true);
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const submit = () => fireEvent.click(screen.getByTestId("calculator-submit"));

describe("CalculatorForm errors", () => {
  it("shows the API error copy for a failed call, with no client_error action", async () => {
    mocks.calculate.mockRejectedValue(new ApiError({ code: "network_error", message: "x" }));
    const onCalculated = vi.fn();
    renderWithI18n(<CalculatorForm kind="path-of-life" initialValues={VALUES as never} onCalculated={onCalculated} />);
    submit();
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("We couldn't reach Staryield. Please try again in a moment.");
    expect(screen.queryByRole("button", { name: "Try again" })).toBeNull();
    expect(onCalculated).not.toHaveBeenCalled();
  });

  it("says offline only when the browser is offline", async () => {
    vi.spyOn(window.navigator, "onLine", "get").mockReturnValue(false);
    mocks.calculate.mockRejectedValue(new ApiError({ code: "network_error", message: "x" }));
    renderWithI18n(<CalculatorForm kind="path-of-life" initialValues={VALUES as never} onCalculated={vi.fn()} />);
    submit();
    expect((await screen.findByRole("alert")).textContent).toContain("You appear to be offline.");
  });

  it("does not call a post-processing failure a network problem, and offers Try again", async () => {
    mocks.calculate.mockResolvedValue(RESULT);
    const onCalculated = vi
      .fn()
      .mockImplementationOnce(() => {
        throw new TypeError("Cannot read properties of undefined (reading 'x')"); // a bug, not a network failure
      })
      .mockImplementation(() => undefined);
    renderWithI18n(<CalculatorForm kind="path-of-life" initialValues={VALUES as never} onCalculated={onCalculated} />);
    submit();
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Something went wrong on this page. Please try again.");
    expect(alert.textContent).not.toMatch(/offline|reach Staryield/);
    expect(mocks.track).toHaveBeenCalledWith("error_displayed", { code: "client_error", area: "calculator" });

    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    await waitFor(() => expect(onCalculated).toHaveBeenCalledTimes(2));
    expect(mocks.calculate).toHaveBeenCalledTimes(2);
  });

  it("hands a good answer to the page", async () => {
    mocks.calculate.mockResolvedValue(RESULT);
    const onCalculated = vi.fn();
    renderWithI18n(<CalculatorForm kind="path-of-life" initialValues={VALUES as never} onCalculated={onCalculated} />);
    submit();
    await waitFor(() => expect(onCalculated).toHaveBeenCalledWith(expect.anything(), RESULT));
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
