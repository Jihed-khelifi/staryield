import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { emptyOrder, lastOrderDateIso, ORDER_NOTE_MAX, orderSchema, preferredAt, todayLocalIso, type OrderValues } from "@/lib/schemas/requests";
import { fieldErrors, identityText as text } from "@/test/schema";

const schema = orderSchema(text);
const errorsFor = (patch: Partial<OrderValues>) => fieldErrors(schema, { ...emptyOrder, ...patch });

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 10, 5, 10, 0)); // 5 Nov 2026, 10:00 local
});
afterEach(() => vi.useRealTimers());

describe("orderSchema", () => {
  it("everything is optional", () => {
    expect(errorsFor({})).toEqual({});
    expect(errorsFor({ note: "Career question", date: "2026-11-20", time: "18:30" })).toEqual({});
    expect(errorsFor({ date: "2026-11-05" })).toEqual({});
  });

  it("limits the note to 2,000 characters", () => {
    expect(errorsFor({ note: "🌙".repeat(ORDER_NOTE_MAX) })).toEqual({});
    expect(errorsFor({ note: "x".repeat(ORDER_NOTE_MAX + 1) }).note).toBe("Use at most 2000 characters.");
  });

  it.each([
    [{ date: "2026-11-04" }, "date", "Choose today or a later date."],
    [{ date: "2026-02-30" }, "date", "Enter a valid date."],
    [{ date: "05/11/2026" }, "date", "Enter a valid date."],
    [{ date: "2027-11-05" }, "date", "Choose a date within the next year."],
    [{ time: "18:30" }, "time", "Choose a date for this time."],
    [{ date: "2026-11-06", time: "7:5" }, "time", "Use the 24-hour format HH:MM."],
    [{ date: "2026-11-05", time: "09:59" }, "time", "Choose a time later than now."],
  ] as const)("rejects %o", (patch, field, message) => {
    expect(errorsFor(patch)[field]).toBe(message);
  });

  it("accepts the last day the API allows and a later time today", () => {
    expect(lastOrderDateIso()).toBe("2027-11-04");
    expect(errorsFor({ date: "2027-11-04", time: "23:59" })).toEqual({});
    expect(errorsFor({ date: "2026-11-05", time: "10:00" })).toEqual({});
  });
});

describe("order dates", () => {
  it("todayLocalIso uses the local calendar day", () => {
    expect(todayLocalIso()).toBe("2026-11-05");
    expect(todayLocalIso(new Date(2026, 0, 9, 23, 59))).toBe("2026-01-09");
  });

  it("lastOrderDateIso handles month and leap-year edges", () => {
    expect(lastOrderDateIso(new Date(2026, 0, 1))).toBe("2026-12-31");
    expect(lastOrderDateIso(new Date(2028, 1, 29))).toBe("2029-02-28");
  });

  it("preferredAt builds a local instant, noon by default", () => {
    expect(preferredAt({ date: "", time: "" })).toBeUndefined();
    expect(preferredAt({ date: "2026-11-20", time: "18:30" })).toBe(new Date(2026, 10, 20, 18, 30).toISOString());
    expect(preferredAt({ date: "2026-11-20", time: "" })).toBe(new Date(2026, 10, 20, 12, 0).toISOString());
  });
});
