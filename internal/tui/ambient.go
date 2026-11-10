package tui

import (
	"math"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/caps"
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
		Dither:    m.cfg.UI.AmbientDither != "off" && caps.Current().LimitedPalette,
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

// renderSplash composes the startup splash: the scene warms in behind
// the mtm wordmark, which rises letter by letter with a glow pulse, then
// the tagline fades in, and a noise dissolve wipes into Home. The whole
// sequence is ~3s; any key skips into the dissolve tail.
func clampUnit(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (m Model) renderSplash() string {
	w, h := m.width, m.height
	if w < 20 || h < 10 {
		return tooSmallView(w, h, lipgloss.Color(m.palette.Base), m.keymap.Seq(aQuit))
	}
	t := m.ambientT.Seconds()
	total := m.splashTotal.Seconds()
	if total < 1.8 {
		total = 3.0
	}
	sceneT := time.Duration(math.Max(0, t-0.25) * float64(time.Second))

	// scene elements drift in behind the fade
	sceneLines := m.sceneFrame(sceneT, w, h, true) // dimmed: wordmark pops
	out := append([]string{}, sceneLines...)

	// wordmark letters rise one by one with a glow pulse
	cy := h / 2
	m.wordmarkLetters(out, w, cy, t)

	// tagline fades in 1.6–2.2s
	taglineT := clampUnit((t-1.6)/0.6, 0, 1)
	if taglineT > 0 && cy+5 < h {
		blend := ambient.BlendHex(m.palette.Base, m.palette.MutedSoft, taglineT)
		blendStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(blend))
		applyAt(out, w, cy+5, centerLine(w, "a calmer desk for your courses"), blendStyle, m.palette.Base)
		applyAt(out, w, cy+3, centerLine(w, "✦  m t m  ✦"), lipgloss.NewStyle().Foreground(lipgloss.Color(ambient.BlendHex(m.palette.Base, m.palette.Accent, taglineT))).Bold(true), m.palette.Base)
	}

	// footer hint fades in early, stays
	footT := clampUnit((t-0.35)/0.4, 0, 1)
	if footT > 0 && h >= 2 {
		blend := ambient.BlendHex(m.palette.Base, m.palette.MutedSoft, footT)
		applyAt(out, w, h-2, centerLine(w, "press any key to skip"), lipgloss.NewStyle().Foreground(lipgloss.Color(blend)), m.palette.Base)
	}

	lines := out

	// head fade-in from the background (0.0–0.55s)
	if t < 0.55 {
		black := ambient.BlackFrame(lines)
		lines = ambient.Crossfade(black, lines, ambient.EaseOutCubic(t/0.55))
	}
	// tail dissolve (last 0.45s)
	if t > total-0.45 {
		prog := clampUnit((t-(total-0.45))/0.45, 0, 1)
		lines = ambient.Dissolve(lines, ambient.EaseInOutCubic(prog))
	}
	return strings.Join(lines, "\n")
}

// wordmarkLetters paints the rising letters of the wordmark into lines.
func (m Model) wordmarkLetters(out []string, w, cy int, t float64) {
	g, large := wordmarkFor(w, m.height)
	letters := g.letters
	letterW := len(letters[0][0])
	gap := 3
	wordW := len(letters)*letterW + (len(letters)-1)*gap
	x0 := (w - wordW) / 2
	letterDur := 0.55
	step := 0.28
	start := 0.9
	for i, letter := range letters {
		appear := clampUnit((t-(start+float64(i)*step))/letterDur, 0, 1)
		if appear <= 0 {
			continue
		}
		rize := ambient.EaseOutBack(appear)
		yOff := int(math.Round((1 - rize) * 2))
		bright := rize
		// glow pulse on landing
		landT := t - (start + float64(i)*step + letterDur*0.55)
		if landT > 0 && landT < 1.2 {
			pulse := ambient.Envelope(landT, 0, 0.25, 0.55, 1.1)
			bright = clampUnit(rize+pulse*0.35, 0, 1)
		}
		col := ambient.BlendHex(m.palette.Base, m.palette.BrightText, bright)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(col)).Bold(true)
		rowOff := 2
		if large {
			rowOff = 3
		}
		for r, row := range letter {
			y := cy - rowOff + r + yOff
			if y < 0 || y >= len(out) {
				continue
			}
			out[y] = spliceCells(out[y], x0+i*(letterW+gap), letterW, style.Render(row))
		}
	}
	_ = large
}

// centerLine left-pads s so it is centered in w columns.
func centerLine(w int, s string) string {
	d := (w - lipglossWidth(s)) / 2
	if d < 0 {
		d = 0
	}
	return strings.Repeat(" ", d) + s
}

// applyAt splices a centered/pre-positioned styled line into lines[y].
func applyAt(out []string, w, y int, line string, style lipgloss.Style, baseColor string) {
	if y < 0 || y >= len(out) {
		return
	}
	// keep background continuity: pad the whole row to w cells
	rendered := style.Render(line)
	pad := w - lipgloss.Width(line)
	if pad > 0 {
		rendered = rendered + lipgloss.NewStyle().Background(lipgloss.Color(baseColor)).Render(strings.Repeat(" ", pad))
	}
	out[y] = clipStyled(rendered, w)
}

// spliceCells replaces cells [x, x+width) in a styled line.
func spliceCells(dst string, x, width int, piece string) string {
	prefix := clipStyled(dst, x)
	if pad := x - lipgloss.Width(prefix); pad > 0 {
		prefix += strings.Repeat(" ", pad)
	}
	suffix := clipStyled(stripAnsiPrefix(dst, x+width), lipgloss.Width(dst)-x-width)
	if pad := lipgloss.Width(dst) - x - width - lipgloss.Width(suffix); pad > 0 {
		suffix += strings.Repeat(" ", pad)
	}
	return prefix + piece + suffix
}

// stripAnsiPrefix returns the cells of dst starting at cell x.
func stripAnsiPrefix(dst string, x int) string {
	// reuse ansi.Cut for a correct slice of visible cells
	return ansi.Cut(dst, x, lipgloss.Width(dst))
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

// skipSplash fast-forwards the splash into its dissolve tail: the next
// ticks play a ~350ms fade rather than an instant cut.
func (m *Model) skipSplash() {
	tail := 350 * time.Millisecond
	if m.splashTotal > tail && m.ambientT < m.splashTotal-tail {
		m.ambientT = m.splashTotal - tail
	}
}
