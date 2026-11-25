// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import natal from "@/test/natal-fixture.json";
import type { NatalChart } from "@/lib/api/astrology";
import { ApiError } from "@/lib/api/errors";
import type { User } from "@/lib/api/types";
import { renderWithI18n, stubResizeObserver } from "@/test/render";

const m = vi.hoisted(() => ({ myChart: vi.fn(), myNumerology: vi.fn(), track: vi.fn() }));
vi.mock("@/lib/api/astrology", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api/astrology")>()),
  astroApi: { myChart: m.myChart, myNumerology: m.myNumerology },
}));
vi.mock("@/lib/analytics", () => ({ track: m.track }));
vi.mock("@/lib/psychics-link", () => ({ usePsychicsHref: () => "/en/psychics" }));

import { ChartTab } from "@/components/account/chart/chart-tab";

const chart = natal.chart as unknown as NatalChart;
const user = { id: "u1", profile: { birth_date: "1990-03-14", birth_time: "08:45", birth_place_id: "gn:2988507" } } as unknown as User;

beforeAll(() => {
  stubResizeObserver();
  Element.prototype.hasPointerCapture ??= () => false;
  Element.prototype.setPointerCapture ??= () => undefined;
  Element.prototype.releasePointerCapture ??= () => undefined;
  Element.prototype.scrollIntoView ??= () => undefined;
});
beforeEach(() => {
  for (const fn of Object.values(m)) fn.mockReset();
  m.myNumerology.mockResolvedValue({ life_path: 9, personal_year: 1, personal_month: 12 });
});
afterEach(cleanup);

describe("ChartTab (signed-in chart through the shared explorer)", () => {
  it("shows the saved chart, the numbers from the API and counts the view once", async () => {
    m.myChart.mockResolvedValue(chart);
    renderWithI18n(<ChartTab name="Cassandra" user={user} />);
    expect(await screen.findByTestId("natal-wheel")).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Pisces sun · Taurus rising" })).toBeTruthy();
    await waitFor(() => expect(within(screen.getByTestId("my-numbers")).getAllByRole("definition").map((d) => d.textContent)).toEqual(["9", "1", "12"]));
    expect(m.track).toHaveBeenCalledWith("chart_viewed", { has_location: true, time_known: true });
    expect(m.myChart).toHaveBeenCalledWith(undefined, expect.any(AbortSignal));
  });

  it("asks the API for the chosen house system and keeps the chart when that fails, with a retry", async () => {
    const u = userEvent.setup();
    m.myChart.mockResolvedValueOnce(chart);
    renderWithI18n(<ChartTab name="Cassandra" user={user} />);
    await screen.findByTestId("natal-wheel");

    m.myChart.mockRejectedValueOnce(new ApiError({ code: "network_error", message: "x" }));
    await u.click(screen.getByTestId("house-system-select"));
    await u.click(await screen.findByRole("option", { name: "Whole Sign" }));
    expect(await screen.findByTestId("chart-update-error")).toBeTruthy();
    expect(m.myChart).toHaveBeenLastCalledWith("whole_sign", expect.any(AbortSignal));
    expect(screen.getByTestId("natal-wheel")).toBeTruthy();

    m.myChart.mockResolvedValueOnce({ ...chart, house_system: "whole_sign" });
    await u.click(screen.getByTestId("chart-update-retry"));
    await waitFor(() => expect(screen.queryByTestId("chart-update-error")).toBeNull());
    expect(m.myChart).toHaveBeenCalledTimes(3);
  });

  it("asks for a birth date in settings when the profile has none", async () => {
    m.myChart.mockRejectedValue(new ApiError({ code: "validation_failed", message: "x", details: { fields: { birth_date: "Add your birth date." } } }));
    renderWithI18n(<ChartTab name="Cassandra" user={user} />);
    expect(await screen.findByText("Add your birth date in settings to see your chart.")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Open settings" }).getAttribute("href")).toBe("/en/profile/settings");
  });
});
