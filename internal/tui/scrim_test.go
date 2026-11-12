package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// scrim_test.go — the splash overlay contract: text composites cell by
// cell over the scene with a soft radial scrim (no backing plates), and
// the emitted frame gives the wordmark ≥ 3:1 and the tagline/hint ≥ 4.5:1
// against the pixels they sit on, with neighbor backgrounds never
// differing by more than the scrim gradient allows.

// settledSplash renders the splash at t=2.4s (wordmark landed, tagline
// and hint fully faded in, dissolve not started).
func settledSplash(t *testing.T, w, h int) (string, Model) {
	t.Helper()
	fixedEnv(t)
	lipgloss.SetColorProfile(termenv.TrueColor)
	ambient.SetQualityTier(ambient.TierFull)
	m := splashModel(t, "on")
	m.ambientSceneName = "night"
	if !m.startSplashIfEnabled(true) {
		t.Fatal("splash did not start")
	}
	up, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m = up.(Model)
	m.ambientT = 2400 * time.Millisecond
	return m.View(), m
}

// cellBGHex converts a parsed grid background ("rgb:r;g;b" / "256:n" /
// "") to #rrggbb. Empty falls back to the theme base (the frame's own).
func cellBGHex(bg, fallback string) (string, bool) {
	switch {
	case strings.HasPrefix(bg, "rgb:"):
		parts := strings.Split(strings.TrimPrefix(bg, "rgb:"), ";")
		if len(parts) != 3 {
			return "", false
		}
		v := make([]int, 3)
		for i, p := range parts {
			n, err := strconv.Atoi(strings.TrimSpace(p))
			if err != nil || n < 0 || n > 255 {
				return "", false
			}
			v[i] = n
		}
		return fmt.Sprintf("#%02x%02x%02x", v[0], v[1], v[2]), true
	case strings.HasPrefix(bg, "256:"):
		n, err := strconv.Atoi(strings.TrimPrefix(bg, "256:"))
		if err != nil {
			return "", false
		}
		return xterm256NearestHex(n), true
	case bg == "":
		return fallback, true
	}
	return "", false
}

// xterm256NearestHex approximates an xterm-256 cube entry (tests run
// truecolor; limited palettes are covered by the limited-mode tests).
func xterm256NearestHex(idx int) string {
	switch {
	case idx < 16:
		levels := []uint8{0, 95, 135, 175, 215, 255, 95, 255}
		_ = levels
		base := [][3]uint8{{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0}, {0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192}, {128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0}, {0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255}}
		c := base[idx]
		return fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2])
	case idx < 232:
		i := idx - 16
		conv := func(v int) uint8 {
			if v == 0 {
				return 0
			}
			return uint8(55 + 40*v)
		}
		return fmt.Sprintf("#%02x%02x%02x", conv(i/36%6), conv(i/6%6), conv(i%6))
	default:
		v := uint8(8 + 10*(idx-232))
		return fmt.Sprintf("#%02x%02x%02x", v, v, v)
	}
}

const scrimMaxNeighborDelta = 48 // per-channel 8-bit slack: scrim gradient + quantization

// splashTextSpans returns the cell spans the splash draws text over:
// the wordmark block, the tagline row and the hint row.
func splashTextSpans(m Model, w, h int) (wmX0, wmX1, wmY0, wmY1, tagY, tagX0, tagX1, hintX0, hintX1 int) {
	wm := wordmarkFor(w, h, m.cfg.UI.Wordmark == "lower")
	rows := len(wm.letters[0])
	wmY0 = wordmarkTop(h, rows)
	wmY1 = wmY0 + rows
	letterW := len(wm.letters[0][0])
	wordW := len(wm.letters)*letterW + (len(wm.letters)-1)*3
	wmX0 = (w - wordW) / 2
	wmX1 = wmX0 + wordW
	tagY = wmY1 + 1
	tagX0 = centerX(w, "a calmer desk for your courses")
	tagX1 = tagX0 + fastWidth("a calmer desk for your courses")
	hintX0 = centerX(w, "press any key to skip")
	hintX1 = hintX0 + fastWidth("press any key to skip")
	return
}

func TestSplashTextContrastAgainstScene(t *testing.T) {
	for _, tc := range []struct{ w, h int }{{100, 30}, {200, 50}} {
		frame, m := settledSplash(t, tc.w, tc.h)
		grid := parseFrame(frame)
		wmX0, wmX1, wmY0, wmY1, tagY, tagX0, tagX1, hintX0, hintX1 := splashTextSpans(m, tc.w, tc.h)
		base := m.palette.Base
		bestRatio := func(fg string, cells [][2]int) float64 {
			worst := math.Inf(1)
			for _, c := range cells {
				_, bg := grid.cell(c[0], c[1])
				hex, ok := cellBGHex(bg, base)
				if !ok {
					t.Fatalf("unparseable cell bg at (%d,%d): %q", c[0], c[1], bg)
				}
				r := theme.ContrastRatio(fg, hex)
				if r < worst {
					worst = r
				}
			}
			return worst
		}
		var wmCells, tagCells, hintCells [][2]int
		for y := wmY0; y < wmY1; y++ {
			for x := wmX0; x < wmX1; x++ {
				_, bg := grid.cell(y, x)
				if bg != "" { // only the cells the scene painted
					wmCells = append(wmCells, [2]int{y, x})
				}
			}
		}
		for x := tagX0; x < tagX1; x++ {
			tagCells = append(tagCells, [2]int{tagY, x})
		}
		for x := hintX0; x < hintX1; x++ {
			hintCells = append(hintCells, [2]int{tc.h - 2, x})
		}
		if len(wmCells) == 0 || len(tagCells) == 0 || len(hintCells) == 0 {
			t.Fatal("could not locate the text cells in the frame")
		}
		if r := bestRatio(m.palette.BrightText, wmCells); r < 3.0 {
			t.Errorf("%dx%d: wordmark contrast %.2f:1 vs scene pixels, want ≥ 3:1", tc.w, tc.h, r)
		}
		if r := bestRatio(m.palette.BrightText, tagCells); r < 4.5 {
			t.Errorf("%dx%d: tagline contrast %.2f:1 vs scene pixels, want ≥ 4.5:1", tc.w, tc.h, r)
		}
		if r := bestRatio(m.palette.MutedSoft, hintCells); r < 4.5 {
			t.Errorf("%dx%d: hint contrast %.2f:1 vs scene pixels, want ≥ 4.5:1", tc.w, tc.h, r)
		}
	}
}

func TestSplashScrimNeighborGradientAndPlate(t *testing.T) {
	for _, tc := range []struct{ w, h int }{{100, 30}, {200, 50}} {
		frame, m := settledSplash(t, tc.w, tc.h)
		grid := parseFrame(frame)
		wmX0, wmX1, wmY0, wmY1, tagY, tagX0, tagX1, hintX0, hintX1 := splashTextSpans(m, tc.w, tc.h)
		base := m.palette.Base
		rgb := func(hex string) [3]int {
			var v [3]int
			if strings.HasPrefix(hex, "#") && len(hex) == 7 {
				fmt.Sscanf(hex[1:], "%02x%02x%02x", &v[0], &v[1], &v[2])
			}
			return v
		}
		abs := func(n int) int {
			if n < 0 {
				return -n
			}
			return n
		}
		channelDelta := func(a, b string) int {
			va, vb := rgb(a), rgb(b)
			d := 0
			for i := 0; i < 3; i++ {
				if dd := abs(va[i] - vb[i]); dd > d {
					d = dd
				}
			}
			return d
		}
		type span struct{ y0, y1, x0, x1 int }
		neighborSpans := []span{{wmY0, wmY1, wmX0, wmX1}, {tagY, tagY + 1, tagX0, tagX1}, {tc.h - 2, tc.h - 1, hintX0, hintX1}}
		plateSpans := neighborSpans // the lake below the horizon varies at every row
		for _, sp := range neighborSpans {
			for y := sp.y0; y < sp.y1; y++ {
				prevHex := ""
				for x := sp.x0; x < sp.x1; x++ {
					_, bg := grid.cell(y, x)
					hex, ok := cellBGHex(bg, base)
					if !ok {
						t.Fatalf("unparseable cell bg at (%d,%d): %q", y, x, bg)
					}
					if prevHex != "" && channelDelta(prevHex, hex) > scrimMaxNeighborDelta {
						t.Errorf("%dx%d row %d col %d: neighbor backgrounds differ by more than the scrim gradient allows (%s vs %s)", tc.w, tc.h, y, x, prevHex, hex)
					}
					prevHex = hex
				}
			}
		}
		// the scene shows through the scrim: no span sits on one flat
		// background color
		for _, sp := range plateSpans {
			distinct := map[string]bool{}
			for y := sp.y0; y < sp.y1; y++ {
				for x := sp.x0; x < sp.x1; x++ {
					_, bg := grid.cell(y, x)
					hex, ok := cellBGHex(bg, base)
					if !ok {
						t.Fatalf("unparseable cell bg at (%d,%d): %q", y, x, bg)
					}
					distinct[hex] = true
				}
			}
			if len(distinct) < 2 {
				t.Errorf("%dx%d span y=%d..%d: flat background under the text — the scrim must not form a plate", tc.w, tc.h, sp.y0, sp.y1)
			}
		}
	}
}
