// Package ambient implements the Phase 4b-B ambient scenes: seeded,
// deterministic animation frames composited out of small independent
// particle systems and noise layers, rendered into the TUI's panel grid.
//
// Determinism is total: a frame is a pure function of (seed, t, size,
// state). Nothing reads the wall clock, no goroutines, no globals. Tests
// pin seeds, times, sizes and states to reproduce frames exactly.
package ambient

import (
	"time"
)

// Line is one rendered line of the frame (ANSI-styled).
type Line = string

// Phase mirrors the timer engine's phases (core.TimerPhase*). Ambient
// takes the string so it never imports internal/core.
type Phase string

const (
	PhaseWork  = "work"
	PhaseBreak = "break"
	PhaseIdle  = "idle"
)

// SceneState is everything ambient knows about the surrounding world.
// The palette rides in state so a theme switch re-renders live.
type SceneState struct {
	Progress  float64 // 0..1 — pomodoro progress (tide/moon/rain/tree growth)
	Phase     Phase   // work | break | idle
	Now       time.Time
	Seed      int64 // per-launch or per-test seed; 0 = fixed default
	Palette   Palette
	Lightning bool // rain scene option; off by default
	Dither    bool // ordered dithering between the two nearest xterm-256 entries
	// PaletteLimited: emit 38;5;N via nearest xterm-256 (for terminals
	// without truecolor). Never emits 24-bit color when set.
	PaletteLimited bool
	// ReducedMotion: slow crawl, low-amplitude stills (idle breathing only).
	ReducedMotion bool
	// Scrims: soft radial darkenings painted after the scene (nil for
	// ordinary frames). The splash sets one behind the wordmark, one
	// behind the tagline and one behind the hint so the overlaid
	// text reads on the scene, not on a plate. Coordinates are in
	// terminal cells.
	Scrims []Scrim
	// Wordmark paints the splash letters through the raster (nil
	// everywhere else) — see wordmark.go. Cell coords; letters inherit
	// the aspect-corrected geometry like every other shape.
	Wordmark *WordmarkPaint
	// FullFrame: the scene fills the whole terminal (splash, Focus,
	// full-screen dumps and previews). The horizon lands at 62-70% of
	// the height so the lower area can carry intentional content;
	// embedded windows (sidebar, Home corner, settings preview) use the
	// aspect-adaptive horizon instead.
	FullFrame bool
	// CellW/CellH are the real terminal cell size in pixels. When zero,
	// the 9:17 fallback is used. Primitives render in aspect-corrected
	// scene space so shapes are correct on screen.
	CellW float64
	CellH float64
}

// subpixelAspect returns the subpixel height/width ratio for mode.
func (s SceneState) subpixelAspect(mode string) float64 {
	return SubpixelAspect(mode, CellPixels{W: s.CellW, H: s.CellH})
}

// Palette carries the resolved theme roles the scenes paint with.
// Colors are hex strings; scenes never invent colors — every painted
// color is a theme role or a theme.Blend toward the base background.
type Palette struct {
	Base          string
	Surface       string
	SurfaceRaised string
	SurfaceStrong string
	Border        string
	Text          string
	BrightText    string
	MutedSoft     string
	Accent        string
	AccentSoft    string
	Success       string
	Warning       string
	Danger        string
	HeatWarm      string
	HeatHot       string
	HeatOverdue   string
	Blue          string
	Cyan          string
	Yellow        string
	Magenta       string
	Orange        string
	Brown         string
}

// Scene renders ambient animation frames.
type Scene interface {
	Name() string
	// Frame renders the scene at animation time t. t is arbitrary and
	// grows monotonically; wrapping handles large t safely.
	Frame(t time.Duration, w, h int, state SceneState) []Line
	// Reward renders the 1.5s celebration at its current t (0..1.5s).
	Reward(t time.Duration, w, h int, state SceneState) []Line
	// FramePixels returns the downsampled pixel raster of the frame
	// (w × 2h pixels for halfblock), clamped to the theme hull but not
	// yet dithered or quantized. Transitions blend these rasters and
	// emit once.
	FramePixels(t time.Duration, w, h int, state SceneState) Raster
}

// Render modes select the output raster.
const (
	ModeAuto      = "auto"
	ModeHalfblock = "halfblock"
	ModeBraille   = "braille"
	ModeASCII     = "ascii"
	ModeQuadrant  = "quadrant"
	ModeSextant   = "sextant"
)

func ResolveMode(m string) string {
	switch m {
	case ModeHalfblock, ModeBraille, ModeASCII, ModeQuadrant, ModeSextant:
		return m
	default:
		return ModeHalfblock // auto → halfblock when we can't probe the terminal
	}
}

// splitmix64 is the seeded generator scenes use. It is small, fast, and
// identical on every platform, which the goldens rely on.
type rng struct{ state uint64 }

func newRNG(seed int64, stream uint64) *rng {
	return &rng{state: uint64(seed)*0x9E3779B97F4A7C15 + stream*0xC2B2AE3D27D4EB4F + 0x165667B19E3779F9}
}

func (r *rng) next() uint64 {
	r.state += 0x9E3779B97F4A7C15
	z := r.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func (r *rng) float64() float64 { return float64(r.next()>>11) * (1.0 / (1 << 53)) }

func (r *rng) rangeF(lo, hi float64) float64 { return lo + r.float64()*(hi-lo) }

func (r *rng) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.next() % uint64(n))
}

func wrapT(t time.Duration, period time.Duration) time.Duration {
	if t < 0 {
		return 0
	}
	return t % period
}

func clampf(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
