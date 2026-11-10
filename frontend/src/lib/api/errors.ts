/**
 * Typed API errors. Every failure — HTTP status, network outage, timeout,
 * malformed response, realtime error frame — becomes an ApiError with a
 * stable `code`, so UI components can choose a localized message and a
 * recovery action without parsing strings.
 */

import { logger } from "@/lib/logger";

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
  | "feature_unavailable"
  | "internal_error"
  | "network_error"
  | "client_error"
  | "aborted"
  | "invalid_response"
  | "realtime_unavailable"
  | "step_up_required"
  | "invalid_code"
  | "limit_reached"
  // --- account settings & billing (identity module, §12) ---
  | "version_conflict"
  | "account_deactivated"
  | "account_closure_pending"
  | "closure_blocked"
  | "export_in_progress"
  | "subscription_exists"
  | "payment_method_required"
  // --- support desk (§16.4) ---
  | "report_exists"
  // --- messages outside live chats (§22.3) ---
  | "off_session_disabled"
  | "off_session_not_eligible"
  | "off_session_limit_reached"
  | "recipient_unavailable"
  // --- revisions 4.0 (§23) ---
  | "google_unavailable"
  | "google_state_invalid"
  | "google_cancelled"
  | "google_email_unverified"
  | "google_account_in_use"
  | "google_not_allowed"
  | "password_not_set"
  | "last_sign_in_method"
  | "reply_target_invalid"
  | "message_not_likeable";

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
    // Switched off by server configuration: retrying cannot succeed.
    if (this.code === "feature_unavailable") return false;
    return (
      this.code === "google_unavailable" ||
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

/**
 * Messages browsers raise when `fetch` itself fails (offline, DNS, CORS,
 * blocked). `api()`, `fetchCsrf()` and the refresh already wrap their own
 * rejections; this recognises raw fetch failures from other callers.
 */
const FETCH_FAILURE = /^(failed to fetch|load failed|networkerror when attempting to fetch resource|network request failed)/i;

const reported = new WeakSet<object>();

/** Normalize anything thrown into an ApiError. */
export function toApiError(error: unknown): ApiError {
  if (error instanceof ApiError) return error;
  if (error instanceof DOMException && error.name === "AbortError")
    return new ApiError({ code: "aborted", message: "Request cancelled.", cause: error });
  if (error instanceof TypeError && FETCH_FAILURE.test(error.message))
    return new ApiError({ code: "network_error", message: "Network request failed.", cause: error });
  // Anything else is a bug in this page (not a network problem): say so,
  // and log it once.
  if (typeof error === "object" && error !== null && !reported.has(error)) {
    reported.add(error);
    logger.warn("unexpected client error", error);
  }
  return new ApiError({
    code: "client_error",
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
  feature_unavailable: "This feature is not available yet. Please check back soon.",
  internal_error: "Something went wrong on our side. Please try again.",
  network_error: "You appear to be offline. Check your connection and try again.",
  client_error: "Something went wrong on this page. Please try again.",
  aborted: "The request was cancelled.",
  invalid_response: "We received an unexpected response. Please try again.",
  realtime_unavailable: "Live updates are reconnecting…",
  step_up_required: "Please confirm your identity with a verification code to continue.",
  invalid_code: "This verification code is incorrect or has expired.",
  limit_reached: "You have reached the limit for this. Remove older items to add new ones.",
  // --- account settings & billing (identity module, §12) ---
  version_conflict: "Your settings were changed on another device. Reload them and try again.",
  account_deactivated: "This account is deactivated. Reactivate it to continue.",
  account_closure_pending: "This account is scheduled for closure. Keep it to continue.",
  closure_blocked: "Your account cannot be closed yet.",
  export_in_progress: "Your previous export is still being prepared.",
  subscription_exists: "You already have a membership.",
  payment_method_required: "Add another card first: your membership renews with this one.",
  // --- support desk (§16.4) ---
  report_exists: "You already reported this conversation.",
  // --- messages outside live chats (§22.3) ---
  off_session_disabled: "Messages outside a live chat are turned off right now. Start a live chat to keep talking.",
  off_session_not_eligible: "You can write once the live chat starts.",
  off_session_limit_reached: "You've used today's messages outside a live chat. Start a live chat to keep talking.",
  recipient_unavailable: "This person can't receive messages right now.",
  // --- revisions 4.0 (§23) ---
  google_unavailable: "Google sign-in isn't available right now. Please try again, or use your email and password.",
  google_state_invalid: "That sign-in attempt expired. Please start again.",
  google_cancelled: "Google sign-in was cancelled. Nothing was changed.",
  google_email_unverified: "Google hasn't verified this email address yet. Verify it with Google, or sign up with your email instead.",
  google_account_in_use: "That Google account is already connected to another Staryield account.",
  google_not_allowed: "Google sign-in isn't available for this account. Please use your email and password.",
  password_not_set: "This needs your account password, and you haven't created one yet.",
  last_sign_in_method: "Create a password before you disconnect Google, so you can still sign in.",
  reply_target_invalid: "The message you replied to is no longer available.",
  message_not_likeable: "This message can't be liked.",
};

/** `network_error` while the browser believes it is online (API unreachable, blocked, DNS). */
export const unreachableCopy = "We couldn't reach Staryield. Please try again in a moment.";

/** The English copy for an error (translate with `text()`). */
export function errorMessage(error: unknown): string {
  const e = toApiError(error);
  // The copy depends on what the browser knows: "offline" only when it says so.
  if (e.code === "network_error" && typeof navigator !== "undefined" && navigator.onLine !== false) return unreachableCopy;
  return errorCopy[e.code] ?? errorCopy.internal_error;
}
