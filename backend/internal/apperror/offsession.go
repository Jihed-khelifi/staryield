package apperror

import (
	"net/http"
	"time"
)

// Messages outside a live chat (revisions 3.3, §22.3).
const (
	// CodeOffSessionDisabled: the feature is switched off or the caller's
	// role has a daily limit of 0.
	CodeOffSessionDisabled Code = "off_session_disabled"
	// CodeOffSessionNotEligible: the conversation never had a started live chat.
	CodeOffSessionNotEligible Code = "off_session_not_eligible"
	// CodeRecipientUnavailable: the other participant cannot receive messages.
	CodeRecipientUnavailable Code = "recipient_unavailable"
	// CodeOffSessionLimit: the caller used today's allowance in this conversation.
	CodeOffSessionLimit Code = "off_session_limit_reached"
)

// OffSessionDisabled refuses a message outside a live chat: feature off.
func OffSessionDisabled() *Error {
	return New(CodeOffSessionDisabled, http.StatusConflict, "Messages outside a live chat are turned off right now.")
}

// OffSessionNotEligible refuses a message in a conversation without a started live chat.
func OffSessionNotEligible() *Error {
	return New(CodeOffSessionNotEligible, http.StatusConflict, "Start a live chat first. You can keep writing here afterwards.")
}

// RecipientUnavailable refuses a message to an account that cannot receive it.
func RecipientUnavailable() *Error {
	return New(CodeRecipientUnavailable, http.StatusConflict, "This person can't receive messages right now.")
}

// OffSessionLimit refuses a message over the rolling 24 h cap; resetsAt is
// when the oldest counted message leaves the window.
func OffSessionLimit(limit, used int, resetsAt, now time.Time) *Error {
	e := New(CodeOffSessionLimit, http.StatusTooManyRequests, "You have used today's messages outside a live chat. Start a live chat to keep talking.")
	retry := resetsAt.Sub(now)
	if retry < time.Second {
		retry = time.Second
	}
	e.RetryAfter = retry
	return e.WithDetails(map[string]any{"limit": limit, "used": used, "resets_at": resetsAt.UTC()})
}
