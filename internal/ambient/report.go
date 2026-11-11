package ambient

import "time"

// report.go — bytes-per-frame reporting (§2.5). Renders one deterministic
// frame per scene (fixed clock, fixed size, default theme) and reports the
// SGR byte count — the size of what a terminal would read for one frame.

// ReportFrameCost renders one deterministic frame per registered scene
// (same frame ReportBytes sizes) and returns the measured wall cost per
// scene, in nanoseconds. Doctor uses it to report measured paint time
// without a live TUI.
func ReportFrameCost(mode string, w, h int) map[string]int64 {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	mode = resolveMode(mode)
	out := make(map[string]int64, len(Registered()))
	st := SceneState{
		Progress: 0.5,
		Phase:    PhaseWork,
		Now:      time.Date(2026, 11, 7, 21, 30, 0, 0, time.Local),
		Seed:     42,
		Palette:  DefaultPalette(),
	}
	for _, name := range Registered() {
		s := New(name, mode)
		start := time.Now()
		_ = s.Frame(3*time.Second, w, h, st)
		out[name] = time.Since(start).Nanoseconds()
	}
	return out
}

// ReportBytes renders one frame per registered scene at w×h in the given
// mode and returns the byte counts keyed by scene name.
func ReportBytes(mode string, w, h int) map[string]int {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	mode = resolveMode(mode)
	out := make(map[string]int, len(Registered()))
	st := SceneState{
		Progress: 0.5,
		Phase:    PhaseWork,
		Now:      time.Date(2026, 11, 7, 21, 30, 0, 0, time.Local),
		Seed:     42,
		Palette:  DefaultPalette(),
	}
	for _, name := range Registered() {
		p, ok := New(name, mode).(paintable)
		if !ok {
			continue
		}
		b := newFrameBuffer(p, mode, 3*time.Second, w, h, st, false)
		b.render(b.palette, w, h)
		out[name] = b.bytesOut()
	}
	return out
}
