package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// splash_fade_test.go — the splash head fade warms the scene in through a
// palette-intensity ramp, so no cell may show a half-cell luminance jump
// worse than the same cell carries in the full frame rendered at the
// same scene instant.

func lumRGB(r, g, b uint8) float64 {
	return 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
}

func splashPair(t *testing.T, ms int) ([]ambient.RGBPix, []ambient.RGBPix, int, int) {
	t.Helper()
	fixedEnv(t)
	lipgloss.SetColorProfile(termenv.TrueColor)
	cfg := config.Default()
	cfg.DataRepo = t.TempDir()
	cfg.Sync.Auto = false
	root := t.TempDir()
	s, err := store.Open(root, root+"/index.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	s.Clock = clock.Fixed(DumpFixedTime)
	m := New(cfg, s, sync.New(s.Root))
	m.cfg.UI.Ambient = "both"
	m.ambientSceneName = "night"
	if !m.startSplashIfEnabled(true) {
		t.Fatal("splash did not start")
	}
	up, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = up.(Model)
	secs := float64(ms) / 1000
	m.ambientT = time.Duration(ms) * time.Millisecond

	// full frame at the same scene instant (headK = 1)
	sceneT := time.Duration(0)
	if secs > 0.25 {
		sceneT = time.Duration((secs - 0.25) * float64(time.Second))
	}
	st := m.sceneState()
	st.Palette = m.dimScenePalette()
	full := m.currentScene().FramePixels(sceneT, 80, 24, st)
	fullLines := full.EmitHalfblock(st.Palette.Base, true, st.PaletteLimited, st.Dither)
	_, _, fullPix := ambient.DecodeFrame(fullLines)

	// faded frame via the real splash scene path
	headK := 1.0
	if secs < 0.55 {
		headK = ambient.EaseOutCubic(secs / 0.55)
	}
	faded := m.sceneFrameFaded(sceneT, secs, 80, 24, headK)
	_, _, fadedPix := ambient.DecodeFrame(faded)
	return fullPix, fadedPix, 80, len(fullPix) / (80 * 2)
}

func TestSplashFadeHalfCellJumpWithinSource(t *testing.T) {
	for _, ms := range []int{0, 100, 200, 400, 550} {
		full, faded, w, rows := splashPair(t, ms)
		jump := func(pix []ambient.RGBPix, cy, cx int) float64 {
			i := cy * 2 * w
			top, bot := pix[i+cx], pix[i+w+cx]
			d := lumRGB(top.R, top.G, top.B) - lumRGB(bot.R, bot.G, bot.B)
			if d < 0 {
				d = -d
			}
			return d
		}
		for cy := 0; cy < rows; cy++ {
			for cx := 0; cx < w; cx++ {
				jf, js := jump(faded, cy, cx), jump(full, cy, cx)
				if jf > js+2 {
					t.Fatalf("t=%dms cell (%d,%d): half-cell luminance jump %.1f exceeds full-frame jump %.1f", ms, cx, cy, jf, js)
				}
			}
		}
	}
}
