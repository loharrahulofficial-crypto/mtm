package ambient

import (
	"testing"
	"time"
)

// quality_test.go — §4.7: adaptive tiers step down over budget and restore
// with headroom, and fSS tracks the tier.

func TestAdaptiveStepsDownAndRestores(t *testing.T) {
	resetAdaptiveForTest()
	defer resetAdaptiveForTest()

	// 3 frames over budget → TierLowSS
	for i := 0; i < 3; i++ {
		ObserveFrameCost(40 * time.Millisecond)
	}
	if tier, name := AmbientTier(); tier != 1 || name != "low-ss" {
		t.Fatalf("after 3 over-budget frames: tier = (%d, %q), want (1, low-ss)", tier, name)
	}
	if fSS != 1 {
		t.Fatalf("fSS = %d, want 1 at low-ss", fSS)
	}

	// 3 more → TierLowFPS
	for i := 0; i < 3; i++ {
		ObserveFrameCost(40 * time.Millisecond)
	}
	if tier, name := AmbientTier(); tier != 2 || name != "low-fps" {
		t.Fatalf("after 6 over-budget frames: tier = (%d, %q), want (2, low-fps)", tier, name)
	}

	// headroom for a long stretch restores one step at a time
	for i := 0; i < 45; i++ {
		ObserveFrameCost(5 * time.Millisecond)
	}
	if tier, _ := AmbientTier(); tier != 1 {
		t.Fatalf("after 45 good frames: tier = %d, want 1", tier)
	}
	if fSS != 1 {
		t.Fatalf("fSS = %d, want 1 at low-ss", fSS)
	}
	for i := 0; i < 45; i++ {
		ObserveFrameCost(5 * time.Millisecond)
	}
	if tier, _ := AmbientTier(); tier != 0 {
		t.Fatalf("after 90 good frames: tier = %d, want 0", tier)
	}
	if fSS != 2 {
		t.Fatalf("fSS = %d, want 2 at full", fSS)
	}
}

func TestAdaptiveMidBudgetDoesNotStep(t *testing.T) {
	resetAdaptiveForTest()
	defer resetAdaptiveForTest()
	for i := 0; i < 10; i++ {
		ObserveFrameCost(FrameBudget * 3 / 4) // neither over nor headroom
	}
	if tier, _ := AmbientTier(); tier != 0 {
		t.Fatalf("mid-budget frames stepped the tier to %d", tier)
	}
}

func TestSetQualityTierForces(t *testing.T) {
	resetAdaptiveForTest()
	defer resetAdaptiveForTest()
	SetQualityTier(TierLowSS)
	if fSS != 1 {
		t.Fatalf("fSS = %d after SetQualityTier(LowSS), want 1", fSS)
	}
	SetQualityTier(TierFull)
	if fSS != 2 {
		t.Fatalf("fSS = %d after SetQualityTier(Full), want 2", fSS)
	}
}
