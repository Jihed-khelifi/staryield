// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";
import * as React from "react";

import { ChipInput, addChip, normalizeChip } from "@/components/ui/chip-input";
import { axeViolations, renderWithI18n } from "@/test/render";

afterEach(cleanup);

describe("chip helpers", () => {
  it("normalizes and rejects empty, duplicate and over-limit chips", () => {
    expect(normalizeChip("  Tarot   reading ")).toBe("Tarot reading");
    const list = ["Tarot"];
    expect(addChip(list, "  ")).toBe(list);
    expect(addChip(list, "tarot")).toBe(list);
    expect(addChip(list, "Astrology", 1)).toBe(list);
    expect(addChip(list, " Astrology ")).toEqual(["Tarot", "Astrology"]);
  });
});

function Harness({ max = 3 }: { max?: number }) {
  const [value, setValue] = React.useState(["Tarot", "Astrology"]);
  return (
    <>
      <label htmlFor="modalities">Modalities</label>
      <ChipInput id="modalities" aria-label="Modalities" value={value} onChange={setValue} max={max} />
      <output data-testid="value">{value.join("|")}</output>
    </>
  );
}

describe("ChipInput", () => {
  it("adds with Enter and removes with the minus button", async () => {
    renderWithI18n(<Harness max={4} />);
    const input = screen.getByRole("textbox", { name: "Modalities" });
    fireEvent.change(input, { target: { value: "Numerology" } });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(screen.getByTestId("value").textContent).toBe("Tarot|Astrology|Numerology");
    expect((input as HTMLInputElement).value).toBe("");
    fireEvent.click(screen.getByRole("button", { name: "Remove Astrology" }));
    expect(screen.getByTestId("value").textContent).toBe("Tarot|Numerology");
    expect(await axeViolations()).toEqual([]);
  });

  it("stops offering the input at the limit", () => {
    renderWithI18n(<Harness max={2} />);
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.getByText("You can add up to 2.")).toBeTruthy();
  });
});
