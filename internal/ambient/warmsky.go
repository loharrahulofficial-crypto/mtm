package ambient

import "math"

// warmsky.go — warm dusk-horizon skies, interpolated in OKLCH from the
// cool zenith down to a soft warm horizon. The hue path follows the
// sunset arc (blue → violet → magenta → coral → amber), the chroma is
// preserved (no grey or brown midpoints), and the horizon color itself
// derives from AccentSoft/Warning, lifted in lightness.

// oklch is a cylindrical OKLCH color.
type oklch struct{ l, c, h float64 }

// hexToOKLCH converts an sRGB hex to OKLCH.
func hexToOKLCH(hex string) oklch {
	o := hexToOKLab(hex)
	h, c := oklabHue(o)
	return oklch{l: o.l, c: c, h: h}
}

// oklchToHex converts back to sRGB (clamped).
func oklchToHex(o oklch) string {
	a := o.c * math.Cos(o.h*math.Pi/180)
	b := o.c * math.Sin(o.h*math.Pi/180)
	return oklabToHex(oklab{l: clampf(o.l, 0, 1), a: a, b: b})
}

// oklchMix blends two hex colors in OKLCH along the shortest hue arc.
// Chroma and lightness lerp linearly, so the midpoint never collapses
// toward grey.
func oklchMix(from, to string, t float64) string {
	a, b := hexToOKLCH(from), hexToOKLCH(to)
	h := a.h
	dh := b.h - a.h
	if dh > 180 {
		dh -= 360
	} else if dh < -180 {
		dh += 360
	}
	return oklchToHex(oklch{
		l: a.l*(1-t) + b.l*t,
		c: a.c*(1-t) + b.c*t,
		h: h + dh*t,
	})
}

// warmHorizon derives the horizon haze from the theme's warm accent
// roles: the more chromatic of AccentSoft/Warning, lifted in lightness,
// chroma preserved (never a muddy brown).
func warmHorizon(p Palette) string {
	soft := hexToOKLCH(p.AccentSoft)
	warn := hexToOKLCH(p.Warning)
	base := soft
	if warn.c > soft.c {
		base = warn
	}
	l := base.l + 0.14
	if l > 0.85 {
		l = 0.85
	}
	c := base.c
	if c < 0.075 {
		c = 0.075
	}
	return oklchToHex(oklch{l: l, c: c, h: base.h})
}

// warmSkyStops builds the warm-horizon sky: a cool zenith lifting
// through a violet haze into the soft warm horizon. Four stops,
// t ∈ [0,1], top first. Lightness is monotonically non-decreasing by
// construction (the horizon is guaranteed to sit above the apex).
func warmSkyStops(p Palette) []skyStop {
	// cool zenith: the deepened base tinted 12% toward Blue — keeps every
	// theme's apex cool (a warm-neutral apex lands in the mud zone)
	top := blendHex(blendHex(p.Base, "#000000", 0.40), p.Blue, 0.12)
	horizon := warmHorizon(p)
	topL := hexToOKLCH(top).l
	horizonL := hexToOKLCH(horizon).l
	if horizonL < topL+0.22 {
		// hold the horizon above the apex so lightness can only rise
		o := hexToOKLCH(horizon)
		o.l = topL + 0.22
		if o.l > 0.85 {
			o.l = 0.85
		}
		horizon = oklchToHex(o)
	}
	return []skyStop{
		{top, 0.0},
		{oklchMix(top, horizon, 0.42), 0.42},
		{oklchMix(top, horizon, 0.75), 0.75},
		{horizon, 1.0},
	}
}

// fillSkyOKLCH paints a vertical multi-stop gradient interpolated in
// OKLCH. Used by the warm-horizon scenes.
func fillSkyOKLCH(b *buffer, x0, y0, x1, y1 float64, stops []skyStop, o blendOp) {
	ss := float64(fSSV())
	sx0, sy0 := x0*ss, y0*ss
	sx1, sy1 := x1*ss, y1*ss
	fx0 := int(math.Floor(sx0))
	fx1 := int(math.Ceil(sx1))
	fy0 := int(math.Floor(sy0))
	fy1 := int(math.Ceil(sy1))
	rems := make([]oklch, len(stops))
	for i, s := range stops {
		rems[i] = hexToOKLCH(s.hex)
	}
	for py := fy0; py < fy1; py++ {
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
		// local interpolation in OKLCH between the two stops' anchors
		mix := oklch{
			l: rems[i].l*(1-local) + rems[i+1].l*local,
			c: rems[i].c*(1-local) + rems[i+1].c*local,
			h: rems[i].h,
		}
		dh := rems[i+1].h - rems[i].h
		if dh > 180 {
			dh -= 360
		} else if dh < -180 {
			dh += 360
		}
		mix.h = rems[i].h + dh*local
		hex := oklchToHex(mix)
		r, g, bl, ok := hexF(hex)
		if !ok {
			continue
		}
		for px := fx0; px < fx1; px++ {
			i := py*b.fb.pw + px
			if i < 0 || i >= len(b.fb.px) {
				continue
			}
			b.fb.px[i] = fpix{r, g, bl, 1}
		}
	}
}
