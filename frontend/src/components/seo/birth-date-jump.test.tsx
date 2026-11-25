// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";

import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const mocks = vi.hoisted(() => ({ push: vi.fn(), track: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: mocks.push }) }));
vi.mock("@/lib/analytics", () => ({ track: mocks.track }));

import { BirthDateJump } from "@/components/seo/birth-date-jump";

beforeAll(stubResizeObserver);
beforeEach(() => {
  mocks.push.mockReset();
  mocks.track.mockReset();
});
afterEach(cleanup);

function fill(day: string, month: string, year: string) {
  const form = screen.getByTestId("seo-birth-form");
  fireEvent.change(within(form).getByLabelText("Day"), { target: { value: day } });
  fireEvent.change(within(form).getByLabelText("Month"), { target: { value: month } });
  fireEvent.change(within(form).getByLabelText("Year"), { target: { value: year } });
}

describe("BirthDateJump", () => {
  it("validates the date like the calculator and shows the error on the field", async () => {
    renderWithI18n(<BirthDateJump calculator="path-of-life" id="calc" />);
    fireEvent.click(screen.getByTestId("seo-birth-submit"));
    expect(await screen.findByText("Enter your date of birth.")).toBeTruthy();
    fill("31", "02", "1990");
    fireEvent.click(screen.getByTestId("seo-birth-submit"));
    expect(await screen.findByText("Enter a valid date.")).toBeTruthy();
    fill("01", "01", "2999");
    fireEvent.click(screen.getByTestId("seo-birth-submit"));
    expect(await screen.findByText("Enter a valid date.")).toBeTruthy();
    expect(screen.getByLabelText("Day").getAttribute("aria-invalid")).toBe("true");
    expect(mocks.push).not.toHaveBeenCalled();
  });

  it("opens the Life Path calculator with the date in the fragment", async () => {
    renderWithI18n(<BirthDateJump calculator="path-of-life" id="calc" />);
    fill("18", "6", "1981");
    fireEvent.click(screen.getByTestId("seo-birth-submit"));
    await waitFor(() => expect(mocks.push).toHaveBeenCalledWith("/en/calculators/path-of-life#v=1&d=1981-06-18"));
    expect(mocks.track).toHaveBeenCalledWith("calculator_started", { kind: "path-of-life", source: "seo_page" });
  });

  it("opens the tarot birth card calculator", async () => {
    renderWithI18n(<BirthDateJump calculator="tarot-birth-card" id="calc" />);
    expect(screen.getByTestId("seo-birth-submit").textContent).toBe("Reveal my birth cards");
    fill("10", "01", "1959");
    fireEvent.click(screen.getByTestId("seo-birth-submit"));
    await waitFor(() => expect(mocks.push).toHaveBeenCalledWith("/en/calculators/tarot-birth-card#v=1&d=1959-01-10"));
  });

  it("is accessible", async () => {
    const { container } = renderWithI18n(
      <section aria-labelledby="calc-title">
        <h2 id="calc-title">Calculate</h2>
        <BirthDateJump calculator="path-of-life" id="calc" />
      </section>,
    );
    expect(await axeViolations(container)).toEqual([]);
  });
});
