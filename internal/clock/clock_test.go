// Package clock is the single injectable clock for mtm (see decision
// D-011). All timestamps — created/updated/completed in frontmatter, due
// comparisons, the statusline clock — are local-naive local times
// (decision D-004) and must come from here, never from a bare time.Now()
// at the call site.
package clock

import (
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func TestFixedReturnsConstantTime(t *testing.T) {
	t0 := time.Date(2026, 11, 7, 14, 32, 45, 0, time.Local)
	c := Fixed(t0)
	for i := 0; i < 3; i++ {
		if got := c.Now(); !got.Equal(t0) {
			t.Fatalf("Now() = %v, want %v", got, t0)
		}
	}
}

func TestRealReturnsSystemTime(t *testing.T) {
	before := time.Now()
	got := Real().Now()
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Fatalf("Real().Now() = %v, outside [%v, %v]", got, before, after)
	}
}

func TestZeroClockFallsBackToReal(t *testing.T) {
	before := time.Now()
	var c Clock // zero value
	got := c.Now()
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Fatalf("zero Clock.Now() = %v, outside [%v, %v]", got, before, after)
	}
}

func TestTodayIsMidnightLocal(t *testing.T) {
	t0 := time.Date(2026, 11, 7, 14, 32, 45, 0, time.Local)
	got := Fixed(t0).Today()
	y, m, d := got.Date()
	if y != 2026 || m != time.November || d != 7 {
		t.Fatalf("Today() date = %v, want 2026-11-07", got)
	}
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 || got.Nanosecond() != 0 {
		t.Fatalf("Today() = %v, want midnight", got)
	}
}

func TestStampLayoutIsLocalNaive(t *testing.T) {
	t0 := time.Date(2026, 11, 7, 14, 32, 45, 0, time.Local)
	got := Fixed(t0).Stamp()
	want := t0.Format(core.TimestampFormat)
	if got != want {
		t.Fatalf("Stamp() = %q, want %q", got, want)
	}
	// Layout must not carry a zone offset (D-004).
	if len(got) != len("2006-01-02T15:04") {
		t.Fatalf("Stamp() = %q, want %d chars, local-naive", got, len("2006-01-02T15:04"))
	}
}

func TestStampAtFormatsGivenTime(t *testing.T) {
	t0 := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)
	if got, want := Fixed(t0).StampAt(t0), "2026-01-02T03:04"; got != want {
		t.Fatalf("StampAt() = %q, want %q", got, want)
	}
}
