import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { birthInfoPayload, birthInfoSchema, type BirthInfoValues } from "@/lib/schemas/birth";
import { fieldErrors, identityText as text } from "@/test/schema";

const schema = birthInfoSchema(text);
const place = { id: "2988507", label: "Paris, Île-de-France, France" };
const valid: BirthInfoValues = {
  timeUnknown: false,
  birth: { day: "14", month: "3", year: "1990", time: "08:45" },
  place,
};
const errorsFor = (patch: Partial<BirthInfoValues>) => fieldErrors(schema, { ...valid, ...patch });

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 10, 5, 10, 0));
});
afterEach(() => vi.useRealTimers());

describe("birthInfoSchema", () => {
  it("accepts a date, time and birthplace", () => {
    expect(errorsFor({})).toEqual({});
  });

  it("requires the time unless it is unknown", () => {
    expect(errorsFor({ birth: { ...valid.birth, time: "" } })).toEqual({
      birth: "Enter your birth time, or tick “I don't know my time”.",
    });
    expect(errorsFor({ timeUnknown: true, birth: { ...valid.birth, time: "" } })).toEqual({});
  });

  it("requires a birthplace with a time, and keeps it optional without one", () => {
    expect(errorsFor({ place: null })).toEqual({ place: "Choose where you were born so we can place your rising sign." });
    expect(errorsFor({ timeUnknown: true, birth: { ...valid.birth, time: "" }, place: null })).toEqual({});
  });

  it.each([
    [{ day: "", month: "3", year: "1990", time: "08:45" }, "Enter your date of birth."],
    [{ day: "31", month: "2", year: "1990", time: "08:45" }, "Enter a valid date."],
    [{ day: "6", month: "11", year: "2008", time: "08:45" }, "You must be at least 18 years old."],
    [{ day: "14", month: "3", year: "1990", time: "24:00" }, "Use the 24-hour format HH:MM."],
  ])("rejects %o", (birth, message) => {
    expect(errorsFor({ birth }).birth).toBe(message);
  });

  it("accepts someone who turns 18 today", () => {
    expect(errorsFor({ birth: { day: "5", month: "11", year: "2008", time: "08:45" } })).toEqual({});
  });
});

describe("birthInfoPayload", () => {
  it("builds the PATCH /me/profile body", () => {
    expect(birthInfoPayload(valid)).toEqual({ birth_date: "1990-03-14", birth_time: "08:45", birth_place_id: "2988507" });
  });

  it("sends an empty time when it is unknown and an empty place when none was picked", () => {
    expect(birthInfoPayload({ ...valid, timeUnknown: true, place: null })).toEqual({
      birth_date: "1990-03-14",
      birth_time: "",
      birth_place_id: "",
    });
  });
});
