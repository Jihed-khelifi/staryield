package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/i18n"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// ErrorBody is the JSON error envelope shared by every endpoint.
type ErrorBody struct {
	Error ErrorPayload `json:"error"`
}

// ErrorPayload describes a failure safely.
type ErrorPayload struct {
	Code      apperror.Code  `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
}

// fromEcho maps framework errors (routing, binding) to application errors.
func fromEcho(he *echo.HTTPError) *apperror.Error {
	switch he.Code {
	case http.StatusNotFound:
		return apperror.New(apperror.CodeNotFound, http.StatusNotFound, "The requested endpoint does not exist.")
	case http.StatusMethodNotAllowed:
		return apperror.New(apperror.CodeBadRequest, http.StatusMethodNotAllowed, "This method is not allowed here.")
	case http.StatusRequestEntityTooLarge:
		return apperror.PayloadTooLarge("The request body is too large.")
	case http.StatusUnsupportedMediaType:
		return apperror.UnsupportedMedia("Unsupported content type.")
	case http.StatusUnauthorized:
		return apperror.Unauthenticated("")
	case http.StatusForbidden:
		return apperror.Forbidden("")
	case http.StatusTooManyRequests:
		return apperror.RateLimited(0)
	case http.StatusServiceUnavailable:
		return apperror.Unavailable(he)
	}
	if he.Code >= 500 {
		return apperror.Internal(he)
	}
	return apperror.New(apperror.CodeBadRequest, he.Code, http.StatusText(he.Code))
}

// ErrorHandler renders errors as JSON and logs them by severity. Internal
// details are logged with the request ID; clients only receive safe text.
func ErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	ctx := c.Request().Context()
	log := logger.FromContext(ctx)

	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		log.Debug("request cancelled by client")
		c.Response().WriteHeader(499)
		return
	}

	var appErr *apperror.Error
	var he *echo.HTTPError
	switch {
	case errors.As(err, &he):
		appErr = fromEcho(he)
	default:
		appErr = apperror.From(err)
	}

	switch {
	case appErr.Status >= 500:
		log.Error("request failed", zap.Error(err), zap.String("code", string(appErr.Code)), zap.Int("status", appErr.Status))
	case appErr.Status == http.StatusTooManyRequests:
		log.Warn("request throttled", zap.String("code", string(appErr.Code)))
	default:
		log.Info("request rejected", zap.String("code", string(appErr.Code)), zap.Int("status", appErr.Status), zap.String("reason", appErr.Message))
	}

	if appErr.RetryAfter > 0 {
		c.Response().Header().Set("Retry-After", strconv.Itoa(int(appErr.RetryAfter.Seconds()+0.999)))
	}
	// Messages are written in English by services and translated here into
	// the caller's language (X-Locale, else Accept-Language).
	locale := reqctx.MetaFrom(ctx).Locale
	body := ErrorBody{Error: ErrorPayload{Code: appErr.Code, Message: i18n.T(locale, appErr.Message),
		Details: i18n.Details(locale, appErr.Details), RequestID: c.Response().Header().Get("X-Request-ID")}}
	if c.Request().Method == http.MethodHead {
		_ = c.NoContent(appErr.Status)
		return
	}
	if err := c.JSON(appErr.Status, body); err != nil {
		log.Error("failed to write error response", zap.Error(err))
	}
}
