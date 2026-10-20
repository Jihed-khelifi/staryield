package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Healthz is the liveness probe.
func (h *Handler) Healthz(c echo.Context) error {
	return ok(c, map[string]string{"status": "ok"})
}

// Readyz checks database and Redis.
func (h *Handler) Readyz(c echo.Context) error {
	checks := h.health(c.Request())
	out := map[string]string{}
	status := http.StatusOK
	for name, err := range checks {
		if err != nil {
			out[name] = "unavailable"
			status = http.StatusServiceUnavailable
		} else {
			out[name] = "ok"
		}
	}
	return c.JSON(status, map[string]any{"status": map[bool]string{true: "ok", false: "degraded"}[status == http.StatusOK], "checks": out})
}

// Psychics lists advisors (public, FR-06).
func (h *Handler) Psychics(c echo.Context) error {
	out, err := h.svc.Psychics.List(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": out})
}

// Psychic returns one advisor profile.
func (h *Handler) Psychic(c echo.Context) error {
	out, err := h.svc.Psychics.Get(c.Request().Context(), c.Param("slug"))
	if err != nil {
		return err
	}
	return ok(c, out)
}

// CreditPacks lists purchasable packs.
func (h *Handler) CreditPacks(c echo.Context) error {
	out, err := h.svc.Credits.Packs(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": out})
}

// Wallet returns the balance.
func (h *Handler) Wallet(c echo.Context) error {
	out, err := h.svc.Wallet.Get(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// Ledger lists credit movements.
func (h *Handler) Ledger(c echo.Context) error {
	limit, err := limitParam(c, 50, 100)
	if err != nil {
		return err
	}
	out, err := h.svc.Wallet.Ledger(c.Request().Context(), principal(c).UserID, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

type purchaseRequest struct {
	PackID         uuid.UUID `json:"pack_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	ReceiptEmail   string    `json:"receipt_email"`
}

// Purchase buys a credit pack (FR-05).
func (h *Handler) Purchase(c echo.Context) error {
	var req purchaseRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	if req.PackID == uuid.Nil {
		return apperror.Validation(map[string]string{"pack_id": "Choose a credit pack."})
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = c.Request().Header.Get("Idempotency-Key")
	}
	out, err := h.svc.Credits.Purchase(c.Request().Context(), principal(c).UserID, service.PurchaseInput{
		PackID: req.PackID, IdempotencyKey: req.IdempotencyKey, ReceiptEmail: req.ReceiptEmail,
	})
	if err != nil {
		return err
	}
	return created(c, out)
}

// Purchases lists the caller's purchases.
func (h *Handler) Purchases(c echo.Context) error {
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := h.svc.Credits.Purchases(c.Request().Context(), principal(c).UserID, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// PaymentWebhook receives signed provider notifications.
func (h *Handler) PaymentWebhook(c echo.Context) error {
	body, err := readAll(c, 64*1024)
	if err != nil {
		return err
	}
	if err := h.svc.Credits.HandleWebhook(c.Request().Context(), c.Request().Header.Get("X-Staryield-Signature"), body); err != nil {
		return err
	}
	return ok(c, map[string]bool{"received": true})
}

type availabilityRequest struct {
	Availability domain.Availability `json:"availability"`
}

// SetAvailability changes a psychic's availability.
func (h *Handler) SetAvailability(c echo.Context) error {
	var req availabilityRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Psychics.SetAvailability(c.Request().Context(), principal(c).UserID, req.Availability)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// UpdatePsychicProfile lets a psychic edit their profile (FR-08).
func (h *Handler) UpdatePsychicProfile(c echo.Context) error {
	var req service.PsychicProfileUpdate
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Psychics.UpdateOwn(c.Request().Context(), principal(c).UserID, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// Earnings returns the psychic's earnings (FR-12).
func (h *Handler) Earnings(c echo.Context) error {
	return h.earningsFor(c, principal(c).UserID)
}

func (h *Handler) earningsFor(c echo.Context, psychicID uuid.UUID) error {
	from, err := optionalTime(c, "from")
	if err != nil {
		return err
	}
	to, err := optionalTime(c, "to")
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 50, 200)
	if err != nil {
		return err
	}
	out, err := h.svc.Psychics.Earnings(c.Request().Context(), psychicID, from, to, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// Places searches birth places.
func (h *Handler) Places(c echo.Context) error {
	limit, err := limitParam(c, 8, 20)
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": h.svc.Astrology.Places(c.QueryParam("q"), limit)})
}

// PublicChart computes a natal chart for the anonymous calculator (FR-01).
func (h *Handler) PublicChart(c echo.Context) error {
	var req service.ChartInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Astrology.Chart(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// SignHoroscope returns a sign's daily horoscope (public).
func (h *Handler) SignHoroscope(c echo.Context) error {
	out, err := h.svc.Astrology.HoroscopeFor(c.Request().Context(), c.Param("sign"), c.QueryParam("date"))
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=900")
	return ok(c, out)
}

// CosmicEvents lists upcoming lunar phases and retrogrades.
func (h *Handler) CosmicEvents(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "public, max-age=900")
	return ok(c, h.svc.Astrology.Events(4))
}

// MyChart returns the client's full birth chart (FR-03).
func (h *Handler) MyChart(c echo.Context) error {
	out, err := h.svc.Astrology.MyChart(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// MyHoroscope returns the client's daily horoscope (FR-04).
func (h *Handler) MyHoroscope(c echo.Context) error {
	out, err := h.svc.Astrology.MyHoroscope(c.Request().Context(), principal(c).UserID, c.QueryParam("date"))
	if err != nil {
		return err
	}
	return ok(c, out)
}

type ingestRequest struct {
	Events []service.ClientEvent `json:"events"`
}

// IngestEvents accepts client analytics batches.
func (h *Handler) IngestEvents(c echo.Context) error {
	var req ingestRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	n, err := h.svc.Analytics.Ingest(c.Request().Context(), req.Events)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusAccepted, map[string]int{"accepted": n})
}

// ClientLog accepts frontend error reports.
func (h *Handler) ClientLog(c echo.Context) error {
	var req service.ClientLog
	if err := bind(c, &req); err != nil {
		return err
	}
	h.svc.Analytics.LogClient(c.Request().Context(), req)
	return c.NoContent(http.StatusAccepted)
}

// ServerTime lets clients measure clock offset for server-authoritative
// timers (session clock, expiries).
func (h *Handler) ServerTime(c echo.Context) error {
	return ok(c, map[string]any{"server_time": time.Now().UTC()})
}
