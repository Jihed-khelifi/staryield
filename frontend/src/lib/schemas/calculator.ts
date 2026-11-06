/**
 * The three public calculators' forms (react-hook-form + yup). The same
 * schemas validate the state restored from the URL fragment, so a shared or
 * edited link can never send anything a visitor could not have typed.
 *
 * - Natal chart: date; a birth time unless "I don't know my birth time" is
 *   ticked; a birthplace whenever a time is given (a clock time only means
 *   something in the place's time zone — the API also answers 422 on
 *   `place_id`); the house system.
 * - Astrology (sun sign): date; time and place are optional and only refine
 *   cusp birthdays, with the same time-needs-a-place rule.
 * - Path of Life: date only (computed in the browser).
 */
import * as yup from "yup";

import type { TranslateCopy } from "@/i18n/translate-copy";
import { HOUSE_SYSTEMS, type HouseSystem } from "@/lib/api/astrology";
import { birthPartsRule, type BirthParts } from "@/lib/schemas/common";

export type CalculatorKind = "natal-chart" | "astrology" | "path-of-life";

/** A gazetteer place picked from the suggestions ("gn:2988507"; legacy "paris-fr"). */
export type CalculatorPlace = { id: string; label: string };

/** Gazetteer ids: GeoNames ("gn:2988507") or the legacy slugs ("paris-fr"). */
export const PLACE_ID_RULE = /^(?:gn:\d{1,10}|[a-z]{2,40}(?:-[a-z]{2,40}){0,5})$/;

export type NatalFormValues = {
  birth: BirthParts;
  timeUnknown: boolean;
  place: CalculatorPlace | null;
  houseSystem: HouseSystem;
};

export type SunSignFormValues = {
  birth: BirthParts;
  place: CalculatorPlace | null;
};

export type LifePathFormValues = {
  birth: BirthParts;
};

export type CalculatorFormValues = NatalFormValues | SunSignFormValues | LifePathFormValues;

function placeRule(text: TranslateCopy) {
  return yup
    .object({
      id: yup
        .string()
        .default("")
        .matches(PLACE_ID_RULE, { excludeEmptyString: true, message: text("Choose a birth place from the list.") }),
      label: yup.string().default(""),
    })
    .nullable()
    .default(null)
    .test("picked", text("Choose a birth place from the list."), (value) => value === null || Boolean(value?.id));
}

/** Natal chart: the time is required unless unknown, and a time needs a place. */
export const natalSchema = (text: TranslateCopy) =>
  yup.object({
    timeUnknown: yup.boolean().default(false),
    birth: birthPartsRule(text).when("timeUnknown", ([timeUnknown], schema) =>
      timeUnknown
        ? schema
        : schema.test("time-required", text("Enter your birth time, or tick “I don't know my birth time”."), (value) =>
            Boolean(value?.time)
          )
    ),
    place: placeRule(text).when(["timeUnknown", "birth"], ([timeUnknown, birth], schema) =>
      !timeUnknown && (birth as BirthParts | undefined)?.time
        ? schema.test("place-required", text("Choose your birth place to use a birth time."), (value) => Boolean(value?.id))
        : schema
    ),
    houseSystem: yup
      .mixed<HouseSystem>()
      .oneOf([...HOUSE_SYSTEMS], text("Choose Placidus, Whole Sign or Equal houses."))
      .default("placidus")
      .required(text("Choose Placidus, Whole Sign or Equal houses.")),
  });

/** Sun sign: an optional time and place settle cusp birthdays. */
export const sunSignSchema = (text: TranslateCopy) =>
  yup.object({
    birth: birthPartsRule(text),
    place: placeRule(text).when("birth", ([birth], schema) =>
      (birth as BirthParts | undefined)?.time
        ? schema.test("place-required", text("Choose your birth place to use a birth time."), (value) => Boolean(value?.id))
        : schema
    ),
  });

/** Path of Life: the birth date alone. */
export const lifePathSchema = (text: TranslateCopy) =>
  yup.object({
    birth: birthPartsRule(text),
  });

export const calculatorSchemas = {
  "natal-chart": natalSchema,
  astrology: sunSignSchema,
  "path-of-life": lifePathSchema,
} as const;

/** "YYYY-MM-DD" of validated parts. */
export function birthDateOf(birth: BirthParts): string {
  return `${birth.year}-${birth.month.padStart(2, "0")}-${birth.day.padStart(2, "0")}`;
}

/** Empty form values for a calculator. */
export function emptyCalculatorValues(kind: CalculatorKind): CalculatorFormValues {
  const birth: BirthParts = { day: "", month: "", year: "", time: "" };
  if (kind === "natal-chart") return { birth, timeUnknown: false, place: null, houseSystem: "placidus" };
  if (kind === "astrology") return { birth, place: null };
  return { birth };
}

/** API field → form path, for mapping 422 `details.fields` onto the form. */
export const CALCULATOR_FIELD_MAP = {
  birth_date: "birth",
  birth_time: "birth",
  place_id: "place",
  house_system: "houseSystem",
} as const;
