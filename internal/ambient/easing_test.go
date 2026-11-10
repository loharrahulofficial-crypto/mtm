package ambient

import (
	"math"
	"testing"
)

// easing_test.go — §4.1: curves are deterministic, bounded, monotonic
// where monotonicity is expected, and identical at any FPS by construction.

func TestEasingEndpoints(t *testing.T) {
	curves := map[string]func(float64) float64{
		"outCubic":   EaseOutCubic,
		"inOutCubic": EaseInOutCubic,
		"outBack":    EaseOutBack,
		"inQuad":     EaseInQuad,
		"outQuad":    EaseOutQuad,
		"inOutSine":  EaseInOutSine,
		"smoothstep": Smoothstep,
	}
	for name, fn := range curves {
		for _, tc := range []struct{ in, want float64 }{
			{0, 0}, {1, 1},
		} {
			if got := fn(tc.in); math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("%s(%v) = %v, want %v", name, tc.in, got, tc.want)
			}
		}
		// bounded in [0, ~1.1] (outBack may overshoot)
		for i := 0; i <= 100; i++ {
			v := fn(float64(i) / 100)
			if math.IsNaN(v) || v < -0.01 || v > 1.15 {
				t.Errorf("%s(%.2f) = %v out of expected range", name, float64(i)/100, v)
			}
		}
	}
}

func TestEnvelopeShape(t *testing.T) {
	// rises 0.1–0.3, holds to 0.6, releases to 0.9
	if got := Envelope(0.05, 0.1, 0.3, 0.6, 0.9); got != 0 {
		t.Errorf("before envelope: %v", got)
	}
	if got := Envelope(0.2, 0.1, 0.3, 0.6, 0.9); got <= 0.5 || got > 1 {
		t.Errorf("rising edge: %v", got)
	}
	if got := Envelope(0.45, 0.1, 0.3, 0.6, 0.9); got != 1 {
		t.Errorf("hold: %v", got)
	}
	if got := Envelope(0.999, 0.1, 0.3, 0.6, 0.9); got != 0 {
		t.Errorf("after envelope: %v", got)
	}
}
