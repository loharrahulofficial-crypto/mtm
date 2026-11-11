package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
)

// Tab-switch transition: on every view change the whole frame cross-fades
// out of the base color back in over ~220ms — a soft page-flip. One code
// path, applied centrally in dispatch whenever the view changes, so tabs,
// footer pills, sidebar picks, palette jumps and mouse clicks all get it.

const viewFadeDur = 220 * time.Millisecond

type viewFadeTickMsg struct{}

func viewFadeTickCmd() tea.Cmd {
	return tea.Tick(33*time.Millisecond, func(time.Time) tea.Msg { return viewFadeTickMsg{} })
}

// viewFadeAmount reports how much the frame is dimmed toward base: 1 at the
// moment of the switch, easing to 0.
func (m Model) viewFadeAmount() float64 {
	if m.viewFadeStart.IsZero() {
		return 0
	}
	d := m.clock.Now().Sub(m.viewFadeStart)
	if d < 0 || d >= viewFadeDur {
		return 0
	}
	return 1 - easeOutCubic(float64(d)/float64(viewFadeDur))
}

func easeOutCubic(t float64) float64 {
	t -= 1
	return t*t*t + 1
}

// fadeBase blends every colored cell toward base by k ∈ [0,1]. Uncolored
// text and the layout are untouched — this is a pure color wash, so the
// pane reads as emerging from the dark rather than sliding.
func fadeBase(s string, k float64, base string) string {
	if k <= 0 {
		return s
	}
	dim := rgbOfHex(base)
	parts := strings.Split(s, "\n")
	for i, l := range parts {
		parts[i] = fadeLine(l, k, dim)
	}
	return strings.Join(parts, "\n")
}

func rgbOfHex(s string) [3]int {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return [3]int{20, 40, 45}
	}
	r, _ := strconv.ParseUint(s[0:2], 16, 8)
	g, _ := strconv.ParseUint(s[2:4], 16, 8)
	b, _ := strconv.ParseUint(s[4:6], 16, 8)
	return [3]int{int(r), int(g), int(b)}
}

func mixChan(c, dim int, k float64) int {
	return int(math.Round((1-k)*float64(c) + k*float64(dim)))
}

func atoiOr(s string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// fadeLine blends every fg/bg color parameter in the SGR sequences of one
// line toward dim by k. Non-color sequences pass through unchanged.
func fadeLine(line string, k float64, dim [3]int) string {
	seqs := scanSGR(line)
	if len(seqs) == 0 {
		return line
	}
	var b strings.Builder
	b.Grow(len(line) + 32)
	last := 0
	for _, s := range seqs {
		b.WriteString(line[last:s.start])
		b.WriteString(fadeSGR(s.seq, k, dim))
		last = s.end
	}
	b.WriteString(line[last:])
	return b.String()
}

func fadeSGR(seq string, k float64, dim [3]int) string {
	params := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	parts := strings.Split(params, ";")
	out := make([]string, 0, len(parts)+4)
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		switch p {
		case "38", "48":
			if i+1 < len(parts) && parts[i+1] == "5" {
				idx := atoiOr(parts[i+2], 255)
				cr, cg, cb := ambient.Xterm256RGBOf(uint8(idx))
				out = append(out, p, "2",
					fmt.Sprint(mixChan(int(cr), dim[0], k)),
					fmt.Sprint(mixChan(int(cg), dim[1], k)),
					fmt.Sprint(mixChan(int(cb), dim[2], k)))
				i += 2
				continue
			}
			if i+4 < len(parts) && parts[i+1] == "2" {
				r := atoiOr(parts[i+2], 255)
				g := atoiOr(parts[i+3], 255)
				bl := atoiOr(parts[i+4], 255)
				out = append(out, p, "2",
					fmt.Sprint(mixChan(r, dim[0], k)),
					fmt.Sprint(mixChan(g, dim[1], k)),
					fmt.Sprint(mixChan(bl, dim[2], k)))
				i += 4
				continue
			}
			out = append(out, p)
		default:
			out = append(out, p)
		}
	}
	return "\x1b[" + strings.Join(out, ";") + "m"
}
