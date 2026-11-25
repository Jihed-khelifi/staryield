/**
 * The compatibility calculator's form: two birth dates (yours and theirs),
 * each validated exactly like the calculators' date (a real calendar date
 * from 1900 to today, the API's rule for POST /astrology/sun-sign).
 */
import * as yup from "yup";

import type { TranslateCopy } from "@/i18n/translate-copy";
import { birthPartsRule, type BirthParts } from "@/lib/schemas/common";

export type CompatibilityPerson = "you" | "them";

export type CompatibilityFormValues = Record<CompatibilityPerson, BirthParts>;

export const compatibilitySchema = (text: TranslateCopy) =>
  yup.object({
    you: birthPartsRule(text),
    them: birthPartsRule(text),
  });

export function emptyCompatibilityValues(): CompatibilityFormValues {
  return { you: { day: "", month: "", year: "", time: "" }, them: { day: "", month: "", year: "", time: "" } };
}
