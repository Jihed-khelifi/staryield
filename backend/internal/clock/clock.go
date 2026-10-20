// Package clock abstracts time so business rules (billing minutes, grace
// periods, expiries) are deterministic under test.
package clock

import (
	"sync"
	"time"
)

// Clock reports the current time in UTC.
type Clock interface {
	Now() time.Time
}

// Real is the wall clock.
type Real struct{}

// Now returns the current UTC time truncated to microseconds, matching
// PostgreSQL timestamptz precision so round-trips compare equal.
func (Real) Now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

// Fake is a manually-advanced clock for tests.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake returns a fake clock starting at t.
func NewFake(t time.Time) *Fake { return &Fake{now: t.UTC().Truncate(time.Microsecond)} }

// Now returns the fake time.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance moves the clock forward by d.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	f.mu.Unlock()
}

// Set moves the clock to t.
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	f.now = t.UTC().Truncate(time.Microsecond)
	f.mu.Unlock()
}
