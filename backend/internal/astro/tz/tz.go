// Package tz resolves civil (wall-clock) birth times to instants using an
// embedded copy of the IANA time zone database compiled WITH the "backzone"
// data (see scripts/tzdata/build.sh). The main tz data merges many zones
// whose clocks only agreed after 1970 into links (Europe/Oslo → Berlin,
// Atlantic/Reykjavik → Abidjan …), which silently shifts older births by an
// hour or more; backzone keeps their real history.
//
// Resolve also reports wall times that a transition skipped or repeated, and
// replaces a zone's "LMT" offset (the local mean time of the zone's
// reference city) with the local mean time of the actual birthplace.
package tz

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed zoneinfo.zip
var zoneinfoZip []byte

var (
	loadOnce sync.Once
	files    map[string]*zip.File
	version  string
	loadErr  error
	cache    sync.Map // name → *time.Location
)

func open() error {
	loadOnce.Do(func() {
		r, err := zip.NewReader(bytes.NewReader(zoneinfoZip), int64(len(zoneinfoZip)))
		if err != nil {
			loadErr = err
			return
		}
		files = make(map[string]*zip.File, len(r.File))
		for _, f := range r.File {
			if f.Name == "+VERSION" {
				data, err := readFile(f)
				if err != nil {
					loadErr = err
					return
				}
				version = strings.TrimSpace(string(data))
				continue
			}
			files[f.Name] = f
		}
	})
	return loadErr
}

func readFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 1<<20))
}

// Version returns the tzdata release embedded, such as "2026d".
func Version() string {
	if err := open(); err != nil {
		return ""
	}
	return version
}

// namePattern restricts zone names to the IANA character set before any
// lookup (no path traversal, no surprises).
var namePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+\-]*(/[A-Za-z0-9_+\-]+){0,2}$`)

// ErrUnknownZone is returned for names that are not IANA zones.
var ErrUnknownZone = errors.New("tz: unknown time zone")

// Load returns the zone called name (such as "Europe/Paris" or "UTC").
// Locations are cached and safe for concurrent use.
func Load(name string) (*time.Location, error) {
	if v, ok := cache.Load(name); ok {
		return v.(*time.Location), nil
	}
	if len(name) > 64 || !namePattern.MatchString(name) {
		return nil, fmt.Errorf("%w: %q", ErrUnknownZone, name)
	}
	if err := open(); err != nil {
		return nil, err
	}
	f, ok := files[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownZone, name)
	}
	data, err := readFile(f)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocationFromTZData(name, data)
	if err != nil {
		return nil, err
	}
	cache.Store(name, loc)
	return loc, nil
}

// Valid reports whether name is an embedded zone.
func Valid(name string) bool {
	if len(name) > 64 || !namePattern.MatchString(name) || open() != nil {
		return false
	}
	_, ok := files[name]
	return ok
}

// Names returns every embedded zone name, sorted.
func Names() []string {
	if open() != nil {
		return nil
	}
	out := make([]string, 0, len(files))
	for n := range files {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// WallStatus describes how a wall-clock time mapped to an instant.
type WallStatus string

// Wall-clock outcomes.
const (
	// WallOK means the wall time existed exactly once.
	WallOK WallStatus = "ok"
	// WallSkipped means a transition skipped the wall time (spring
	// forward); the offset in force before the transition was used.
	WallSkipped WallStatus = "skipped"
	// WallAmbiguous means the wall time occurred twice (fall back); the
	// earlier instant was used.
	WallAmbiguous WallStatus = "ambiguous"
	// WallLMT means the zone kept local mean time then; the birthplace's own
	// mean time (4 minutes per degree of longitude) was used.
	WallLMT WallStatus = "lmt"
)

// ErrInvalidClock is returned for a malformed time of day.
var ErrInvalidClock = errors.New("tz: invalid clock time, want HH:MM")

// ParseClock parses a 24-hour "HH:MM" (a one-digit hour is accepted).
func ParseClock(clock string) (hour, minute int, err error) {
	clock = strings.TrimSpace(clock)
	hs, ms, ok := strings.Cut(clock, ":")
	if !ok || len(hs) < 1 || len(hs) > 2 || len(ms) != 2 || !digits(hs) || !digits(ms) {
		return 0, 0, ErrInvalidClock
	}
	hour, _ = strconv.Atoi(hs)
	minute, _ = strconv.Atoi(ms)
	if hour > 23 || minute > 59 {
		return 0, 0, ErrInvalidClock
	}
	return hour, minute, nil
}

func digits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// Resolve converts the calendar day of date (its year, month and day; its
// location and time of day are ignored) and the wall clock hour:minute in
// loc to an instant. lonEast is the birthplace's longitude in degrees (east
// positive), used only when the zone kept local mean time; pass NaN (or any
// value outside ±180°) when it is unknown to keep the zone's own LMT offset.
func Resolve(date time.Time, hour, minute int, loc *time.Location, lonEast float64) (time.Time, WallStatus) {
	y, m, d := date.Date()
	wall := time.Date(y, m, d, hour, minute, 0, 0, time.UTC).Unix()

	offsetAt := func(unix int64) (int64, string) {
		name, off := time.Unix(unix, 0).In(loc).Zone()
		return int64(off), name
	}
	before, _ := offsetAt(wall - 86400)
	after, _ := offsetAt(wall + 86400)

	var valid []int64
	for _, off := range []int64{before, after} {
		u := wall - off
		if o, _ := offsetAt(u); o == off && (len(valid) == 0 || valid[0] != u) {
			valid = append(valid, u)
		}
	}
	status := WallOK
	var instant int64
	switch len(valid) {
	case 0:
		status, instant = WallSkipped, wall-before
	case 1:
		instant = valid[0]
	default:
		status, instant = WallAmbiguous, min(valid[0], valid[1])
	}
	if _, abbr := offsetAt(instant); abbr == "LMT" && lonEast >= -180 && lonEast <= 180 { // false for NaN
		lmt := int64(math.Round(lonEast * 240)) // 4 minutes per degree
		return time.Unix(wall-lmt, 0).UTC(), WallLMT
	}
	return time.Unix(instant, 0).UTC(), status
}

// OffsetAt returns the UTC offset in seconds and the abbreviation in force
// in loc at instant t.
func OffsetAt(t time.Time, loc *time.Location) (int, string) {
	name, off := t.In(loc).Zone()
	return off, name
}
