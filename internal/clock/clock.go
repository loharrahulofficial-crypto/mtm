// Package clock is the single injectable clock for mtm (see decision
// D-011). All timestamps — created/updated/completed in frontmatter, due
// comparisons, heat thresholds, the statusline clock — are local-naive
// local times (decision D-004) and must come from here, never from a bare
// time.Now() at the call site.
//
// Tests inject Now: override Clock.now or pass an explicit time.
package clock

import (
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// Clock produces times. Real uses the system clock; tests use Fixed.
// Store/derived fields are always the caller's local zone (D-004).
type Clock struct {
	now func() time.Time
}

// Real returns the system clock.
func Real() Clock { return Clock{now: time.Now} }

// Fixed returns a clock that always answers t.
func Fixed(t time.Time) Clock { return Clock{now: func() time.Time { return t }} }

// Now is the current local time.
func (c Clock) Now() time.Time {
	if c.now == nil {
		return time.Now()
	}
	return c.now()
}

// Today is midnight of Now() in the local zone.
func (c Clock) Today() time.Time { return dayStart(c.Now()) }

// Stamp formats Now in the frontmatter timestamp layout (local-naive).
func (c Clock) Stamp() string { return c.Now().Format(core.TimestampFormat) }

// StampAt formats t in the frontmatter layout (local-naive).
func (c Clock) StampAt(t time.Time) string { return t.Format(core.TimestampFormat) }

func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
