import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import { formatDateParts, lifePath, lifePathReduction, localDateIn, personalCycles, reduceNumber, romanNumeral, validBirthDate } from "@/lib/calculators";

type Vector = { birth_date: string; as_of: string; life_path: number; life_path_master: boolean; personal_year: number; personal_month: number; personal_day: number; instant?: string; tz?: string };
const fixtures = JSON.parse(readFileSync(new URL("../../../backend/internal/astro/testdata/numerology_vectors.json", import.meta.url), "utf8")) as {
  vectors: Vector[];
  timezone_vectors: Vector[];
};

describe("numerology matches the Go engine's shared vectors", () => {
  for (const v of fixtures.vectors)
    it(`${v.birth_date} as of ${v.as_of}`, () => {
      expect(lifePath(v.birth_date)).toBe(v.life_path);
      expect(lifePathReduction(v.birth_date).master).toBe(v.life_path_master);
      expect(personalCycles(v.birth_date, v.as_of)).toMatchObject({ personalYear: v.personal_year, personalMonth: v.personal_month, personalDay: v.personal_day });
    });
  for (const v of fixtures.timezone_vectors)
    it(`today in ${v.tz}`, () => {
      expect(localDateIn(new Date(v.instant!), v.tz)).toBe(v.as_of);
    });
});

describe("helpers", () => {
  it("records the worked example", () => {
    const r = lifePathReduction("1990-11-29");
    expect(r.components.map((c) => c.reduced)).toEqual([11, 11, 1]);
    expect(r.total).toBe(23);
    expect(r.final).toEqual(["2+3=5"]);
  });
  it("keeps master numbers only when asked", () => {
    expect(reduceNumber(22)).toBe(22);
    expect(reduceNumber(22, false)).toBe(4);
  });
  it("rejects malformed dates", () => {
    expect(() => lifePath("1990-1-1")).toThrow(RangeError);
  });
  it("formats", () => {
    expect(romanNumeral(9)).toBe("IX");
    expect(romanNumeral(0)).toBe("0");
    expect(formatDateParts("1945-04-30")).toBe("30 / 04 / 1945");
  });
  it("validates birth dates against 1900 and today", () => {
    const today = new Date("2026-10-01T12:00:00");
    expect(validBirthDate(29, 2, 2023, today)).toBe(false);
    expect(validBirthDate(31, 12, 1899, today)).toBe(false);
    expect(validBirthDate(1, 10, 2026, today)).toBe(true);
  });
});
