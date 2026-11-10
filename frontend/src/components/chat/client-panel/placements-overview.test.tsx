// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, screen, within } from "@testing-library/react";

import { axeViolations, renderWithI18n } from "@/test/render";
import { PlacementsOverview } from "@/components/chat/client-panel/placements-overview";
import { placementsFor } from "@/components/chat/client-panel/panel-helpers";

afterEach(cleanup);

const chart = (extra: Record<string, unknown> = {}, bodies = ["Sun:Aries", "Mercury:Pisces", "Venus:Taurus", "Mars:Leo", "Pluto:Capricorn"]) => ({
  astro: { sun_sign: "Aries", time_known: false, placements: bodies.map((b) => ({ body: b.split(":")[0], sign: b.split(":")[1], retrograde: false })), ...extra },
});

describe("PlacementsOverview (§23.5)", () => {
  it("the preview is the first row: Sun, Moon, Ascendant; unknown slots stay in place", () => {
    renderWithI18n(<PlacementsOverview placements={placementsFor(chart())} timeKnown={false} />);
    const items = within(screen.getByTestId("client-placements")).getAllByRole("listitem");
    expect(items).toHaveLength(3);
    expect(items.map((li) => li.getAttribute("title"))).toEqual(["Sun", "Moon", "Ascendant"]);
  });

  it("an unknown slot reads 'Unknown' with an accessible label, muted", () => {
    renderWithI18n(<PlacementsOverview placements={placementsFor(chart())} timeKnown={false} />);
    const moon = screen.getByTestId("placement-moon");
    expect(moon.classList.contains("placement-unknown")).toBe(true);
    expect(moon.textContent).toContain("Unknown");
    expect(within(moon).getByText("Moon sign unknown").className).toContain("sr-only");
    expect(within(screen.getByTestId("placement-ascendant")).getByText("Rising sign unknown")).toBeTruthy();
    // The known one is announced as before.
    expect(within(screen.getByTestId("placement-sun")).getByText("Sun in")).toBeTruthy();
  });

  it("expanding shows the remaining slots in order, three per row", () => {
    renderWithI18n(<PlacementsOverview placements={placementsFor(chart())} />);
    fireEvent.click(screen.getByRole("button", { name: "+ expand" }));
    const titles = within(screen.getByTestId("client-placements")).getAllByRole("listitem").map((li) => li.getAttribute("title"));
    expect(titles).toEqual(["Sun", "Moon", "Ascendant", "Mercury", "Venus", "Mars", "Pluto"]);
  });

  it("has no serious accessibility violations", async () => {
    renderWithI18n(<PlacementsOverview placements={placementsFor(chart())} timeKnown={false} />);
    expect(await axeViolations()).toEqual([]);
  });
});
