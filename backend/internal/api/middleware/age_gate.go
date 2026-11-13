package middleware

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// AgeGate is the 18+ gate for client actions (revisions 5.0, §24.6). It reads
// the status the authenticator already loaded with the principal (cached
// authorization state), so it costs no query. Advisors and administrators
// pass; a client without an adult date of birth gets 403 birth_date_required
// (details.next) or 403 age_restricted. Place it after Authenticate.
func AgeGate() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			p, ok := reqctx.PrincipalFrom(c.Request().Context())
			if !ok {
				return apperror.Unauthenticated("")
			}
			if err := p.RequireAdult(); err != nil {
				logger.FromContext(c.Request().Context()).Debug("age gate: refused",
					zap.String("route", c.Path()), zap.String("age", string(p.Age)))
				return err
			}
			return next(c)
		}
	}
}
