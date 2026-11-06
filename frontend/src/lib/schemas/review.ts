/** Rate a finished session (§9.2): 1–5 stars and an optional comment. */
import * as yup from "yup"

import type { TranslateCopy } from "@/i18n/translate-copy"
import { trimmedText } from "@/lib/schemas/common"

export const REVIEW_COMMENT_MAX = 1000

export const reviewSchema = (text: TranslateCopy) => {
  const stars = text("Choose a rating from 1 to 5 stars.")
  return yup.object({
    rating: yup
      .number()
      .transform((value, original) => (original === "" || original === null || original === 0 ? undefined : value))
      .typeError(stars)
      .required(stars)
      .integer(stars)
      .min(1, stars)
      .max(5, stars),
    comment: trimmedText(text, { max: REVIEW_COMMENT_MAX, required: false }),
  })
}

export type ReviewValues = yup.InferType<ReturnType<typeof reviewSchema>>
