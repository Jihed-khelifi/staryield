// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";
import * as React from "react";

import {
  DatePartsInput,
  datePartsToIso,
  emptyDateParts,
  isoToDateParts,
  parsePastedDate,
  type DateParts,
} from "@/components/ui/date-parts-input";
import { axeViolations, renderWithI18n } from "@/test/render";

afterEach(cleanup);

describe("date parts helpers", () => {
  it("parses pasted dates in common formats", () => {
    expect(parsePastedDate("29/11/1990")).toEqual({ day: "29", month: "11", year: "1990", time: "" });
    expect(parsePastedDate("1990-11-29 14:30")).toEqual({ day: "29", month: "11", year: "1990", time: "14:30" });
    expect(parsePastedDate("11/29/1990", "mdy")).toEqual({ day: "29", month: "11", year: "1990", time: "" });
    expect(parsePastedDate("5.3.2001 9h05")).toEqual({ day: "05", month: "03", year: "2001", time: "09:05" });
    expect(parsePastedDate("yesterday")).toBeNull();
  });
  it("converts to and from ISO dates", () => {
    expect(datePartsToIso({ day: "5", month: "3", year: "2001" })).toBe("2001-03-05");
    expect(datePartsToIso({ day: "5", month: "3", year: "201" })).toBeNull();
    expect(isoToDateParts("1990-11-29", "14:30")).toEqual({ day: "29", month: "11", year: "1990", time: "14:30" });
    expect(isoToDateParts("garbage")).toEqual({ ...emptyDateParts, time: "" });
  });
});

function Harness({ withTime = false }: { withTime?: boolean }) {
  const [value, setValue] = React.useState<DateParts>(emptyDateParts);
  return (
    <>
      <DatePartsInput aria-label="Date of birth" value={value} onChange={setValue} withTime={withTime} />
      <output data-testid="iso">{datePartsToIso(value) ?? ""}</output>
      <output data-testid="time">{value.time}</output>
    </>
  );
}

describe("DatePartsInput", () => {
  it("keeps digits only and moves to the next box when one is full", async () => {
    renderWithI18n(<Harness />);
    const day = screen.getByRole("textbox", { name: "Day" });
    const month = screen.getByRole("textbox", { name: "Month" });
    fireEvent.change(day, { target: { value: "2a9" } });
    expect((day as HTMLInputElement).value).toBe("29");
    expect(document.activeElement).toBe(month);
    fireEvent.change(month, { target: { value: "11" } });
    fireEvent.change(screen.getByRole("textbox", { name: "Year" }), { target: { value: "1990" } });
    expect(screen.getByTestId("iso").textContent).toBe("1990-11-29");
    expect(await axeViolations()).toEqual([]);
  });

  it("fills every box from a pasted date, including the time", () => {
    renderWithI18n(<Harness withTime />);
    fireEvent.paste(screen.getByRole("textbox", { name: "Day" }), {
      clipboardData: { getData: () => "1990-11-29 07:45" },
    });
    expect(screen.getByTestId("iso").textContent).toBe("1990-11-29");
    expect(screen.getByTestId("time").textContent).toBe("07:45");
  });

  it("formats the time as HH:MM while typing", () => {
    renderWithI18n(<Harness withTime />);
    const time = screen.getByRole("textbox", { name: "Time" });
    fireEvent.change(time, { target: { value: "1430" } });
    expect((time as HTMLInputElement).value).toBe("14:30");
  });
});
