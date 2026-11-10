package ambient

import "math"

// easing.go — shared animation curves. Every motion in ambient and the
// TUI uses these with elapsed time; nothing is ever driven by frame
// counts, so animation speed is FPS-independent.

func EaseOutCubic(t float64) float64 {
	x := clampf(t, 0, 1)
	x = 1 - x
	return 1 - x*x*x
}

func EaseInOutCubic(t float64) float64 {
	x := clampf(t, 0, 1)
	if x < 0.5 {
		return 4 * x * x * x
	}
	x = 2*x - 2
	return 0.5*x*x*x + 1
}

func EaseOutBack(t float64) float64 {
	const c1 = 1.70158
	const c3 = c1 + 1
	x := clampf(t, 0, 1) - 1
	return 1 + c3*x*x*x + c1*x*x
}

func EaseInQuad(t float64) float64 {
	x := clampf(t, 0, 1)
	return x * x
}

func EaseOutQuad(t float64) float64 {
	x := clampf(t, 0, 1)
	return 1 - (1-x)*(1-x)
}

func EaseInOutSine(t float64) float64 {
	x := clampf(t, 0, 1)
	return -(math.Cos(math.Pi*x) - 1) / 2
}

// Smoothstep is the classic Perlin-style falloff.
func Smoothstep(t float64) float64 {
	x := clampf(t, 0, 1)
	return x * x * (3 - 2*x)
}

// Envelope returns a 0..1 pulse: ramps up over [a, b], holds, releases over
// [c, d] — for flashes, pulses and celebrations.
func Envelope(t, a, b, c, d float64) float64 {
	if t < a || t > d {
		return 0
	}
	if t < b {
		return EaseOutCubic((t - a) / (b - a))
	}
	if t <= c {
		return 1
	}
	return 1 - EaseInQuad((t-c)/(d-c))
}
