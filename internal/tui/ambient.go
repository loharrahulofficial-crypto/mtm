package tui

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/caps"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/paths"
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
	switch m.cfg.UI.Ambient {
	case "home":
		return m.ambientEnabled()
	case "sidebar", "both":
		// Hidden by default while the sidebar window is on; the corner is
		// the fallback ambient surface when the window is toggled off.
		return m.ambientEnabled() && !m.cfg.UI.AmbientSidebar
	}
	return false
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
	if m.ambientIdleState() == idlePaused {
		// Idle pause wins: the scene holds a still frame until the next
		// key or mouse event (which resumes it via armAmbient).
		return false
	}
	if m.mode == modeSplash {
		return true
	}
	if m.crossfadeActive() {
		return true
	}
	if m.phaseFadeActive {
		if m.clock.Now().Sub(m.phaseFadeStart) < 1500*time.Millisecond {
			return true
		}
		// fade window is over; intensity holds at phaseFadeTo
	}
	if m.rewardUntil.After(time.Now()) {
		return true
	}
	if m.mode == modeFocus {
		// Gentle idle: a paused Pomodoro still wants frames — the scene
		// breathes slowly. An idle focus also breathes fully.
		return true
	}
	if m.mode == modeNormal && m.projectPage == "" && m.ambientSidebarWindowWanted() {
		if _, _, ok := sidebarFreeRows(m); ok {
			return true
		}
	}
	if m.mode == modeNormal && m.view == viewHome && m.ambientAllowHome() {
		if _, ok := homeSceneCornerRect(m, m.mainW(), m.height-2); ok {
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
		case h >= 5 && h < 8:
			return "mountains" // dawn: ridges in first light
		case h >= 8 && h < 15:
			return "ocean" // morning into midday: tide climbing
		case h >= 15 && h < 17:
			return "river" // late afternoon: golden-hour flow
		case h >= 17 && h < 20:
			return "rain"
		case h >= 20 && h < 23:
			return "night" // evening: moon and aurora
		default:
			return "meteor" // deep night: meteor shower
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

// reducedMotionActive honors ui.reduced_motion and NO_MOTION=1: no
// crossfades or dissolves, scenes crawl as slow low-amplitude stills.
func (m Model) reducedMotionActive() bool {
	if m.cfg.UI.ReducedMotion {
		return true
	}
	return os.Getenv("NO_MOTION") == "1"
}

// crossfadeActive reports whether a scene crossfade is running.
func (m Model) crossfadeActive() bool {
	return m.scenePrev != nil && m.clock.Now().Sub(m.crossfadeStart) < m.crossfadeDur
}

// beginCrossfade arms a scene crossfade, keeping the previous scene alive
// as the blend source. Reduced motion switches instantly.
func (m *Model) beginCrossfade() {
	if m.reducedMotionActive() {
		m.scenePrev = nil
		return
	}
	prev := m.scene
	if prev == nil && m.sceneName != "" {
		prev = ambient.New(m.sceneName, m.cfg.UI.AmbientRender)
	}
	m.scenePrev = prev
	m.crossfadeStart = m.clock.Now()
	m.crossfadeDur = 600 * time.Millisecond
}

// endCrossfadeIfDone drops the previous scene once the fade completes.
func (m *Model) endCrossfadeIfDone() {
	if m.scenePrev != nil && m.clock.Now().Sub(m.crossfadeStart) >= m.crossfadeDur {
		m.scenePrev = nil
	}
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
	if m.scene != nil && m.sceneName == m.ambientSceneName {
		// "cycle" onto the same scene rebuild is unnecessary but harmless
	}
	if m.scene != nil && m.sceneName != m.ambientSceneName {
		m.beginCrossfade()
	}
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

// sceneIntensity eases through a Pomodoro phase change (work↔break):
// palettes and particle intensity crossfade over 1.5s rather than switch.
func (m Model) sceneIntensity() float64 {
	if !m.phaseFadeActive {
		return 1
	}
	el := m.clock.Now().Sub(m.phaseFadeStart)
	const dur = 1500 * time.Millisecond
	if el >= dur {
		return m.phaseFadeTo
	}
	e := ambient.EaseInOutCubic(float64(el) / float64(dur))
	return m.phaseFadeFrom + (m.phaseFadeTo-m.phaseFadeFrom)*e
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
	pal := m.scenePalette()
	if k := 1 - m.sceneIntensity(); k > 0.004 {
		pal = ambient.FadePalette(pal, k*0.62)
	}
	trueColor, _ := caps.Effective(m.cfg.UI.TrueColor, os.Getenv)
	return ambient.SceneState{
		Progress:       progress,
		Phase:          ambient.Phase(v.Phase),
		Now:            m.clock.Now(),
		Seed:           42,
		Palette:        pal,
		Lightning:      m.cfg.UI.RainLightning,
		Dither:         m.cfg.UI.AmbientDither != "off" && !trueColor,
		PaletteLimited: !trueColor,
		ReducedMotion:  m.reducedMotionActive(),
		CellW:          m.cellW,
		CellH:          m.cellH,
	}
}

// dimScenePalette renders a copy of the scene palette blended toward Base
// so bright text stays legible over the Focus background scene. The text
// extremes (Text, BrightText) are NOT dimmed: they are the letter colors
// the splash paints through the scene raster, and dimming them caps every
// glyph at the dim palette's brightest role — letters that can no longer
// reach 3:1 against the scene. Everything else dims as before.
func (m Model) dimScenePalette() ambient.Palette {
	p := m.scenePalette()
	dim := func(c string) string { return theme.Blend(c, p.Base, 0.42) }
	return ambient.Palette{
		Base: p.Base, Surface: dim(p.Surface), SurfaceRaised: dim(p.SurfaceRaised), SurfaceStrong: dim(p.SurfaceStrong),
		Border: dim(p.Border), Text: p.Text, BrightText: p.BrightText, MutedSoft: dim(p.MutedSoft),
		Accent: dim(p.Accent), AccentSoft: dim(p.AccentSoft), Success: dim(p.Success), Warning: dim(p.Warning), Danger: dim(p.Danger),
		HeatWarm: dim(p.HeatWarm), HeatHot: dim(p.HeatHot), HeatOverdue: dim(p.HeatOverdue),
		Blue: dim(p.Blue), Cyan: dim(p.Cyan), Yellow: dim(p.Yellow), Magenta: dim(p.Magenta), Orange: dim(p.Orange), Brown: dim(p.Brown),
	}
}

// sceneFrame renders one ambient frame for the place being drawn.
func (m *Model) sceneFrame(t time.Duration, w, h int, dim bool) []string {
	if dim {
		return m.sceneFrameWith(t, w, h, func(ambient.Palette) ambient.Palette {
			return m.dimScenePalette()
		}, false)
	}
	return m.sceneFrameWith(t, w, h, nil, false)
}

// sceneFrameWith renders one frame through an optional palette adjustment
// (the Focus scrim, spec 5.3). Deterministic: same t, seed and palette,
// same frame. The frame cost is observed by View() as the end-to-end
// budget — this is just the scene share of it.
func (m *Model) sceneFrameWith(t time.Duration, w, h int, adjust func(ambient.Palette) ambient.Palette, fullFrame bool) []string {
	st := m.sceneState()
	st.FullFrame = fullFrame
	if adjust != nil {
		st.Palette = adjust(st.Palette)
	}
	cur := m.currentScene()
	blend := m.crossfadeActive()
	if !blend {
		return cur.Frame(t, w, h, st)
	}
	stOld := st
	stOld.Seed = st.Seed // outgoing scene must render with the same seed as the live scene
	oldR := m.scenePrev.FramePixels(t, w, h, stOld)
	newR := cur.FramePixels(t, w, h, st)
	prog := float64(m.clock.Now().Sub(m.crossfadeStart)) / float64(m.crossfadeDur)
	if prog >= 1 {
		m.scenePrev = nil
		return newR.EmitTheme(st, false)
	}
	e := ambient.EaseInOutCubic(prog)
	// blend in pixel space, quantize and emit once
	return ambient.BlendRasters(oldR, newR, e).EmitTheme(st, false)
}

// sceneRewardFrame renders the celebration overlay frame.
func (m *Model) sceneRewardFrame(t time.Duration, w, h int) []string {
	st := m.sceneState()
	st.Progress = 1
	st.Phase = ambient.PhaseWork
	if m.reducedMotionActive() {
		t = 900 * time.Millisecond // still frame of the burst
	}
	return m.currentScene().Reward(t, w, h, st)
}

// ambientFPS resolves the configured cap: up to 60 FPS on every surface
// (adaptive quality steps down on its own when frames blow the budget).
// The idle policy drops to a crawl after 30 s without input.
func ambientFPS(m Model) int {
	fps := m.cfg.UI.AmbientFPS
	if fps < 1 {
		fps = 60
	}
	if fps > 60 {
		fps = 60
	}
	if m.ambientIdleState() == idleSlow && fps > idleSlowFPS {
		fps = idleSlowFPS
	}
	return fps
}

// armAmbient schedules the next frame when — and only when — a surface
// wants it. The ticker stops entirely otherwise: zero work while idle.
// A paused Pomodoro in Focus mode crawls instead: the scene breathes at
// a gentle rate, progress-linked elements stay frozen.
func (m *Model) armAmbient() tea.Cmd {
	if !m.ambientActive() {
		m.ambientArmed = false
		return nil
	}
	if m.ambientArmed {
		return nil
	}
	m.ambientArmed = true
	fps := ambientFPS(*m)
	if m.slowIdleActive() {
		return ambientTickCmd(slowIdleFPS(fps))
	}
	return ambientTickCmd(fps)
}

// slowIdleActive reports whether the scene is in gentle idle: a paused
// Pomodoro shown full-screen in Focus mode.
func (m Model) slowIdleActive() bool {
	return m.mode == modeFocus && m.focusSceneFrozen()
}

// slowIdleFPS is the gentle-idle frame rate (quarter rate, clamped).
func slowIdleFPS(full int) int {
	f := full / 4
	if f < 2 {
		f = 2
	}
	if f > 8 {
		f = 8
	}
	return f
}

func ambientTickCmd(fps int) tea.Cmd {
	if tier, _ := ambient.AmbientTier(); tier >= int(ambient.TierLowFPS) {
		// Adaptive quality stepped down to the low-FPS tier: halve the rate.
		fps /= 2
		if fps < 4 {
			fps = 4
		}
	}
	d := time.Second / time.Duration(fps)
	return tea.Tick(d, func(time.Time) tea.Msg { return ambientTickMsg{} })
}

func splashTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return splashTickMsg{} })
}

// handleAmbientTick advances the scene clock and keeps the ticker alive
// only while a surface wants frames. In gentle idle the clock crawls at
// a quarter rate so scenes breathe without advancing progress-linked elements.
func (m Model) handleAmbientTick() (tea.Model, tea.Cmd) {
	now := m.clock.Now()
	if m.rewardArmed && !time.Now().Before(m.rewardUntil) {
		m.rewardArmed = false
		m.ambientStamp++ // one final frame drops the reward panel
	}
	if !m.ambientActive() {
		m.ambientArmed = false
		return m, nil
	}
	m.ambientStamp++
	fps := ambientFPS(m)
	slow := m.slowIdleActive()
	if slow {
		fps = slowIdleFPS(fps)
	}
	d := time.Second / time.Duration(fps)
	var elapsed time.Duration
	if m.lastAmbientTick.IsZero() {
		elapsed = d
	} else {
		elapsed = now.Sub(m.lastAmbientTick)
	}
	if slow {
		m.ambientT += elapsed / 4
	} else {
		m.ambientT += elapsed
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

// splashStampPath is where the once-a-day stamp lives.
func splashStampPath() string { return paths.StateFile("splash.stamp") }

// splashShownToday reports whether the splash was already shown on the
// local day of t (the stamp may be missing or stale — either shows it).
func splashShownToday(t time.Time) bool {
	data, err := os.ReadFile(splashStampPath())
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == t.Format("2006-01-02")
}

// markSplashShown records that the splash was shown now (local date).
// Errors are swallowed — a failed stamp must not block startup.
func markSplashShown(t time.Time) {
	p := splashStampPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(t.Format("2006-01-02")+"\n"), 0o644)
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
	switch m.cfg.UI.Splash {
	case "off":
		return false
	case "once-a-day":
		if splashShownToday(m.clock.Now()) {
			return false
		}
	}
	m.mode = modeSplash
	m.splashStart = m.clock.Now()
	m.splashTotal = time.Duration(m.cfg.UI.SplashSeconds * float64(time.Second))
	m.ambientT = 0
	m.ambientArmed = false
	if m.cfg.UI.Splash == "once-a-day" {
		markSplashShown(m.clock.Now())
	}
	return true
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
	reduced := m.reducedMotionActive()
	if reduced {
		t = total // static still: the full composition from the first frame
	}
	sceneT := time.Duration(math.Max(0, t-0.25) * float64(time.Second))
	if reduced {
		sceneT = 0 // the scene is a still; idle breathing is the only motion
	}

	// scene elements drift in behind the fade (head fade is a palette
	// ramp, not a post-process; overlays composite afterwards)
	headK := 1.0
	if !reduced && t < 0.55 {
		headK = ambient.EaseOutCubic(t / 0.55)
	}
	sextant := ambient.ResolveMode(m.cfg.UI.AmbientRender) == ambient.ModeSextant
	// wordmark geometry first — the scrim and the overlay layout share it.
	// Bitmap pixels are square on screen (1 cell wide, cellW/cellH rows
	// tall), so the block is shorter than its bitmap height in rows.
	lay := splashLayoutFor(w, h, m.cellW, m.cellH, m.cfg.UI.Wordmark == "lower")
	wm := lay.wm
	pixelH := lay.pixelH
	wmRows := lay.rows
	wmTop := lay.top
	wordW := lay.wordW
	wmX := lay.x0
	tagY := lay.tagY
	const taglineText = "a calmer desk for your courses"
	const hintText = "press any key to skip"
	dissolveProg := 0.0
	if !reduced && t > total-0.45 {
		dissolveProg = clampUnit((t-(total-0.45))/0.45, 0, 1)
	}
	fade := 1 - ambient.EaseInOutCubic(dissolveProg)
	scrims := []ambient.Scrim{
		// one very soft local darkening: gaussian falloff, at most 18% at
		// the center, radius ≥ 1.6x the wordmark width — no visible edge,
		// no plate.
		{CX: float64(wmX) + float64(wordW)/2, CY: wmTop + wmRows/2, RadiusX: 1.6 * float64(wordW), RadiusY: wmRows * 2.0, Strength: 0.18},
	}
	var r ambient.Raster
	var baseHex string
	var limited, dither bool
	var out []string
	// one shared scene state for both paths (wordmark + scrim included)
	st := m.sceneState()
	st.Palette = m.dimScenePalette()
	if headK < 1 {
		st.Palette = ambient.FadePalette(st.Palette, 1-headK)
	}
	st.Scrims = scrims
	st.Wordmark = m.wordmarkRects(wm, wmX, wmTop, pixelH, t, fade)
	st.FullFrame = true
	baseHex = st.Palette.Base
	limited, dither = st.PaletteLimited, st.Dither
	if sextant {
		// Sextant path: the scene emits 2×3 mosaics natively (Frame is
		// geometry-aware). The raster-space dissolve cannot run — the
		// raster is 3 subrows tall — so the tail eases colors to base
		// (glyphs stay, colors vanish). Overlays sample the decoded
		// scene (see cellSampler), never a flat backing.
		out = m.currentScene().Frame(sceneT, w, h, st)
		if !reduced && t > total-0.45 {
			out = fadeToBaseLines(out, baseHex, ambient.EaseInOutCubic(dissolveProg))
		}
	} else {
		r = m.currentScene().FramePixels(sceneT, w, h, st)
		if !reduced && t > total-0.45 {
			r = ambient.DissolveRaster(r, ambient.EaseInOutCubic(dissolveProg), baseHex)
		}
		out = r.EmitHalfblock(baseHex, true, limited, dither)
	}
	// overlays fade with the dissolve tail: 1 = fully lit, 0 = base

	var sampler cellSampler
	if sextant {
		sampler = cellSampler{r: ambient.RasterFromLines(out, baseHex), base: baseHex, sx: 2, sy: 3}
	} else {
		sampler = cellSampler{r: r, base: baseHex, sx: 1, sy: 2}
	}

	// tagline fades in 1.6–2.2s, one row below the wordmark block; its
	// color keeps ≥ 4.5:1 against every cell of the scene span it sits on
	taglineT := clampUnit((t-1.6)/0.6, 0, 1) * fade
	if taglineT > 0 && tagY < h {
		x0 := centerX(w, taglineText)
		blend := ambient.BlendHex(baseHex, m.palette.BrightText, taglineT)
		piece := overlayCells(x0, tagY, taglineText, blend, false, sampler, limited, dither)
		out[tagY] = spliceCells(out[tagY], x0, fastWidth(taglineText), piece)
	}

	// footer hint fades in early, stays; same contrast contract, no pill
	footT := clampUnit((t-0.35)/0.4, 0, 1) * fade
	if footT > 0 && h >= 2 {
		x0 := centerX(w, hintText)
		blend := ambient.BlendHex(baseHex, m.palette.MutedSoft, footT)
		piece := overlayCells(x0, h-2, hintText, blend, false, sampler, limited, dither)
		out[h-2] = spliceCells(out[h-2], x0, fastWidth(hintText), piece)
	}

	return strings.Join(out, "\n")
}

// cellSampler resolves the scene's own per-cell background for overlay
// cells. Halfblock frames sample the bottom subcell of the cell (the bg
// SGR the pairOnly emission carries); sextant frames are decoded to a
// raster and sampled at the bottom dot-row of the cell. Every sampled
// color is the scene's own painting with the radial scrim applied —
// never a flat backing plate.
type cellSampler struct {
	r      ambient.Raster
	base   string
	sx, sy int // subcells per cell: 1x2 halfblock, 2x3 sextant
}

func (s cellSampler) bg(x, y int) string {
	px := x * s.sx
	py := y*s.sy + s.sy - 1
	if x < 0 || px >= s.r.W || py < 0 || py >= s.r.H {
		return s.base
	}
	rr, gg, bb := s.r.Pixel(px, py)
	return fmt.Sprintf("#%02x%02x%02x", rr, gg, bb)
}

// overlayCells renders one row of text cells: each cell carries the
// scene's own background at that position (the scene shows through
// between glyph strokes and inside counters) with fg = fgHex. Per cell,
// if fgHex cannot keep 4.5:1 against that cell's sampled background,
// the better of white/black is used instead — the span may sit over
// both dark sky and a bright cloud, and no single foreground clears
// 4.5:1 against both. bold adds weight. On truecolor (limited=false)
// raw 24-bit SGR is emitted; on limited palettes every fg/bg goes
// through the xterm-256 quantizer so the frame never contains an
// escape the terminal would drop.
func overlayCells(x0, y int, text, fgHex string, bold bool, sampler cellSampler, limited, dither bool) string {
	runes := []rune(text)
	var sb strings.Builder
	curFG, curBG := "", ""
	emit := func(sgr string) {
		sb.WriteString("\x1b[")
		sb.WriteString(sgr)
		sb.WriteString("m")
	}
	for i, ch := range runes {
		bg := sampler.bg(x0+i, y)
		fgHex := overlayContrast(fgHex, bg, 4.5)
		fgSGR := overlayColorSGR(true, fgHex, bold, x0+i, y, limited, dither)
		bgSGR := overlayColorSGR(false, bg, false, x0+i, y, limited, dither)
		if fgSGR != curFG {
			emit(fgSGR)
			curFG = fgSGR
		}
		if bgSGR != curBG {
			emit(bgSGR)
			curBG = bgSGR
		}
		sb.WriteRune(ch)
	}
	sb.WriteString("\x1b[0m")
	return sb.String()
}

// overlayContrast returns the best foreground for a cell: seed when it
// keeps min contrast against the sampled background, else the better of
// white and black.
func overlayContrast(seed, bg string, min float64) string {
	if theme.ContrastRatio(seed, bg) >= min {
		return seed
	}
	if theme.ContrastRatio("#ffffff", bg) >= theme.ContrastRatio("#000000", bg) {
		return "#ffffff"
	}
	return "#000000"
}

// overlayColorSGR builds the SGR payload for one overlay color.
func overlayColorSGR(fg bool, hex string, bold bool, x, y int, limited, dither bool) string {
	r, g, b := hexToRGB(hex)
	if limited {
		idx := ambient.Xterm256For(r, g, b, x, y, dither)
		prefix := "48;5;"
		if fg {
			prefix = "38;5;"
		}
		if fg && bold {
			return fmt.Sprintf("1;%s%d", prefix, idx)
		}
		return fmt.Sprintf("%s%d", prefix, idx)
	}
	if fg && bold {
		return "1;38;2;" + rgbTriplet(hex)
	}
	if fg {
		return "38;2;" + rgbTriplet(hex)
	}
	return "48;2;" + rgbTriplet(hex)
}

// hexToRGB parses #rrggbb into bytes (defaults 255 on malformed input).
func hexToRGB(hex string) (uint8, uint8, uint8) {
	s := strings.TrimPrefix(hex, "#")
	if len(s) != 6 {
		return 255, 255, 255
	}
	r, g, b := uint8(255), uint8(255), uint8(255)
	if _, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b); err != nil {
		return 255, 255, 255
	}
	return r, g, b
}

// rgbTriplet converts #rrggbb to "r;g;b" (clamped, defaults white).
func rgbTriplet(hex string) string {
	s := strings.TrimPrefix(hex, "#")
	if len(s) != 6 {
		return "255;255;255"
	}
	r, g, b := 0, 0, 0
	if _, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b); err != nil {
		return "255;255;255"
	}
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}

// centerX returns the cell x that centers s in w.
func centerX(w int, s string) int {
	d := (w - fastWidth(s)) / 2
	if d < 0 {
		d = 0
	}
	return d
}

// spliceCells replaces cells [x, x+width) in a styled line.
func spliceCells(dst string, x, width int, piece string) string {
	prefix := clipStyled(dst, x)
	if pad := x - fastWidth(prefix); pad > 0 {
		prefix += strings.Repeat(" ", pad)
	}
	suffix := clipStyled(stripAnsiPrefix(dst, x+width), fastWidth(dst)-x-width)
	if pad := fastWidth(dst) - x - width - fastWidth(suffix); pad > 0 {
		suffix += strings.Repeat(" ", pad)
	}
	return prefix + piece + suffix
}

// stripAnsiPrefix returns the cells of dst starting at cell x.
func stripAnsiPrefix(dst string, x int) string {
	// reuse ansi.Cut for a correct slice of visible cells
	return ansi.Cut(dst, x, fastWidth(dst))
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
	fillN := rw - 4 - fastWidth(head)
	if fillN < 0 {
		fillN = 0
	}
	top := edge.Render("╭─ " + head + " " + strings.Repeat("─", fillN) + "╮")
	bot := edge.Render("╰" + strings.Repeat("─", rw-2) + "╯")
	right := edge.Render("│")
	inner := m.sceneRewardFrame(m.ambientT, rw-2, rh-2)
	lines := []string{top}
	for _, l := range inner {
		if w := fastWidth(l); w > rw-2 {
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
	m.rewardArmed = true
	// Never start a second chain: if ambient is already ticking, return nil
	// and let the existing chain renew (handleAmbientTick preserves it).
	return m.armAmbient()
}

// skipSplash fast-forwards the splash into its dissolve tail: the next
// ticks play a ~350ms fade rather than an instant cut.
func (m *Model) skipSplash() {
	tail := 350 * time.Millisecond
	if m.splashTotal > tail && m.ambientT < m.splashTotal-tail {
		m.ambientT = m.splashTotal - tail
	}
}
