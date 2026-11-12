package ambient

import (
	"math"
	"time"
)

// moonPhase returns the lunar phase for `t`: fraction of the synodic
// month since the reference new moon (2000-01-06 18:14 UTC). 0 = new,
// 0.5 = full. Deterministic; no network, no ephemeris tables.
func moonPhase(t time.Time) float64 {
	ref := time.Date(2000, 1, 6, 18, 14, 0, 0, time.UTC)
	synodic := 29.53058867 * 24 * float64(time.Hour)
	d := float64(t.Sub(ref)) / synodic
	d = math.Mod(d, 1)
	if d < 0 {
		d += 1
	}
	return d
}

// moonIllumination returns the lit fraction of the disc (0 new .. 1 full).
func moonIllumination(phase float64) float64 {
	return 0.5 * (1 - math.Cos(2*math.Pi*phase))
}

// moonWaxing reports whether the visible disc is growing toward full.
func moonWaxing(phase float64) bool { return phase < 0.5 }

// moonAge returns days since the last new moon.
func moonAge(t time.Time) float64 { return moonPhase(t) * 29.53058867 }

// sunTimes approximates local sunrise/sunset (hours since midnight, local)
// for the date at a fixed mid-latitude (23.3°N — a reasonable default for
// the user's timezone; the scenes only need the relative shape).
func sunTimes(date time.Time) (rise, set float64) {
	dayOfYear := float64(date.YearDay())
	decl := -23.44 * math.Cos(2*math.Pi*(dayOfYear+10)/365.25)
	lat := 23.3 * math.Pi / 180
	dc := math.Pi / 180
	cosH := -math.Tan(lat) * math.Tan(decl*dc)
	cosH = clampf(cosH, -1, 1)
	hourAngle := math.Acos(cosH) * 180 / math.Pi // degrees
	noon := 12.0
	rise = noon - hourAngle/15
	set = noon + hourAngle/15
	return rise, set
}

// sunAltitude returns the sun's fractional elevation for local hour (−1
// below horizon .. 1 zenith). Used to switch ocean sun/sky tones.
func sunAltitude(date time.Time) float64 {
	h := float64(date.Hour()) + float64(date.Minute())/60 + float64(date.Second())/3600
	rise, set := sunTimes(date)
	if h <= rise || h >= set {
		// Night: altitude below horizon, deeper as midnight nears.
		midnight := 0.0
		if h < 6 {
			midnight = -0.5 - 0.5*math.Cos(2*math.Pi*h/rise)
		} else {
			midnight = -0.5 - 0.5*math.Cos(2*math.Pi*(h-set)/(24-set))
		}
		return clampf(midnight, -1, 0)
	}
	frac := (h - rise) / (set - rise)
	return clampf(math.Sin(math.Pi*frac), 0, 1)
}

// isNight reports whether it is dark enough to see stars at `date`.
func isNight(date time.Time) bool { return sunAltitude(date) <= -0.05 }

// moonCols bundles the precomputed float channels of the moon's layered
// tones (the current three-disc look) for per-pixel evaluation.
type moonCols struct{ deep, mid, high, shadow [3]float32 }

// MoonCols builds the moon's layered tones from theme colors: the outer
// limb tone, the mid disc, the bright core, and the shadow side. Colors
// arrive as hex and are float channels in 0..1.
func moonColsFrom(deep, mid, high, shadow string) moonCols {
	pick := func(hex string) [3]float32 {
		r, g, b, ok := hexF(hex)
		if !ok {
			// invalid theme role: stay on a neutral light gray rather than drop the body
			return [3]float32{0.8, 0.8, 0.8}
		}
		return [3]float32{r, g, b}
	}
	return moonCols{pick(deep), pick(mid), pick(high), pick(shadow)}
}

// moonShade evaluates the moon's layered shading at (x, y) in sub-cell
// coordinates. Returns the layer color, per-channel, plus an anti-aliased
// coverage 0..1 (0 outside the disc). The terminator is the sliding
// shadow disc of the current painter; item-phase realism (Lambert
// sphere, earthshine, maria) replaces these layers in-place later — the
// callers (direct draw, lake reflection) stay unchanged.
func moonShade(mX, mY, mR, phase float64, cols moonCols, x, y float64) ([3]float32, float64) {
	var zero [3]float32
	d := math.Hypot(x-mX, y-mY)
	if d > mR+0.8 {
		return zero, 0
	}
	col := cols.deep
	switch {
	case d <= mR*0.45:
		col = cols.high
	case d <= mR*0.78:
		col = cols.mid
	}
	// terminator: the shadow disc slides across with the phase
	term := math.Cos(2*math.Pi*phase) * mR * 1.12
	scx := mX - term
	if !moonWaxing(phase) {
		scx = mX + term
	}
	if sd := math.Hypot(x-scx, y-mY); sd <= mR {
		mix := func(a, b float32) float32 { return a*0.03 + b*0.97 }
		col = [3]float32{mix(col[0], cols.shadow[0]), mix(col[1], cols.shadow[1]), mix(col[2], cols.shadow[2])}
	}
	// anti-aliased limb (≈1 sub-pixel)
	cov := clampf(mR-d+0.7, 0, 1)
	return col, cov
}
