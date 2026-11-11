package tui

import (
	"bytes"
	"math"
	"strconv"
	"strings"

	"github.com/rivo/uniseg"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// scrim.go — spec 5.3: the Focus backdrop. The scene fills every cell; a
// soft dark scrim is blended over the scene's palette so bright body text
// stays legible. The scrim alpha is computed automatically per theme: the
// brightest palette role bounds the frame luminance (all scene cells derive
// from these roles or darker blends), and alpha rises until BrightText vs
// that bound passes WCAG 4.5.

const (
	focusScrimMinRatio = 4.5
	focusScrimFloor    = 0.15 // even a dark scene gets a soft scrim
	focusScrimCeil     = 0.85 // never a black void
)

func hexLuminance(hex string) float64 {
	c := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(c) != 6 {
		return 0
	}
	v, err := strconv.ParseUint(c, 16, 32)
	if err != nil {
		return 0
	}
	ch := func(x uint8) float64 {
		s := float64(x) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(uint8(v>>16)) + 0.7152*ch(uint8(v>>8)) + 0.0722*ch(uint8(v))
}

// paletteMaxRole returns the brightest role's hex and its luminance —
// upper bound for any cell the scene renderer produces.
func paletteMaxRole(p ambient.Palette) (string, float64) {
	best := ""
	bestLum := 0.0
	for _, c := range []string{
		p.Base, p.Surface, p.SurfaceRaised, p.SurfaceStrong, p.Border,
		p.Text, p.BrightText, p.MutedSoft,
		p.Accent, p.AccentSoft, p.Success, p.Warning, p.Danger,
		p.HeatWarm, p.HeatHot, p.HeatOverdue,
		p.Blue, p.Cyan, p.Yellow, p.Magenta, p.Orange, p.Brown,
	} {
		if l := hexLuminance(c); l > bestLum {
			bestLum = l
			best = c
		}
	}
	return best, bestLum
}

// paletteMaxLuminance is the brightest role — upper bound for any cell
// the scene renderer produces.
func paletteMaxLuminance(p ambient.Palette) float64 {
	_, l := paletteMaxRole(p)
	return l
}

// scrimDirection picks the scrim target by text polarity: light text gets
// a dark scene, dark text gets a light scene.
type scrimDirection int

const (
	scrimDarken scrimDirection = iota
	scrimLighten
)

func focusScrimDir(brightText string) scrimDirection {
	if hexLuminance(brightText) >= 0.5 {
		return scrimDarken
	}
	return scrimLighten
}

// scrimTarget is the hex the scrim blends toward.
func scrimTarget(d scrimDirection) string {
	if d == scrimDarken {
		return "#000000"
	}
	return "#ffffff"
}

// focusScrimAlpha computes the scrim alpha for this theme. Darkening
// multiplies luminance by (1-a): (Ltext+0.05) / ((1-a)*Lmax+0.05) ≥ 4.5.
// Lightening blends toward white: L' = (1-a)*L + a must satisfy
// (L'+0.05) / (Ltext+0.05) ≥ 4.5. Solved directly, clamped to a sane band.
func focusScrimAlpha(brightText string, p ambient.Palette) float64 {
	lText := hexLuminance(brightText)
	lMax := paletteMaxLuminance(p)
	var a float64
	if focusScrimDir(brightText) == scrimDarken {
		a = 1 - ((lText+0.05)/focusScrimMinRatio-0.05)/lMax
	} else {
		required := focusScrimMinRatio*(lText+0.05) - 0.05
		if lMax >= 1 || required <= 0 {
			a = 0
		} else {
			a = (required - lMax) / (1 - lMax)
		}
	}
	if a < focusScrimFloor {
		a = focusScrimFloor
	}
	if a > focusScrimCeil {
		a = focusScrimCeil
	}
	return a
}

// scrimPalette blends every role toward the scrim target by a — equivalent
// to a constant-alpha scrim painted over the whole frame.
func scrimPalette(p ambient.Palette, a float64, d scrimDirection) ambient.Palette {
	target := scrimTarget(d)
	s := func(c string) string { return theme.Blend(c, target, a) }
	return ambient.Palette{
		Base: s(p.Base), Surface: s(p.Surface), SurfaceRaised: s(p.SurfaceRaised),
		SurfaceStrong: s(p.SurfaceStrong), Border: s(p.Border),
		Text: s(p.Text), BrightText: s(p.BrightText), MutedSoft: s(p.MutedSoft),
		Accent: s(p.Accent), AccentSoft: s(p.AccentSoft),
		Success: s(p.Success), Warning: s(p.Warning), Danger: s(p.Danger),
		HeatWarm: s(p.HeatWarm), HeatHot: s(p.HeatHot), HeatOverdue: s(p.HeatOverdue),
		Blue: s(p.Blue), Cyan: s(p.Cyan), Yellow: s(p.Yellow), Magenta: s(p.Magenta),
		Orange: s(p.Orange), Brown: s(p.Brown),
	}
}

// ---- cell-aware overlay (spec 5.3) ----

type ovCell struct {
	x, width int    // cell coordinates and display width
	bytes    []byte // grapheme bytes only — styles ride on state
	state    string // canonical fingerprint of the SGR state active for this cell
	ink      bool   // grapheme isn't a space, or any color is set
}

// sgrState is a minimal SGR emulator: it canonicalizes the active params
// (clearing on 0, replacing color components whole) so overlay output can
// re-state any cell absolutely.
type sgrState struct{ fp string }

func (s *sgrState) apply(seq string) {
	params := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	parts := strings.Split(params, ";")
	var act []string
	if s.fp != "" {
		act = strings.Split(s.fp, "|")
	}
	drop := func(pred func(string) bool) {
		kept := act[:0]
		for _, p := range act {
			if !pred(p) {
				kept = append(kept, p)
			}
		}
		act = kept
	}
	add := func(p string) { act = append(act, p) }
	isFG := func(p string) bool { return p == "39" || strings.HasPrefix(p, "38;") || isBaseFG(p) }
	isBG := func(p string) bool { return p == "49" || strings.HasPrefix(p, "48;") || isBaseBG(p) }
	dropFG := func() { drop(isFG) }
	dropBG := func() { drop(isBG) }
	clearAttr := map[string]string{"1": "22", "3": "23", "4": "24", "7": "27", "9": "29"}
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		switch {
		case p == "" || p == "0" || p == "00":
			act = act[:0]
		case p == "39":
			dropFG()
		case p == "49":
			dropBG()
		case p == "22":
			drop(func(q string) bool { return q == "1" })
		case p == "23":
			drop(func(q string) bool { return q == "3" })
		case p == "24":
			drop(func(q string) bool { return q == "4" })
		case p == "27":
			drop(func(q string) bool { return q == "7" })
		case p == "29":
			drop(func(q string) bool { return q == "9" })
		case p == "38" || p == "58":
			dropFG()
			if i+1 < len(parts) {
				switch parts[i+1] {
				case "2":
					if i+4 < len(parts) {
						add("38;2;" + parts[i+2] + ";" + parts[i+3] + ";" + parts[i+4])
						i += 4
					}
				case "5":
					if i+2 < len(parts) {
						add("38;5;" + parts[i+2])
						i += 2
					}
				default:
					add("38;" + parts[i+1])
					i += 1
				}
			}
		case p == "48":
			dropBG()
			if i+1 < len(parts) {
				switch parts[i+1] {
				case "2":
					if i+4 < len(parts) {
						add("48;2;" + parts[i+2] + ";" + parts[i+3] + ";" + parts[i+4])
						i += 4
					}
				case "5":
					if i+2 < len(parts) {
						add("48;5;" + parts[i+2])
						i += 2
					}
				default:
					add("48;" + parts[i+1])
					i += 1
				}
			}
		case p == "1" || p == "3" || p == "4" || p == "7" || p == "9":
			drop(func(q string) bool { return q == p || q == clearAttr[p] })
			add(p)
		default:
			add(p)
		}
	}
	s.fp = strings.Join(act, "|")
}

func isBaseFG(p string) bool {
	if len(p) < 2 || p[0] < '0' || p[0] > '9' {
		return false
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return false
	}
	switch {
	case n >= 30 && n <= 37, n >= 90 && n <= 97:
		return true
	}
	return false
}

func isBaseBG(p string) bool {
	if len(p) < 2 || p[0] < '0' || p[0] > '9' {
		return false
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return false
	}
	switch {
	case n >= 40 && n <= 47, n >= 100 && n <= 107:
		return true
	}
	return false
}

// absolute renders the canonical state as one SGR sequence.
func (s sgrState) absolute() string {
	if s.fp == "" {
		return "\x1b[0m"
	}
	return "\x1b[" + strings.ReplaceAll(s.fp, "|", ";") + "m"
}

// splitStyled walks a styled line into grapheme cells with their canonical
// SGR state. Byte offsets attach preceding escapes to the next cell.
func splitStyled(line string) []ovCell {
	cells := []ovCell{}
	st := sgrState{}
	pos, cellX := 0, 0
	for pos < len(line) {
		if line[pos] == '\x1b' {
			j := pos + 1
			for j < len(line) && !(line[j] >= 'a' && line[j] <= 'z' || line[j] >= 'A' && line[j] <= 'Z') {
				j++
			}
			if j >= len(line) {
				j = len(line) - 1
			}
			seq := line[pos : j+1]
			if strings.HasSuffix(seq, "m") {
				st.apply(seq)
			}
			pos = j + 1
			continue
		}
		cluster, _, w, _ := uniseg.FirstGraphemeClusterInString(line[pos:], -1)
		ink := strings.Trim(cluster, " ") != "" || hasColor(st.fp)
		cells = append(cells, ovCell{
			x: cellX, width: w,
			bytes: []byte(line[pos : pos+len(cluster)]), state: st.fp, ink: ink,
		})
		pos += len(cluster)
		cellX += w
	}
	return cells
}

// hasColor reports whether the fingerprint carries any fg or bg color.
func hasColor(fp string) bool {
	for _, p := range strings.Split(fp, "|") {
		if strings.HasPrefix(p, "38;") || strings.HasPrefix(p, "48;") || isBaseFG(p) || isBaseBG(p) {
			return true
		}
	}
	return false
}

// overlayFrame composites body rows over scene rows. Body cells that are
// ink replace the scene cell beneath them; all other cells are transparent
// and leave the scene showing through. Rows past the body keep the scene.
func overlayFrame(scene, body []string, width int) []string {
	out := make([]string, len(scene))
	for i := range scene {
		if i >= len(body) {
			// Scene rows past the body are full-width already; avoid the
			// lipgloss MaxWidth parse on the hot path.
			if fastWidth(scene[i]) <= width {
				out[i] = scene[i]
			} else {
				out[i] = clipStyled(scene[i], width)
			}
			continue
		}
		out[i] = overlayLine(scene[i], body[i])
	}
	return out
}

// overlayLine splices the body row's ink runs over the scene row. The
// emitted SGR is tracked explicitly; scene cells are restated only when
// their recorded state diverges from it. normalizeBlock clips afterwards.
func overlayLine(sceneRow, bodyRow string) string {
	scells := splitStyled(sceneRow)
	bcells := splitStyled(bodyRow)

	type run struct{ x0, x1, i0, i1 int } // cell coords + body cell indices
	var runs []run
	for i, c := range bcells {
		if !c.ink {
			continue
		}
		if n := len(runs); n > 0 {
			r := &runs[n-1]
			if r.x1 == c.x {
				r.x1 = c.x + c.width
				r.i1 = i + 1
				continue
			}
		}
		runs = append(runs, run{c.x, c.x + c.width, i, i + 1})
	}

	var out bytes.Buffer
	emitted := "" // the SGR fingerprint the terminal currently holds
	restate := func(target string) {
		if target != emitted {
			out.WriteString(sgrState{fp: target}.absolute())
			emitted = target
		}
	}

	sc := 0
	for _, r := range runs {
		for sc < len(scells) && scells[sc].x < r.x0 {
			restate(scells[sc].state)
			out.Write(scells[sc].bytes)
			sc++
		}
		restate(bcells[r.i0].state)
		for i := r.i0; i < r.i1; i++ {
			out.Write(bcells[i].bytes)
		}
		out.WriteString("\x1b[0m")
		emitted = ""
	}
	for sc < len(scells) {
		restate(scells[sc].state)
		out.Write(scells[sc].bytes)
		sc++
	}
	return out.String()
}
