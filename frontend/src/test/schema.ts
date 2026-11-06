/**
 * Helpers for schema unit tests: an identity translator (English source copy
 * with placeholders filled) and per-field errors reported the way the
 * react-hook-form yup resolver reports them (first message per path).
 */
import * as yup from "yup";

import type { TranslateCopy } from "@/i18n/translate-copy";
import type { TranslationValues } from "@/i18n/types";

export const identityText: TranslateCopy = <T,>(value: T, values?: TranslationValues): T =>
  typeof value === "string" && values
    ? (value.replace(/\{(\w+)\}/g, (match, name: string) => (name in values ? String(values[name]) : match)) as T)
    : value;

/** First error message per field path ({} when the values are valid). */
export function fieldErrors(schema: yup.AnySchema, values: unknown, context?: object): Record<string, string> {
  try {
    schema.validateSync(values, { abortEarly: false, context });
    return {};
  } catch (error) {
    if (!(error instanceof yup.ValidationError)) throw error;
    const out: Record<string, string> = {};
    for (const e of error.inner.length ? error.inner : [error]) {
      const path = e.path ?? "";
      if (!(path in out)) out[path] = e.message;
    }
    return out;
  }
}
