/**
 * "Report an issue" about a chat conversation → POST /api/v1/me/support/chat-reports
 * (docs/platform-extensions.md §16.4). Limits mirror the API: details 0–2,000
 * characters, 10–2,000 when the reason is "other".
 */
import * as yup from "yup"

import type { TranslateCopy } from "@/i18n/translate-copy"
import type { ChatReportReason } from "@/lib/api/types"

export type { ChatReportReason }

/** Offered reasons, in display order (must match the API allow-list). */
export const CHAT_REPORT_REASONS: readonly ChatReportReason[] = ["advisor_unresponsive", "inappropriate", "billing", "technical", "other"]

export const CHAT_REPORT_DETAILS_MAX = 2000
export const CHAT_REPORT_DETAILS_MIN_OTHER = 10

/** Untranslated reason labels (rendered through `text()`). */
export const chatReportReasonLabels: Record<ChatReportReason, string> = {
  advisor_unresponsive: "The advisor stopped responding",
  inappropriate: "Inappropriate or offensive behaviour",
  billing: "A billing or credits problem",
  technical: "A technical problem (messages, images, connection)",
  other: "Something else",
}

export function isChatReportReason(value: unknown): value is ChatReportReason {
  return typeof value === "string" && (CHAT_REPORT_REASONS as readonly string[]).includes(value)
}

const runes = (value: string | undefined) => Array.from(value ?? "").length

export const chatReportSchema = (text: TranslateCopy) =>
  yup.object({
    reason: yup
      .mixed<ChatReportReason>()
      .oneOf([...CHAT_REPORT_REASONS], text("Choose what went wrong."))
      .required(text("Choose what went wrong.")),
    details: yup
      .string()
      .trim()
      .default("")
      .test("max-chars", text("Use at most {value0} characters.", { value0: CHAT_REPORT_DETAILS_MAX }), (value) => runes(value) <= CHAT_REPORT_DETAILS_MAX)
      .test("other-needs-details", text("Tell us what happened (at least {value0} characters).", { value0: CHAT_REPORT_DETAILS_MIN_OTHER }), function (value) {
        return this.parent.reason !== "other" || runes(value) >= CHAT_REPORT_DETAILS_MIN_OTHER
      }),
  })

export type ChatReportValues = yup.InferType<ReturnType<typeof chatReportSchema>>
