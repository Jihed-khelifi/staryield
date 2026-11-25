// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const mocks = vi.hoisted(() => ({ push: vi.fn(), track: vi.fn(), sunSign: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: mocks.push }) }));
vi.mock("@/lib/analytics", () => ({ track: mocks.track }));
vi.mock("@/lib/api/astrology", () => ({ astroApi: { sunSign: mocks.sunSign } }));

import { CompatibilityCalculator } from "@/components/seo/compatibility-calculator";

const SIGN_BY_DATE: Record<string, object> = {
  "1990-04-10": { sun_sign: "Aries", uncertain: false, element: "Fire", modality: "Cardinal" },
  "1988-08-05": { sun_sign: "Leo", uncertain: false, element: "Fire", modality: "Fixed" },
  "1990-04-20": { sun_sign: "Taurus", uncertain: true, alternatives: ["Aries", "Taurus"], element: "Earth", modality: "Fixed" },
};

beforeAll(stubResizeObserver);
beforeEach(() => {
  mocks.push.mockReset();
  mocks.track.mockReset();
  mocks.sunSign.mockReset();
  mocks.sunSign.mockImplementation(async ({ birth_date }: { birth_date: string }) => SIGN_BY_DATE[birth_date]);
  vi.spyOn(console, "error").mockImplementation(() => undefined);
  vi.spyOn(window.navigator, "onLine", "get").mockReturnValue(true);
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function fill(index: number, day: string, month: string, year: string) {
  const group = screen.getByTestId("compatibility-form").querySelectorAll<HTMLElement>("[data-slot=date-parts-input]")[index];
  fireEvent.change(within(group).getByLabelText("Day"), { target: { value: day } });
  fireEvent.change(within(group).getByLabelText("Month"), { target: { value: month } });
  fireEvent.change(within(group).getByLabelText("Year"), { target: { value: year } });
}
const submit = () => fireEvent.click(screen.getByTestId("compatibility-submit"));

describe("CompatibilityCalculator", () => {
  it("validates both dates on their own fields", async () => {
    renderWithI18n(<CompatibilityCalculator id="calc" />);
    submit();
    expect(await screen.findAllByText("Enter your date of birth.")).toHaveLength(2);
    fill(0, "10", "04", "1990");
    fill(1, "30", "02", "1988");
    submit();
    expect(await screen.findByText("Enter a valid date.")).toBeTruthy();
    expect(mocks.sunSign).not.toHaveBeenCalled();
  });

  it("finds both signs in parallel and opens the canonical pair page, in either order", async () => {
    renderWithI18n(<CompatibilityCalculator id="calc" />);
    fill(0, "05", "08", "1988");
    fill(1, "10", "04", "1990");
    submit();
    await waitFor(() => expect(mocks.push).toHaveBeenCalledWith("/en/compatibility/aries-and-leo"));
    expect(mocks.sunSign).toHaveBeenCalledTimes(2);
    expect(mocks.sunSign.mock.calls.map((call) => call[0])).toEqual([{ birth_date: "1988-08-05" }, { birth_date: "1990-04-10" }]);
    expect(mocks.track).toHaveBeenCalledWith("calculator_started", { kind: "compatibility" });
    expect(mocks.track).toHaveBeenCalledWith("calculator_completed", { kind: "compatibility" });
  });

  it("confirms the pair in place when the visitor is already on its page", async () => {
    renderWithI18n(<CompatibilityCalculator id="calc" current="aries-and-leo" />);
    fill(0, "10", "04", "1990");
    fill(1, "05", "08", "1988");
    submit();
    const here = await screen.findByTestId("compatibility-here");
    expect(here.textContent).toContain("Your signs are Aries and Leo: this is your pairing.");
    expect(here.getAttribute("role")).toBe("status");
    expect(mocks.push).not.toHaveBeenCalled();
  });

  it("lets the visitor choose a sign on a cusp birthday, with a link to add a birth time", async () => {
    renderWithI18n(<CompatibilityCalculator id="calc" />);
    fill(0, "20", "04", "1990");
    fill(1, "05", "08", "1988");
    submit();
    const cusp = await screen.findByTestId("compatibility-cusp");
    expect(mocks.push).not.toHaveBeenCalled();
    expect(within(cusp).getByTestId("cusp-you")).toBeTruthy();
    expect(within(cusp).queryByTestId("cusp-them")).toBeNull();
    const link = within(cusp).getByRole("link", { name: /Add a birth time in the zodiac sign calculator/ });
    expect(link.getAttribute("href")).toBe("/en/calculators/astrology#v=1&d=1990-04-20");
    expect(link.getAttribute("target")).toBe("_blank");
    // The API's own answer (Taurus) is preselected; choosing Aries changes the pair.
    const aries = within(cusp).getByRole("radio", { name: "Aries" });
    expect(within(cusp).getByRole("radio", { name: "Taurus" }).getAttribute("aria-checked")).toBe("true");
    fireEvent.click(aries);
    fireEvent.click(within(cusp).getByTestId("compatibility-continue"));
    await waitFor(() => expect(mocks.push).toHaveBeenCalledWith("/en/compatibility/aries-and-leo"));
  });

  it("puts the API's field message on the right date", async () => {
    mocks.sunSign.mockImplementation(async ({ birth_date }: { birth_date: string }) => {
      if (birth_date === "1988-08-05")
        throw new ApiError({ code: "validation_failed", message: "invalid", status: 422, details: { fields: { birth_date: "Birth date out of range." } } });
      return SIGN_BY_DATE[birth_date];
    });
    renderWithI18n(<CompatibilityCalculator id="calc" />);
    fill(0, "10", "04", "1990");
    fill(1, "05", "08", "1988");
    submit();
    expect(await screen.findByText("Birth date out of range.")).toBeTruthy();
    const second = screen.getByTestId("compatibility-form").querySelectorAll<HTMLElement>("[data-slot=date-parts-input]")[1];
    expect(within(second).getByLabelText("Day").getAttribute("aria-invalid")).toBe("true");
    expect(mocks.push).not.toHaveBeenCalled();
    expect(mocks.track).toHaveBeenCalledWith("error_displayed", { code: "validation_failed", area: "compatibility" });
  });

  it("shows a network failure with a retry that works", async () => {
    mocks.sunSign.mockRejectedValueOnce(new ApiError({ code: "network_error", message: "offline" }));
    renderWithI18n(<CompatibilityCalculator id="calc" />);
    fill(0, "10", "04", "1990");
    fill(1, "05", "08", "1988");
    submit();
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("We couldn't reach Staryield. Please try again in a moment.");
    fireEvent.click(within(alert).getByRole("button", { name: "Try again" }));
    await waitFor(() => expect(mocks.push).toHaveBeenCalledWith("/en/compatibility/aries-and-leo"));
  });

  it("is accessible, with the result step too", async () => {
    const { container } = renderWithI18n(
      <section aria-labelledby="calc-title">
        <h2 id="calc-title">Find your compatibility</h2>
        <CompatibilityCalculator id="calc" />
      </section>,
    );
    expect(await axeViolations(container)).toEqual([]);
    fill(0, "20", "04", "1990");
    fill(1, "05", "08", "1988");
    submit();
    await screen.findByTestId("compatibility-cusp");
    expect(await axeViolations(container)).toEqual([]);
  });
});
