/**
 * Contact form → POST /api/v1/support/tickets. Limits mirror the API
 * (internal/service/support.go): subject 3–120, message 10–5,000 characters.
 */
import * as yup from "yup"

import type { TranslateCopy } from "@/i18n/translate-copy"
import type { SupportTopic } from "@/lib/api/types"
import { consentRule, emailRule, trimmedText } from "@/lib/schemas/common"

export type { SupportTopic }

/** Offered topics, in display order (must match the API allow-list). */
export const SUPPORT_TOPICS: readonly SupportTopic[] = ["general", "privacy", "billing", "advisor", "technical"]

/** Untranslated topic labels (rendered through `text()`). */
export const supportTopicLabels: Record<SupportTopic, string> = {
  general: "General question",
  privacy: "Privacy & my data",
  billing: "Billing & credits",
  advisor: "An advisor or a reading",
  technical: "Technical problem",
}

export function isSupportTopic(value: unknown): value is SupportTopic {
  return typeof value === "string" && (SUPPORT_TOPICS as readonly string[]).includes(value)
}

export const contactSchema = (text: TranslateCopy) =>
  yup.object({
    topic: yup
      .mixed<SupportTopic>()
      .oneOf([...SUPPORT_TOPICS], text("Choose a topic from the list."))
      .required(text("Choose a topic from the list.")),
    subject: trimmedText(text, { min: 3, max: 120 }),
    email: emailRule(text),
    message: trimmedText(text, { min: 10, max: 5000 }),
    consent: consentRule(text),
  })

export type ContactValues = yup.InferType<ReturnType<typeof contactSchema>>
