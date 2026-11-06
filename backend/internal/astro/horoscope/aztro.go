package horoscope

import (
	"context"
	"strconv"
	"strings"
)

// Aztro is the response shape of the public aztro API
// (POST https://aztro.sameerkumar.website/?sign=…&day=…), extended with a
// "staryield" block. Unlike aztro, values carry no leading spaces and are
// localised.
type Aztro struct {
	DateRange     string    `json:"date_range"`
	CurrentDate   string    `json:"current_date"`
	Description   string    `json:"description"`
	Compatibility string    `json:"compatibility"`
	Mood          string    `json:"mood"`
	Color         string    `json:"color"`
	LuckyNumber   string    `json:"lucky_number"`
	LuckyTime     string    `json:"lucky_time"`
	Staryield     AztroMeta `json:"staryield"`
}

// AztroMeta carries the canonical values behind an aztro response.
type AztroMeta struct {
	Sign          string `json:"sign"`
	Date          string `json:"date"`
	TZ            string `json:"tz"`
	Locale        string `json:"locale"`
	MoonSign      string `json:"moon_sign"`
	MoonPhase     string `json:"moon_phase"`
	Provider      string `json:"provider"`
	EngineVersion string `json:"engine_version"`
}

// ToAztro maps a reading to the aztro shape.
func ToAztro(r Reading) Aztro {
	return Aztro{
		DateRange:     r.DateRange.Label,
		CurrentDate:   r.CurrentDate,
		Description:   r.Text,
		Compatibility: r.CompatibleSignName,
		Mood:          r.Mood,
		Color:         r.Color,
		LuckyNumber:   strconv.Itoa(r.LuckyNumber),
		LuckyTime:     r.LuckyTime,
		Staryield: AztroMeta{
			Sign: r.Sign, Date: r.Date, TZ: r.TZ, Locale: r.Locale, MoonSign: r.MoonSign, MoonPhase: r.MoonPhase,
			Provider: r.Provider, EngineVersion: r.EngineVersion,
		},
	}
}

// Day names accepted by the aztro API.
const (
	DayYesterday = "yesterday"
	DayToday     = "today"
	DayTomorrow  = "tomorrow"
)

// ParseDay returns the day offset for "yesterday", "today" (also "") or
// "tomorrow", case-insensitively.
func ParseDay(day string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(day)) {
	case "", DayToday:
		return 0, true
	case DayYesterday:
		return -1, true
	case DayTomorrow:
		return 1, true
	}
	return 0, false
}

// Feed is a placeholder provider for a licensed third-party horoscope feed.
// Until a feed is contracted it is never configured, and it delegates every
// request to its fallback (normally the Engine), so swapping providers is a
// wiring change rather than an API change.
type Feed struct {
	// Endpoint and APIKey identify the feed once one is licensed.
	Endpoint string
	APIKey   string
	Fallback Provider
}

// Name implements Provider.
func (f Feed) Name() string {
	if f.Endpoint == "" {
		return f.fallback().Name()
	}
	return "feed"
}

func (f Feed) fallback() Provider {
	if f.Fallback != nil {
		return f.Fallback
	}
	return Engine{}
}

// Daily implements Provider. No feed is integrated yet, so the fallback
// always answers.
func (f Feed) Daily(ctx context.Context, req Request) (Reading, error) {
	return f.fallback().Daily(ctx, req)
}
