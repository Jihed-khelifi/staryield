// Package apperror defines typed application errors. Services return these
// errors; the HTTP and WebSocket layers translate them into stable, safe
// client-facing payloads while the wrapped cause is only ever logged.
package apperror

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Code is a stable, machine-readable error identifier shared with clients.
type Code string

// Error codes. Keep in sync with the frontend error catalogue.
const (
	CodeBadRequest          Code = "bad_request"
	CodeValidation          Code = "validation_failed"
	CodeUnauthenticated     Code = "unauthenticated"
	CodeInvalidCredentials  Code = "invalid_credentials"
	CodeSessionExpired      Code = "session_expired"
	CodeForbidden           Code = "forbidden"
	CodeCSRF                Code = "csrf_failed"
	CodeAccountDisabled     Code = "account_disabled"
	CodeAccountLocked       Code = "account_locked"
	CodeNotFound            Code = "not_found"
	CodeConflict            Code = "conflict"
	CodeEmailTaken          Code = "email_taken"
	CodeInsufficientCredits Code = "insufficient_credits"
	CodePsychicUnavailable  Code = "psychic_unavailable"
	CodeSessionOpen         Code = "session_already_open"
	CodeInvalidState        Code = "invalid_state"
	CodeRateLimited         Code = "rate_limited"
	CodePayloadTooLarge     Code = "payload_too_large"
	CodeUnsupportedMedia    Code = "unsupported_media_type"
	CodeInvalidToken        Code = "invalid_token"
	CodePaymentFailed       Code = "payment_failed"
	CodeTimeout             Code = "timeout"
	CodeUnavailable         Code = "service_unavailable"
	CodeInternal            Code = "internal_error"
)

// Error is an application error with a safe message and an internal cause.
type Error struct {
	Code       Code
	Status     int
	Message    string
	Details    map[string]any
	RetryAfter time.Duration
	Err        error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// Is matches errors by Code so callers can use errors.Is(err, apperror.ErrNotFound).
func (e *Error) Is(target error) bool {
	var t *Error
	if errors.As(target, &t) {
		return t.Code == e.Code
	}
	return false
}

// WithDetails returns a copy with the given details merged in.
func (e *Error) WithDetails(details map[string]any) *Error {
	cp := *e
	cp.Details = make(map[string]any, len(e.Details)+len(details))
	for k, v := range e.Details {
		cp.Details[k] = v
	}
	for k, v := range details {
		cp.Details[k] = v
	}
	return &cp
}

// Wrap returns a copy that wraps cause for logging.
func (e *Error) Wrap(cause error) *Error {
	cp := *e
	cp.Err = cause
	return &cp
}

// New creates an error.
func New(code Code, status int, message string) *Error {
	return &Error{Code: code, Status: status, Message: message}
}

// Sentinels for errors.Is comparisons.
var (
	ErrNotFound     = New(CodeNotFound, http.StatusNotFound, "Resource not found.")
	ErrInvalidState = New(CodeInvalidState, http.StatusConflict, "This action is not possible right now.")
	ErrForbidden    = New(CodeForbidden, http.StatusForbidden, "You do not have access to this resource.")
)

// Constructors.

func BadRequest(message string) *Error { return New(CodeBadRequest, http.StatusBadRequest, message) }

// Validation reports field-level validation failures.
func Validation(fields map[string]string) *Error {
	details := make(map[string]any, len(fields))
	for k, v := range fields {
		details[k] = v
	}
	return &Error{Code: CodeValidation, Status: http.StatusUnprocessableEntity, Message: "Some fields need attention.", Details: map[string]any{"fields": details}}
}

func Unauthenticated(message string) *Error {
	if message == "" {
		message = "Please sign in to continue."
	}
	return New(CodeUnauthenticated, http.StatusUnauthorized, message)
}

func InvalidCredentials() *Error {
	return New(CodeInvalidCredentials, http.StatusUnauthorized, "The email or password is incorrect.")
}

func SessionExpired() *Error {
	return New(CodeSessionExpired, http.StatusUnauthorized, "Your session has expired. Please sign in again.")
}

func Forbidden(message string) *Error {
	if message == "" {
		message = "You do not have access to this resource."
	}
	return New(CodeForbidden, http.StatusForbidden, message)
}

func AccountDisabled() *Error {
	return New(CodeAccountDisabled, http.StatusForbidden, "This account has been deactivated. Please contact support.")
}

func AccountLocked(retryAfter time.Duration) *Error {
	e := New(CodeAccountLocked, http.StatusTooManyRequests, "Too many sign-in attempts. Please try again later.")
	e.RetryAfter = retryAfter
	return e
}

func NotFound(resource string) *Error {
	return New(CodeNotFound, http.StatusNotFound, fmt.Sprintf("%s not found.", resource))
}

func Conflict(code Code, message string) *Error { return New(code, http.StatusConflict, message) }

func InvalidState(message string) *Error { return New(CodeInvalidState, http.StatusConflict, message) }

func InsufficientCredits(required, available int64) *Error {
	return &Error{
		Code:    CodeInsufficientCredits,
		Status:  http.StatusPaymentRequired,
		Message: "You do not have enough credits for this.",
		Details: map[string]any{"required": required, "available": available},
	}
}

func RateLimited(retryAfter time.Duration) *Error {
	e := New(CodeRateLimited, http.StatusTooManyRequests, "Too many requests. Please slow down.")
	e.RetryAfter = retryAfter
	return e
}

func PayloadTooLarge(message string) *Error {
	return New(CodePayloadTooLarge, http.StatusRequestEntityTooLarge, message)
}

func UnsupportedMedia(message string) *Error {
	return New(CodeUnsupportedMedia, http.StatusUnsupportedMediaType, message)
}

func InvalidToken(message string) *Error {
	return New(CodeInvalidToken, http.StatusBadRequest, message)
}

func Unavailable(cause error) *Error {
	return &Error{Code: CodeUnavailable, Status: http.StatusServiceUnavailable, Message: "The service is temporarily unavailable. Please try again.", Err: cause, RetryAfter: 5 * time.Second}
}

func Timeout(cause error) *Error {
	return &Error{Code: CodeTimeout, Status: http.StatusGatewayTimeout, Message: "The request took too long. Please try again.", Err: cause}
}

// Internal wraps an unexpected failure. The cause is logged, never exposed.
func Internal(cause error) *Error {
	return &Error{Code: CodeInternal, Status: http.StatusInternalServerError, Message: "Something went wrong on our side. Please try again.", Err: cause}
}

// From converts any error into an *Error, mapping context errors to timeout
// or cancellation and everything else to Internal.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout(err)
	}
	return Internal(err)
}

// As is a convenience wrapper around errors.As.
func As(err error) (*Error, bool) {
	var appErr *Error
	ok := errors.As(err, &appErr)
	return appErr, ok
}
