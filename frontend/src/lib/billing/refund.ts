/**
 * Refund requests in the billing history (docs/platform-extensions.md §24.4):
 * the reasons a client can give, the copy for every request status, and how a
 * failed request is told. Pure, so every rule is unit-tested; the dialogs only
 * render what is decided here.
 */
import * as yup from "yup";

import type { TranslateCopy } from "@/i18n/translate-copy";
import { errorMessage, type ApiError } from "@/lib/api/errors";
import type { RefundReason, RefundRequestStatus, TransactionRefundInfo } from "@/lib/api/pricing";
import { trimmedText } from "@/lib/schemas/common";

/** Policy window and contact: the e-mail channel always works (§24.4). */
export const REFUND_SUPPORT_EMAIL = "support@staryield.net";
export const REFUND_EMAIL_SUBJECT = "Refund Request";
export const REFUND_DETAILS_MAX = 2000;

export const REFUND_REASONS: readonly RefundReason[] = [
  "unused_credits",
  "technical_problem",
  "incorrect_charge",
  "unauthorized_charge",
  "reading_concern",
  "other",
];

/** English source copy for each reason (translate with `text()`). */
export const REFUND_REASON_COPY: Record<RefundReason, string> = {
  unused_credits: "I have credits I haven't used",
  technical_problem: "Something went wrong technically",
  incorrect_charge: "I was charged the wrong amount",
  unauthorized_charge: "I don't recognize this charge",
  reading_concern: "I have a concern about a reading",
  other: "Something else",
};

export const REFUND_STATUS_COPY: Record<RefundRequestStatus, string> = {
  open: "Refund requested",
  approved: "Refund approved",
  denied: "Refund declined",
  credited: "Credits granted",
  withdrawn: "Request withdrawn",
};

export const refundSchema = (text: TranslateCopy) =>
  yup.object({
    reason: yup
      .string()
      .oneOf([...REFUND_REASONS], text("Choose a reason."))
      .required(text("Choose a reason.")),
    details: trimmedText(text, { max: REFUND_DETAILS_MAX, required: false }),
  });

export type RefundFormValues = { reason: RefundReason; details: string };

/** What the history row offers for a credit purchase. */
export type RefundRowState =
  | { kind: "none" }
  | { kind: "request" }
  /** A request exists: its status and reference; `withdrawable` while open. */
  | { kind: "status"; status: RefundRequestStatus; reference: string; id: string; withdrawable: boolean; canRequestAgain: boolean };

export function refundRowState(info: TransactionRefundInfo | undefined): RefundRowState {
  if (!info) return { kind: "none" };
  if (info.request) {
    return {
      kind: "status",
      status: info.request.status,
      reference: info.request.reference,
      id: info.request.id,
      withdrawable: info.request.status === "open",
      canRequestAgain: info.can_request && info.request.status !== "open",
    };
  }
  return info.can_request ? { kind: "request" } : { kind: "none" };
}

/** `refund_not_eligible` reasons, in the client's words. */
export const REFUND_NOT_ELIGIBLE_COPY: Record<string, string> = {
  not_credit_purchase: "Only credit purchases can be refunded here.",
  not_paid: "This payment was not completed, so there is nothing to refund.",
  fully_refunded: "This payment has already been fully refunded.",
  outside_window: "The refund window for unused credits has ended. You can still tell us about a problem with this payment.",
};

export type RefundFailure =
  /** An open request already exists: show it (the list is reloaded). */
  | { kind: "exists"; requestId?: string }
  | { kind: "not_eligible"; reason: string; message: string }
  /** In-app requests are switched off: offer the e-mail channel. */
  | { kind: "email" }
  | { kind: "rate_limited"; message: string }
  | { kind: "error"; message: string; retryable: boolean };

export function classifyRefundFailure(error: ApiError): RefundFailure {
  switch (error.code) {
    case "refund_request_exists": {
      const id = error.details?.request_id;
      return { kind: "exists", requestId: typeof id === "string" ? id : undefined };
    }
    case "refund_not_eligible": {
      const reason = typeof error.details?.reason === "string" ? error.details.reason : "";
      return { kind: "not_eligible", reason, message: REFUND_NOT_ELIGIBLE_COPY[reason] ?? errorMessage(error) };
    }
    case "feature_unavailable":
      return { kind: "email" };
    case "rate_limited":
      return { kind: "rate_limited", message: "You have sent several refund requests recently. Please try again later or write to us by e-mail." };
  }
  return { kind: "error", message: errorMessage(error), retryable: error.retryable };
}

/** mailto: link for the e-mail channel. */
export function refundMailto(): string {
  return `mailto:${REFUND_SUPPORT_EMAIL}?subject=${encodeURIComponent(REFUND_EMAIL_SUBJECT)}`;
}
