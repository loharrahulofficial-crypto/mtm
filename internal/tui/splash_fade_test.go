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
// worse than the source frame's own worst jump. (Per-cell affine scaling
// is not available: BuildRamps' enforceContrast iterates discrete nudges,
// so the ramp can diverge locally by one ladder step or one additive-glow
// clip; the whole-frame luminance bound catches the half-cell pop this
// fade once exhibited.)

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
	fadedRaster, baseHex, limited, dither, _ := m.splashSceneRaster(sceneT, secs, 80, 24, headK, m.splashTotal.Seconds(), m.reducedMotionActive(), nil)
	faded := fadedRaster.EmitHalfblock(baseHex, true, limited, dither)
	_, _, fadedPix := ambient.DecodeFrame(faded)
	return fullPix, fadedPix, 80, len(fullPix) / (80 * 2)
}

func maxHalfCellJump(pix []ambient.RGBPix, cells, rows int) float64 {
	jump := func(cy, cx int) float64 {
		i := cy * 2 * cells
		top, bot := pix[i+cx], pix[i+cells+cx]
		d := lumRGB(top.R, top.G, top.B) - lumRGB(bot.R, bot.G, bot.B)
		if d < 0 {
			d = -d
		}
		return d
	}
	maxJ := 0.0
	for cy := 0; cy < rows; cy++ {
		for cx := 0; cx < cells; cx++ {
			if j := jump(cy, cx); j > maxJ {
				maxJ = j
			}
		}
	}
	return maxJ
}

func TestSplashFadeHalfCellJumpWithinSource(t *testing.T) {
	for _, ms := range []int{0, 100, 200, 400, 550} {
		full, faded, w, rows := splashPair(t, ms)
		fadedMax := maxHalfCellJump(faded, w, rows)
		if ms == 0 {
			// the very first frame is a plain base-colored frame
			if fadedMax != 0 {
				t.Fatalf("t=0: first frame has half-cell jump %.1f, want 0 (plain base frame)", fadedMax)
			}
			continue
		}
		fullMax := maxHalfCellJump(full, w, rows)
		// every t's worst half-cell jump stays within two 8-bit steps of
		// the source frame's own worst jump
		if fadedMax > fullMax+2 {
			t.Fatalf("t=%dms: half-cell jump %.1f exceeds the frame's own jump %.1f", ms, fadedMax, fullMax)
		}
	}
}
