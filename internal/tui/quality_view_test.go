package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
)

// Item 4: the adaptive quality tiers trigger on the end-to-end frame
// cost (full View() assembly, not just the scene raster). With the
// budget shrunk to nothing, consecutive ambient rebuilds step the tier
// down exactly as if the machine were slow.
func TestQualityTiersTriggerOnEndToEndViewCost(t *testing.T) {
	m, s := newTestModel(t, nil)
	defer func() { _ = s.Close() }()
	m.cfg.UI.Ambient = "both"
	m.cfg.UI.AmbientSidebar = true
	m.cfg.UI.Scene = "night"

	orig := ambient.FrameBudget
	ambient.FrameBudget = time.Nanosecond // every real build is "over budget"
	defer func() { ambient.FrameBudget = orig }()
	ambient.SetQualityTier(ambient.TierFull)
	defer ambient.SetQualityTier(ambient.TierFull)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	tier0, _ := ambient.AmbientTier()
	if tier0 != 0 {
		t.Fatalf("starting tier = %d, want full", tier0)
	}
	for i := 1; i <= 3; i++ {
		v := m
		v.ambientStamp += int64(i) // force a real rebuild, as an ambient tick does
		_ = v.View()
	}
	tier, _ := ambient.AmbientTier()
	if tier == 0 {
		t.Fatal("tier stayed FULL after 3 consecutive over-budget frames — end-to-end cost not observed")
	}
}
