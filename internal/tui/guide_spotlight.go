package tui

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// guide_spotlight.go — Phase 3: soft dim of the screen around the step's
// spotlighted pane + a breathing accent beacon on the pane's top edge +
// 220ms card reposition. The dim uses the same SGR discipline as scrim.go
// (viewfade primitives); bounded amounts so the effect stays subtle.

// R is an inclusive-exclusive pixel rect used by the spotlight region math.
type R struct{ X0, Y0, X1, Y1 int }

func (r R) Empty() bool            { return r.X0 >= r.X1 || r.Y0 >= r.Y1 }
func (r R) Dx() int                { return r.X1 - r.X0 }
func (r R) Dy() int                { return r.Y1 - r.Y0 }
func (r R) Contains(x, y int) bool { return x >= r.X0 && x < r.X1 && y >= r.Y0 && y < r.Y1 }

// guideSpotRegion returns the screen rect the step's spotlight targets.
func (m Model) guideSpotRegion(w, h int, step int) R {
	s := guideSteps[step]
	sideW := sideWidth(m.width, m.sidebarHidden)
	switch s.Spotlight {
	case spotSidebar:
		return R{0, 0, sideW, h}
	case spotMain:
		detailW := detailWidth(m.width, m.detailOn)
		return R{sideW, 0, w - detailW, h}
	case spotDetail:
		return R{w - detailWidth(m.width, m.detailOn), 0, w, h}
	case spotFooter:
		return R{0, h - 1, w, h}
	}
	return R{}
}

// guideDimAmount scales down outside the target pane (0..1).
func (m Model) guideDimAmount() float64 {
	switch guideSteps[m.guide.Step].Spotlight {
	case spotSidebar:
		return 0.1
	case spotMain:
		return 0.25
	case spotDetail:
		return 0.5
	case spotFooter:
		return 0.15
	}
	return 0
}

func hexFadeSGR(hex string) fadeSGR {
	v, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimSpace(hex), "#"), 16, 32)
	if err != nil {
		return fadeSGR{}
	}
	return fadeSGR{fgKind: fadeColorRGB, fgR: uint8(v >> 16 & 0xff), fgG: uint8(v >> 8 & 0xff), fgB: uint8(v & 0xff)}
}

func dimLineOutsideRegion(line string, reg R, y int, base fadeSGR, amount float64) string {
	cells, ok := parseFadeLine(line)
	if !ok {
		return line
	}
	var sb strings.Builder
	var emitted fadeSGR
	x := 0
	for _, c := range cells {
		in := y >= reg.Y0 && y < reg.Y1 && x >= reg.X0 && x < reg.X1
		st := c.sgr
		if !in {
			var e = amount
			if e > 0 {
				st.fgKind, st.fgR, st.fgG, st.fgB = lerpFadeColor(st.fgKind, st.fgR, st.fgG, st.fgB, st.fgIdx, base.fgKind, base.fgR, base.fgG, base.fgB, base.fgIdx, e)
				st.bgKind, st.bgR, st.bgG, st.bgB = lerpFadeColor(st.bgKind, st.bgR, st.bgG, st.bgB, st.bgIdx, base.bgKind, base.bgR, base.bgG, base.bgB, base.bgIdx, e)
			}
		}
		if st != emitted {
			sb.WriteString(fadeSGRString(st))
			emitted = st
		}
		sb.WriteString(c.glyph)
		x += runewidth.StringWidth(c.glyph)
	}
	sb.WriteString("\x1b[0m")
	return sb.String()
}

func dimLinesOutsideRegion(lines []string, reg R, base fadeSGR, amount float64) []string {
	if amount <= 0 || reg.Empty() {
		return lines
	}
	out := make([]string, len(lines))
	for i, ln := range lines {
		out[i] = dimLineOutsideRegion(ln, reg, i, base, amount)
	}
	return out
}

func phaseF(now time.Time, period time.Duration) float64 {
	f := math.Mod(float64(now.UnixNano())/float64(period), 1)
	if f < 0 {
		f++
	}
	return f
}

// drawGuideBeacon draws the tiny ✦ focus marker on the target pane's top edge.
func drawGuideBeacon(m Model, lines []string, reg R, now time.Time, w int) {
	if reg.Empty() || w <= 0 {
		return
	}
	k := 0.5 + 0.5*ambient.EaseInOutSine(phaseF(now, 1600*time.Millisecond))
	hex := theme.Blend(m.palette.Accent, m.palette.BrightText, k)
	x, y := reg.X0+1, reg.Y0+1
	if y >= len(lines) || x >= w {
		y = reg.Y1 - 1
	}
	box := lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render("✦")
	lines[y] = spliceLine(lines[y], x, w, box, lipgloss.Color(m.palette.Base))
}

// handleGuideCardMove steps the card toward its target on each guide tick.
func (m *Model) handleGuideCardMove() {
	if !m.guide.cardMoving {
		return
	}
	now := m.clock.Now()
	el := now.Sub(m.guide.cardMoveStart)
	k := clampUnitF(float64(el)/float64(220*time.Millisecond), 0, 1)
	dstX, dstY := m.guide.cardDstX, m.guide.cardDstY
	m.guide.cardX = dstX + int(math.Round(float64(m.guide.cardX-dstX)*(1-k)))
	m.guide.cardY = dstY + int(math.Round(float64(m.guide.cardY-dstY)*(1-k)))
	if k >= 1 {
		m.guide.cardMoving = false
		m.guide.cardX, m.guide.cardY = dstX, dstY
	}
}

func clampUnitF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
