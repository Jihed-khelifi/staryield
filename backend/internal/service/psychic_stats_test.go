package service

import (
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

func TestStatsRange(t *testing.T) {
	now := time.Date(2026, 11, 4, 15, 4, 31, 0, time.UTC)
	paris, _ := time.LoadLocation("Europe/Paris")

	r, err := statsRange("", "", time.UTC, now)
	if err != nil || !r.To.Equal(time.Date(2026, 11, 4, 15, 5, 0, 0, time.UTC)) || r.To.Sub(r.From) != 30*24*time.Hour {
		t.Fatalf("default: %+v %v", r, err)
	}
	r, err = statsRange("2026-10-01", "2026-10-31", paris, now)
	if err != nil || !r.From.Equal(time.Date(2026, 9, 30, 22, 0, 0, 0, time.UTC)) || !r.To.Equal(time.Date(2026, 10, 31, 23, 0, 0, 0, time.UTC)) {
		t.Fatalf("paris dates (DST ends on Oct 25): %+v %v", r, err)
	}
	r, err = statsRange("2026-10-01T00:00:00Z", "2026-10-02T00:00:00Z", paris, now)
	if err != nil || r.To.Sub(r.From) != 24*time.Hour {
		t.Fatalf("rfc3339 bounds are exact: %+v %v", r, err)
	}
	r, err = statsRange("", "2026-10-31", time.UTC, now)
	if err != nil || r.To.Sub(r.From) != 30*24*time.Hour {
		t.Fatalf("to only: %+v %v", r, err)
	}
	for _, tc := range [][2]string{{"2026-10-02", "2026-10-01"}, {"2024-01-01", "2026-01-01"}, {"yesterday", ""}, {"", "2026-13-01"}} {
		if _, err := statsRange(tc[0], tc[1], time.UTC, now); err == nil {
			t.Errorf("statsRange(%q, %q) accepted", tc[0], tc[1])
		}
	}
}

func TestStatsRatio(t *testing.T) {
	if r := ratio(repository.Ratio{}); r.Pct != nil {
		t.Fatalf("empty ratio should have no percentage: %+v", r)
	}
	if r := ratio(repository.Ratio{Numerator: 1, Denominator: 3}); r.Pct == nil || *r.Pct != 33.3 {
		t.Fatalf("1/3: %+v", r)
	}
	if r := ratio(repository.Ratio{Numerator: 2, Denominator: 3}); r.Pct == nil || *r.Pct != 66.7 {
		t.Fatalf("2/3: %+v", r)
	}
}
