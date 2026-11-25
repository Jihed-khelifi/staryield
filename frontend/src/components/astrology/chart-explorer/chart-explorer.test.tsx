// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import natal from "@/test/natal-fixture.json";
import type { NatalChart } from "@/lib/api/astrology";
import { ApiError } from "@/lib/api/errors";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

vi.mock("@/lib/psychics-link", () => ({ usePsychicsHref: () => "/en#psychics" }));

import { ChartExplorer } from "@/components/astrology/chart-explorer/chart-explorer";
import { ChartBalance } from "@/components/astrology/chart-explorer/chart-balance";
import { NumbersSummary } from "@/components/astrology/chart-explorer/numbers-summary";
import { rowBodies } from "@/components/astrology/chart-explorer/planet-rows";

const chart = natal.chart as unknown as NatalChart;
const noTime = { ...chart, time_known: false, rising_sign: undefined, angles: undefined, houses: undefined, house_cusps: undefined, warnings: ["birth_place_missing"] } as unknown as NatalChart;

beforeAll(() => {
  stubResizeObserver();
  // Radix Select needs pointer capture and scrolling, which jsdom lacks.
  Element.prototype.hasPointerCapture ??= () => false;
  Element.prototype.setPointerCapture ??= () => undefined;
  Element.prototype.releasePointerCapture ??= () => undefined;
  Element.prototype.scrollIntoView ??= () => undefined;
});
afterEach(cleanup);

const notes = { timeNote: "Add your birth time.", placeNote: "Add your birthplace." };

describe("ChartExplorer", () => {
  it("shows a skeleton until the first chart arrives", () => {
    renderWithI18n(<ChartExplorer chart={null} onHouseSystemChange={vi.fn()} {...notes} />);
    expect(screen.getByTestId("chart-explorer-loading").getAttribute("aria-busy")).toBe("true");
    expect(screen.getByRole("status").textContent).toBe("Calculating your chart…");
  });

  it("draws the wheel with the Ascendant and the ten planets as accordion rows", async () => {
    const u = userEvent.setup();
    const { container } = renderWithI18n(<ChartExplorer chart={chart} onHouseSystemChange={vi.fn()} {...notes} />);
    expect(screen.getByTestId("natal-wheel")).toBeTruthy();
    const rows = rowBodies(chart).map((p) => p.body);
    expect(rows[0]).toBe("Ascendant");
    expect(rows).toHaveLength(11);
    expect(rows).not.toContain("Midheaven");
    // Split in two columns around the wheel.
    expect(within(screen.getByTestId("planet-rows-left")).getAllByRole("button")).toHaveLength(6);
    expect(within(screen.getByTestId("planet-rows-right")).getAllByRole("button")).toHaveLength(5);

    const sun = screen.getByTestId("planet-row-Sun");
    expect(sun.getAttribute("aria-expanded")).toBe("false");
    await u.click(sun);
    expect(sun.getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByText(/The Sun reflects your core identity/)).toBeTruthy();
    expect(container.querySelector('[data-testid=natal-wheel] [data-body="Sun"]')!.getAttribute("data-active")).toBe("true");
    // Each interpretation offers an advisor (the directory entry point for this session).
    expect(screen.getByRole("link", { name: "Get more insight from one of our advisors" }).getAttribute("href")).toBe("/en#psychics");
    expect(screen.queryByTestId("chart-update-error")).toBeNull();
    expect(screen.getByText(chart.disclaimer)).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
  });

  it("switches the house system and shows the update and failure states", async () => {
    const u = userEvent.setup();
    const onChange = vi.fn();
    const onRetry = vi.fn();
    const { rerender } = renderWithI18n(<ChartExplorer chart={chart} onHouseSystemChange={onChange} {...notes} />);
    await u.click(screen.getByTestId("house-system-select"));
    await u.click(await screen.findByRole("option", { name: "Whole Sign" }));
    expect(onChange).toHaveBeenCalledWith("whole_sign");

    // Recalculating: the chart stays, the explorer is busy and says so.
    rerender(<ChartExplorer chart={chart} houseSystem="whole_sign" updating onHouseSystemChange={onChange} {...notes} />);
    expect(screen.getByTestId("chart-explorer").getAttribute("aria-busy")).toBe("true");
    expect(screen.getByText("Updating…")).toBeTruthy();
    expect(screen.getByTestId("natal-wheel")).toBeTruthy();
    expect(screen.getByTestId("house-system-select").textContent).toContain("Whole Sign");

    // The update failed: an inline message with the API's copy and a retry.
    const error = new ApiError({ code: "network_error", message: "x" });
    rerender(<ChartExplorer chart={chart} houseSystem="whole_sign" updateError={error} onRetry={onRetry} onHouseSystemChange={onChange} {...notes} />);
    const alert = screen.getByTestId("chart-update-error");
    expect(alert.getAttribute("role")).toBe("alert");
    expect(alert.textContent).toContain("We could not update your houses.");
    await u.click(screen.getByTestId("chart-update-retry"));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("without a birth time: the select is disabled and the page's own notes are shown", () => {
    renderWithI18n(<ChartExplorer chart={noTime} onHouseSystemChange={vi.fn()} {...notes} />);
    expect(screen.getByTestId("house-system-select").hasAttribute("disabled")).toBe(true);
    expect(screen.getByText("Add your birth time.")).toBeTruthy();
    expect(screen.getByText("Add your birthplace.")).toBeTruthy();
  });

  it("lists the chart as tables: positions, house cusps and aspects", async () => {
    const u = userEvent.setup();
    renderWithI18n(<ChartExplorer chart={chart} onHouseSystemChange={vi.fn()} {...notes} />);
    await u.click(screen.getByTestId("chart-details-toggle"));
    const positions = screen.getByRole("table", { name: "Positions" });
    expect(within(positions).getByRole("rowheader", { name: "Sun" })).toBeTruthy();
    expect(within(screen.getByRole("table", { name: "House cusps" })).getAllByRole("row")).toHaveLength(13);
    expect(within(screen.getByRole("table", { name: "Aspects" })).getAllByRole("row")).toHaveLength(chart.aspects.length + 1);
    expect(await axeViolations()).toEqual([]);
  });
});

describe("ChartBalance", () => {
  it("counts the points per element and modality", () => {
    const { container } = renderWithI18n(<ChartBalance chart={chart} />);
    expect(screen.getByRole("heading", { name: "Elements and modalities" })).toBeTruthy();
    for (const [name, count] of Object.entries({ ...chart.elements, ...chart.modalities }))
      expect(container.querySelector(`[data-balance="${name}"]`)!.getAttribute("data-count")).toBe(String(count));
    expect(container.querySelectorAll("[data-balance]")).toHaveLength(7);
  });

  it("is left out when the chart has no balance", () => {
    const { container } = renderWithI18n(<ChartBalance chart={{ elements: undefined, modalities: undefined } as never} />);
    expect(container.innerHTML).toBe("");
  });
});

describe("NumbersSummary", () => {
  it("shows the three numbers, a skeleton while loading, and an error with a retry", async () => {
    const u = userEvent.setup();
    const onRetry = vi.fn();
    const { rerender } = renderWithI18n(<NumbersSummary numbers={null} />);
    expect(screen.queryByTestId("my-numbers")).toBeNull();

    rerender(<NumbersSummary numbers={{ lifePath: 9, personalYear: 3, personalMonth: 4 }} />);
    const numbers = screen.getByTestId("my-numbers");
    expect(within(numbers).getAllByRole("definition").map((d) => d.textContent)).toEqual(["9", "3", "4"]);

    rerender(<NumbersSummary numbers={null} error={new ApiError({ code: "network_error", message: "x" })} onRetry={onRetry} />);
    await u.click(screen.getByRole("button", { name: "Try again" }));
    expect(onRetry).toHaveBeenCalled();
  });
});
