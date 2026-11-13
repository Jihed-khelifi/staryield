package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/migrations"
)

// revisions 5.0: the 0027 backfill (§24.6) run against seeded rows. The
// migration already ran once on an empty table; its set-based UPDATE is
// extracted from the embedded file and run again on rows that look like an
// existing user base.
func TestAgeGateBackfill(t *testing.T) {
	e := newEnv(t)
	all, err := migrations.Load()
	if err != nil {
		t.Fatal(err)
	}
	var backfill string
	for _, m := range all {
		if m.Version != "0027_age_gate" {
			continue
		}
		// The backfill is the file's last statement.
		if i := strings.Index(m.SQL, "UPDATE users"); i >= 0 {
			backfill = strings.TrimSpace(m.SQL[i:])
		}
	}
	if backfill == "" {
		t.Fatal("0027_age_gate has no UPDATE users backfill")
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	type seed struct {
		name       string
		role       domain.Role
		birth      *time.Time
		wantAdult  bool
		wantRestr  bool
		preAdult   bool // already confirmed before the backfill (must stay untouched)
		preRestr   bool
		wantStatus domain.AgeStatus
	}
	d := func(years int, days int) *time.Time { t := today.AddDate(-years, 0, days); return &t }
	cases := []seed{
		{name: "adult client", role: domain.RoleClient, birth: d(30, 0), wantAdult: true, wantStatus: domain.AgeAdult},
		{name: "turned 18 today", role: domain.RoleClient, birth: d(18, 0), wantAdult: true, wantStatus: domain.AgeAdult},
		{name: "turns 18 tomorrow", role: domain.RoleClient, birth: d(18, 1), wantRestr: true, wantStatus: domain.AgeRestricted},
		{name: "minor client", role: domain.RoleClient, birth: d(12, 0), wantRestr: true, wantStatus: domain.AgeRestricted},
		{name: "no birth date", role: domain.RoleClient, birth: nil, wantStatus: domain.AgeBirthDateRequired},
		{name: "adult advisor", role: domain.RolePsychic, birth: d(40, 0), wantAdult: true, wantStatus: domain.AgeAdult},
		{name: "advisor with an under-age date is not restricted", role: domain.RolePsychic, birth: d(10, 0), wantStatus: domain.AgeAdult},
		{name: "already confirmed stays", role: domain.RoleClient, birth: d(12, 0), preAdult: true, wantAdult: true, wantStatus: domain.AgeAdult},
		{name: "already restricted stays", role: domain.RoleClient, birth: d(40, 0), preRestr: true, wantRestr: true, wantStatus: domain.AgeRestricted},
	}
	ids := make([]uuid.UUID, len(cases))
	now := time.Now().UTC()
	for i, c := range cases {
		u := domain.User{ID: domain.NewID(), Email: uniq("backfill") + "@example.com", PasswordHash: "x", Role: c.role, Status: domain.UserActive,
			DisplayName: c.name, Locale: "en", TermsAcceptedAt: now, CreatedAt: now, UpdatedAt: now}
		if c.preAdult {
			u.AdultConfirmedAt = &now
		}
		if c.preRestr {
			u.AgeRestrictedAt = &now
		}
		if err := e.app.DB.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		if err := e.app.DB.Create(&domain.Profile{UserID: u.ID, BirthDate: c.birth, Mood: "Calm", Theme: "Warm Beige", UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
		ids[i] = u.ID
	}

	if err := e.app.DB.Exec(backfill).Error; err != nil {
		t.Fatalf("backfill: %v", err)
	}
	for i, c := range cases {
		var u domain.User
		if err := e.app.DB.First(&u, "id = ?", ids[i]).Error; err != nil {
			t.Fatal(err)
		}
		if (u.AdultConfirmedAt != nil) != c.wantAdult || (u.AgeRestrictedAt != nil) != c.wantRestr {
			t.Errorf("%s: adult_confirmed_at=%v age_restricted_at=%v", c.name, u.AdultConfirmedAt, u.AgeRestrictedAt)
		}
		if got := u.AgeStatus(); got != c.wantStatus {
			t.Errorf("%s: status %s, want %s", c.name, got, c.wantStatus)
		}
	}

	// Running it again changes nothing (rows with a decision are skipped).
	var before, after int64
	e.app.DB.Raw("SELECT count(*) FROM users WHERE updated_at IS NOT NULL AND (adult_confirmed_at IS NOT NULL OR age_restricted_at IS NOT NULL)").Scan(&before)
	if err := e.app.DB.Exec(backfill).Error; err != nil {
		t.Fatal(err)
	}
	e.app.DB.Raw("SELECT count(*) FROM users WHERE updated_at IS NOT NULL AND (adult_confirmed_at IS NOT NULL OR age_restricted_at IS NOT NULL)").Scan(&after)
	if before != after {
		t.Errorf("a second run changed the decided rows: %d then %d", before, after)
	}
}
