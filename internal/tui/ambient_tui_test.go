package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// ambient_tui_test.go — Phase 4b-B integration: splash policy, scene
// cycling, reward rate limiting, the :ambient screen, and the frame
// integration points (corner, focus, reward overlay).

func TestSplashPolicyMatrix(t *testing.T) {
	for _, ambientSetting := range []string{"off", "splash", "home", "sidebar", "both"} {
		for _, anim := range []bool{true, false} {
			for _, isTTY := range []bool{true, false} {
				m := freshAmbientModel(t)
				m.cfg.UI.Ambient = ambientSetting
				m.animations = anim
				isSplash := m.startSplashIfEnabled(isTTY)
				want := (ambientSetting == "splash" || ambientSetting == "both") && anim && isTTY
				if isSplash != want {
					t.Errorf("ambient=%s anim=%v tty=%v → splash=%v, want %v", ambientSetting, anim, isTTY, isSplash, want)
				}
			}
		}
	}
	t.Setenv("MTM_NO_SPLASH", "1")
	m := freshAmbientModel(t)
	m.cfg.UI.Ambient = "both"
	if m.startSplashIfEnabled(true) {
		t.Fatal("MTM_NO_SPLASH=1 did not skip the splash")
	}
}

func freshAmbientModel(t *testing.T) Model {
	t.Helper()
	m, s := newTestModel(t, nil)
	_ = s.Close()
	return m
}

// TestSplashDoesNotStartTimer guards spec 0.3 for the splash path.
func TestSplashDoesNotStartTimer(t *testing.T) {
	m := freshAmbientModel(t)
	m.cfg.UI.Ambient = "both"
	if !m.startSplashIfEnabled(true) {
		t.Fatal("splash not enabled for ambient=both on a TTY")
	}
	if m.timer.Phase != core.TimerPhaseIdle {
		t.Fatalf("splash started a timer: %+v", m.timer)
	}
}

// TestRenderSplashExactDimensions: the splash is always exactly w×h rows.
func TestRenderSplashExactDimensions(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 50}} {
		m, s := newTestModel(t, nil)
		defer func() { _ = s.Close() }()
		m.cfg.UI.Ambient = "both"
		m.startSplashIfEnabled(true)
		up, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = up.(Model)
		frame := m.View()
		lines := strings.Split(frame, "\n")
		if len(lines) != size[1] {
			t.Fatalf("%dx%d: splash %d rows", size[0], size[1], len(lines))
		}
		for i, l := range lines {
			if w := lipglossWidth(l); w != size[0] {
				t.Fatalf("%dx%d row %d: %d cells", size[0], size[1], i, w)
			}
		}
	}
}

// TestCycleSceneWalksRegistered: cycle order is the registry order, wrapped.
func TestCycleSceneWalksRegistered(t *testing.T) {
	m := freshAmbientModel(t)
	m.cfg.UI.Scene = "night"
	m.ambientSceneName = "night"
	names := ambient.Registered()
	for i, want := range names[1:] {
		m.cycleScene(1)
		if m.currentSceneName() != want {
			t.Fatalf("cycle %d = %q, want %q", i, m.currentSceneName(), want)
		}
	}
	// wraps back to the first registered scene
	m.cycleScene(1)
	if m.currentSceneName() != names[0] {
		t.Fatalf("wrap = %q, want %q", m.currentSceneName(), names[0])
	}
}

// TestCommandScene covers :scene <name> and :scene list.
func TestCommandScene(t *testing.T) {
	m := freshAmbientModel(t)
	m2, _ := m.commandScene("night")
	m = m2
	if m.cfg.UI.Scene != "night" || m.currentSceneName() != "night" {
		t.Fatalf(":scene night → cfg=%q name=%q", m.cfg.UI.Scene, m.currentSceneName())
	}
	m2, _ = m.commandScene("bogus-scene")
	m = m2
	if !strings.Contains(m.statusMsg, "unknown scene") {
		t.Fatalf(":scene bogus → status=%q", m.statusMsg)
	}
	m2, _ = m.commandScene("list")
	m = m2
	if !strings.Contains(m.statusMsg, "night") || !strings.Contains(m.statusMsg, "bonsai") {
		t.Fatalf(":scene list → %q", m.statusMsg)
	}
	m2, _ = m.commandScene("auto")
	m = m2
	if m.cfg.UI.Scene != "auto" {
		t.Fatalf(":scene auto → %q", m.cfg.UI.Scene)
	}
}

// TestRewardRateLimit: at most three bursts per rolling minute.
func TestRewardRateLimit(t *testing.T) {
	m := freshAmbientModel(t)
	m.cfg.UI.Ambient = "both"
	m.cfg.UI.RewardAnimation = true
	m.animations = true
	for i := 0; i < 3; i++ {
		if cmd := m.fireReward("task"); cmd == nil {
			t.Fatalf("reward %d fired nothing", i)
		}
	}
	if cmd := m.fireReward("task"); cmd != nil {
		t.Fatal("fourth reward inside the minute rate-limit window was not skipped")
	}
	// window passes → fires again
	m.rewardFires = m.rewardFires[:1]
	m.rewardFires[0] = m.clock.Now().Add(-2 * time.Minute)
	if cmd := m.fireReward("task"); cmd == nil {
		t.Fatal("reward after the rate-limit window fired nothing")
	}
}

// TestRewardBurstVisibleInFrame: after a completion the overlay shows.
func TestRewardBurstVisibleInFrame(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{testTask("write the report", core.KindTask)})
	defer func() { _ = s.Close() }()
	m.cfg.UI.Ambient = "both"
	m.cfg.UI.RewardAnimation = true
	m.animations = true
	up, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = up.(Model)
	m.fireReward("write the report")
	if !m.rewardUntil.After(m.clock.Now()) {
		t.Fatal("fireReward did not arm rewardUntil")
	}
	frame := m.View()
	plain := StripANSI(frame)
	if !strings.Contains(plain, "task done") {
		t.Fatal("reward overlay missing from the frame")
	}
}

// TestAmbientScreenArrowKeysMoveSelection: j/k moves, esc closes.
func TestAmbientScreenArrowKeysMoveSelection(t *testing.T) {
	m := freshAmbientModel(t)
	m.mode = modeAmbient
	m.ambientSettingSel = 0
	up, _ := m.updateAmbientKey(keyMsg("j"))
	m = up.(Model)
	if m.ambientSettingSel != 1 {
		t.Fatalf("j → sel=%d, want 1", m.ambientSettingSel)
	}
	up, _ = m.updateAmbientKey(keyMsg("k"))
	m = up.(Model)
	if m.ambientSettingSel != 0 {
		t.Fatalf("k → sel=%d, want 0", m.ambientSettingSel)
	}
	up, _ = m.updateAmbientKey(keyMsg("esc"))
	m = up.(Model)
	if m.mode != modeNormal {
		t.Fatalf("esc → mode=%v, want modeNormal", m.mode)
	}
}

// TestAmbientScreenAdjustsScene: h/l cycles values and rebuilds scenes.
func TestAmbientScreenAdjustsScene(t *testing.T) {
	m := freshAmbientModel(t)
	m.mode = modeAmbient
	m.ambientSettingSel = 0 // scene row
	up, _ := m.updateAmbientKey(keyMsg("l"))
	m = up.(Model)
	if m.cfg.UI.Scene == "auto" {
		t.Fatal("l did not advance the scene setting")
	}
}

// TestAltSCyclesScene: alt+s advances to the next registered scene.
func TestAltSCyclesScene(t *testing.T) {
	m := freshAmbientModel(t)
	m.cfg.UI.Scene = "night"
	m.ambientSceneName = "night"
	up, _ := m.Update(keyMsg("alt+s"))
	m = up.(Model)
	if m.currentSceneName() == "night" {
		t.Fatal("alt+s did not advance the scene")
	}
}

// TestAmbientTickCrawlsWhenPaused: a paused Pomodoro slows the scene
// clock to a gentle crawl (progress-linked elements frozen) but keeps
// breathing instead of freezing.
func TestAmbientTickCrawlsWhenPaused(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{testTask("deep work", core.KindStudy)})
	defer func() { _ = s.Close() }()
	m2, _ := m.enterFocusTask(s.Tasks()[0])
	m = m2
	up, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = up.(Model)
	up, _ = m.Update(keyMsg(" ")) // start the pomodoro
	m = up.(Model)
	m.ambientT = time.Millisecond
	m.lastAmbientTick = m.clock.Now()
	up1, _ := m.Update(ambientTickMsg{})
	m1 := up1.(Model)
	t1 := m1.ambientT
	if t1 <= time.Millisecond {
		t.Fatal("ambientT did not advance on tick")
	}
	up, _ = m1.Update(keyMsg(" ")) // pause
	m3 := up.(Model)
	m3.ambientT = t1
	m3.lastAmbientTick = m3.clock.Now().Add(-time.Second) // pretend a second elapsed
	up2, _ := m3.Update(ambientTickMsg{})
	m3 = up2.(Model)
	advanced := m3.ambientT - t1
	if advanced < 245*time.Millisecond || advanced > 255*time.Millisecond {
		t.Fatalf("paused focus advanced the scene clock by %v, want ~250ms (gentle crawl)", advanced)
	}
	if !m3.ambientArmed {
		t.Fatal("paused focus stopped the ticker — gentle breathing expected")
	}
}

// TestAmbientFPSCappedPerSurface: the frame budget is ≤ 24 FPS on the
// focus surface (and splash), ≤ 8 FPS behind Home/sidebar.
func TestAmbientFPSCappedPerSurface(t *testing.T) {
	set := func(m Model, v int) Model { m.cfg.UI.AmbientFPS = v; return m }
	normal := freshAmbientModel(t)
	focus := freshAmbientModel(t)
	splash := freshAmbientModel(t)
	normal.mode = modeNormal
	focus.mode = modeFocus
	splash.mode = modeSplash
	for _, tc := range []struct {
		cfg   int
		home  int
		focus int
	}{
		{30, 8, 24},
		{20, 8, 20},
		{8, 8, 8},
		{0, 8, 8},
	} {
		m := set(normal, tc.cfg)
		if got := ambientFPS(m); got != tc.home {
			t.Errorf("cfg %d: home/sidebar cap = %d, want %d", tc.cfg, got, tc.home)
		}
		m = set(focus, tc.cfg)
		if got := ambientFPS(m); got != tc.focus {
			t.Errorf("cfg %d: focus cap = %d, want %d", tc.cfg, got, tc.focus)
		}
		m = set(splash, tc.cfg)
		if got := ambientFPS(m); got != tc.focus {
			t.Errorf("cfg %d: splash cap = %d, want %d", tc.cfg, got, tc.focus)
		}
	}
}
