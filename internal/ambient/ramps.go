package ambient

import (
	"fmt"
	"math"
)

// ramps.go — the shared palette builder for scene layers (§2.6).
//
// Every ramp derives from the theme roles only (background → accent →
// foreground). Adjacent ramp stops are guaranteed at least minLayerContrast
// apart in WCAG 2.x relative luminance: the darker side is blended toward
// Base and the brighter side toward BrightText until the bound is met
// (never violating "theme roles only" — both ends stay on the Base→
// BrightText axis of the theme itself).
//
// Atmospheric perspective: ApplyHaze mixes a color toward the haze/sky
// color by depth, so farther layers read hazier and closer to the sky.

// Ramps are the six shared layer colors scenes paint with.
type Ramps struct {
	SkyTop     string // deep zenith
	SkyHorizon string // warm horizon haze
	Haze       string // distant-layer haze (toward the sky)
	Mid        string // midtone layer body
	Highlight  string // lit surfaces
	Glow       string // glow/bloom core
}

// relativeLuminance returns the WCAG 2.x relative luminance (0..1).
func relativeLuminance(hex string) float64 {
	c, ok := parseHex(hex)
	if !ok {
		return 0
	}
	ch := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.r) + 0.7152*ch(c.g) + 0.0722*ch(c.b)
}

// contrastRatio returns the WCAG contrast ratio between two colors (≥1).
func contrastRatio(a, b string) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

const minLayerContrast = 1.15

// BuildRamps derives the six ramp stops from the theme roles and enforces
// the minimum contrast between adjacent layers.
func BuildRamps(p Palette) Ramps {
	r := Ramps{
		SkyTop:     blendHex(blendHex(p.Base, "#000000", 0.40), p.Accent, 0.06),
		SkyHorizon: blendHex(p.Base, p.Accent, 0.52),
		Mid:        blendHex(p.Base, p.SurfaceStrong, 0.85),
		Highlight:  blendHex(p.Base, p.BrightText, 0.52),
		Glow:       p.BrightText,
	}
	// haze sits between horizon and highlight — distant layers read through it
	r.Haze = blendHex(r.SkyHorizon, p.BrightText, 0.28)
	// enforce adjacency bounds: SkyTop < SkyHorizon < Mid?? no — layers are
	// ordered zenith → horizon → land; each pair below must separate.
	r.SkyTop, r.SkyHorizon = enforceContrast(r.SkyTop, r.SkyHorizon, p, minLayerContrast)
	r.SkyHorizon, r.Haze = enforceContrast(r.SkyHorizon, r.Haze, p, minLayerContrast)
	r.Mid, r.Highlight = enforceContrast(r.Mid, r.Highlight, p, minLayerContrast)
	r.Highlight, r.Glow = enforceContrast(r.Highlight, r.Glow, p, minLayerContrast)
	return r
}

// enforceContrast nudges a dark/bright pair apart toward Base/BrightText
// until their WCAG ratio reaches min. The direction is fixed: the darker
// color gets darker (toward Base), the brighter gets brighter (toward
// BrightText). Bounded iterations keep it total — a monochrome theme
// converges since Base and BrightText always exist.
func enforceContrast(dark, bright string, p Palette, min float64) (string, string) {
	for i := 0; i < 40; i++ {
		ratio := contrastRatio(dark, bright)
		if ratio >= min {
			return dark, bright
		}
		dark = blendHex(dark, p.Base, 0.09)
		bright = blendHex(bright, p.BrightText, 0.09)
	}
	return dark, bright
}

// HazeFor returns the haze fraction for depth d ∈ [0,1] (0 = at the
// viewer, 1 = at the horizon). Monotonic, eased — distant layers dissolve
// gradually instead of flipping from clear to fog.
func HazeFor(depth float64) float64 {
	d := clampf(depth, 0, 1)
	return math.Pow(d, 1.55) * 0.78
}

// ApplyHaze mixes hex toward the haze color by the atmospheric depth.
func (r Ramps) ApplyHaze(hex string, depth float64) string {
	return blendHex(hex, r.Haze, HazeFor(depth))
}

// Summary renders a one-line human description (for logs/doctor).
func (r Ramps) Summary() string {
	return fmt.Sprintf("sky-top %s → horizon %s (haze %s, mid %s, highlight %s, glow %s)",
		r.SkyTop, r.SkyHorizon, r.Haze, r.Mid, r.Highlight, r.Glow)
}
