package node

import (
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/scheduler"
)

// makeCandidates returns a slice of PathCandidates with the given IDs and
// admin weights. Quality is left zero (unmeasured) so ScoreCandidate uses
// only AdminWeight. The score formula yields score ≈ AdminWeight-based value,
// which is sufficient for stickiness threshold comparisons in these tests.
func makeCandidates(idWeights ...interface{}) []scheduler.PathCandidate {
	var result []scheduler.PathCandidate
	for i := 0; i < len(idWeights)-1; i += 2 {
		id := idWeights[i].(string)
		weight := idWeights[i+1].(int)
		result = append(result, scheduler.PathCandidate{
			ID:          id,
			Class:       scheduler.PathClassDirectPublic,
			AdminWeight: uint8(weight),
			Health:      scheduler.HealthStateActive,
		})
	}
	return result
}

// TestFirstSelectionPassesAll verifies that when no current path is set,
// AdjustCandidates returns all candidates unchanged.
func TestFirstSelectionPassesAll(t *testing.T) {
	p := NewMultiWANStickinessPolicy(DefaultMultiWANStickinessConfig())
	candidates := makeCandidates("pathA", 80, "pathB", 70)

	out, eval := p.AdjustCandidates(candidates)
	if len(out) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(out))
	}
	if eval.SwitchSuppressed {
		t.Error("expected SwitchSuppressed=false on first selection")
	}
	if eval.CurrentCandidateID != "" {
		t.Errorf("expected empty CurrentCandidateID, got %q", eval.CurrentCandidateID)
	}
}

// TestTrivialImprovementSuppressed verifies that a small score advantage
// (at or below the threshold) does not trigger a switch.
func TestTrivialImprovementSuppressed(t *testing.T) {
	cfg := MultiWANStickinessConfig{
		StickinessThreshold: 3,
		HoldDownDuration:    30 * time.Second,
	}
	p := NewMultiWANStickinessPolicy(cfg)

	// Establish pathA as current by recording a first selection.
	candidates := makeCandidates("pathA", 80)
	out, _ := p.AdjustCandidates(candidates)
	if len(out) != 1 {
		t.Fatalf("setup: expected 1 candidate, got %d", len(out))
	}
	p.RecordSelection("pathA")

	// Now present pathA (score ~80) and pathB (score ~82 — improvement of 2,
	// which is ≤ threshold=3). Expect suppression.
	//
	// ScoreCandidate uses AdminWeight as base score (capped to 100).
	// With no quality penalties, score ≈ AdminWeight. To get a 2-point
	// improvement, use pathA=80, pathB=82.
	candidates2 := makeCandidates("pathA", 80, "pathB", 82)
	out2, eval := p.AdjustCandidates(candidates2)
	if !eval.SwitchSuppressed {
		t.Errorf("expected SwitchSuppressed=true; reason: %s", eval.Reason)
	}
	if len(out2) != 1 || out2[0].ID != "pathA" {
		t.Errorf("expected only pathA in filtered list, got %v", out2)
	}
}

// TestClearImprovementAllowed verifies that a score advantage strictly above
// the threshold allows a switch.
func TestClearImprovementAllowed(t *testing.T) {
	cfg := MultiWANStickinessConfig{
		StickinessThreshold: 3,
		HoldDownDuration:    30 * time.Second,
	}
	p := NewMultiWANStickinessPolicy(cfg)

	// Establish pathA as current.
	candidates := makeCandidates("pathA", 70)
	p.AdjustCandidates(candidates)
	p.RecordSelection("pathA")

	// pathB scores 75 — improvement of 5 > threshold=3. Switch allowed.
	candidates2 := makeCandidates("pathA", 70, "pathB", 75)
	out, eval := p.AdjustCandidates(candidates2)
	if eval.SwitchSuppressed {
		t.Errorf("expected SwitchSuppressed=false; reason: %s", eval.Reason)
	}
	if len(out) != 2 {
		t.Errorf("expected both candidates passed through, got %d", len(out))
	}
}

// TestHoldDownSuppressesSwitch verifies that after a switch, subsequent
// evaluations are suppressed for the hold-down duration regardless of quality.
func TestHoldDownSuppressesSwitch(t *testing.T) {
	now := time.Now()
	fakeClock := func() time.Time { return now }

	cfg := MultiWANStickinessConfig{
		StickinessThreshold: 3,
		HoldDownDuration:    30 * time.Second,
	}
	p := NewMultiWANStickinessPolicy(cfg)
	p.now = fakeClock

	// First selection: pathA.
	p.AdjustCandidates(makeCandidates("pathA", 70))
	p.RecordSelection("pathA")

	// Advance time to allow the first switch (no hold-down yet).
	// pathB scores much better: improvement >> threshold.
	p.AdjustCandidates(makeCandidates("pathA", 70, "pathB", 90))
	p.RecordSelection("pathB") // switch occurs; hold-down starts at `now`

	// Immediately after the switch, present pathC (extremely high score).
	// Hold-down should suppress any switch.
	candidates3 := makeCandidates("pathB", 70, "pathC", 100)
	out, eval := p.AdjustCandidates(candidates3)
	if !eval.HoldDownActive {
		t.Error("expected HoldDownActive=true immediately after switch")
	}
	if !eval.SwitchSuppressed {
		t.Errorf("expected SwitchSuppressed=true during hold-down; reason: %s", eval.Reason)
	}
	if len(out) != 1 || out[0].ID != "pathB" {
		t.Errorf("expected only pathB during hold-down, got %v", out)
	}
}

// TestHoldDownExpiryAllowsSwitch verifies that after hold-down expires,
// threshold-based switching resumes normally.
func TestHoldDownExpiryAllowsSwitch(t *testing.T) {
	now := time.Now()
	fakeClock := func() time.Time { return now }

	cfg := MultiWANStickinessConfig{
		StickinessThreshold: 3,
		HoldDownDuration:    30 * time.Second,
	}
	p := NewMultiWANStickinessPolicy(cfg)
	p.now = fakeClock

	// Set up: pathA current.
	p.AdjustCandidates(makeCandidates("pathA", 70))
	p.RecordSelection("pathA")

	// Switch to pathB; hold-down starts.
	p.AdjustCandidates(makeCandidates("pathA", 70, "pathB", 90))
	switchTime := now
	_ = switchTime
	p.RecordSelection("pathB")

	// Advance time past hold-down.
	now = now.Add(31 * time.Second)

	// pathC scores 5 points above pathB — above threshold.
	candidates := makeCandidates("pathB", 70, "pathC", 76)
	out, eval := p.AdjustCandidates(candidates)
	if eval.HoldDownActive {
		t.Error("expected HoldDownActive=false after hold-down expiry")
	}
	if eval.SwitchSuppressed {
		t.Errorf("expected SwitchSuppressed=false after hold-down expiry; reason: %s", eval.Reason)
	}
	if len(out) != 2 {
		t.Errorf("expected both candidates after hold-down expiry, got %d", len(out))
	}
}

// TestCurrentPathDisappearsAllowsSwitch verifies that when the current path
// is absent from the candidate list, switching is allowed unconditionally.
func TestCurrentPathDisappearsAllowsSwitch(t *testing.T) {
	p := NewMultiWANStickinessPolicy(DefaultMultiWANStickinessConfig())

	// Establish pathA as current.
	p.AdjustCandidates(makeCandidates("pathA", 80))
	p.RecordSelection("pathA")

	// pathA is no longer in the list (excluded by refinement/fallback).
	candidates := makeCandidates("pathB", 60)
	out, eval := p.AdjustCandidates(candidates)
	if eval.SwitchSuppressed {
		t.Errorf("expected SwitchSuppressed=false when current path absent; reason: %s", eval.Reason)
	}
	if len(out) != 1 || out[0].ID != "pathB" {
		t.Errorf("expected pathB passed through, got %v", out)
	}
}

// TestRecordSelectionDetectsSwitch verifies that RecordSelection sets
// SwitchOccurred=true when a different path is chosen.
func TestRecordSelectionDetectsSwitch(t *testing.T) {
	p := NewMultiWANStickinessPolicy(DefaultMultiWANStickinessConfig())

	// First selection: no switch.
	eval1 := p.RecordSelection("pathA")
	if eval1.SwitchOccurred {
		t.Error("expected SwitchOccurred=false on first selection")
	}

	// Same path: no switch.
	eval2 := p.RecordSelection("pathA")
	if eval2.SwitchOccurred {
		t.Error("expected SwitchOccurred=false when same path selected again")
	}

	// Different path: switch.
	eval3 := p.RecordSelection("pathB")
	if !eval3.SwitchOccurred {
		t.Error("expected SwitchOccurred=true when different path selected")
	}
}

// TestPerAssociationIsolation verifies that stickiness state for one
// association does not affect another.
func TestPerAssociationIsolation(t *testing.T) {
	store := NewAssociationStickinessStore(DefaultMultiWANStickinessConfig())

	// Association A: establish pathA as current.
	store.AdjustCandidates("assocA", makeCandidates("pathA", 80))
	store.RecordSelection("assocA", "pathA")

	// Association B: first selection, no current path.
	_, evalB := store.AdjustCandidates("assocB", makeCandidates("pathX", 70, "pathY", 80))
	if evalB.SwitchSuppressed {
		t.Errorf("assocB first selection should not suppress; reason: %s", evalB.Reason)
	}

	// Association A: trivial improvement should be suppressed.
	_, evalA := store.AdjustCandidates("assocA", makeCandidates("pathA", 80, "pathB", 82))
	if !evalA.SwitchSuppressed {
		t.Errorf("assocA trivial improvement should be suppressed; reason: %s", evalA.Reason)
	}
}

// TestThresholdZeroDisablesCheck verifies that StickinessThreshold=0 passes
// all candidates through without any quality comparison.
func TestThresholdZeroDisablesCheck(t *testing.T) {
	cfg := MultiWANStickinessConfig{
		StickinessThreshold: 0,
		HoldDownDuration:    30 * time.Second,
	}
	p := NewMultiWANStickinessPolicy(cfg)

	// Establish pathA as current.
	p.AdjustCandidates(makeCandidates("pathA", 80))
	p.RecordSelection("pathA")

	// Even a trivial improvement should pass through (threshold disabled).
	candidates := makeCandidates("pathA", 80, "pathB", 81)
	out, eval := p.AdjustCandidates(candidates)
	if eval.SwitchSuppressed {
		t.Errorf("expected SwitchSuppressed=false when threshold=0; reason: %s", eval.Reason)
	}
	if len(out) != 2 {
		t.Errorf("expected both candidates with threshold=0, got %d", len(out))
	}
}

// TestAssociationStickinessStoreRemove verifies that Remove clears per-association
// state so the next interaction starts fresh (first-selection epoch).
func TestAssociationStickinessStoreRemove(t *testing.T) {
	store := NewAssociationStickinessStore(DefaultMultiWANStickinessConfig())

	store.AdjustCandidates("assocA", makeCandidates("pathA", 80))
	store.RecordSelection("assocA", "pathA")

	// Remove assocA's state.
	store.Remove("assocA")

	// Next AdjustCandidates should treat it as a first selection.
	_, eval := store.AdjustCandidates("assocA", makeCandidates("pathA", 80, "pathB", 90))
	if eval.SwitchSuppressed {
		t.Errorf("expected first-selection behavior after Remove; reason: %s", eval.Reason)
	}
	if eval.CurrentCandidateID != "" {
		t.Errorf("expected empty CurrentCandidateID after Remove, got %q", eval.CurrentCandidateID)
	}
}
