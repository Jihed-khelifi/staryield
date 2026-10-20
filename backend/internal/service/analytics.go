package service

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// ClientEvents are the only event names accepted from browsers; server-side
// business events (payments, sessions) are recorded by services and cannot
// be forged by clients.
var ClientEvents = map[string]bool{
	"page_view": true, "calculator_started": true, "calculator_completed": true,
	"psychic_list_viewed": true, "psychic_profile_viewed": true, "chat_cta_clicked": true,
	"checkout_opened": true, "checkout_pack_selected": true, "signup_step_viewed": true,
	"signup_step_completed": true, "horoscope_viewed": true, "chart_viewed": true,
	"service_viewed": true, "service_order_started": true, "error_displayed": true,
	"web_vital": true, "realtime_reconnected": true, "realtime_fallback": true,
	"locale_changed": true, "chat_opened": true,
}

// ClientEvent is one browser-reported event.
type ClientEvent struct {
	EventID     string         `json:"event_id"`
	Name        string         `json:"name"`
	OccurredAt  time.Time      `json:"occurred_at"`
	AnonymousID string         `json:"anonymous_id"`
	SessionKey  string         `json:"session_key"`
	Path        string         `json:"path"`
	Referrer    string         `json:"referrer"`
	Locale      string         `json:"locale"`
	Properties  map[string]any `json:"properties"`
}

// ClientLog is a browser-reported error or warning.
type ClientLog struct {
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Stack     string         `json:"stack"`
	Code      string         `json:"code"`
	URL       string         `json:"url"`
	RequestID string         `json:"request_id"`
	Context   map[string]any `json:"context"`
}

var anonID = regexp.MustCompile(`^[A-Za-z0-9_\-]{8,64}$`)

// AnalyticsService ingests client events and answers dashboard queries.
type AnalyticsService struct {
	tracker  analytics.Tracker
	events   repository.AnalyticsRepository
	reports  repository.ReportRepository
	clock    clock.Clock
	ipSecret string
	maxBatch int
}

// NewAnalyticsService wires the analytics service.
func NewAnalyticsService(tracker analytics.Tracker, events repository.AnalyticsRepository, reports repository.ReportRepository,
	clk clock.Clock, ipSecret string, maxBatch int) *AnalyticsService {
	return &AnalyticsService{tracker: tracker, events: events, reports: reports, clock: clk, ipSecret: ipSecret, maxBatch: maxBatch}
}

// stripQuery removes query strings and fragments, which may carry tokens or
// personal data, from URLs before storage.
func stripQuery(raw string, max int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil {
		u.RawQuery, u.Fragment, u.User = "", "", nil
		raw = u.String()
	}
	if len(raw) > max {
		raw = raw[:max]
	}
	return raw
}

// Ingest validates and enqueues a batch of client events. Invalid events are
// skipped individually; the count of accepted events is returned.
func (s *AnalyticsService) Ingest(ctx context.Context, events []ClientEvent) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	if len(events) > s.maxBatch {
		return 0, apperror.Validation(map[string]string{"events": "Too many events in one batch."})
	}
	meta := reqctx.MetaFrom(ctx)
	principal, authed := reqctx.PrincipalFrom(ctx)
	now := s.clock.Now()
	accepted := 0
	for _, e := range events {
		if !ClientEvents[e.Name] {
			continue
		}
		id, err := uuid.Parse(e.EventID)
		if err != nil {
			continue
		}
		occurred := e.OccurredAt
		if occurred.IsZero() || occurred.Before(now.Add(-24*time.Hour)) || occurred.After(now.Add(5*time.Minute)) {
			occurred = now
		}
		evt := domain.AnalyticsEvent{
			EventID: id, Name: "web." + e.Name, Source: "client", OccurredAt: occurred, ReceivedAt: now,
			Properties: analytics.SanitizeProperties(e.Properties),
			Path:       stripQuery(e.Path, 512), Referrer: stripQuery(e.Referrer, 512),
			Locale: truncateString(normalizeLocale(e.Locale), 8), UserAgent: truncateString(meta.UserAgent, 256),
			IPHash: security.Keyed(s.ipSecret, meta.IP, 32), RequestID: truncateString(meta.RequestID, 64),
		}
		if anonID.MatchString(e.AnonymousID) {
			evt.AnonymousID = e.AnonymousID
		}
		if anonID.MatchString(e.SessionKey) {
			evt.SessionKey = e.SessionKey
		}
		if authed {
			uid := principal.UserID
			evt.UserID, evt.UserRole = &uid, string(principal.Role)
		}
		if s.tracker.Enqueue(evt) {
			accepted++
		}
	}
	return accepted, nil
}

// LogClient records a browser error in the server logs (with the request
// correlation ID when provided) so frontend failures are observable.
func (s *AnalyticsService) LogClient(ctx context.Context, l ClientLog) {
	log := logger.FromContext(ctx).With(zap.String("component", "web"),
		zap.String("client_code", truncateString(CleanLine(l.Code), 64)),
		zap.String("client_url", stripQuery(l.URL, 512)),
		zap.String("client_request_id", truncateString(CleanLine(l.RequestID), 64)),
		zap.Any("client_context", analytics.SanitizeProperties(l.Context)),
	)
	msg := truncateString(CleanLine(l.Message), 1000)
	stack := truncateString(CleanText(l.Stack), 4000)
	switch l.Level {
	case "error", "fatal":
		log.Error("client error: "+msg, zap.String("client_stack", stack))
	case "warn":
		log.Warn("client warning: "+msg, zap.String("client_stack", stack))
	default:
		log.Info("client log: " + msg)
	}
	s.tracker.Track(ctx, "web.client_error", map[string]any{"level": l.Level, "code": truncateString(l.Code, 64)})
}

// Range is a validated reporting window.
type Range struct{ From, To time.Time }

// ParseRange validates from/to (RFC 3339 or YYYY-MM-DD), defaulting to the
// last 30 days and capping the window at 400 days.
func ParseRange(fromRaw, toRaw string, now time.Time) (Range, error) {
	parse := func(s string) (time.Time, error) {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t.UTC(), nil
		}
		return time.Parse("2006-01-02", s)
	}
	r := Range{From: now.AddDate(0, 0, -30), To: now}
	if fromRaw != "" {
		t, err := parse(fromRaw)
		if err != nil {
			return r, apperror.Validation(map[string]string{"from": "Use RFC 3339 or YYYY-MM-DD."})
		}
		r.From = t
	}
	if toRaw != "" {
		t, err := parse(toRaw)
		if err != nil {
			return r, apperror.Validation(map[string]string{"to": "Use RFC 3339 or YYYY-MM-DD."})
		}
		if len(toRaw) == 10 {
			t = t.AddDate(0, 0, 1) // inclusive end date
		}
		r.To = t
	}
	if !r.From.Before(r.To) {
		return r, apperror.Validation(map[string]string{"from": "from must be before to."})
	}
	if r.To.Sub(r.From) > 400*24*time.Hour {
		return r, apperror.Validation(map[string]string{"from": "The reporting window is limited to 400 days."})
	}
	return r, nil
}

// Overview returns the headline business report (FR-14).
func (s *AnalyticsService) Overview(ctx context.Context, r Range) (*repository.Overview, error) {
	o, err := s.reports.Overview(ctx, r.From, r.To)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return o, nil
}

// Daily returns per-day metrics.
func (s *AnalyticsService) Daily(ctx context.Context, r Range) ([]repository.DailyMetric, error) {
	if r.To.Sub(r.From) > 370*24*time.Hour {
		return nil, apperror.Validation(map[string]string{"from": "Daily reports are limited to about a year."})
	}
	d, err := s.reports.Daily(ctx, r.From, r.To)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return d, nil
}

var eventName = regexp.MustCompile(`^[a-z][a-z0-9_.]{1,63}$`)

// Timeseries counts one event per bucket.
func (s *AnalyticsService) Timeseries(ctx context.Context, name, interval string, r Range) ([]repository.SeriesPoint, error) {
	if !eventName.MatchString(name) {
		return nil, apperror.Validation(map[string]string{"name": "Invalid event name."})
	}
	if interval == "" {
		interval = "day"
	}
	if interval == "hour" && r.To.Sub(r.From) > 31*24*time.Hour {
		return nil, apperror.Validation(map[string]string{"interval": "Hourly series are limited to 31 days."})
	}
	pts, err := s.events.Timeseries(ctx, name, r.From, r.To, interval)
	if err != nil {
		if strings.Contains(err.Error(), "invalid interval") {
			return nil, apperror.Validation(map[string]string{"interval": "Use hour, day, week or month."})
		}
		return nil, apperror.Internal(err)
	}
	return pts, nil
}

// Funnel computes ordered conversion across event names.
func (s *AnalyticsService) Funnel(ctx context.Context, steps []string, r Range) ([]repository.FunnelStep, error) {
	if len(steps) < 2 || len(steps) > 8 {
		return nil, apperror.Validation(map[string]string{"steps": "Provide between 2 and 8 steps."})
	}
	for _, st := range steps {
		if !eventName.MatchString(st) {
			return nil, apperror.Validation(map[string]string{"steps": "Invalid event name."})
		}
	}
	out, err := s.events.Funnel(ctx, steps, r.From, r.To)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// TopEvents ranks event names by volume.
func (s *AnalyticsService) TopEvents(ctx context.Context, r Range, limit int) ([]repository.NameCount, error) {
	out, err := s.events.TopEvents(ctx, r.From, r.To, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// EventsPage is a page of raw events.
type EventsPage struct {
	Items  []domain.AnalyticsEvent `json:"items"`
	NextID int64                   `json:"next_before_id,omitempty"`
}

// Events lists raw events for drill-down.
func (s *AnalyticsService) Events(ctx context.Context, f repository.EventFilter) (*EventsPage, error) {
	if f.Name != "" && !eventName.MatchString(f.Name) {
		return nil, apperror.Validation(map[string]string{"name": "Invalid event name."})
	}
	rows, err := s.events.List(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	page := &EventsPage{Items: rows}
	if len(rows) > 0 && len(rows) == clampPage(f.Limit, 100, 1000) {
		page.NextID = rows[len(rows)-1].ID
	}
	return page, nil
}
