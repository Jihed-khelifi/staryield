package tz

import (
	"math"
	"testing"
	"time"
	_ "time/tzdata"
)

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := Load(name)
	if err != nil {
		t.Fatalf("Load(%q): %v", name, err)
	}
	return loc
}

func TestVersionAndNames(t *testing.T) {
	if v := Version(); len(v) != 5 || v < "2024a" {
		t.Errorf("Version() = %q", v)
	}
	names := Names()
	if len(names) < 400 {
		t.Fatalf("only %d zones embedded", len(names))
	}
	for _, n := range []string{"UTC", "Europe/Paris", "America/New_York", "Asia/Kolkata", "Pacific/Kiritimati", "Pacific/Pago_Pago", "Europe/Oslo"} {
		if !Valid(n) {
			t.Errorf("%s missing", n)
		}
	}
}

func TestLoadRejectsBadNames(t *testing.T) {
	for _, n := range []string{"", "../etc/passwd", "Europe/../../x", "Nowhere/City", "Europe/Paris\x00", "a/b/c/d", string(make([]byte, 80))} {
		if _, err := Load(n); err == nil {
			t.Errorf("Load(%q) succeeded", n)
		}
		if Valid(n) {
			t.Errorf("Valid(%q) = true", n)
		}
	}
}

// TestBackzoneHistory checks zones whose pre-1970 history differs between the
// backzone build and the main data (where they are links).
func TestBackzoneHistory(t *testing.T) {
	tests := []struct {
		zone   string
		at     time.Time // UTC
		offset int       // seconds
	}{
		{"Europe/Oslo", time.Date(1960, 7, 1, 12, 0, 0, 0, time.UTC), 2 * 3600},         // Norway kept summer time; Berlin did not
		{"Atlantic/Reykjavik", time.Date(1960, 1, 1, 12, 0, 0, 0, time.UTC), -1 * 3600}, // Iceland at −1 before 1968
		{"Europe/Amsterdam", time.Date(1935, 6, 1, 12, 0, 0, 0, time.UTC), 4772},        // Amsterdam summer time +1:19:32
		{"Europe/Stockholm", time.Date(1947, 7, 1, 12, 0, 0, 0, time.UTC), 3600},        // no double summer time (Berlin had +3)
		{"Asia/Kolkata", time.Date(1943, 1, 1, 12, 0, 0, 0, time.UTC), 6*3600 + 30*60},  // war time +6:30
		{"Europe/Paris", time.Date(1940, 6, 15, 12, 0, 0, 0, time.UTC), 2 * 3600},       // occupation time from 14 June 1940
		{"America/New_York", time.Date(1970, 7, 1, 12, 0, 0, 0, time.UTC), -4 * 3600},   // control
		{"Australia/Sydney", time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC), 11 * 3600},  // southern summer
		{"Pacific/Kiritimati", time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), 14 * 3600}, // UTC+14
		{"Pacific/Pago_Pago", time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), -11 * 3600}, // UTC−11
	}
	for _, tt := range tests {
		off, _ := OffsetAt(tt.at, mustLoad(t, tt.zone))
		if off != tt.offset {
			t.Errorf("%s at %s: offset %+d s, want %+d s", tt.zone, tt.at.Format(time.DateOnly), off, tt.offset)
		}
	}
}

func TestResolve(t *testing.T) {
	ny := mustLoad(t, "America/New_York")
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	tests := []struct {
		name         string
		date         time.Time
		hour, minute int
		loc          *time.Location
		lon          float64
		want         time.Time
		status       WallStatus
	}{
		{"ordinary", day(2024, 6, 1), 14, 30, ny, math.NaN(), time.Date(2024, 6, 1, 18, 30, 0, 0, time.UTC), WallOK},
		{"spring-forward gap", day(2024, 3, 10), 2, 30, ny, math.NaN(), time.Date(2024, 3, 10, 7, 30, 0, 0, time.UTC), WallSkipped},
		{"fall-back overlap takes the earlier instant", day(2024, 11, 3), 1, 30, ny, math.NaN(), time.Date(2024, 11, 3, 5, 30, 0, 0, time.UTC), WallAmbiguous},
		{"midnight", day(2024, 1, 1), 0, 0, mustLoad(t, "Europe/Paris"), math.NaN(), time.Date(2023, 12, 31, 23, 0, 0, 0, time.UTC), WallOK},
		// Calcutta kept local mean time before 1854; Kolkata lies at 88.36° E.
		{"local mean time of the birthplace", day(1850, 5, 1), 12, 0, mustLoad(t, "Asia/Kolkata"), 88.3639, time.Date(1850, 5, 1, 6, 6, 33, 0, time.UTC), WallLMT},
		{"LMT without longitude keeps the zone offset", day(1850, 5, 1), 12, 0, mustLoad(t, "Asia/Kolkata"), math.NaN(), time.Date(1850, 5, 1, 6, 6, 32, 0, time.UTC), WallOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, status := Resolve(tt.date, tt.hour, tt.minute, tt.loc, tt.lon)
			if !got.Equal(tt.want) || status != tt.status {
				t.Errorf("got %s %s, want %s %s", got.Format(time.RFC3339), status, tt.want.Format(time.RFC3339), tt.status)
			}
		})
	}
}

func TestParseClock(t *testing.T) {
	good := map[string][2]int{"00:00": {0, 0}, "9:05": {9, 5}, "23:59": {23, 59}, " 07:30 ": {7, 30}}
	for in, want := range good {
		h, m, err := ParseClock(in)
		if err != nil || h != want[0] || m != want[1] {
			t.Errorf("ParseClock(%q) = %d, %d, %v", in, h, m, err)
		}
	}
	for _, in := range []string{"", "24:00", "12:60", "1230", "12:5", "123:00", "ab:cd", "-1:00", "12:00:00", "１２:００"} {
		if _, _, err := ParseClock(in); err == nil {
			t.Errorf("ParseClock(%q) accepted", in)
		}
	}
}

func FuzzParseClock(f *testing.F) {
	for _, s := range []string{"12:30", "0:00", "23:59", "24:00", "", ":", "1:2", "99:99"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		h, m, err := ParseClock(s)
		if err == nil && (h < 0 || h > 23 || m < 0 || m > 59) {
			t.Fatalf("ParseClock(%q) = %d:%d without error", s, h, m)
		}
	})
}

// fuzzZones have unusual transitions: half-hour DST (Lord Howe), skipped
// days (Kiritimati 1994-12-31, Apia 2011-12-30), Ramadan DST (Casablanca),
// +2h DST (Troll), double summer time (London 1941–47) and LMT eras.
var fuzzZones = []string{
	"Europe/Paris", "America/New_York", "Australia/Lord_Howe", "Pacific/Kiritimati", "Pacific/Apia", "Asia/Kathmandu",
	"America/St_Johns", "Europe/London", "Africa/Casablanca", "Antarctica/Troll", "Asia/Kolkata", "America/Sao_Paulo",
}

// Resolve's result reads back as the requested wall time unless that time
// was skipped, in which case it reads later (within a day).
func FuzzResolve(f *testing.F) {
	f.Add(2024, 69, 150, 1, 0.0)    // New York spring forward, 02:30
	f.Add(2024, 307, 90, 1, 0.0)    // New York fall back, 01:30
	f.Add(1994, 364, 720, 3, 0.0)   // Kiritimati's skipped day
	f.Add(2011, 363, 720, 4, 0.0)   // Apia's skipped day
	f.Add(1850, 120, 720, 10, 88.4) // Kolkata LMT
	f.Add(1944, 100, 30, 7, -0.1)   // London double summer time
	f.Fuzz(func(t *testing.T, year, yday, minuteOfDay, zone int, lon float64) {
		loc, err := Load(fuzzZones[uint(zone)%uint(len(fuzzZones))])
		if err != nil {
			t.Fatal(err)
		}
		y := 1800 + int(uint(year)%301)
		day := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(uint(yday)%366))
		mod := int(uint(minuteOfDay) % 1440)
		h, m := mod/60, mod%60
		got, status := Resolve(day, h, m, loc, lon)
		wall := time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, time.UTC)
		l := got.In(loc)
		read := time.Date(l.Year(), l.Month(), l.Day(), l.Hour(), l.Minute(), 0, 0, time.UTC)
		switch status {
		case WallOK, WallAmbiguous:
			if !read.Equal(wall) {
				t.Fatalf("%s %s %02d:%02d → %s reads %s", loc, day.Format(time.DateOnly), h, m, got, read)
			}
		case WallSkipped:
			if d := read.Sub(wall); d <= 0 || d > 25*time.Hour {
				t.Fatalf("%s %s %02d:%02d skipped → reads %s", loc, day.Format(time.DateOnly), h, m, read)
			}
		case WallLMT:
			want := wall.Add(-time.Duration(math.Round(lon*240)) * time.Second)
			if !got.Equal(want) {
				t.Fatalf("LMT: got %s want %s", got, want)
			}
		default:
			t.Fatalf("status %q", status)
		}
	})
}

func BenchmarkLoadCached(b *testing.B) {
	if _, err := Load("Europe/Paris"); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		_, _ = Load("Europe/Paris")
	}
}
