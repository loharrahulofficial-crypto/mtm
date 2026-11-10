package ambient

import (
	"fmt"
	"strconv"
	"strings"
)

// Palette model for scenes: every paintable color is a resolved hex in a
// fixed table, built once per frame-render from the theme roles. The table
// holds the roles themselves plus quantized blends toward Base. Goldens and
// invariants can therefore enumerate every color the renderer will emit.
type paletteModel struct {
	base   int
	table  []rgb
	hex    []string
	lookup map[string]int
	lum    []int // luminance rank per table index (higher = brighter)
}

const (
	cBase = iota
	cSurface
	cSurfaceRaised
	cSurfaceStrong
	cBorder
	cText
	cBrightText
	cMutedSoft
	cAccent
	cAccentSoft
	cSuccess
	cWarning
	cDanger
	cHeatWarm
	cHeatHot
	cHeatOverdue
	cBlue
	cCyan
	cYellow
	cMagenta
	cOrange
	cBrown
	cRoleCount
)

// blendSteps is the quantized set of interpolations toward Base.
var blendSteps = []float64{0, 0.15, 0.30, 0.45, 0.60, 0.75, 0.875, 0.94}

func buildPalette(p Palette) *paletteModel {
	m := &paletteModel{lookup: map[string]int{}}
	roles := []string{p.Base, p.Surface, p.SurfaceRaised, p.SurfaceStrong, p.Border, p.Text, p.BrightText, p.MutedSoft,
		p.Accent, p.AccentSoft, p.Success, p.Warning, p.Danger, p.HeatWarm, p.HeatHot, p.HeatOverdue,
		p.Blue, p.Cyan, p.Yellow, p.Magenta, p.Orange, p.Brown}
	add := func(hex string) int {
		if i, ok := m.lookup[hex]; ok {
			return i
		}
		c, ok := parseHex(hex)
		if !ok {
			c = rgb{255, 0, 255} // loud fallback for malformed inputs, never silently black
		}
		m.lookup[hex] = len(m.table)
		m.table = append(m.table, c)
		m.hex = append(m.hex, hex)
		return m.lookup[hex]
	}
	for _, r := range roles {
		add(r)
	}
	// Blends toward Base for atmosphere. Only the atmospheric roles get
	// the full ladder; text roles stay raw so type remains legible.
	for _, role := range roles {
		for _, t := range blendSteps {
			if t == 0 {
				continue
			}
			add(blendHex(role, p.Base, t))
		}
	}
	m.base = m.lookup[p.Base]
	m.lum = make([]int, len(m.table))
	for i, c := range m.table {
		m.lum[i] = int(0.2126*float64(c.r) + 0.7152*float64(c.g) + 0.0722*float64(c.b))
	}
	return m
}

// Bright returns the brighter of two indices of the same role family (or two raw indices).
func (m *paletteModel) Bright(i, j int) int {
	if m.lum[i] >= m.lum[j] {
		return i
	}
	return j
}

// Less reports whether table index i is darker than j.
func (m *paletteModel) Less(i, j int) bool { return m.lum[i] < m.lum[j] }

func (m *paletteModel) hexAt(i int) string { return m.hex[i] }

// blendHex is theme.Blend specialized locally so ambient does not import
// the theme package (TUI passes roles in; ambient stays value-only).
func blendHex(from, to string, t float64) string {
	fr, ok1 := parseHex(from)
	to_, ok2 := parseHex(to)
	if !ok1 || !ok2 {
		return from
	}
	if t <= 0 {
		return from
	}
	if t >= 1 {
		return to
	}
	mix := func(a, b uint8) uint8 { return uint8(float64(a)*(1-t) + float64(b)*t + 0.5) }
	return fmt.Sprintf("#%02x%02x%02x", mix(fr.r, to_.r), mix(fr.g, to_.g), mix(fr.b, to_.b))
}

type rgb struct{ r, g, b uint8 }

func parseHex(s string) (rgb, bool) {
	s = strings.TrimSpace(s)
	if len(s) == 4 {
		s = "#" + strings.Repeat(string(s[1]), 2) + strings.Repeat(string(s[2]), 2) + strings.Repeat(string(s[3]), 2)
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

// mixIndex returns the table index of blend(role, base, t), quantizing t
// to the prebuilt ladder so the palette stays finite and deterministic.
func mixIndex(p *paletteModel, roleIdx, baseIdx int, t float64) int {
	if t <= 0 {
		return roleIdx
	}
	if t >= 1 {
		return baseIdx
	}
	best := 1
	bestD := 1e9
	for i, s := range blendSteps {
		d := absF(s - t)
		if d < bestD {
			bestD = d
			best = i
		}
	}
	hex := blendHex(p.hex[roleIdx], p.hex[baseIdx], blendSteps[best])
	if i, ok := p.lookup[hex]; ok {
		return i
	}
	return roleIdx
}

func absF(a float64) float64 {
	if a < 0 {
		return -a
	}
	return a
}
