package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// statsFixture inserts sessions with exact values so every KPI has a known
// answer. Times are UTC.
type statsFixture struct {
	e       *env
	psychic uuid.UUID
}

func (f statsFixture) session(t *testing.T, client uuid.UUID, startedAt string, credits int64, billed, free int, activeSeconds int64) {
	t.Helper()
	db := f.e.app.DB
	var conv domain.Conversation
	err := db.Where("client_id = ? AND psychic_id = ?", client, f.psychic).First(&conv).Error
	if err != nil {
		now := time.Now().UTC()
		conv = domain.Conversation{ID: domain.NewID(), ClientID: client, PsychicID: f.psychic, CreatedAt: now, UpdatedAt: now}
		if err := db.Create(&conv).Error; err != nil {
			t.Fatalf("conversation: %v", err)
		}
	}
	start, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		t.Fatal(err)
	}
	end := start.Add(time.Duration(activeSeconds) * time.Second)
	s := domain.Session{ID: domain.NewID(), ConversationID: conv.ID, ClientID: client, PsychicID: f.psychic,
		Status: domain.SessionSettled, EndReason: domain.EndReason("client_ended"), RatePerMinute: 10, ListRatePerMinute: 10, RequestKey: uniq("stats-"),
		RequestedAt: start.Add(-time.Minute), ExpiresAt: start, AcceptedAt: &start, StartedAt: &start, EndedAt: &end, SettledAt: &end,
		ActiveSeconds: activeSeconds, BilledMinutes: billed, FreeMinutesUsed: free, CreditsCharged: credits,
		CreatedAt: start, UpdatedAt: end}
	if err := db.Create(&s).Error; err != nil {
		t.Fatalf("session: %v", err)
	}
}

func (f statsFixture) presence(t *testing.T, status domain.PresenceStatus, from, to string) {
	t.Helper()
	start, _ := time.Parse(time.RFC3339, from)
	end, _ := time.Parse(time.RFC3339, to)
	iv := domain.PresenceInterval{ID: domain.NewID(), PsychicID: f.psychic, Status: status, StartedAt: start, EndedAt: &end,
		CreatedAt: start, UpdatedAt: end}
	if err := f.e.app.DB.Create(&iv).Error; err != nil {
		t.Fatalf("presence: %v", err)
	}
}

func TestAdvisorStatsOverview(t *testing.T) {
	e := newEnv(t)
	psychic := e.createPsychic(10)
	f := statsFixture{e: e, psychic: psychic.user.ID}
	a, b, c, d := e.registerClient().user.ID, e.registerClient().user.ID, e.registerClient().user.ID, e.registerClient().user.ID

	// October 2026 is the reporting window; September is "previous".
	f.session(t, a, "2026-10-05T10:00:00Z", 0, 1, 1, 60)   // A: first session, free …
	f.session(t, a, "2026-10-20T10:00:00Z", 30, 3, 0, 180) // … then pays
	f.session(t, b, "2026-10-03T10:00:00Z", 40, 4, 0, 240) // B: pays …
	f.session(t, b, "2026-10-06T10:00:00Z", 20, 2, 0, 120) // … and returns 3 days later
	f.session(t, c, "2026-10-10T10:00:00Z", 0, 1, 1, 60)   // C: free only
	f.session(t, d, "2026-09-20T10:00:00Z", 0, 1, 1, 60)   // D: first (free) session in September …
	f.session(t, d, "2026-10-15T10:00:00Z", 50, 5, 0, 300) // … first paid in October

	// Another advisor's sessions never count.
	other := e.createPsychic(10)
	statsFixture{e: e, psychic: other.user.ID}.session(t, a, "2026-10-07T10:00:00Z", 99, 9, 0, 540)

	f.presence(t, domain.PresenceOnline, "2026-09-30T22:00:00Z", "2026-10-01T01:00:00Z") // 1 h inside
	f.presence(t, domain.PresenceOnline, "2026-10-01T10:00:00Z", "2026-10-01T12:00:00Z") // 2 h
	f.presence(t, domain.PresenceBusy, "2026-10-31T23:00:00Z", "2026-11-01T02:00:00Z")   // 1 h inside
	f.presence(t, domain.PresenceAway, "2026-10-12T10:00:00Z", "2026-10-12T13:00:00Z")   // away: excluded

	var out service.StatsOverview
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/stats/overview?from=2026-10-01&to=2026-10-31&tz=UTC", nil, &out)
	k := out.KPIs
	pct := func(r service.StatsRatio) float64 {
		if r.Pct == nil {
			return -1
		}
		return *r.Pct
	}
	if k.FreeToPaid.Numerator != 1 || k.FreeToPaid.Denominator != 2 || pct(k.FreeToPaid) != 50 {
		t.Errorf("free→paid: %+v", k.FreeToPaid)
	}
	if k.PaidToRetained.Numerator != 1 || k.PaidToRetained.Denominator != 3 || pct(k.PaidToRetained) != 33.3 {
		t.Errorf("paid→retained: %+v", k.PaidToRetained)
	}
	if k.PaidMinutes != 14 {
		t.Errorf("paid minutes: %d", k.PaidMinutes)
	}
	if k.RetainedRepeatDays.Sample != 1 || k.RetainedRepeatDays.MedianDays == nil || *k.RetainedRepeatDays.MedianDays != 3 {
		t.Errorf("repeat days: %+v", k.RetainedRepeatDays)
	}
	if k.ActiveHours.Hours != 4 || k.ActiveHours.InSessionHours != 0.3 || k.ActiveHours.TrackedSince == nil ||
		!k.ActiveHours.TrackedSince.Equal(time.Date(2026, 9, 30, 22, 0, 0, 0, time.UTC)) {
		t.Errorf("active hours: %+v", k.ActiveHours)
	}
	if !out.Range.From.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) || !out.Range.To.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("range: %+v", out.Range)
	}

	// September: D's first session was free and D paid later; nobody first-paid.
	p := out.Previous
	if p.FreeToPaid.Numerator != 1 || p.FreeToPaid.Denominator != 1 || pct(p.FreeToPaid) != 100 {
		t.Errorf("previous free→paid: %+v", p.FreeToPaid)
	}
	if p.PaidToRetained.Denominator != 0 || p.PaidToRetained.Pct != nil {
		t.Errorf("previous paid→retained should be empty: %+v", p.PaidToRetained)
	}

	// Calendar dates are read in the advisor's time zone.
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/stats/overview?from=2026-10-01&to=2026-10-31&tz=America/Los_Angeles", nil, &out)
	if out.Range.TZ != "America/Los_Angeles" || !out.Range.From.Equal(time.Date(2026, 10, 1, 7, 0, 0, 0, time.UTC)) ||
		!out.Range.To.Equal(time.Date(2026, 11, 1, 7, 0, 0, 0, time.UTC)) {
		t.Errorf("tz range: %+v", out.Range)
	}

	// Validation and access.
	for _, q := range []string{"from=2026-10-31&to=2026-10-01", "from=2025-01-01&to=2026-10-01", "from=nope", "tz=Mars/Base"} {
		if r := psychic.call(http.MethodGet, "/api/v1/psychic/stats/overview?"+q, nil); r.errCode(t) != "validation_failed" {
			t.Errorf("%s: %d %s", q, r.status, r.body)
		}
	}
	if r := e.registerClient().call(http.MethodGet, "/api/v1/psychic/stats/overview", nil); r.status != http.StatusForbidden {
		t.Errorf("client reading advisor stats: %d", r.status)
	}
	// The default window (last 30 days) works without parameters.
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/stats/overview", nil, &out)
}
