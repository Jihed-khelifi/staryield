import { describe, expect, it } from "vitest";

import { formatMinutes, formatPackPrice, packMinutes } from "@/lib/pricing";

describe("packMinutes", () => {
  it("divides credits by the rate (spec §04 examples)", () => {
    expect(packMinutes(600, 30)).toBe(20);
    expect(packMinutes(600, 60)).toBe(10);
    expect(packMinutes(315, 30)).toBe(10.5);
    expect(packMinutes(675, 60)).toBe(11.25);
  });

  it("is zero for impossible inputs", () => {
    expect(packMinutes(600, 0)).toBe(0);
    expect(packMinutes(-5, 30)).toBe(0);
    expect(packMinutes(Number.NaN, 30)).toBe(0);
  });
});

describe("formatMinutes", () => {
  it("keeps at most two decimals without trailing zeros", () => {
    expect(formatMinutes(20, "en")).toBe("20");
    expect(formatMinutes(10.5, "en")).toBe("10.5");
    expect(formatMinutes(5.25, "en")).toBe("5.25");
    expect(formatMinutes(600 / 45, "en")).toBe("13.33");
  });

  it("truncates instead of rounding up", () => {
    expect(formatMinutes(2 / 3, "en")).toBe("0.66");
    expect(formatMinutes(1.999, "en")).toBe("1.99");
  });

  it("localizes the decimal separator", () => {
    expect(formatMinutes(10.5, "fr")).toBe("10,5");
  });
});

describe("formatPackPrice", () => {
  it("drops cents for whole amounts", () => {
    expect(formatPackPrice(1000, "USD", "en")).toBe("$10");
    expect(formatPackPrice(25000, "USD", "en")).toBe("$250");
  });

  it("keeps cents otherwise", () => {
    expect(formatPackPrice(1999, "USD", "en")).toBe("$19.99");
  });
});
