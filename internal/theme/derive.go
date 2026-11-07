package theme

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Blend mixes hex color `from` toward hex color `to` by fraction t
// (0 = from, 1 = to). Non-hex inputs (terminal color names/numbers the
// terminal-config fallback produced) are returned unchanged — blending
// them is impossible, so callers keep working with the raw color.
func Blend(from, to string, t float64) string {
	fr, ok := parseHex(from)
	if !ok {
		return from
	}
	to_, ok := parseHex(to)
	if !ok {
		return from
	}
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	mix := func(a, b uint8) uint8 { return uint8(math.Round(float64(a)*(1-t) + float64(b)*t)) }
	return fmt.Sprintf("#%02x%02x%02x", mix(fr.r, to_.r), mix(fr.g, to_.g), mix(fr.b, to_.b))
}

// Soften blends c 15% toward the base background — used for course colors
// when applied as fills, stripes and tints (design language:
// "soften it by blending 15% toward the background").
func Soften(c, base string) string { return Blend(c, base, 0.15) }

type rgb struct{ r, g, b uint8 }

func parseHex(s string) (rgb, bool) {
	s = strings.TrimSpace(s)
	if len(s) == 4 { // #abc → #aabbcc
		return parseHex("#" + strings.Repeat(string(s[1]), 2) + strings.Repeat(string(s[2]), 2) + strings.Repeat(string(s[3]), 2))
	}
	if len(s) != 7 || s[0] != '#' {
		return rgb{}, false
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return rgb{}, false
	}
	return rgb{uint8(v >> 16), uint8(v >> 8), uint8(v)}, true
}

// relLuminance returns the WCAG relative luminance of a hex color.
func relLuminance(hex string) float64 {
	c, ok := parseHex(hex)
	if !ok {
		return 0
	}
	chan_ := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*chan_(c.r) + 0.7152*chan_(c.g) + 0.0722*chan_(c.b)
}

// ContrastRatio returns the WCAG contrast ratio between two hex colors.
func ContrastRatio(a, b string) float64 {
	la, lb := relLuminance(a), relLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// ensureContrast blends fg toward `toward` until it meets min ratio against
// bg, or gives up at t=1 (the strongest correction available).
func ensureContrast(fg, bg, toward string, min float64) string {
	if ContrastRatio(fg, bg) >= min {
		return fg
	}
	for t := 0.1; t <= 1.0; t += 0.1 {
		candidate := Blend(fg, toward, t)
		if ContrastRatio(candidate, bg) >= min {
			return candidate
		}
	}
	return Blend(fg, toward, 1)
}

const (
	surfaceT        = 0.04
	raisedT         = 0.08
	strongT         = 0.12
	borderT         = 0.22
	brightenT       = 0.30
	minTextOnBase   = 4.5
	minAccentOnBase = 3.0
	minMutedOnBase  = 3.0
)

// Derive computes the semantic roles from the raw Omarchy palette and
// stores them on p. It is idempotent. All roles are derived by blending
// Omarchy colors — no colors are invented outside this function.
func (p *Palette) Derive() {
	base := p.Background
	toward := "#ffffff"
	if strings.EqualFold(p.Mode, "light") {
		toward = "#000000"
	}

	p.Base = base
	p.Surface = Blend(base, p.Foreground, surfaceT)
	p.SurfaceRaised = Blend(base, p.Foreground, raisedT)
	p.SurfaceStrong = Blend(base, p.Foreground, strongT)
	p.Border = Blend(base, p.Foreground, borderT)
	p.Text = ensureContrast(p.Foreground, p.Surface, p.BrightFg, minTextOnBase)
	p.BrightText = ensureContrast(p.BrightFg, p.SurfaceRaised, toward, minTextOnBase)
	p.MutedSoft = ensureContrast(p.Muted, p.Surface, p.Text, minMutedOnBase)
	p.AccentSoft = Soften(p.Accent, base)
	p.Accent = ensureContrast(p.Accent, p.Surface, toward, minAccentOnBase)
	p.Success = p.Green
	p.Warning = p.Yellow
	p.Danger = p.Red

	p.HeatWarm = p.Warning
	p.HeatHot = p.Orange
	p.HeatOverdue = p.Danger

	// Course palette: Omarchy ANSI-ish hues, softened 15% toward base.
	// Order is stable so a given project always lands on the same hue.
	raw := []string{p.Red, p.Orange, p.Yellow, p.Green, p.Cyan, p.Blue, p.Magenta, p.Brown}
	p.CoursePalette = make([]string, 0, len(raw))
	for _, c := range raw {
		if c == "" {
			continue
		}
		p.CoursePalette = append(p.CoursePalette, Soften(c, base))
	}

	// Heatmap: empty cell = strong surface, then 4 accent steps.
	p.Heatmap = []string{
		p.SurfaceStrong,
		Blend(base, p.Accent, 0.25),
		Blend(base, p.Accent, 0.45),
		Blend(base, p.Accent, 0.70),
		p.Accent,
	}
}

// courseNamed resolves a user-supplied color name from project.yaml
// ("red", "bright_blue", "ansi:3", or a hex string) against p.
func courseNamed(p Palette, name string) (string, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(n, "#") {
		if _, ok := parseHex(n); ok {
			return n, true
		}
		return "", false
	}
	if strings.HasPrefix(n, "ansi:") {
		i, err := strconv.Atoi(strings.TrimPrefix(n, "ansi:"))
		if err == nil && i >= 0 && i < len(p.CoursePalette) {
			// ANSI indices follow the course palette order
			// (red orange yellow green cyan blue magenta brown).
			return p.CoursePalette[i], true
		}
		return "", false
	}
	named := map[string]string{
		"red": p.Red, "orange": p.Orange, "yellow": p.Yellow,
		"green": p.Green, "cyan": p.Cyan, "blue": p.Blue,
		"magenta": p.Magenta, "brown": p.Brown, "accent": p.Accent,
	}
	c, ok := named[n]
	return c, ok && c != ""
}

// CourseColor returns the display color for a project/course. A `color:`
// override in project.yaml wins (hex, named or "ansi:N"); otherwise the
// color is chosen deterministically from the course palette by hashing the
// project name. Returned colors are softened toward the base.
func CourseColor(p Palette, project, override string) string {
	if override != "" {
		if c, ok := courseNamed(p, override); ok {
			return Soften(c, p.Base)
		}
	}
	if len(p.CoursePalette) == 0 {
		return Soften(p.Accent, p.Base)
	}
	h := fnv32a(project)
	return p.CoursePalette[int(h)%len(p.CoursePalette)]
}

// CourseColorStrong is CourseColor without softening — for dots and stripe
// accents where a touch more saturation reads better.
func CourseColorStrong(p Palette, project, override string) string {
	if override != "" {
		if c, ok := courseNamed(p, override); ok {
			return c
		}
	}
	if len(p.CoursePalette) == 0 {
		return p.Accent
	}
	return p.CoursePalette[int(fnv32a(project))%len(p.CoursePalette)]
}

func fnv32a(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}
