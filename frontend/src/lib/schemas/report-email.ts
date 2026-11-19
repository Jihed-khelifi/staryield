/**
 * The "Save your reading" dialog (revisions 6.0, §26.4): the e-mail to send
 * the calculator's report to, the marketing box and the honeypot.
 */
import * as yup from "yup";

import type { TranslateCopy } from "@/i18n/translate-copy";
import { emailRule } from "@/lib/schemas/common";

export const reportEmailSchema = (text: TranslateCopy) =>
  yup.object({
    email: emailRule(text),
    /** "Send me horoscopes, offers and news…": never required. */
    marketing: yup.boolean().default(false),
    /** Honeypot (hidden from people): anything typed here is a bot. */
    website: yup.string().default("").max(500),
  });

export type ReportEmailValues = yup.InferType<ReturnType<typeof reportEmailSchema>>;
