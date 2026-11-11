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
// moment of the switch, easing to 0 linearly as the crossfade settles.
// ambient.Crossfade applies its own ease to the pixel blend.
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
// 0 at the switch, 1 once settled. ambient.Crossfade applies
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
