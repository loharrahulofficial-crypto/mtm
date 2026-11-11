package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// splash_golden_test.go — the splash at five timestamps (fade-in, scene
// drift, letters landed, tagline in, dissolve tail) at 100x30 halfblock.

var splashGoldenTimes = []int{200, 900, 1500, 2200, 3100}

func TestSplashGolden(t *testing.T) {
	update := os.Getenv("MTM_UPDATE_GOLDEN") == "1"
	dir := filepath.Join("testdata", "splash")
	if update {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, ms := range splashGoldenTimes {
		t.Run(fmt.Sprintf("%04dms", ms), func(t *testing.T) {
			fixedEnv(t)
			// Pin the color profile: without it lipgloss falls back to the
			// environment's default (Ascii when piped), and the frames change
			// under any test that forces TrueColor later in the suite.
			lipgloss.SetColorProfile(termenv.TrueColor)
			cfg := config.Default()
			cfg.DataRepo = t.TempDir()
			cfg.Sync.Auto = false
			// Golden frames must not depend on the machine's speed: pin
			// the adaptive-quality tier, or slow renders trip the frame
			// budget and supersampling drops mid-suite.
			ambient.SetQualityTier(ambient.TierFull)
			root := t.TempDir()
			s, err := store.Open(root, filepath.Join(root, "index.json"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = s.Close() }()
			s.Clock = clock.Fixed(DumpFixedTime) // pin BEFORE New() captures it
			m := New(cfg, s, sync.New(root))
			m.cfg.UI.Ambient = "both"
			m.ambientSceneName = "night"
			if !m.startSplashIfEnabled(true) {
				t.Fatal("splash did not start")
			}
			up, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
			m = up.(Model)
			m.ambientT = time.Duration(ms) * time.Millisecond
			frame := m.View()
			got := strings.Join(strings.Split(frame, "\n"), "\n") + "\n"
			golden := filepath.Join(dir, fmt.Sprintf("night-%04dms.golden", ms))
			if update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("missing golden %s (run MTM_UPDATE_GOLDEN=1): %v", golden, err)
			}
			if string(want) != got {
				t.Fatalf("splash mismatch at %dms — regenerate with MTM_UPDATE_GOLDEN=1 if intended", ms)
			}
		})
	}
}
