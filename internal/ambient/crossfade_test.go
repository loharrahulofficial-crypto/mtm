package ambient

import (
	"testing"
	"time"
)

// crossfade_test.go — §4.3: endpoints exact, midpoint is the per-cell mean.

func TestCrossfadeEndpointsExact(t *testing.T) {
	s := New("night", ModeHalfblock)
	st := stateFor(0.5, PhaseWork)
	a := s.Frame(2*time.Second, 60, 14, st)
	st.Progress = 0.75
	b := s.Frame(9*time.Second, 60, 14, st)
	if got := Crossfade(a, b, 0); !equalLines(got, a) {
		t.Fatal("t=0 did not return the source frame")
	}
	if got := Crossfade(a, b, 1); !equalLines(got, b) {
		t.Fatal("t=1 did not return the target frame")
	}
}

func equalLines(a, b []Line) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCrossfadeMidpointIsCellMean(t *testing.T) {
	triplet := func(hex string) string {
		r, g, b := hexToRGB(hex)
		return rgbTriplet(r, g, b)
	}
	mk := func(hexBase, hexInk string) []Line {
		return []Line{"\x1b[38;2;" + triplet(hexInk) + "m\x1b[48;2;" + triplet(hexBase) + "m█\x1b[0m"}
	}
	a := mk("#000000", "#000000")
	b := mk("#ffffff", "#ffffff")
	got := Crossfade(a, b, 0.5)
	want := "\x1b[38;2;128;128;128m\x1b[48;2;128;128;128m█\x1b[0m"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("midpoint = %q, want %q", got, want)
	}
}

func TestCrossfadeDeterministic(t *testing.T) {
	s := New("ocean", ModeHalfblock)
	st := stateFor(0.5, PhaseWork)
	s2 := New("rain", ModeHalfblock)
	a := s.Frame(1*time.Second, 40, 12, st)
	b := s2.Frame(2*time.Second, 40, 12, st)
	for _, tt := range []float64{0.15, 0.5, 0.83} {
		got1 := Crossfade(a, b, tt)
		got2 := Crossfade(a, b, tt)
		if stringsJoin(got1) != stringsJoin(got2) {
			t.Fatalf("t=%v: crossfade not deterministic", tt)
		}
	}
}

func stringsJoin(lines []Line) string {
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}
