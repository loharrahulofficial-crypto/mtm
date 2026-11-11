package tui

import (
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
)

// Tab-switch transition: a true crossfade out of the previously displayed
// frame into the new one over ~150ms — a dissolve, not a dim. One code
// path, applied centrally in dispatch whenever the view changes, so tabs,
// footer pills, sidebar picks, palette jumps and mouse clicks all get it.
// Reduced motion skips arming entirely.

const viewFadeDur = 150 * time.Millisecond

type viewFadeTickMsg struct{}

func viewFadeTickCmd() tea.Cmd {
	return tea.Tick(33*time.Millisecond, func(time.Time) tea.Msg { return viewFadeTickMsg{} })
}

// viewFadeAmount reports how much of the transition remains: 1 at the
// moment of the switch, easing to 0 linearly as the crossfade settles.
// crossfadeFrames applies EaseInOutCubic to the eased progress.
func (m Model) viewFadeAmount() float64 {
	if m.viewFadeStart.IsZero() {
		return 0
	}
	d := m.clock.Now().Sub(m.viewFadeStart)
	if d <= 0 {
		return 1
	}
	if d >= viewFadeDur {
		return 0
	}
	return 1 - float64(d)/float64(viewFadeDur)
}

// viewFadeProgress is the raw linear time fraction of the transition:
// 0 at the switch, 1 once settled. crossfadeFrames applies
// EaseInOutCubic to this inside the blend — do not pre-ease it here.
// A zero viewFadeStart means settled.
func viewFadeProgress(m Model) float64 {
	if m.viewFadeStart.IsZero() {
		return 1
	}
	return 1 - m.viewFadeAmount()
}

// stableFrame caches the last pre-overlay composite the user actually
// sees — refreshed on EVERY render (blended or settled), so a re-switch
// mid-dissolve freezes from what is on screen instead of snapping back
// to the last fully settled frame. armFrame is the frozen source for
// the in-flight transition; armStamp pins it to one viewFadeStart.
// View is a value-receiver render, so package vars are the right place
// (matches the package-level panel caches); the TUI is single-threaded.
var stableFrame string
var armFrame string
var armStamp time.Time

// crossfadeFrames renders the in-flight tab transition as the settled
// frame with its SGR colors eased from prev to curr. Glyphs and layout are
// byte-identical to curr in every intermediate frame — only fg/bg RGB
// channels interpolate — so the transition can never tear text, borders
// or panel structure mid-fade. Works for any render mode (halfblock,
// quadrant, sextant): glyphs are copied, never interpreted.
// EaseInOutCubic is applied here — t is not pre-eased by the caller.
func crossfadeFrames(prev, curr string, t float64) string {
	if prev == "" || t <= 0 || t >= 1 {
		return curr
	}
	prevLines := strings.Split(prev, "\n")
	currLines := strings.Split(curr, "\n")
	// Never emit a taller/shorter frame than the settled one.
	if len(prevLines) != len(currLines) {
		return curr
	}
	e := ambient.EaseInOutCubic(t)
	out := make([]string, len(currLines))
	for i := range currLines {
		if blended, ok := lerpANSILine(prevLines[i], currLines[i], e); ok {
			out[i] = blended
		} else {
			out[i] = currLines[i]
		}
	}
	return strings.Join(out, "\n")
}

// fadeColorKind tags how a channel color is encoded in an SGR cell.
type fadeColorKind uint8

const (
	fadeColorNone fadeColorKind = iota // channel unset (39/49 or reset)
	fadeColorRGB                       // 38;2;r;g;b
	fadeColorIdx                       // 38;5;n
)

// fadeSGR is the parsed color/attribute state of an SGR cell.
// attrs holds non-color params (e.g. "1" for bold), ';'-joined.
type fadeSGR struct {
	fgKind        fadeColorKind
	fgR, fgG, fgB uint8
	fgIdx         uint8
	bgKind        fadeColorKind
	bgR, bgG, bgB uint8
	bgIdx         uint8
	attrs         string
}

// fadeCell is one terminal cell: the SGR state in effect and the raw
// bytes of its glyph.
type fadeCell struct {
	sgr   fadeSGR
	glyph string
}

// parseFadeLine splits an ANSI line into cells. Returns false on any
// malformed SGR sequence.
func parseFadeLine(line string) ([]fadeCell, bool) {
	cells := []fadeCell{}
	st := fadeSGR{}
	rest := line
	for len(rest) > 0 {
		if strings.HasPrefix(rest, "\x1b[") {
			end := strings.IndexByte(rest, 'm')
			if end < 0 {
				return nil, false
			}
			var ok bool
			st, ok = applyFadeSGR(st, rest[2:end])
			if !ok {
				return nil, false
			}
			rest = rest[end+1:]
			continue
		}
		_, size := utf8.DecodeRuneInString(rest)
		if size == 0 {
			return nil, false
		}
		cells = append(cells, fadeCell{sgr: st, glyph: rest[:size]})
		rest = rest[size:]
	}
	return cells, true
}

// applyFadeSGR folds one SGR parameter list (without the CSI prefix or
// the trailing 'm') into st.
func applyFadeSGR(st fadeSGR, params string) (fadeSGR, bool) {
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		p := strings.TrimSpace(parts[i])
		switch p {
		case "", "0":
			st = fadeSGR{}
		case "39":
			st.fgKind = fadeColorNone
		case "49":
			st.bgKind = fadeColorNone
		case "38", "48":
			if i+2 >= len(parts) {
				return st, false
			}
			target := &st.fgKind
			tr, tg, tb, tidx := &st.fgR, &st.fgG, &st.fgB, &st.fgIdx
			if p == "48" {
				target = &st.bgKind
				tr, tg, tb, tidx = &st.bgR, &st.bgG, &st.bgB, &st.bgIdx
			}
			switch strings.TrimSpace(parts[i+1]) {
			case "5":
				idx, ok := fadeAtoi(parts[i+2])
				if !ok {
					return st, false
				}
				*target = fadeColorIdx
				*tidx = uint8(idx)
				i += 2
			case "2":
				if i+4 >= len(parts) {
					return st, false
				}
				r, ok1 := fadeAtoi(parts[i+2])
				g, ok2 := fadeAtoi(parts[i+3])
				b, ok3 := fadeAtoi(parts[i+4])
				if !ok1 || !ok2 || !ok3 {
					return st, false
				}
				*target = fadeColorRGB
				*tr, *tg, *tb = uint8(r), uint8(g), uint8(b)
				i += 4
			default:
				return st, false
			}
		default:
			// keep other params (bold, dim, ...) from curr verbatim
			if st.attrs != "" {
				st.attrs += ";"
			}
			st.attrs += p
		}
	}
	return st, true
}

func fadeAtoi(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		v = v*10 + int(r-'0')
		if v > 255 {
			return 0, false
		}
	}
	return v, true
}

// lerpFadeColor eases one channel (fg or bg). Only RGB-to-RGB blends;
// any other combination (xterm-256, unset on either side, kind change)
// takes curr verbatim so limited-palette output stays valid.
func lerpFadeColor(pKind fadeColorKind, pR, pG, pB uint8, _ uint8,
	cKind fadeColorKind, cR, cG, cB uint8, _ uint8, e float64) (fadeColorKind, uint8, uint8, uint8) {
	if pKind != fadeColorRGB || cKind != fadeColorRGB {
		return cKind, cR, cG, cB
	}
	mix := func(a, b uint8) uint8 {
		v := float64(a) + (float64(b)-float64(a))*e
		if v <= 0 {
			return 0
		}
		if v >= 255 {
			return 255
		}
		return uint8(v + 0.5)
	}
	return fadeColorRGB, mix(pR, cR), mix(pG, cG), mix(pB, cB)
}

// fadeSGRString renders the full SGR state: a reset prefix followed by
// the attrs and both channels, so dropped channels never leak across
// cells. Empty state renders as a bare reset.
func fadeSGRString(s fadeSGR) string {
	var sb strings.Builder
	sb.WriteString("\x1b[0m")
	params := make([]string, 0, 8)
	if s.attrs != "" {
		params = append(params, s.attrs)
	}
	switch s.fgKind {
	case fadeColorRGB:
		params = append(params, "38;2;"+fadeItoa(s.fgR)+";"+fadeItoa(s.fgG)+";"+fadeItoa(s.fgB))
	case fadeColorIdx:
		params = append(params, "38;5;"+fadeItoa(s.fgIdx))
	}
	switch s.bgKind {
	case fadeColorRGB:
		params = append(params, "48;2;"+fadeItoa(s.bgR)+";"+fadeItoa(s.bgG)+";"+fadeItoa(s.bgB))
	case fadeColorIdx:
		params = append(params, "48;5;"+fadeItoa(s.bgIdx))
	}
	if len(params) > 0 {
		sb.WriteString("\x1b[")
		sb.WriteString(strings.Join(params, ";"))
		sb.WriteString("m")
	}
	return sb.String()
}

func fadeItoa(v uint8) string {
	if v >= 100 {
		return string([]byte{'0' + v/100, '0' + (v/10)%10, '0' + v%10})
	}
	if v >= 10 {
		return string([]byte{'0' + v/10, '0' + v%10})
	}
	return string([]byte{'0' + v})
}

// lerpANSILine eases the SGR colors of curr's line toward/away from
// prev's, cell by cell. Glyph bytes are curr's, untouched. Returns false
// when the lines cannot be paired cell-for-cell.
func lerpANSILine(prev, curr string, e float64) (string, bool) {
	pCells, ok := parseFadeLine(prev)
	if !ok {
		return "", false
	}
	cCells, ok := parseFadeLine(curr)
	if !ok {
		return "", false
	}
	// Cell counts must match: curr is the settled geometry, prev is the
	// same screen a frame earlier.
	if len(pCells) != len(cCells) {
		return "", false
	}
	var sb strings.Builder
	sb.Grow(len(curr) + 64)
	cur := fadeSGR{}
	for i := range cCells {
		p, c := &pCells[i], &cCells[i]
		s := c.sgr
		s.fgKind, s.fgR, s.fgG, s.fgB = lerpFadeColor(p.sgr.fgKind, p.sgr.fgR, p.sgr.fgG, p.sgr.fgB, p.sgr.fgIdx,
			c.sgr.fgKind, c.sgr.fgR, c.sgr.fgG, c.sgr.fgB, c.sgr.fgIdx, e)
		s.bgKind, s.bgR, s.bgG, s.bgB = lerpFadeColor(p.sgr.bgKind, p.sgr.bgR, p.sgr.bgG, p.sgr.bgB, p.sgr.bgIdx,
			c.sgr.bgKind, c.sgr.bgR, c.sgr.bgG, c.sgr.bgB, c.sgr.bgIdx, e)
		if s != cur {
			sb.WriteString(fadeSGRString(s))
			cur = s
		}
		sb.WriteString(c.glyph)
	}
	sb.WriteString("\x1b[0m")
	return sb.String(), true
}
