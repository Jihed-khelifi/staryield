import type { FieldPath, FieldValues, UseFormReturn } from "react-hook-form"

import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors"
import type { TranslateCopy } from "@/i18n/translate-copy"

type Options<T extends FieldValues> = {
  /** Map API field names to form paths, e.g. `{ birth_date: "birth" }`. */
  fieldMap?: Partial<Record<string, FieldPath<T>>>
  /** Translator for the generic (non-field) message. */
  text: TranslateCopy
  /** Focus the first field that received a server error (default true). */
  focus?: boolean
}

function hasPath(values: unknown, path: string): boolean {
  let node: unknown = values
  for (const key of path.split(".")) {
    if (node === null || typeof node !== "object" || !(key in (node as Record<string, unknown>))) return false
    node = (node as Record<string, unknown>)[key]
  }
  return true
}

/**
 * Put an API failure onto a react-hook-form form:
 *  - `validation_failed` field messages (already localized by the API) go to
 *    their fields; fields the form doesn't have fall back to the form-level
 *    error, so nothing is silently dropped;
 *  - any other failure becomes `errors.root.server` with the localized copy
 *    for its code, rendered by <FormRootError>.
 * Returns the normalized ApiError so callers can branch on `code`.
 */
export function applyApiError<T extends FieldValues>(
  form: Pick<UseFormReturn<T>, "setError" | "getValues" | "setFocus">,
  error: unknown,
  { fieldMap = {}, text, focus = true }: Options<T>
): ApiError {
  const e = toApiError(error)
  if (e.code === "aborted") return e
  const values = form.getValues()
  let first: FieldPath<T> | null = null
  let unmapped = false

  if (e.code === "validation_failed") {
    for (const [field, message] of Object.entries(e.fields)) {
      const path = (fieldMap[field] ?? field) as FieldPath<T>
      if (typeof message === "string" && message && hasPath(values, path)) {
        form.setError(path, { type: "server", message })
        first ??= path
      } else {
        unmapped = true
      }
    }
    if (!first) unmapped = true
  }

  if (e.code !== "validation_failed" || unmapped) {
    form.setError("root.server" as FieldPath<T>, { type: e.code, message: text(errorMessage(e)) })
  }
  if (focus && first) {
    try {
      form.setFocus(first)
    } catch {
      // Field not registered with a ref (custom control); focus is optional.
    }
  }
  return e
}
