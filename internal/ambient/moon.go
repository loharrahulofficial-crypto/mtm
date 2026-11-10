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
