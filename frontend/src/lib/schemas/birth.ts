/**
 * Onboarding "Date and time of Birth" + "Location" step.
 *
 * - The date is required. Whether the person is 18 or older is decided by the
 *   API, not here (revisions 5.0, §24.6): an under-age date must reach it, so
 *   that the account is restricted and the date cannot simply be retyped.
 * - The time is required unless "I don't know my time" is ticked (the chart
 *   then uses local noon and skips the rising sign and houses).
 * - A birthplace is required whenever a time is given: a clock time only
 *   means something with the place's time zone (the API rejects a time
 *   without a place). Without a time the place stays optional.
 */
import * as yup from "yup"

import type { TranslateCopy } from "@/i18n/translate-copy"
import { birthPartsRule, placeRefRule, type BirthParts } from "@/lib/schemas/common"

export type PlaceRef = { id: string; label: string }

export const birthInfoSchema = (text: TranslateCopy) =>
  yup.object({
    timeUnknown: yup.boolean().default(false),
    birth: birthPartsRule(text).when("timeUnknown", ([timeUnknown], schema) =>
      timeUnknown
        ? schema
        : schema.test("time-required", text("Enter your birth time, or tick “I don't know my time”."), (value) =>
            Boolean(value?.time)
          )
    ),
    place: placeRefRule(text, false).when(["timeUnknown", "birth"], ([timeUnknown, birth], schema) =>
      !timeUnknown && (birth as BirthParts | undefined)?.time
        ? schema.test("place-required", text("Choose where you were born so we can place your rising sign."), (value) =>
            Boolean(value?.id)
          )
        : schema
    ),
  })

export type BirthInfoValues = {
  timeUnknown: boolean
  birth: BirthParts
  place: PlaceRef | null
}

/** The PATCH /me/profile body for a validated step. */
export function birthInfoPayload(values: BirthInfoValues): {
  birth_date: string
  birth_time: string
  birth_place_id: string
} {
  const { day, month, year, time = "" } = values.birth
  return {
    birth_date: `${year}-${month.padStart(2, "0")}-${day.padStart(2, "0")}`,
    birth_time: values.timeUnknown ? "" : time,
    birth_place_id: values.place?.id ?? "",
  }
}
