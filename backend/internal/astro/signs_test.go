package astro

import (
	"math"
	"testing"
)

func TestSunSignForDate(t *testing.T) {
	tests := []struct {
		month, day int
		want       string
	}{
		{1, 1, "Capricorn"}, {1, 19, "Capricorn"}, {1, 20, "Aquarius"},
		{2, 18, "Aquarius"}, {2, 19, "Pisces"},
		{3, 20, "Pisces"}, {3, 21, "Aries"},
		{4, 19, "Aries"}, {4, 20, "Taurus"},
		{5, 20, "Taurus"}, {5, 21, "Gemini"},
		{6, 20, "Gemini"}, {6, 21, "Cancer"},
		{7, 22, "Cancer"}, {7, 23, "Leo"},
		{8, 22, "Leo"}, {8, 23, "Virgo"},
		{9, 22, "Virgo"}, {9, 23, "Libra"},
		{10, 22, "Libra"}, {10, 23, "Scorpio"},
		{11, 21, "Scorpio"}, {11, 22, "Sagittarius"},
		{12, 21, "Sagittarius"}, {12, 22, "Capricorn"}, {12, 31, "Capricorn"},
		{0, 10, ""}, {13, 10, ""}, {5, 0, ""}, {5, 32, ""},
	}
	for _, tt := range tests {
		if got := SunSignForDate(tt.month, tt.day); got != tt.want {
			t.Errorf("SunSignForDate(%d, %d) = %q, want %q", tt.month, tt.day, got, tt.want)
		}
	}
}

func TestSignForLongitude(t *testing.T) {
	tests := []struct {
		lon  float64
		want string
	}{
		{0, "Aries"}, {29.9999, "Aries"}, {30, "Taurus"}, {95.5, "Cancer"},
		{180, "Libra"}, {359.9999, "Pisces"}, {360, "Aries"}, {-1, "Pisces"},
		{725, "Aries"}, {math.NaN(), ""}, {math.Inf(1), ""},
	}
	for _, tt := range tests {
		if got := SignForLongitude(tt.lon); got != tt.want {
			t.Errorf("SignForLongitude(%v) = %q, want %q", tt.lon, got, tt.want)
		}
	}
}

func TestElementOf(t *testing.T) {
	want := map[string]string{
		"Aries": "Fire", "Taurus": "Earth", "Gemini": "Air", "Cancer": "Water",
		"Leo": "Fire", "Virgo": "Earth", "Libra": "Air", "Scorpio": "Water",
		"Sagittarius": "Fire", "Capricorn": "Earth", "Aquarius": "Air", "Pisces": "Water",
		"leo": "Fire", " PISCES ": "Water", "Ophiuchus": "", "": "",
	}
	for sign, el := range want {
		if got := ElementOf(sign); got != el {
			t.Errorf("ElementOf(%q) = %q, want %q", sign, got, el)
		}
	}
	if s, ok := CanonicalSign("sAgItTaRiUs"); !ok || s != "Sagittarius" {
		t.Errorf("CanonicalSign = %q, %v", s, ok)
	}
	if _, ok := CanonicalSign("Ophiuchus"); ok {
		t.Error("CanonicalSign accepted Ophiuchus")
	}
}

func TestReduceNumber(t *testing.T) {
	tests := []struct {
		n           int
		keepMasters bool
		want        int
	}{
		{0, true, 0}, {7, true, 7}, {10, true, 1}, {11, true, 11}, {11, false, 2},
		{22, true, 22}, {22, false, 4}, {33, true, 33}, {33, false, 6}, {44, true, 8},
		{29, true, 11}, {29, false, 2}, {1945, true, 1}, {99, true, 9}, {-38, true, 11},
		{2025, false, 9}, {999999999, false, 9},
	}
	for _, tt := range tests {
		if got := ReduceNumber(tt.n, tt.keepMasters); got != tt.want {
			t.Errorf("ReduceNumber(%d, %v) = %d, want %d", tt.n, tt.keepMasters, got, tt.want)
		}
	}
}

func TestLifePath(t *testing.T) {
	tests := []struct {
		y, m, d, want int
	}{
		{1945, 4, 30, 8},  // 1945→19→10→1, 4, 30→3: 1+4+3 = 8
		{1990, 5, 17, 5},  // 1, 5, 8 → 14 → 5
		{1961, 8, 4, 2},   // 8, 8, 4 → 20 → 2
		{1991, 1, 8, 11},  // 2, 1, 8 → 11 (master kept)
		{2003, 11, 6, 22}, // 5, 11, 6 → 22 (master kept)
		{1985, 11, 29, 9}, // 5, 11, 11 → 27 → 9
	}
	for _, tt := range tests {
		if got := LifePath(tt.y, tt.m, tt.d); got != tt.want {
			t.Errorf("LifePath(%d-%02d-%02d) = %d, want %d", tt.y, tt.m, tt.d, got, tt.want)
		}
	}
}
