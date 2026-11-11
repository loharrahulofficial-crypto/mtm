package tui

// cachedStringPanel returns the memoized pane render when the key matches,
// rebuilding (and caching) otherwise. A nil cache (hand-built test models)
// always rebuilds.
func cachedStringPanel(pc *cachedPanel, key panelKey, build func() string) string {
	if pc != nil && pc.ok && pc.key == key {
		return pc.value
	}
	v := build()
	if pc != nil {
		pc.key, pc.value, pc.ok = key, v, true
	}
	return v
}

// mainSceneStamp reports the ambient stamp to mix into the main pane's
// cache key: nonzero only when Home's corner scene is live, so ambient
// ticks rebuild the main pane only then (and only the scene moves inside
// it).
func (m Model) mainSceneStamp(w, h int) int64 {
	if m.view != viewHome || !m.ambientAllowHome() {
		return 0
	}
	if _, ok := homeSceneCornerRect(m, w, h); ok {
		return m.ambientStamp
	}
	return 0
}
