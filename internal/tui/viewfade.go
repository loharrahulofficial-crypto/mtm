package tui

import (
	"strings"
	"time"

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
// moment of the switch, easing to 0 as the crossfade settles.
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

// viewFadeProgress is the transition's completion: 0 at the switch, →1
// once settled. A zero viewFadeStart means settled.
func viewFadeProgress(m Model) float64 {
	if m.viewFadeStart.IsZero() {
		return 1
	}
	return 1 - m.viewFadeAmount()
}

func easeOutCubic(t float64) float64 {
	t -= 1
	return t*t*t + 1
}

// stableFrame caches the last fully settled composite (the string at the
// point the crossfade is applied — after joinBlocksV, before overlays).
// View is a value-receiver render, so the package var is the right place
// (matches the package-level panel caches); the TUI is single-threaded.
var stableFrame string

// crossfadeFrames blends prev into curr by t ∈ [0,1] in raster space.
// ambient.Crossfade eases internally (EaseInOutCubic) — t is not pre-eased
// here. This runs only during the ~150ms transition (~5 ticks), so the SGR
// parse cost is fine; the panel caches are unaffected.
func crossfadeFrames(prev, curr string, t float64) string {
	if prev == "" || t <= 0 {
		return curr
	}
	prevLines := strings.Split(prev, "\n")
	currLines := strings.Split(curr, "\n")
	// Never emit a taller/shorter frame than the settled one; Crossfade
	// tolerates width drift via fitRaster, but not line-count drift.
	if len(prevLines) != len(currLines) {
		return curr
	}
	return strings.Join(ambient.Crossfade(prevLines, currLines, t), "\n")
}
