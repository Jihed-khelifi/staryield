/**
 * Typed API errors. Every failure — HTTP status, network outage, timeout,
 * malformed response, realtime error frame — becomes an ApiError with a
 * stable `code`, so UI components can choose a localized message and a
 * recovery action without parsing strings.
 */

export type ErrorCode =
  | "bad_request"
  | "validation_failed"
  | "unauthenticated"
  | "invalid_credentials"
  | "session_expired"
  | "forbidden"
  | "csrf_failed"
  | "account_disabled"
  | "account_locked"
  | "not_found"
  | "conflict"
  | "email_taken"
  | "insufficient_credits"
  | "psychic_unavailable"
  | "session_already_open"
  | "invalid_state"
  | "rate_limited"
  | "payload_too_large"
  | "unsupported_media_type"
  | "invalid_token"
  | "payment_failed"
  | "refresh_conflict"
  | "timeout"
  | "service_unavailable"
  | "internal_error"
  | "network_error"
  | "aborted"
  | "invalid_response"
  | "realtime_unavailable";

export class ApiError extends Error {
  readonly code: ErrorCode | string;
  readonly status: number;
  readonly details?: Record<string, unknown>;
  readonly requestId?: string;
  readonly retryAfter?: number;

  constructor(init: {
    code: ErrorCode | string;
    message: string;
    status?: number;
    details?: Record<string, unknown>;
    requestId?: string;
    retryAfter?: number;
    cause?: unknown;
  }) {
    super(init.message, { cause: init.cause });
    this.name = "ApiError";
    this.code = init.code;
    this.status = init.status ?? 0;
    this.details = init.details;
    this.requestId = init.requestId;
    this.retryAfter = init.retryAfter;
  }

  /** Field-level validation messages, keyed by field name. */
  get fields(): Record<string, string> {
    const fields = this.details?.fields;
    return fields && typeof fields === "object"
      ? (fields as Record<string, string>)
      : {};
  }

  /** True when retrying the same request may succeed. */
  get retryable(): boolean {
    return (
      this.code === "network_error" ||
      this.code === "timeout" ||
      this.code === "service_unavailable" ||
      this.code === "rate_limited" ||
      this.status >= 500
    );
  }
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

/** Normalize anything thrown into an ApiError. */
export function toApiError(error: unknown): ApiError {
  if (error instanceof ApiError) return error;
  if (error instanceof DOMException && error.name === "AbortError")
    return new ApiError({ code: "aborted", message: "Request cancelled.", cause: error });
  if (error instanceof TypeError)
    return new ApiError({ code: "network_error", message: "Network request failed.", cause: error });
  return new ApiError({
    code: "internal_error",
    message: error instanceof Error ? error.message : "Unexpected error.",
    cause: error,
  });
}

/**
 * English source copy per error code. Components pass these through the
 * i18n `text()` helper, so every message is localized (copy-*.json).
 */
export const errorCopy: Record<string, string> = {
  bad_request: "Something about this request was not right. Please try again.",
  validation_failed: "Some fields need attention.",
  unauthenticated: "Please sign in to continue.",
  invalid_credentials: "The email or password is incorrect.",
  session_expired: "Your session has expired. Please sign in again.",
  forbidden: "You do not have access to this page.",
  csrf_failed: "Your session security token expired. Please retry.",
  account_disabled: "This account has been deactivated. Please contact support.",
  account_locked: "Too many sign-in attempts. Please try again later.",
  not_found: "We could not find what you were looking for.",
  conflict: "This change conflicts with the current state. Please refresh.",
  email_taken: "An account with this email already exists.",
  insufficient_credits: "You do not have enough credits for this.",
  psychic_unavailable: "This advisor is not available right now.",
  session_already_open: "You already have a live chat in progress.",
  invalid_state: "This action is not possible right now.",
  rate_limited: "Too many requests. Please wait a moment.",
  payload_too_large: "This file or message is too large.",
  unsupported_media_type: "This file type is not supported.",
  invalid_token: "This link is invalid or has expired.",
  payment_failed: "Your payment was declined. No credits were added.",
  refresh_conflict: "Your session was refreshed in another tab. Please retry.",
  timeout: "The request took too long. Please try again.",
  service_unavailable: "The service is temporarily unavailable. Please try again.",
  internal_error: "Something went wrong on our side. Please try again.",
  network_error: "You appear to be offline. Check your connection and try again.",
  aborted: "The request was cancelled.",
  invalid_response: "We received an unexpected response. Please try again.",
  realtime_unavailable: "Live updates are reconnecting…",
};

/** The English copy for an error (translate with `text()`). */
export function errorMessage(error: unknown): string {
  const e = toApiError(error);
  return errorCopy[e.code] ?? errorCopy.internal_error;
}
