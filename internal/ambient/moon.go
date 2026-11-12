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

// moonCols bundles the precomputed float channels of the moon's tones.
type moonCols struct{ lit, earth [3]float32 }

// sphereCols builds the moon's tones from theme colors: the lit base
// (BrightText toward Base), and the earthshine — the dark side rendered
// a whisper ABOVE the sky's own luminance so it never reads as a black
// sticker on the sky. skyAt is the sky color at the moon's altitude.
func sphereCols(bright, base, skyAt string) moonCols {
	return moonCols{
		lit:   hexToFloat(blendHex(bright, base, 0.42)),
		earth: hexToFloat(blendHex(skyAt, bright, 0.14)),
	}
}

// haloCol builds the warm-white halo tone shared by everyone.
func haloCol(bright, accent string) [3]float32 {
	return hexToFloat(blendHex(bright, accent, 0.30))
}

func hexToFloat(hex string) [3]float32 {
	r, g, b, ok := hexF(hex)
	if !ok {
		// invalid theme role: stay on a neutral light gray rather than drop the body
		return [3]float32{0.8, 0.8, 0.8}
	}
	return [3]float32{r, g, b}
}

// mariaNoise is the faint fixed noise field for the moon's seas.
var mariaNoise = cachedPerlin(42, 777)

// moonShade evaluates the moon's shading at (x, y) in sub-cell
// coordinates. Physical model: a lit sphere — Lambert diffuse from the
// real phase (n·l over the visible hemisphere), faint noise-based seas,
// and earthshine on the dark side lifted a hair above the sky.
// Anti-aliased limb via coverage. Returns (color, coverage) —
// coverage 0 outside the disc.
func moonShade(mX, mY, mR, phase float64, cols moonCols, x, y float64) ([3]float32, float64) {
	var zero [3]float32
	d := math.Hypot(x-mX, y-mY)
	cov := clampf(mR-d+0.6, 0, 1)
	if cov <= 0 {
		return zero, 0
	}
	nx, ny := (x-mX)/mR, (y-mY)/mR
	z := math.Sqrt(math.Max(0, 1-nx*nx-ny*ny))
	// light: psi is the phase angle from full; at full the sun is
	// face-on (L=+z), at new it is behind the moon (L=-z). Waxing phases
	// carry the light from the right (northern hemisphere).
	illum := moonIllumination(phase)
	psi := math.Pi * (1 - illum)
	s := 1.0
	if !moonWaxing(phase) {
		s = -1.0
	}
	ndotl := nx*math.Sin(psi)*s + z*math.Cos(psi)
	// maria: faint fixed seas, dimming the lit and earthlit sides alike
	mv := mariaNoise.At(nx*2.1+3.1, ny*2.1+7.7)
	mariaDim := 1 - 0.34*clampf((mv-0.12)/0.6, 0, 1)
	if ndotl > 0 {
		l := math.Pow(ndotl, 0.55) * mariaDim
		return [3]float32{cols.lit[0] * float32(l), cols.lit[1] * float32(l), cols.lit[2] * float32(l)}, cov
	}
	return [3]float32{cols.earth[0] * float32(mariaDim), cols.earth[1] * float32(mariaDim), cols.earth[2] * float32(mariaDim)}, cov
}

// MoonGlowAlpha is the halo's radial falloff: Gaussian-like, smooth to
// zero at the rim. Exported so the falloff can be pinned by tests.
func MoonGlowAlpha(d, sigma, amp float64) float64 {
	if d < 0 || sigma <= 0 {
		return 0
	}
	return amp * math.Exp(-(d*d)/(2*sigma*sigma))
}

// paintMoonSphere paints the halo and then the sphere into the
// supersampled raster, directly, pixel by pixel. The halo breathes with
// the scene clock, so reduced motion crawls it automatically.
func paintMoonSphere(b *buffer, mX, mY, mR, phase, ts float64, cols moonCols, haloCol [3]float32) {
	ss := float64(fSSV())
	fb := b.fb
	pulse := 0.62 + 0.38*math.Sin(ts*0.42) // breathes with the scene clock; reduced motion crawls it
	sigma := mR * 0.95
	rOut := mR + sigma*2.7
	amp := 0.40 * pulse
	// halo: gaussian falloff, added under the disc
	x0 := maxInt(0, int((mX-rOut)*ss))
	x1 := minInt(fb.pw, int((mX+rOut)*ss)+1)
	y0 := maxInt(0, int((mY-rOut)*ss))
	y1 := minInt(fb.ph, int((mY+rOut)*ss)+1)
	for py := y0; py < y1; py++ {
		pyf := float64(py)/ss + 0.5/ss
		dy := pyf - mY
		for px := x0; px < x1; px++ {
			pxf := float64(px)/ss + 0.5/ss
			d2 := (pxf-mX)*(pxf-mX) + dy*dy
			if d2 >= rOut*rOut {
				continue
			}
			a := MoonGlowAlpha(math.Sqrt(d2), sigma, amp)
			if a <= 0.004 {
				continue
			}
			// halo also feeds the ink raster a hair so ASCII mode shows a
			// soft blob instead of a hard disc
			if b.mode == ModeASCII {
				fb.inkAt(px, py, float32(a*0.5))
			}
			fb.blendPx(px, py, fpix{haloCol[0], haloCol[1], haloCol[2], float32(a)}, opAdd)
		}
	}
	// sphere
	x0 = maxInt(0, int((mX-mR-1)*ss))
	x1 = minInt(fb.pw, int((mX+mR+1)*ss)+1)
	y0 = maxInt(0, int((mY-mR-1)*ss))
	y1 = minInt(fb.ph, int((mY+mR+1)*ss)+1)
	for py := y0; py < y1; py++ {
		pyf := float64(py)/ss + 0.5/ss
		for px := x0; px < x1; px++ {
			pxf := float64(px)/ss + 0.5/ss
			col, cov := moonShade(mX, mY, mR, phase, cols, pxf, pyf)
			if cov <= 0.004 {
				continue
			}
			fb.inkAt(px, py, float32(cov))
			fb.blendPx(px, py, fpix{col[0], col[1], col[2], float32(cov)}, opNormal)
		}
	}
}
