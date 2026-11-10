package apperror

import (
	"net/http"
	"testing"
)

func TestGoogleErrorsMatchTheContract(t *testing.T) {
	for _, tc := range []struct {
		err    *Error
		code   Code
		status int
	}{
		{GoogleUnavailable(nil), "google_unavailable", http.StatusServiceUnavailable},
		{GoogleStateInvalid(), "google_state_invalid", http.StatusBadRequest},
		{GoogleCancelled(), "google_cancelled", http.StatusBadRequest},
		{GoogleEmailUnverified(), "google_email_unverified", http.StatusForbidden},
		{GoogleAccountInUse(), "google_account_in_use", http.StatusConflict},
		{GoogleNotAllowed(), "google_not_allowed", http.StatusForbidden},
		{PasswordNotSet(), "password_not_set", http.StatusConflict},
		{LastSignInMethod(), "last_sign_in_method", http.StatusConflict},
	} {
		if tc.err.Code != tc.code || tc.err.Status != tc.status || tc.err.Message == "" {
			t.Errorf("%s: %d %q", tc.err.Code, tc.err.Status, tc.err.Message)
		}
	}
	if e := GoogleUnavailable(nil); e.RetryAfter <= 0 {
		t.Error("google_unavailable is retryable")
	}
}
