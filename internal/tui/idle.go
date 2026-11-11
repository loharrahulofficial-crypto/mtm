package tui

import "time"

// idleState is the animation-throttle tier for the Home/sidebar scene.
type idleState int

const (
	idleActive idleState = iota // recently used — full cap
	idleSlow                    // 30 s without input — ~4 FPS
	idlePaused                  // 5 min without input — frozen still
)

const (
	idleSlowAfter  = 30 * time.Second
	idlePauseAfter = 5 * time.Minute
	idleSlowFPS    = 4
)

// ambientIdleState resolves the throttle tier for the active surface.
// Exemptions while the user is engaged: Focus with a running timer (the
// ring and the work scene earn their keep) and the splash (brief by
// construction). ui.ambient_idle selects the policy:
//
//	slow  (default)  30 s → ~4 FPS; 5 min → frozen still
//	pause            30 s → frozen still (no 4 FPS stage)
//	off              never throttles
func (m Model) ambientIdleState() idleState {
	if m.cfg.UI.AmbientIdle == "off" {
		return idleActive
	}
	if m.mode == modeSplash {
		return idleActive
	}
	if m.mode == modeFocus && m.timer.Running {
		return idleActive
	}
	d := m.clock.Now().Sub(m.lastInput)
	switch m.cfg.UI.AmbientIdle {
	case "pause":
		if d >= idleSlowAfter {
			return idlePaused
		}
		return idleActive
	default: // "slow"
		switch {
		case d >= idlePauseAfter:
			return idlePaused
		case d >= idleSlowAfter:
			return idleSlow
		default:
			return idleActive
		}
	}
}

// ambientIdleFPS caps the frame rate for the throttled tier.
func ambientIdleFPS(state idleState) int {
	if state == idleSlow {
		return idleSlowFPS
	}
	return 0
}
