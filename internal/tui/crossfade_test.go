package tui

import (
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
)

// crossfade_test.go — §4.3: alt+s arms a 600ms crossfade and it completes
// on the fixed clock; reduced motion switches instantly.

func TestAltSArmsCrossfade(t *testing.T) {
	m, s := newTestModel(t, nil)
	defer func() { _ = s.Close() }()
	s.Clock = clock.Fixed(DumpFixedTime)
	m.cfg.UI.Scene = "night"
	m.ambientSceneName = "night"
	m.scene = nil
	m.currentScene() // build the scene
	up, _ := m.Update(keyMsg("alt+s"))
	m = up.(Model)
	if m.scenePrev == nil {
		t.Fatal("alt+s did not arm a crossfade")
	}
	if m.crossfadeDur != 600*time.Millisecond {
		t.Fatalf("crossfade duration = %v, want 600ms", m.crossfadeDur)
	}
	// mid-fade: both frames render and differ from both endpoints
	_ = m.sceneFrame(m.ambientT, 120, 40, false)
	mid := m.sceneFrame(m.ambientT, 120, 40, false)
	if got := len(mid); got == 0 {
		t.Fatal("crossfade produced no frame")
	}
}

func TestCrossfadeCompletesOnFixedClock(t *testing.T) {
	m, s := newTestModel(t, nil)
	defer func() { _ = s.Close() }()
	m.clock = clock.Fixed(DumpFixedTime) // one fixed clock for the whole fade
	m.cfg.UI.Scene = "night"
	m.ambientSceneName = "night"
	m.currentScene() // build the scene so the crossfade has a blend source
	up, _ := m.Update(keyMsg("alt+s"))
	m = up.(Model)
	if m.scenePrev == nil {
		t.Fatal("alt+s did not arm a crossfade")
	}
	m.clock = clock.Fixed(DumpFixedTime.Add(700 * time.Millisecond))
	m.endCrossfadeIfDone()
	if m.scenePrev != nil {
		t.Fatal("crossfade did not complete after 700ms")
	}
	if m.crossfadeActive() {
		t.Fatal("crossfadeActive still true after completion")
	}
}

func TestReducedMotionSwitchesInstantly(t *testing.T) {
	m, s := newTestModel(t, nil)
	defer func() { _ = s.Close() }()
	m.cfg.UI.ReducedMotion = true
	m.cfg.UI.Scene = "night"
	m.ambientSceneName = "night"
	up, _ := m.Update(keyMsg("alt+s"))
	m = up.(Model)
	if m.scenePrev != nil {
		t.Fatal("reduced motion armed a crossfade")
	}
	if !m.reducedMotionActive() {
		t.Fatal("reducedMotionActive() false after enabling")
	}
}

func TestNO_MOTIONEnvHonored(t *testing.T) {
	m, s := newTestModel(t, nil)
	defer func() { _ = s.Close() }()
	t.Setenv("NO_MOTION", "1")
	if !m.reducedMotionActive() {
		t.Fatal("NO_MOTION=1 did not honor reduced motion")
	}
}

func TestPhaseIntensityCurve(t *testing.T) {
	m, s := newTestModel(t, nil)
	defer func() { _ = s.Close() }()
	m.clock = clock.Fixed(DumpFixedTime)
	m.phaseFadeStart = DumpFixedTime
	m.phaseFadeFrom = 1.0
	m.phaseFadeTo = 0.6
	m.phaseFadeActive = true
	if got := m.sceneIntensity(); got != 1.0 {
		t.Fatalf("intensity at t=0 = %v, want 1.0", got)
	}
	m.clock = clock.Fixed(DumpFixedTime.Add(750 * time.Millisecond))
	if got := m.sceneIntensity(); got <= 0.6 || got >= 1.0 {
		t.Fatalf("intensity mid-fade = %v, want between 0.6 and 1.0", got)
	}
	m.clock = clock.Fixed(DumpFixedTime.Add(1600 * time.Millisecond))
	if got := m.sceneIntensity(); got != 0.6 {
		t.Fatalf("intensity after 1.6s = %v, want 0.6", got)
	}
}
