package service

import (
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

func TestCleanText(t *testing.T) {
	for in, want := range map[string]string{
		"  hello  ":                    "hello",
		"line1\r\nline2":               "line1\nline2",
		"a\n\n\n\n\n\nb":               "a\n\n\nb",
		"bell\x07 and nul\x00":         "bell and nul",
		"rtl‮override​":                "rtloverride",
		"tabs\tstay":                   "tabs\tstay",
		"<script>alert(1)</script>":    "<script>alert(1)</script>", // stored as text; rendering escapes
		string([]byte{0xff, 'o', 'k'}): "ok",
	} {
		if got := CleanText(in); got != want {
			t.Errorf("CleanText(%q) = %q, want %q", in, got, want)
		}
	}
	if CleanLine("a \n b\tc") != "a b c" {
		t.Fatal("CleanLine")
	}
}

func TestNormalizeEmail(t *testing.T) {
	for in, want := range map[string]string{
		" Moon@Example.COM ":                     "moon@example.com",
		"a@b.co":                                 "a@b.co",
		"no-at":                                  "",
		"a@b":                                    "",
		"Name <a@b.co>":                          "",
		"a@b.co\r\nBcc: x":                       "",
		strings.Repeat("a", 65) + "@example.com": "",
	} {
		got, ok := NormalizeEmail(in)
		if (want == "") == ok || got != want {
			t.Errorf("NormalizeEmail(%q) = %q,%v", in, got, ok)
		}
	}
}

func TestParseBirthDate(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if d, msg := ParseBirthDate("2008-10-01", now, 18); msg != "" || d == nil {
		t.Fatalf("exactly 18: %v %s", d, msg)
	}
	if _, msg := ParseBirthDate("2008-10-02", now, 18); msg == "" {
		t.Fatal("under 18 accepted")
	}
	for _, bad := range []string{"2026-02-30", "1899-12-31", "2027-01-01", "01/02/1990"} {
		if _, msg := ParseBirthDate(bad, now, 0); msg == "" {
			t.Errorf("%s accepted", bad)
		}
	}
	if d, msg := ParseBirthDate("", now, 18); d != nil || msg != "" {
		t.Fatal("empty is optional")
	}
}

func TestEffectiveStatus(t *testing.T) {
	cases := []struct {
		av        domain.Availability
		connected bool
		busy      bool
		want      domain.PresenceStatus
	}{
		{domain.AvailabilityOnline, true, false, domain.PresenceOnline},
		{domain.AvailabilityOnline, false, false, domain.PresenceOffline},
		{domain.AvailabilityOnline, true, true, domain.PresenceBusy},
		{domain.AvailabilityAway, true, false, domain.PresenceAway},
		{domain.AvailabilityOffline, true, true, domain.PresenceOffline},
	}
	for _, c := range cases {
		if got := EffectiveStatus(c.av, c.connected, c.busy); got != c.want {
			t.Errorf("%+v → %s", c, got)
		}
	}
}

func TestParseRange(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	r, err := ParseRange("", "", now)
	if err != nil || r.To.Sub(r.From) != 30*24*time.Hour {
		t.Fatalf("default: %+v %v", r, err)
	}
	r, err = ParseRange("2026-10-01", "2026-10-01", now)
	if err != nil || r.To.Sub(r.From) != 24*time.Hour {
		t.Fatalf("inclusive day: %+v %v", r, err)
	}
	for _, bad := range [][2]string{{"2026-10-05", "2026-10-01"}, {"x", ""}, {"2020-01-01", "2026-01-01"}} {
		if _, err := ParseRange(bad[0], bad[1], now); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
}

func TestStatusFilters(t *testing.T) {
	if s, err := ParseSessionStatuses("active, settled"); err != nil || len(s) != 2 {
		t.Fatalf("sessions: %v %v", s, err)
	}
	if _, err := ParseSessionStatuses("active,bogus"); err == nil {
		t.Fatal("unknown session status accepted")
	}
	if _, err := ParseRequestStatuses("pending,delivered"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseRequestStatuses("'; drop table"); err == nil {
		t.Fatal("injection-shaped status accepted")
	}
}

func TestIdempotencyKeys(t *testing.T) {
	for k, ok := range map[string]bool{"abcd1234": true, "purchase-2026:01.x": true, "short": false, "spaces not ok": false, strings.Repeat("a", 65): false} {
		if ValidIdempotencyKey(k) != ok {
			t.Errorf("%q", k)
		}
	}
}

func TestStatusEntryEncoding(t *testing.T) {
	e := statusEntry{Status: domain.UserActive, Role: domain.RolePsychic, TokenVersion: 7}
	got, ok := decodeStatus(e.encode())
	if !ok || got != e {
		t.Fatalf("roundtrip: %+v", got)
	}
	if _, ok := decodeStatus("garbage"); ok {
		t.Fatal("garbage decoded")
	}
}
