/**
 * Conversation reports (docs/platform-extensions.md §16.4): how a failed
 * report is shown, and the helpers shared by the list and the detail screen.
 * Pure, so it is unit-tested.
 */
import { toApiError } from "@/lib/api/errors";
import type { CustomerTicketRow } from "@/lib/api/types";
import { SUPPORT_REFERENCE } from "@/lib/support/support";

export type ReportErrorKind =
  | "fields" // 422: field messages go to the form
  | "exists" // 409 report_exists: an open report already covers this conversation
  | "rate_limited"
  | "retry" // network, timeout, 5xx: the same request may succeed
  | "sign_in" // 401
  | "forbidden" // 403: only clients can report
  | "not_found" // 404: the conversation is gone (or is not the caller's)
  | "other"

export type ReportFailure = { kind: ReportErrorKind; retryAfter?: number; reference?: string }

/** The reference carried by a 409 `report_exists`, when it looks like one. */
export function existingReference(details: Record<string, unknown> | undefined): string | undefined {
  const reference = details?.reference
  return typeof reference === "string" && SUPPORT_REFERENCE.test(reference) ? reference : undefined
}

export function classifyReportError(error: unknown): ReportFailure {
  const e = toApiError(error)
  switch (e.code) {
    case "validation_failed":
      return { kind: "fields" }
    case "report_exists":
      return { kind: "exists", reference: existingReference(e.details) }
    case "rate_limited":
      return { kind: "rate_limited", retryAfter: e.retryAfter && e.retryAfter > 0 ? Math.ceil(e.retryAfter) : undefined }
    case "unauthenticated":
    case "session_expired":
      return { kind: "sign_in" }
    case "forbidden":
      return { kind: "forbidden" }
    case "not_found":
      return { kind: "not_found" }
    default:
      return { kind: e.retryable ? "retry" : "other" }
  }
}

/** A conversation report with its context (older API builds omit `kind`: those are contact requests). */
export function isChatReport(row: Pick<CustomerTicketRow, "kind" | "report">): row is typeof row & { kind: "chat_report"; report: NonNullable<CustomerTicketRow["report"]> } {
  return row.kind === "chat_report" && Boolean(row.report)
}

/** Link to the conversation the report is about (signed-in views): `/{lang}/chatroom?reader={slug}`. */
export function reportConversationHref(locale: string, row: Pick<CustomerTicketRow, "report">): string | null {
  const slug = row.report?.advisor?.slug
  return slug ? `/${locale}/chatroom?reader=${encodeURIComponent(slug)}` : null
}
