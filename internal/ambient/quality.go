package ambient

import (
	"sync"
	"time"
)

// quality.go — adaptive frame-rate control (§4.7).
//
// The TUI reports every rendered end-to-end frame (scene raster + full
// View()) via ObserveFrameCost. If frames blow the budget for three
// consecutive frames the quality level steps down (halve supersampling →
// halve FPS); with forty-five consecutive frames of headroom it steps
// back up. The active level is reported in `mtm doctor --json`.
type QualityLevel int

const (
	TierFull   QualityLevel = iota // everything on
	TierLowSS                      // supersampling factor 2 → 1
	TierLowFPS                     // FPS caps halved by the TUI
)

// FrameBudget is the per-frame cost budget; three consecutive frames
// slower than this means the machine cannot afford the current level.
// A variable (not a constant) so tests can shrink it and drive the
// tier state machine through the real View() path.
var FrameBudget = 30 * time.Millisecond

var (
	qualityMu    sync.Mutex
	qualityTierV QualityLevel
	qOvers       int
	qGoods       int
)

// ObserveFrameCost records one rendered end-to-end frame (scene raster +
// full View()). Level changes apply immediately to the supersampling
// factor fSS.
func ObserveFrameCost(d time.Duration) {
	qualityMu.Lock()
	defer qualityMu.Unlock()
	if d > FrameBudget {
		qOvers++
		qGoods = 0
	} else if d < FrameBudget/2 {
		qGoods++
		qOvers = 0
	} else {
		qOvers = 0
		qGoods = 0
	}
	switch {
	case qOvers >= 3 && qualityTierV < TierLowFPS:
		qualityTierV++
		qOvers, qGoods = 0, 0
		applyTierLocked()
	case qGoods >= 45 && qualityTierV > TierFull:
		qualityTierV--
		qOvers, qGoods = 0, 0
		applyTierLocked()
	}
}

// applyTierLocked needs the caller to hold qualityMu.
func applyTierLocked() {
	if qualityTierV >= TierLowSS {
		fSS = 1
		return
	}
	fSS = 2
}

// AmbientTier reports the active level as (id, name).
func AmbientTier() (int, string) {
	qualityMu.Lock()
	t := qualityTierV
	qualityMu.Unlock()
	switch t {
	case TierFull:
		return 0, "full"
	case TierLowSS:
		return 1, "low-ss"
	case TierLowFPS:
		return 2, "low-fps"
	default:
		return int(t), "unknown"
	}
}

// SetQualityTier forces a level (used by tests and any manual override).
func SetQualityTier(t QualityLevel) {
	qualityMu.Lock()
	qualityTierV = t
	qOvers, qGoods = 0, 0
	applyTierLocked()
	qualityMu.Unlock()
}

// resetAdaptiveForTest returns the decider to full quality and clears state.
func resetAdaptiveForTest() {
	SetQualityTier(TierFull)
}
