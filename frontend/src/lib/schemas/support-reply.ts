/** Reply composer of a support request (1–5,000 characters, §16.2). */
import * as yup from "yup"

import type { TranslateCopy } from "@/i18n/translate-copy"
import { trimmedText } from "@/lib/schemas/common"
import { REPLY_MAX } from "@/lib/support/support"

export const supportReplySchema = (text: TranslateCopy) =>
  yup.object({
    body: trimmedText(text, { min: 1, max: REPLY_MAX }),
  })

export type SupportReplyValues = yup.InferType<ReturnType<typeof supportReplySchema>>
