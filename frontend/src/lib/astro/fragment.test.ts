import { describe, expect, it } from "vitest";

import { decodeFragment, encodeFragment, FRAGMENT_VERSION } from "@/lib/astro/fragment";
import { calculatorSchemas, type NatalFormValues, type SunSignFormValues } from "@/lib/schemas/calculator";
import { fieldErrors, identityText } from "@/test/schema";

const natal: NatalFormValues = {
  birth: { day: "14", month: "03", year: "1990", time: "08:45" },
  timeUnknown: false,
  place: { id: "gn:2988507", label: "Paris, France" },
  houseSystem: "whole_sign",
};

describe("fragment encode/decode", () => {
  it("round-trips natal state without the place label", () => {
    const hash = encodeFragment("natal-chart", natal);
    expect(hash).toBe("#v=1&d=1990-03-14&t=08%3A45&p=gn%3A2988507&h=whole_sign");
    const decoded = decodeFragment("natal-chart", hash, identityText);
    expect(decoded.status).toBe("ok");
    if (decoded.status === "ok") expect(decoded.values).toEqual({ ...natal, place: { id: "gn:2988507", label: "" } });
  });
  it("omits default house system and unknown time", () => {
    const hash = encodeFragment("natal-chart", { ...natal, timeUnknown: true, place: null, houseSystem: "placidus", birth: { ...natal.birth, time: "" } });
    expect(hash).toBe(`#v=${FRAGMENT_VERSION}&d=1990-03-14`);
    const decoded = decodeFragment("natal-chart", hash, identityText);
    expect(decoded.status === "ok" && decoded.values).toMatchObject({ timeUnknown: true, houseSystem: "placidus" });
  });
  it("sun sign keeps optional time and place; path of life only the date", () => {
    const sun: SunSignFormValues = { birth: natal.birth, place: natal.place };
    expect(decodeFragment("astrology", encodeFragment("astrology", sun), identityText).status).toBe("ok");
    expect(encodeFragment("path-of-life", { birth: natal.birth })).toBe("#v=1&d=1990-03-14");
    expect(decodeFragment("path-of-life", "#v=1&d=1990-03-14&t=08:45", identityText)).toEqual({ status: "invalid", reason: "unknown_key" });
  });
  it("is empty without a fragment", () => {
    expect(decodeFragment("natal-chart", "", identityText)).toEqual({ status: "empty" });
    expect(decodeFragment("natal-chart", "#", identityText)).toEqual({ status: "empty" });
  });
});

describe("tampered fragments are rejected", () => {
  const cases: [string, string, string][] = [
    ["unknown key", "#v=1&d=1990-03-14&x=1", "unknown_key"],
    ["duplicate key", "#v=1&d=1990-03-14&d=1991-03-14", "duplicate"],
    ["bad version", "#v=2&d=1990-03-14", "version"],
    ["missing version", "#d=1990-03-14", "version"],
    ["bad date shape", "#v=1&d=1990-3-14", "format"],
    ["impossible date", "#v=1&d=1990-02-31", "schema"],
    ["future date", "#v=1&d=2999-01-01", "schema"],
    ["before 1900", "#v=1&d=1899-12-31", "schema"],
    ["script in place", "#v=1&d=1990-03-14&t=08:45&p=%3Cscript%3E", "format"],
    ["bad time shape", "#v=1&d=1990-03-14&t=8:45&p=gn:1", "format"], ["bad time value", "#v=1&d=1990-03-14&t=25:99&p=gn:1", "schema"],
    ["time out of range pattern ok but invalid", "#v=1&d=1990-03-14&t=24:30&p=gn:1", "schema"],
    ["time without place", "#v=1&d=1990-03-14&t=08:45", "schema"],
    ["unknown house system", "#v=1&d=1990-03-14&h=koch", "format"],
    ["oversized", `#v=1&d=1990-03-14&p=${"a".repeat(300)}`, "too_long"],
  ];
  for (const [name, hash, reason] of cases)
    it(name, () => {
      expect(decodeFragment("natal-chart", hash, identityText)).toEqual({ status: "invalid", reason });
    });
});

describe("calculator schemas", () => {
  const schema = calculatorSchemas["natal-chart"](identityText);
  it("needs a time unless unknown, and a place with a time", () => {
    expect(fieldErrors(schema, { ...natal, birth: { ...natal.birth, time: "" } }).birth).toMatch(/birth time/);
    expect(fieldErrors(schema, { ...natal, place: null }).place).toBe("Choose your birth place to use a birth time.");
    expect(fieldErrors(schema, { ...natal, timeUnknown: true, place: null, birth: { ...natal.birth, time: "" } })).toEqual({});
    expect(fieldErrors(schema, natal)).toEqual({});
  });
  it("rejects a free-text place id and unknown house systems", () => {
    expect(fieldErrors(schema, { ...natal, place: { id: "Not A Place!", label: "x" } })["place.id"]).toBeTruthy();
    expect(fieldErrors(schema, { ...natal, houseSystem: "koch" })["houseSystem"]).toBeTruthy();
  });
  it("sun sign: time optional but then needs a place", () => {
    const sun = calculatorSchemas.astrology(identityText);
    expect(fieldErrors(sun, { birth: { ...natal.birth, time: "" }, place: null })).toEqual({});
    expect(fieldErrors(sun, { birth: natal.birth, place: null }).place).toBeTruthy();
  });
  it("path of life needs a valid date only", () => {
    const lp = calculatorSchemas["path-of-life"](identityText);
    expect(fieldErrors(lp, { birth: { day: "", month: "", year: "", time: "" } }).birth).toBeTruthy();
    expect(fieldErrors(lp, { birth: { day: "31", month: "04", year: "2000", time: "" } }).birth).toBe("Enter a valid date.");
  });
});
