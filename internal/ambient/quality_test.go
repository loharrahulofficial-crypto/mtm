package ambient

import (
	"testing"
	"time"
)

// quality_test.go — §4.7: adaptive tiers step down over budget and restore
// with headroom, and fSSV() tracks the tier.

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
	if fSSV() != 2 {
		t.Fatalf("fSSV() = %d, want 2 at low-ss", fSSV())
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
	if fSSV() != 2 {
		t.Fatalf("fSSV() = %d, want 2 at low-ss", fSSV())
	}
	for i := 0; i < 45; i++ {
		ObserveFrameCost(5 * time.Millisecond)
	}
	if tier, _ := AmbientTier(); tier != 0 {
		t.Fatalf("after 90 good frames: tier = %d, want 0", tier)
	}
	if fSSV() != 3 {
		t.Fatalf("fSSV() = %d, want 3 at full", fSSV())
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
	if fSSV() != 2 {
		t.Fatalf("fSSV() = %d after SetQualityTier(LowSS), want 2", fSSV())
	}
	SetQualityTier(TierFull)
	if fSSV() != 3 {
		t.Fatalf("fSSV() = %d after SetQualityTier(Full), want 3", fSSV())
	}
}
