package ambient

import (
	"fmt"
	"math"
)

// oklab.go — OKLab conversions (Björn Ottosson) for perceptually uniform
// gradients. Used by the night-sky ramp; the interpolation stays in the
// perceptual space the whole way, so a Base→Cyan ramp passes through no
// muddy detours.

type oklab struct{ l, a, b float64 }

func parseSRGBHexF(hex string) (r, g, b float64, ok bool) {
	c, ok := parseHex(hex)
	if !ok {
		return 0, 0, 0, false
	}
	return float64(c.r) / 255, float64(c.g) / 255, float64(c.b) / 255, true
}

func linearToOKLab(r, g, b float64) oklab {
	// sRGB (0..1) → linear is done by the caller via srgbToLinear.
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	return oklab{
		l: 0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		a: 1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		b: 0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
	}
}

func oklabToLinearSRGB(o oklab) (r, g, b float64) {
	l := o.l + 0.3963377774*o.a + 0.2158037573*o.b
	m := o.l - 0.1055613458*o.a - 0.0638541728*o.b
	s := o.l - 0.0894841775*o.a - 1.2914855480*o.b
	l, m, s = l*l*l, m*m*m, s*s*s
	return 4.0767416621*l - 3.3077115913*m + 0.2309699292*s,
		-1.2684380046*l + 2.6097574011*m - 0.3413193965*s,
		-0.0041960863*l - 0.7034186147*m + 1.7076147010*s
}

// hexToOKLab converts a hex color (or any parseHex-able string) to OKLab.
// Uses the same linearization as blendHex keeps the two paths in the
// same color-managing world.
func hexToOKLab(hex string) oklab {
	c, ok := parseHex(hex)
	if !ok {
		return oklab{0.5, 0, 0}
	}
	return linearToOKLab(srgbToLinear(c.r), srgbToLinear(c.g), srgbToLinear(c.b))
}

// oklabToHex converts back to #rrggbb (clamped).
func oklabToHex(o oklab) string {
	r, g, b := oklabToLinearSRGB(o)
	return fmt.Sprintf("#%02x%02x%02x",
		uint8(math.Round(linearToSRGB(clampf(r, 0, 1)))),
		uint8(math.Round(linearToSRGB(clampf(g, 0, 1)))),
		uint8(math.Round(linearToSRGB(clampf(b, 0, 1)))),
	)
}

// oklabMix blends two hex colors by t in OKLab space — no hue detours
// through gray mush or brown detours.
func oklabMix(from, to string, t float64) string {
	fromO, toO := hexToOKLab(from), hexToOKLab(to)
	return oklabToHex(oklab{
		l: fromO.l*(1-t) + toO.l*t,
		a: fromO.a*(1-t) + toO.a*t,
		b: fromO.b*(1-t) + toO.b*t,
	})
}

// oklabHue returns the OKLab hue angle in degrees [0, 360) and the
// chroma magnitude.
func oklabHue(o oklab) (degrees, chroma float64) {
	c := math.Hypot(o.a, o.b)
	h := math.Atan2(o.b, o.a) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return h, c
}

// nightSkyStops builds the night sky: Base at the top lifting through
// Blue into a Cyan horizon, ending a touch lifted toward BrightText —
// all OKLab-interpolated. Shared by the painter and the hue test.
func nightSkyStops(p Palette) []skyStop {
	base := p.Base
	bright := p.BrightText
	blue := p.Blue
	cyan := p.Cyan
	return []skyStop{
		{base, 0.0},
		{oklabMix(base, blue, 0.42), 0.45},
		{oklabMix(base, cyan, 0.50), 0.76},
		{oklabMix(oklabMix(base, cyan, 0.55), bright, 0.12), 1.0},
	}
}

// skyStop is one anchor of the night-sky gradient (t ∈ [0,1], 0 = top).
type skyStop struct {
	hex string
	t   float64
}

// fillSkyOKLab paints a vertical multi-stop gradient interpolated in
// OKLab. Stops must span [0,1] in t order; the ends are clamped.
func fillSkyOKLab(b *buffer, x0, y0, x1, y1 float64, stops []skyStop, o blendOp) {
	if len(stops) < 2 {
		return
	}
	ss := float64(fSSV())
	sx0, sy0 := x0*ss, y0*ss
	sx1, sy1 := x1*ss, y1*ss
	fx0 := int(math.Floor(sx0))
	fx1 := int(math.Ceil(sx1))
	fy0 := int(math.Floor(sy0))
	fy1 := int(math.Ceil(sy1))
	rem := make([]oklab, len(stops))
	for i, s := range stops {
		rem[i] = hexToOKLab(s.hex)
	}
	rgbAt := func(py int) (float32, float32, float32) {
		pyf := float64(py) + 0.5
		t := (pyf - sy0) / (sy1 - sy0)
		t = clampf(t, 0, 1)
		i := 0
		for i < len(stops)-2 && t > stops[i+1].t {
			i++
		}
		lo, hi := stops[i], stops[i+1]
		local := 0.0
		if hi.t > lo.t {
			local = (t - lo.t) / (hi.t - lo.t)
		}
		o := oklab{
			l: rem[i].l*(1-local) + rem[i+1].l*local,
			a: rem[i].a*(1-local) + rem[i+1].a*local,
			b: rem[i].b*(1-local) + rem[i+1].b*local,
		}
		r, g, b := oklabToLinearSRGB(o)
		// fb stores sRGB-encoded 0..1 channel values (like rgbF/baseFill)
		const inv255 = 1.0 / 255.0
		return float32(linearToSRGB(clampf(r, 0, 1)) * inv255),
			float32(linearToSRGB(clampf(g, 0, 1)) * inv255),
			float32(linearToSRGB(clampf(b, 0, 1)) * inv255)
	}
	for py := fy0; py < fy1; py++ {
		r, g, bch := rgbAt(py)
		for px := fx0; px < fx1; px++ {
			b.fb.blendPx(px, py, fpix{r, g, bch, 1}, o)
		}
	}
}
