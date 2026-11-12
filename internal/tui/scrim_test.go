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

// fgCellLum parses a parsed foreground SGR ("rgb:r;g;b" / "256:n" / "")
// into WCAG relative luminance; default foreground reads as 0 (dark).
func fgCellLum(fg string) float64 {
	hex, ok := cellBGHex(fg, "")
	if !ok {
		return 0
	}
	return relLumHex(hex)
}

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

// splashTextSpans returns the cell spans the splash draws text over:
// the wordmark block, the tagline row and the hint row.
func splashTextSpans(m Model, w, h int) (wmX0, wmX1, wmY0, wmY1, tagY, tagX0, tagX1, hintX0, hintX1 int) {
	lay := splashLayoutFor(w, h, m.cellW, m.cellH, m.cfg.UI.Wordmark == "lower")
	wmX0 = lay.x0
	wmX1 = lay.x0 + lay.wordW
	wmY0 = int(math.Floor(lay.top))
	wmY1 = int(math.Ceil(lay.top + lay.rows))
	tagY = lay.tagY
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
		hexOf := func(parsed string, fallback string) string {
			hex, ok := cellBGHex(parsed, fallback)
			if !ok {
				t.Fatalf("unparseable cell color %q", parsed)
			}
			return hex
		}
		_ = base
		// wordmark: the letters are bright against the sampled scene at
		// the wordmark's center — ≥ 3:1, per the spec. Compute the ratio
		// against the sampled raster (wordmark band, center cell).
		quiet, lay := lettersOffSplashRaster(t, m, tc.w, tc.h, 2400, true)
		cx := lay.x0 + lay.wordW/2
		cy := int(math.Floor(lay.top+lay.rows/2)) * 2
		sampleHex := rasterHexAt(quiet, cx, cy+1)
		if got := theme.ContrastRatio(m.palette.BrightText, sampleHex); got < 3.0 {
			t.Errorf("%dx%d: wordmark letter color vs the sampled scene %.2f:1, want ≥ 3:1 (sample %s)", tc.w, tc.h, got, sampleHex)
		}
		// at least 24 cells in the span carry the bright letters
		brightLetters := 0
		for y := wmY0; y < wmY1; y++ {
			for x := wmX0; x < wmX1; x++ {
				if fg := grid.foreground(y, x); fg != "" && fgCellLum(fg) > 0.55 {
					brightLetters++
				}
			}
		}
		if brightLetters < 24 {
			t.Errorf("%dx%d: only %d bright cells in the wordmark span, want >= 24", tc.w, tc.h, brightLetters)
		}
		// tagline and hint: EVERY cell (each was emitted with the span's
		// worst-case-picked color) must keep ≥ 4.5:1.
		for _, span := range []struct {
			name   string
			x0, x1 int
			y      int
		}{
			{"tagline", tagX0, tagX1, tagY},
			{"hint", hintX0, hintX1, tc.h - 2},
		} {
			for x := span.x0; x < span.x1; x++ {
				fg := hexOf(grid.foreground(span.y, x), base)
				bg := hexOf(grid.bg[span.y][x], base)
				if r := theme.ContrastRatio(fg, bg); r < 4.5 {
					t.Errorf("%dx%d: %s cell %d contrast %.2f:1, want ≥ 4.5:1 (fg %s on bg %s)", tc.w, tc.h, span.name, x, r, fg, bg)
				}
			}
		}
	}
}

// lettersOffSplashRaster renders the same frame as the splash with the
// wordmark letters disabled. With scrims=false the soft darkening is
// omitted too — the plate check compares the two.
func lettersOffSplashRaster(t *testing.T, m Model, w, h int, ms int, scrims bool) (ambient.Raster, splashLayout) {
	t.Helper()
	lay := splashLayoutFor(w, h, m.cellW, m.cellH, m.cfg.UI.Wordmark == "lower")
	st := m.sceneState()
	st.Palette = m.dimScenePalette()
	if scrims {
		st.Scrims = []ambient.Scrim{{
			CX: float64(lay.x0) + float64(lay.wordW)/2, CY: lay.top + lay.rows/2,
			RadiusX: 1.6 * float64(lay.wordW), RadiusY: lay.rows * 2.0, Strength: 0.18,
		}}
	}
	st.Wordmark = nil
	st.FullFrame = true
	sceneT := time.Duration(0)
	if secs := float64(ms) / 1000; secs > 0.25 {
		sceneT = time.Duration((secs - 0.25) * float64(time.Second))
	}
	return m.currentScene().FramePixels(sceneT, w, h, st), lay
}

// rasterHexAt returns the 8-bit #rrggbb of a raster pixel.
func rasterHexAt(r ambient.Raster, x, y int) string {
	rr, gg, bb := r.Pixel(x, y)
	return fmt.Sprintf("#%02x%02x%02x", rr, gg, bb)
}

// rasterLumAt returns the WCAG luminance of a raster pixel (0..1).
func rasterLumAt(r ambient.Raster, subX, subY int) float64 {
	rr, gg, bb := r.Pixel(subX, subY)
	f := func(c uint8) float64 {
		v := float64(c) / 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*f(rr) + 0.7152*f(gg) + 0.0722*f(bb)
}

func TestSplashScrimNeighborGradientAndPlate(t *testing.T) {
	for _, tc := range []struct{ w, h int }{{100, 30}, {200, 50}} {
		_, m := settledSplash(t, tc.w, tc.h)
		withScrim, lay := lettersOffSplashRaster(t, m, tc.w, tc.h, 2400, true)
		plain, _ := lettersOffSplashRaster(t, m, tc.w, tc.h, 2400, false)
		// halfblock geometry: 1x2 subcells per cell
		type span struct{ y0, y1, x0, x1 int }
		wmY0 := int(math.Floor(lay.top))
		wmY1 := int(math.Ceil(lay.top + lay.rows))
		tagX0 := centerX(tc.w, "a calmer desk for your courses")
		tagX1 := tagX0 + fastWidth("a calmer desk for your courses")
		hintX0 := centerX(tc.w, "press any key to skip")
		hintX1 := hintX0 + fastWidth("press any key to skip")
		neighborSpans := []span{{wmY0, wmY1, lay.x0, lay.x0 + lay.wordW}, {lay.tagY, lay.tagY + 1, tagX0, tagX1}, {tc.h - 2, tc.h - 1, hintX0, hintX1}}
		// halfblock paints cell (x, y) as two subrows; the bottom one is
		// the bg SGR. The scrim darkens toward the base multiplicatively
		// (alpha ≤ 0.18), so on background pixels it can only compress
		// steps — unless it adds structure of its own (a visible plate).
		// Bound the amplification at alpha/(1-alpha) ≈ 22% + slack for the
		// scrim's own per-cell slope and 8-bit quantization.
		for _, sp := range neighborSpans {
			for y := sp.y0; y < sp.y1; y++ {
				for x := sp.x0 + 1; x < sp.x1; x++ {
					lumA, lumB := rasterLumAt(plain, x, y*2+1), rasterLumAt(plain, x-1, y*2+1)
					// stars and the hull-clamped glow cores are content,
					// not background — the plate invariant covers the sky.
					if math.Max(lumA, lumB) > 0.5 {
						continue
					}
					stepWith := math.Abs(rasterLumAt(withScrim, x, y*2+1) - rasterLumAt(withScrim, x-1, y*2+1))
					stepPlain := math.Abs(lumA - lumB)
					if stepWith > stepPlain*1.25+0.03 {
						t.Errorf("%dx%d row %d col %d: background step %.3f exceeds the un-scrimmed step %.3f x1.25 + 0.03 — visible plate edge", tc.w, tc.h, y, x, stepWith, stepPlain)
					}
				}
			}
		}
	}
}
