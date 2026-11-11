package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// splash_sextant_test.go — the splash must honor ui.ambient_render: sextant:
// the scene frame carries real 2×3 mosaics, the frame keeps the terminal's
// row count, and the dissolve tail fades colors to base. (The halfblock
// default path is covered by the golden/startup tests.)

var splashColorRe = regexp.MustCompile(`(?:38|48);2;(\d+);(\d+);(\d+)`)

func TestSplashRendersSextantSceneWhenConfigured(t *testing.T) {
	fixedEnv(t)
	m := splashModel(t, "on")
	m.ambientSceneName = "night"
	m.cfg.UI.AmbientRender = "sextant"
	if !m.startSplashIfEnabled(true) {
		t.Fatal("splash did not start")
	}
	up, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = up.(Model)
	m.ambientT = time.Second // head fade done (t ≥ 0.55)
	frame := m.View()
	lines := strings.Split(frame, "\n")
	if len(lines) != 30 {
		t.Fatalf("splash emitted %d lines, want 30", len(lines))
	}
	found := 0
	for _, l := range lines {
		if w := fastWidth(l); w != 100 {
			t.Fatalf("line width = %d cells, want 100", w)
		}
		for _, r := range stripANSI(l) {
			if r >= 0x1FB00 && r <= 0x1FB3B {
				found++
			}
		}
	}
	if found == 0 {
		t.Fatal("sextant splash emitted no 2×3 mosaics — the halfblock path ran instead")
	}
}

func TestSplashSextantDissolveFadesToBase(t *testing.T) {
	fixedEnv(t)
	m := splashModel(t, "on")
	m.ambientSceneName = "night"
	m.cfg.UI.AmbientRender = "sextant"
	if !m.startSplashIfEnabled(true) {
		t.Fatal("splash did not start")
	}
	up, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = up.(Model)
	m.ambientT = 3 * time.Second // inside the 0.45s dissolve tail
	frame := m.View()
	for i, l := range strings.Split(frame, "\n") {
		for _, mm := range splashColorRe.FindAllStringSubmatch(l, -1) {
			// #14282d is the default base (20;40;45) — the dissolve end
			// state is base-colored cells; any strongly colored pixel
			// means the fade was skipped.
			r := atoiSmallReg(mm[1])
			g := atoiSmallReg(mm[2])
			b := atoiSmallReg(mm[3])
			if r+g+b > 135 { // base sum is 105; allow slack for ramp halos
				t.Fatalf("line %d: color %d;%d;%d survived the dissolve tail", i, r, g, b)
			}
		}
	}
}

func atoiSmallReg(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}
