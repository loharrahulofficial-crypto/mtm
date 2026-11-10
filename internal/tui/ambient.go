package tui

import (
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// ambient.go — Phase 4b-B integration: ambient scenes behind the splash,
// the Home corner, the Focus background and the task-complete reward burst.
// Determinism lives in internal/ambient; this file owns scheduling,
// placement and the config wiring.
//
// Budget: scenes only render when a surface is visible and the terminal is
// focused; the ambient tick stops entirely when nothing animates.

type ambientTickMsg struct{}
type splashTickMsg struct{}

const rewardDuration = 1500 * time.Millisecond

// ambientEnabled reports whether the ambient engine is allowed anywhere.
func (m Model) ambientEnabled() bool { return m.cfg.UI.Ambient != "off" && m.animations }

func (m Model) ambientAllowSplash() bool {
	return m.cfg.UI.Ambient == "splash" || m.cfg.UI.Ambient == "both"
}
func (m Model) ambientAllowHome() bool {
	return m.cfg.UI.Ambient == "home" || m.cfg.UI.Ambient == "both"
}
func (m Model) ambientAllowFocus() bool  { return m.ambientEnabled() }
func (m Model) ambientAllowReward() bool { return m.ambientEnabled() && m.cfg.UI.RewardAnimation }

// ambientActive reports whether any surface currently wants frames.
func (m Model) ambientActive() bool {
	if !m.ambientEnabled() {
		return false
	}
	if !m.ambientFocused {
		return false
	}
	if m.mode == modeSplash {
		return true
	}
	if m.rewardUntil.After(time.Now()) {
		return true
	}
	if m.mode == modeFocus {
		return !m.focusSceneFrozen()
	}
	if m.mode == modeNormal && m.view == viewHome && m.ambientAllowHome() {
		if _, ok := homeSceneCornerRect(m, m.mainW(), m.height-1); ok {
			return true
		}
	}
	return false
}

func (m Model) mainW() int {
	_, w := mainPaneSize(m)
	return w
}

// focusSceneFrozen reports whether the scene behind Focus should hold
// still: explicit pause, or the timer not running at all.
func (m Model) focusSceneFrozen() bool {
	v := m.timer
	if v.Phase == core.TimerPhaseIdle {
		return false // the scene still breathes behind an idle focus
	}
	return !v.Running
}

// sceneForLaunch resolves ui.scene: auto (by local hour) | random (per
// launch) | explicit name.
func (m Model) sceneForLaunch() string {
	switch m.cfg.UI.Scene {
	case "auto":
		h := m.clock.Now().Hour()
		switch {
		case h >= 5 && h < 17:
			return "ocean" // day: tide climbing
		case h >= 17 && h < 20:
			return "rain"
		default:
			return "night" // evening and overnight
		}
	case "random":
		names := ambient.Registered()
		seed := m.ambientSeed
		if seed == 0 {
			seed = m.clock.Now().UnixNano() / int64(time.Second)
		}
		return names[abs64(seed)%int64(len(names))]
	default:
		return m.cfg.UI.Scene
	}
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func (m *Model) currentScene() ambient.Scene {
	name := m.ambientSceneName
	if name == "" {
		name = m.sceneForLaunch()
		m.ambientSceneName = name
	}
	if m.scene == nil || m.sceneName != name {
		m.scene = ambient.New(name, m.cfg.UI.AmbientRender)
		m.sceneName = name
	}
	return m.scene
}

// cycleScene advances through the registered scenes; wraps around.
func (m *Model) cycleScene(dir int) {
	names := ambient.Registered()
	cur := m.ambientSceneName
	if cur == "" {
		cur = m.sceneForLaunch()
	}
	idx := 0
	for i, n := range names {
		if n == cur {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(names)) % len(names)
	m.ambientSceneName = names[idx]
	m.sceneName = "" // force rebuild
	m.statusMsg = "scene: " + m.ambientSceneName
}

// scenePalette converts the theme roles into the ambient palette.
func (m Model) scenePalette() ambient.Palette {
	p := m.palette
	return ambient.Palette{
		Base: p.Base, Surface: p.Surface, SurfaceRaised: p.SurfaceRaised, SurfaceStrong: p.SurfaceStrong,
		Border: p.Border, Text: p.Text, BrightText: p.BrightText, MutedSoft: p.MutedSoft,
		Accent: p.Accent, AccentSoft: p.AccentSoft, Success: p.Success, Warning: p.Warning, Danger: p.Danger,
		HeatWarm: p.HeatWarm, HeatHot: p.HeatHot, HeatOverdue: p.HeatOverdue,
		Blue: p.Blue, Cyan: p.Cyan, Yellow: p.Yellow, Magenta: p.Magenta, Orange: p.Orange, Brown: p.Brown,
	}
}

// sceneState builds the deterministic scene input from the live model.
func (m Model) sceneState() ambient.SceneState {
	v := m.timer
	progress := v.Progress
	if v.Phase == core.TimerPhaseIdle {
		progress = 1 // resting scenes breathe at full growth
	}
	if m.mode == modeFocus && m.timer.Phase == core.TimerPhaseIdle {
		progress = 0 // an idle focus should sit at the beginning of the arc
	}
	return ambient.SceneState{
		Progress:  progress,
		Phase:     ambient.Phase(v.Phase),
		Now:       m.clock.Now(),
		Seed:      42,
		Palette:   m.scenePalette(),
		Lightning: m.cfg.UI.RainLightning,
	}
}

// dimScenePalette renders a copy of the scene palette blended toward Base
// so bright text stays legible over the Focus background scene.
func (m Model) dimScenePalette() ambient.Palette {
	p := m.scenePalette()
	dim := func(c string) string { return theme.Blend(c, p.Base, 0.42) }
	return ambient.Palette{
		Base: p.Base, Surface: dim(p.Surface), SurfaceRaised: dim(p.SurfaceRaised), SurfaceStrong: dim(p.SurfaceStrong),
		Border: dim(p.Border), Text: dim(p.Text), BrightText: dim(p.BrightText), MutedSoft: dim(p.MutedSoft),
		Accent: dim(p.Accent), AccentSoft: dim(p.AccentSoft), Success: dim(p.Success), Warning: dim(p.Warning), Danger: dim(p.Danger),
		HeatWarm: dim(p.HeatWarm), HeatHot: dim(p.HeatHot), HeatOverdue: dim(p.HeatOverdue),
		Blue: dim(p.Blue), Cyan: dim(p.Cyan), Yellow: dim(p.Yellow), Magenta: dim(p.Magenta), Orange: dim(p.Orange), Brown: dim(p.Brown),
	}
}

// sceneFrame renders one ambient frame for the place being drawn.
func (m *Model) sceneFrame(t time.Duration, w, h int, dim bool) []string {
	st := m.sceneState()
	if dim {
		st.Palette = m.dimScenePalette()
	}
	return m.currentScene().Frame(t, w, h, st)
}

// sceneRewardFrame renders the celebration overlay frame.
func (m *Model) sceneRewardFrame(t time.Duration, w, h int) []string {
	st := m.sceneState()
	st.Progress = 1
	st.Phase = ambient.PhaseWork
	return m.currentScene().Reward(t, w, h, st)
}

// ambientTick schedules the next frame when — and only when — a surface
// wants it. The ticker stops entirely otherwise: zero work while idle.
func (m Model) armAmbient() tea.Cmd {
	if !m.ambientActive() {
		m.ambientArmed = false
		return nil
	}
	if m.ambientArmed {
		return nil
	}
	m.ambientArmed = true
	fps := m.cfg.UI.AmbientFPS
	if fps < 1 {
		fps = 10
	}
	if fps > 30 {
		fps = 30
	}
	return ambientTickCmd(fps)
}

func ambientTickCmd(fps int) tea.Cmd {
	d := time.Second / time.Duration(fps)
	return tea.Tick(d, func(time.Time) tea.Msg { return ambientTickMsg{} })
}

func splashTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return splashTickMsg{} })
}

// handleAmbientTick advances the scene clock and keeps the ticker alive
// only while a surface wants frames.
func (m Model) handleAmbientTick() (tea.Model, tea.Cmd) {
	if !m.ambientActive() {
		m.ambientArmed = false
		return m, nil
	}
	fps := m.cfg.UI.AmbientFPS
	if fps < 1 {
		fps = 10
	}
	if fps > 30 {
		fps = 30
	}
	d := time.Second / time.Duration(fps)
	now := m.clock.Now()
	if m.lastAmbientTick.IsZero() {
		m.ambientT += d
	} else {
		m.ambientT += now.Sub(m.lastAmbientTick)
	}
	m.lastAmbientTick = now
	if m.mode == modeSplash && m.ambientT >= m.splashTotal {
		m.mode = modeNormal
		m.ambientArmed = false
		return m, m.armAmbient()
	}
	m.ambientArmed = true
	return m, ambientTickCmd(fps)
}

// startSplashIfEnabled arms the splash when policy allows. Called by Run.
func (m *Model) startSplashIfEnabled(isTTY bool) bool {
	if !m.ambientAllowSplash() || !m.animations {
		return false
	}
	if !isTTY {
		return false
	}
	if os.Getenv("MTM_NO_SPLASH") == "1" {
		return false
	}
	m.mode = modeSplash
	m.splashStart = m.clock.Now()
	m.splashTotal = time.Duration(m.cfg.UI.SplashSeconds * float64(time.Second))
	m.ambientT = 0
	m.ambientArmed = false
	return true
}

// wordmarkGlyphs holds the block-letter glyphs for "mtm" as per-letter
// blocks so tests can assert the letters render distinct, legible shapes.
type wordmarkGlyphs struct{ letters [][]string }

// wordmarkSmall is the 3-row wordmark; the t carries a top bar over a stem
// (reads as a t, never as an I).
var wordmarkSmall = wordmarkGlyphs{letters: [][]string{
	{"█▄ ▄█", "█▀▄▀█", "█   █"}, // m
	{"█████", "  █  ", "  █  "}, // t
	{"█▄ ▄█", "█▀▄▀█", "█   █"}, // m
}}

// wordmarkLarge is the 5-row wordmark for roomy terminals.
var wordmarkLarge = wordmarkGlyphs{letters: [][]string{
	{"██   ██", "███ ███", "██▀█▀██", "██   ██", "██   ██"}, // m
	{"███████", "  ███  ", "  ███  ", "  ███  ", "  ███  "}, // t
	{"██   ██", "███ ███", "██▀█▀██", "██   ██", "██   ██"}, // m
}}

func (g wordmarkGlyphs) rows() []string {
	rows := make([]string, 0, len(g.letters[0]))
	for i := range g.letters[0] {
		parts := make([]string, 0, len(g.letters))
		for _, l := range g.letters {
			parts = append(parts, l[i])
		}
		rows = append(rows, strings.Join(parts, "   "))
	}
	return rows
}

// wordmarkFor chooses the wordmark size the terminal can hold.
func wordmarkFor(w, h int) (wordmarkGlyphs, bool) {
	if w >= 110 && h >= 28 {
		return wordmarkLarge, true
	}
	return wordmarkSmall, false
}

// renderSplash composes the startup splash: the current scene behind the
// mtm wordmark. Any key skips.
func (m Model) renderSplash() string {
	w, h := m.width, m.height
	if w < 20 || h < 10 {
		return tooSmallView(w, h, lipgloss.Color(m.palette.Base), m.keymap.Seq(aQuit))
	}
	lines := m.sceneFrame(m.ambientT, w, h, true) // dimmed: wordmark pops
	bright := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Bold(true)
	glyphs, large := wordmarkFor(w, h)
	unit := glyphs.rows()
	out := append([]string{}, lines...)
	cy := h / 2
	place := func(y int, line string) {
		if y < 0 || y >= h {
			return
		}
		out[y] = strings.Repeat(" ", maxInt((w-lipgloss.Width(line))/2, 0)) + line
	}
	if large {
		place(cy-3, bright.Render(unit[0]))
		place(cy-2, bright.Render(unit[1]))
		place(cy-1, bright.Render(unit[2]))
		place(cy, bright.Render(unit[3]))
		place(cy+1, bright.Render(unit[4]))
		place(cy+3, accent.Render("✦  m t m  ✦"))
		place(cy+5, muted.Render("a calmer desk for your courses"))
	} else {
		place(cy-2, bright.Render(unit[0]))
		place(cy-1, bright.Render(unit[1]))
		place(cy, bright.Render(unit[2]))
		place(cy+2, accent.Render("✦  m t m  ✦"))
		place(cy+4, muted.Render("a calmer desk for your courses"))
	}
	place(h-2, muted.Render("press any key to skip"))
	return strings.Join(out, "\n")
}

// renderRewardPanel composes the floating 1.5s completion celebration:
// the current scene's Reward frame inside an accent-edged panel.
func (m Model) renderRewardPanel() string {
	if !m.ambientEnabled() {
		return ""
	}
	rw := minInt(56, m.width-4)
	rh := minInt(15, m.height-2)
	if rw < 22 || rh < 7 {
		return ""
	}
	edge := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(lipgloss.Color(m.palette.Base))
	head := "✦ task done"
	fillN := rw - 4 - lipgloss.Width(head)
	if fillN < 0 {
		fillN = 0
	}
	top := edge.Render("╭─ " + head + " " + strings.Repeat("─", fillN) + "╮")
	bot := edge.Render("╰" + strings.Repeat("─", rw-2) + "╯")
	right := edge.Render("│")
	inner := m.sceneRewardFrame(m.ambientT, rw-2, rh-2)
	lines := []string{top}
	for _, l := range inner {
		if w := lipgloss.Width(l); w > rw-2 {
			l = clipStyled(l, rw-2)
		} else if w < rw-2 {
			l = l + lipgloss.NewStyle().Background(lipgloss.Color(m.palette.Base)).Render(strings.Repeat(" ", rw-2-w))
		}
		lines = append(lines, edge.Render("│")+l+right)
	}
	lines = append(lines, bot)
	return strings.Join(lines, "\n")
}

// rateLimitReward reports whether another reward burst may fire now.
// At most three bursts per rolling minute — bulk completions don't spam.
func (m *Model) rateLimitReward() bool {
	now := m.clock.Now()
	keep := m.rewardFires[:0]
	for _, t := range m.rewardFires {
		if now.Sub(t) < time.Minute {
			keep = append(keep, t)
		}
	}
	m.rewardFires = keep
	if len(m.rewardFires) >= 3 {
		return false
	}
	m.rewardFires = append(m.rewardFires, now)
	return true
}

// fireReward shows the burst and caption; never when disabled or limited.
// Returns the arm command so the ticker starts immediately.
func (m *Model) fireReward(title string) tea.Cmd {
	if !m.ambientAllowReward() {
		return nil
	}
	if !m.rateLimitReward() {
		return nil
	}
	m.rewardUntil = m.clock.Now().Add(rewardDuration)
	m.rewardTitle = title
	m.armAmbient()
	fps := m.cfg.UI.AmbientFPS
	if fps < 1 {
		fps = 10
	}
	if fps > 30 {
		fps = 30
	}
	return ambientTickCmd(fps)
}
