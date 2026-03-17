package node

import (
	"strings"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/scheduler"
)

// fixedClock returns a function that always returns t, for deterministic timing.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// --- DirectRelayFallbackPolicy unit tests ---

func TestFallbackPolicy_StartsDirect(t *testing.T) {
	p := NewDirectRelayFallbackPolicy(DefaultFallbackConfig())
	if p.State() != FallbackStatePreferDirect {
		t.Fatalf("expected initial state prefer-direct, got %s", p.State())
	}
}

func TestFallbackPolicy_PreferDirectWhenDirectUsable(t *testing.T) {
	p := NewDirectRelayFallbackPolicy(DefaultFallbackConfig())
	p.now = fixedClock(time.Now())

	eval := p.Evaluate(true, true)
	if eval.State != FallbackStatePreferDirect {
		t.Errorf("expected prefer-direct, got %s", eval.State)
	}
	if eval.FilterDirect {
		t.Error("FilterDirect should be false when direct is usable")
	}
	if eval.Reason == "" {
		t.Error("Reason must not be empty")
	}
}

func TestFallbackPolicy_PreferDirectWhenOnlyDirectUsable(t *testing.T) {
	p := NewDirectRelayFallbackPolicy(DefaultFallbackConfig())
	p.now = fixedClock(time.Now())

	eval := p.Evaluate(true, false) // direct usable, relay not
	if eval.State != FallbackStatePreferDirect {
		t.Errorf("expected prefer-direct, got %s", eval.State)
	}
	if eval.FilterDirect {
		t.Error("FilterDirect should be false when direct is usable")
	}
}

func TestFallbackPolicy_FallbackToRelayWhenDirectUnusable(t *testing.T) {
	p := NewDirectRelayFallbackPolicy(DefaultFallbackConfig())
	p.now = fixedClock(time.Now())

	eval := p.Evaluate(false, true) // direct unusable, relay usable
	if eval.State != FallbackStateRelay {
		t.Errorf("expected fallen-back-to-relay, got %s", eval.State)
	}
	if !eval.FilterDirect {
		t.Error("FilterDirect should be true in fallen-back-to-relay state")
	}
	if eval.Reason == "" {
		t.Error("Reason must not be empty")
	}
}

func TestFallbackPolicy_BothUnusable_StaysPreferDirect(t *testing.T) {
	// When both direct and relay are unusable, the policy stays in PreferDirect
	// and does NOT filter direct. The scheduler should see all candidates and
	// return ModeNoEligiblePath. Filtering when both are down makes it worse.
	p := NewDirectRelayFallbackPolicy(DefaultFallbackConfig())
	p.now = fixedClock(time.Now())

	eval := p.Evaluate(false, false)
	if eval.State != FallbackStatePreferDirect {
		t.Errorf("expected prefer-direct when both unavailable, got %s", eval.State)
	}
	if eval.FilterDirect {
		t.Error("FilterDirect must be false when both paths are unavailable")
	}
}

// TestFallbackPolicy_AntiFlap_StaysOnRelayDuringDwellTime verifies the core
// anti-flap guarantee: even when direct re-appears, the policy stays on relay
// until MinRelayDwell has elapsed.
func TestFallbackPolicy_AntiFlap_StaysOnRelayDuringDwellTime(t *testing.T) {
	cfg := FallbackConfig{
		MinRelayDwell:         30 * time.Second,
		RecoveryConfirmWindow: 15 * time.Second,
	}
	p := NewDirectRelayFallbackPolicy(cfg)
	base := time.Now()
	p.now = fixedClock(base)

	// Step 1: fall back to relay
	eval := p.Evaluate(false, true)
	if eval.State != FallbackStateRelay {
		t.Fatalf("expected relay state after direct failure, got %s", eval.State)
	}

	// Step 2: direct re-appears at 10s, but dwell requires 30s → stay on relay
	p.now = fixedClock(base.Add(10 * time.Second))
	eval = p.Evaluate(true, true)
	if eval.State != FallbackStateRelay {
		t.Errorf("expected relay during dwell (10s < 30s), got %s", eval.State)
	}
	if !eval.FilterDirect {
		t.Error("FilterDirect must be true during dwell time")
	}
	if !strings.Contains(eval.Reason, "dwell not satisfied") {
		t.Errorf("reason should mention dwell not satisfied, got: %s", eval.Reason)
	}

	// Step 3: still not enough at 25s
	p.now = fixedClock(base.Add(25 * time.Second))
	eval = p.Evaluate(true, true)
	if eval.State != FallbackStateRelay {
		t.Errorf("expected relay at 25s, got %s", eval.State)
	}

	// Step 4: dwell satisfied at 31s → transition to recovering
	p.now = fixedClock(base.Add(31 * time.Second))
	eval = p.Evaluate(true, true)
	if eval.State != FallbackStateRecovering {
		t.Errorf("expected recovering after dwell satisfied, got %s", eval.State)
	}
	// Direct is still filtered during the recovery confirmation window.
	if !eval.FilterDirect {
		t.Error("FilterDirect should be true during recovery window")
	}
}

// TestFallbackPolicy_RecoveryConfirmation verifies that after the relay dwell
// expires, the policy stays in RecoveringToDirect until the full confirmation
// window elapses, then returns to PreferDirect.
func TestFallbackPolicy_RecoveryConfirmation(t *testing.T) {
	cfg := FallbackConfig{
		MinRelayDwell:         10 * time.Second,
		RecoveryConfirmWindow: 15 * time.Second,
	}
	p := NewDirectRelayFallbackPolicy(cfg)
	base := time.Now()
	p.now = fixedClock(base)

	// Fall back to relay
	p.Evaluate(false, true)

	// Dwell satisfied at 11s → enter recovering
	p.now = fixedClock(base.Add(11 * time.Second))
	eval := p.Evaluate(true, true)
	if eval.State != FallbackStateRecovering {
		t.Fatalf("expected recovering after dwell, got %s", eval.State)
	}
	recoveryEnteredAt := base.Add(11 * time.Second)

	// Partway through recovery window (5s into 15s) → still recovering
	p.now = fixedClock(recoveryEnteredAt.Add(5 * time.Second))
	eval = p.Evaluate(true, true)
	if eval.State != FallbackStateRecovering {
		t.Errorf("expected still recovering at 5s into window, got %s", eval.State)
	}
	if !eval.FilterDirect {
		t.Error("FilterDirect must be true during recovery window")
	}
	if !strings.Contains(eval.Reason, "awaiting") {
		t.Errorf("reason should mention awaiting recovery window, got: %s", eval.Reason)
	}

	// Recovery window complete at 16s → return to PreferDirect
	p.now = fixedClock(recoveryEnteredAt.Add(16 * time.Second))
	eval = p.Evaluate(true, true)
	if eval.State != FallbackStatePreferDirect {
		t.Errorf("expected prefer-direct after recovery complete, got %s", eval.State)
	}
	if eval.FilterDirect {
		t.Error("FilterDirect must be false after recovery complete")
	}
	if !strings.Contains(eval.Reason, "recovery complete") {
		t.Errorf("reason should say recovery complete, got: %s", eval.Reason)
	}
}

// TestFallbackPolicy_RecoveryAbort verifies that if direct becomes unusable
// during the recovery confirmation window, the policy returns to FallenBackToRelay
// and resets the dwell timer.
func TestFallbackPolicy_RecoveryAbort(t *testing.T) {
	cfg := FallbackConfig{
		MinRelayDwell:         10 * time.Second,
		RecoveryConfirmWindow: 15 * time.Second,
	}
	p := NewDirectRelayFallbackPolicy(cfg)
	base := time.Now()
	p.now = fixedClock(base)

	// Fall back to relay
	p.Evaluate(false, true)

	// Dwell satisfied at 11s → recovering
	p.now = fixedClock(base.Add(11 * time.Second))
	eval := p.Evaluate(true, true)
	if eval.State != FallbackStateRecovering {
		t.Fatalf("expected recovering, got %s", eval.State)
	}
	recoveryEnteredAt := base.Add(11 * time.Second)

	// Direct drops again at 5s into recovery window → abort, return to relay
	abortTime := recoveryEnteredAt.Add(5 * time.Second)
	p.now = fixedClock(abortTime)
	eval = p.Evaluate(false, true)
	if eval.State != FallbackStateRelay {
		t.Errorf("expected relay after recovery abort, got %s", eval.State)
	}
	if !eval.FilterDirect {
		t.Error("FilterDirect must be true after recovery abort")
	}
	if !strings.Contains(eval.Reason, "returning to relay") {
		t.Errorf("reason should mention returning to relay, got: %s", eval.Reason)
	}

	// After abort: dwell timer was reset at abortTime. Direct appearing 5s
	// later must NOT trigger recovery — the full new dwell (10s) is required.
	p.now = fixedClock(abortTime.Add(5 * time.Second))
	eval = p.Evaluate(true, true)
	if eval.State != FallbackStateRelay {
		t.Errorf("expected relay during new dwell after abort (5s < 10s), got %s", eval.State)
	}
	if !strings.Contains(eval.Reason, "dwell not satisfied") {
		t.Errorf("reason should mention dwell not satisfied, got: %s", eval.Reason)
	}

	// After full new dwell (11s after abort) → recovering again
	p.now = fixedClock(abortTime.Add(11 * time.Second))
	eval = p.Evaluate(true, true)
	if eval.State != FallbackStateRecovering {
		t.Errorf("expected recovering after new dwell satisfied, got %s", eval.State)
	}
}

// TestFallbackPolicy_RelayUnusable_ReturnsToPreferDirect verifies that when
// relay itself becomes unavailable while in FallenBackToRelay, the policy
// returns to PreferDirect (FilterDirect=false) rather than remaining stuck.
func TestFallbackPolicy_RelayUnusable_ReturnsToPreferDirect(t *testing.T) {
	p := NewDirectRelayFallbackPolicy(DefaultFallbackConfig())
	p.now = fixedClock(time.Now())

	// Fall back to relay
	eval := p.Evaluate(false, true)
	if eval.State != FallbackStateRelay {
		t.Fatalf("expected relay state, got %s", eval.State)
	}

	// Relay also becomes unavailable
	eval = p.Evaluate(false, false)
	if eval.State != FallbackStatePreferDirect {
		t.Errorf("expected prefer-direct when relay also unavailable, got %s", eval.State)
	}
	if eval.FilterDirect {
		t.Error("FilterDirect must be false when relay is also unavailable")
	}
	if !strings.Contains(eval.Reason, "both paths down") {
		t.Errorf("reason should mention both paths down, got: %s", eval.Reason)
	}
}

// TestFallbackPolicy_ReasonAlwaysNonEmpty verifies that every state transition
// and stable-state evaluation produces a non-empty Reason field.
// This is the core observability guarantee: no silent fallback transitions.
func TestFallbackPolicy_ReasonAlwaysNonEmpty(t *testing.T) {
	cfg := FallbackConfig{
		MinRelayDwell:         5 * time.Second,
		RecoveryConfirmWindow: 3 * time.Second,
	}
	p := NewDirectRelayFallbackPolicy(cfg)
	base := time.Now()
	current := base

	type step struct {
		advanceBy    time.Duration
		directUsable bool
		relayUsable  bool
	}

	// Simulate a full cycle: prefer-direct → relay → recovering → prefer-direct
	sequence := []step{
		{0, true, true},               // prefer-direct (direct usable)
		{0, false, false},             // prefer-direct (both down)
		{0, false, true},              // fall back to relay
		{2 * time.Second, true, true}, // dwell active
		{6 * time.Second, true, true}, // dwell done → recovering
		{2 * time.Second, true, true}, // recovery partial
		{4 * time.Second, true, true}, // recovery complete
	}

	for i, step := range sequence {
		current = current.Add(step.advanceBy)
		p.now = fixedClock(current)
		eval := p.Evaluate(step.directUsable, step.relayUsable)
		if eval.Reason == "" {
			t.Errorf("step %d: Reason must not be empty (state=%s direct=%v relay=%v)",
				i, eval.State, step.directUsable, step.relayUsable)
		}
		// Invariant: FilterDirect must be false in PreferDirect state.
		if eval.State == FallbackStatePreferDirect && eval.FilterDirect {
			t.Errorf("step %d: FilterDirect must be false in prefer-direct state", i)
		}
	}
}

// TestFallbackPolicy_NoFlapping_TableDriven exercises the no-flapping invariant
// across multiple scenarios using time-controlled transitions.
func TestFallbackPolicy_NoFlapping_TableDriven(t *testing.T) {
	type transition struct {
		advanceBy    time.Duration
		directUsable bool
		relayUsable  bool
		wantState    FallbackState
		wantFilter   bool
	}

	tests := []struct {
		name        string
		cfg         FallbackConfig
		transitions []transition
	}{
		{
			name: "stable direct: no fallback occurs",
			cfg: FallbackConfig{
				MinRelayDwell:         30 * time.Second,
				RecoveryConfirmWindow: 15 * time.Second,
			},
			transitions: []transition{
				{0, true, true, FallbackStatePreferDirect, false},
				{5 * time.Second, true, true, FallbackStatePreferDirect, false},
				{10 * time.Second, true, true, FallbackStatePreferDirect, false},
			},
		},
		{
			name: "full cycle: fallback then recovery",
			cfg: FallbackConfig{
				MinRelayDwell:         20 * time.Second,
				RecoveryConfirmWindow: 10 * time.Second,
			},
			transitions: []transition{
				{0, false, true, FallbackStateRelay, true},                    // fail → relay
				{5 * time.Second, true, true, FallbackStateRelay, true},       // dwell not done (5 < 20)
				{10 * time.Second, true, true, FallbackStateRelay, true},      // dwell not done (15 < 20)
				{6 * time.Second, true, true, FallbackStateRecovering, true},  // dwell done (21 >= 20)
				{5 * time.Second, true, true, FallbackStateRecovering, true},  // window partial (5 < 10)
				{6 * time.Second, true, true, FallbackStatePreferDirect, false}, // window done (11 >= 10)
			},
		},
		{
			name: "recovery abort resets dwell timer",
			cfg: FallbackConfig{
				MinRelayDwell:         10 * time.Second,
				RecoveryConfirmWindow: 10 * time.Second,
			},
			transitions: []transition{
				{0, false, true, FallbackStateRelay, true},                    // fallback
				{11 * time.Second, true, true, FallbackStateRecovering, true}, // dwell done
				{5 * time.Second, false, true, FallbackStateRelay, true},      // abort recovery
				// After abort, dwell timer was reset at the abort moment.
				// Direct re-appearing 1s later must still require full new dwell.
				{1 * time.Second, true, true, FallbackStateRelay, true},          // new dwell (1s < 10s)
				{10 * time.Second, true, true, FallbackStateRecovering, true},    // new dwell done (11s >= 10s)
				{11 * time.Second, true, true, FallbackStatePreferDirect, false}, // recovery done
			},
		},
		{
			name: "relay disappears while in relay state: return to prefer-direct",
			cfg: FallbackConfig{
				MinRelayDwell:         30 * time.Second,
				RecoveryConfirmWindow: 15 * time.Second,
			},
			transitions: []transition{
				{0, false, true, FallbackStateRelay, true},                        // fallback
				{5 * time.Second, false, false, FallbackStatePreferDirect, false}, // relay gone
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewDirectRelayFallbackPolicy(tt.cfg)
			current := time.Now()

			for i, tr := range tt.transitions {
				current = current.Add(tr.advanceBy)
				p.now = fixedClock(current)

				eval := p.Evaluate(tr.directUsable, tr.relayUsable)

				if eval.State != tr.wantState {
					t.Errorf("transition %d: want state %s, got %s (reason: %s)",
						i, tr.wantState, eval.State, eval.Reason)
				}
				if eval.FilterDirect != tr.wantFilter {
					t.Errorf("transition %d: want FilterDirect=%v, got %v (reason: %s)",
						i, tr.wantFilter, eval.FilterDirect, eval.Reason)
				}
				if eval.Reason == "" {
					t.Errorf("transition %d: Reason must not be empty", i)
				}
				// Core invariant: prefer-direct state must never filter direct.
				if eval.State == FallbackStatePreferDirect && eval.FilterDirect {
					t.Errorf("transition %d: FilterDirect must be false in prefer-direct state", i)
				}
			}
		})
	}
}

// --- AssociationFallbackStore tests ---

func TestAssociationFallbackStore_IsolatesAssociations(t *testing.T) {
	store := NewAssociationFallbackStore(DefaultFallbackConfig())

	// Fall back assoc1 to relay.
	eval1 := store.Evaluate("assoc1", false, true)
	if eval1.State != FallbackStateRelay {
		t.Errorf("assoc1: expected relay, got %s", eval1.State)
	}

	// assoc2 is independent: should still be in PreferDirect.
	eval2 := store.Evaluate("assoc2", true, true)
	if eval2.State != FallbackStatePreferDirect {
		t.Errorf("assoc2 should be unaffected by assoc1 fallback, got %s", eval2.State)
	}

	if state := store.PolicyState("assoc1"); state != FallbackStateRelay {
		t.Errorf("PolicyState assoc1: expected relay, got %s", state)
	}
	if state := store.PolicyState("assoc2"); state != FallbackStatePreferDirect {
		t.Errorf("PolicyState assoc2: expected prefer-direct, got %s", state)
	}
}

func TestAssociationFallbackStore_UnknownAssociation_DefaultsDirect(t *testing.T) {
	store := NewAssociationFallbackStore(DefaultFallbackConfig())
	// PolicyState for an association that has never been evaluated returns
	// the default (PreferDirect).
	state := store.PolicyState("never-seen")
	if state != FallbackStatePreferDirect {
		t.Errorf("expected prefer-direct for unknown association, got %s", state)
	}
}

func TestAssociationFallbackStore_Remove_ResetsPolicyState(t *testing.T) {
	store := NewAssociationFallbackStore(DefaultFallbackConfig())

	// Fall back assoc1.
	store.Evaluate("assoc1", false, true)
	if store.PolicyState("assoc1") != FallbackStateRelay {
		t.Fatal("assoc1 should be in relay state before Remove")
	}

	// Remove resets the association's state.
	store.Remove("assoc1")

	if state := store.PolicyState("assoc1"); state != FallbackStatePreferDirect {
		t.Errorf("after Remove, expected prefer-direct, got %s", state)
	}

	// Re-evaluating after Remove starts a fresh policy in PreferDirect.
	eval := store.Evaluate("assoc1", true, true)
	if eval.State != FallbackStatePreferDirect {
		t.Errorf("after Remove + re-evaluate, expected prefer-direct, got %s", eval.State)
	}
}

// --- applyFallbackFilter tests ---

func TestApplyFallbackFilter_Passthrough_WhenNotFiltering(t *testing.T) {
	sc := []scheduler.PathCandidate{
		{ID: "d1", Class: scheduler.PathClassDirectPublic},
		{ID: "r1", Class: scheduler.PathClassCoordinatorRelay},
	}

	eval := FallbackEval{FilterDirect: false}
	result := applyFallbackFilter(sc, eval)

	if len(result) != 2 {
		t.Errorf("expected 2 candidates passthrough (FilterDirect=false), got %d", len(result))
	}
}

func TestApplyFallbackFilter_RemovesDirect_WhenFiltering(t *testing.T) {
	sc := []scheduler.PathCandidate{
		{ID: "d1", Class: scheduler.PathClassDirectPublic},
		{ID: "d2", Class: scheduler.PathClassDirectIntranet},
		{ID: "r1", Class: scheduler.PathClassCoordinatorRelay},
	}

	eval := FallbackEval{FilterDirect: true}
	result := applyFallbackFilter(sc, eval)

	if len(result) != 1 {
		t.Errorf("expected 1 relay candidate after filter, got %d", len(result))
	}
	if !result[0].Class.IsRelay() {
		t.Error("remaining candidate should be relay class")
	}
	if result[0].ID != "r1" {
		t.Errorf("remaining candidate ID should be r1, got %s", result[0].ID)
	}
}

func TestApplyFallbackFilter_AllRelay_PassesThrough_WhenFiltering(t *testing.T) {
	sc := []scheduler.PathCandidate{
		{ID: "r1", Class: scheduler.PathClassCoordinatorRelay},
		{ID: "r2", Class: scheduler.PathClassNodeRelay},
	}

	eval := FallbackEval{FilterDirect: true}
	result := applyFallbackFilter(sc, eval)

	if len(result) != 2 {
		t.Errorf("expected 2 relay candidates, got %d", len(result))
	}
}

func TestApplyFallbackFilter_EmptyInput(t *testing.T) {
	eval := FallbackEval{FilterDirect: true}
	result := applyFallbackFilter(nil, eval)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(result))
	}
}

// --- hasUsableDirectCandidate / hasUsableRelayCandidate tests ---

func TestHasUsableDirectCandidate(t *testing.T) {
	tests := []struct {
		name       string
		candidates []scheduler.PathCandidate
		want       bool
	}{
		{
			"empty",
			nil,
			false,
		},
		{
			"only relay",
			[]scheduler.PathCandidate{{ID: "r1", Class: scheduler.PathClassCoordinatorRelay}},
			false,
		},
		{
			"direct present",
			[]scheduler.PathCandidate{{ID: "d1", Class: scheduler.PathClassDirectPublic}},
			true,
		},
		{
			"direct intranet present",
			[]scheduler.PathCandidate{{ID: "d1", Class: scheduler.PathClassDirectIntranet}},
			true,
		},
		{
			"mixed: direct and relay",
			[]scheduler.PathCandidate{
				{ID: "r1", Class: scheduler.PathClassCoordinatorRelay},
				{ID: "d1", Class: scheduler.PathClassDirectPublic},
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasUsableDirectCandidate(tt.candidates)
			if got != tt.want {
				t.Errorf("hasUsableDirectCandidate = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasUsableRelayCandidate(t *testing.T) {
	tests := []struct {
		name       string
		candidates []scheduler.PathCandidate
		want       bool
	}{
		{
			"empty",
			nil,
			false,
		},
		{
			"only direct",
			[]scheduler.PathCandidate{{ID: "d1", Class: scheduler.PathClassDirectPublic}},
			false,
		},
		{
			"coordinator relay present",
			[]scheduler.PathCandidate{{ID: "r1", Class: scheduler.PathClassCoordinatorRelay}},
			true,
		},
		{
			"node relay present",
			[]scheduler.PathCandidate{{ID: "r1", Class: scheduler.PathClassNodeRelay}},
			true,
		},
		{
			"mixed: direct and relay",
			[]scheduler.PathCandidate{
				{ID: "d1", Class: scheduler.PathClassDirectPublic},
				{ID: "r1", Class: scheduler.PathClassCoordinatorRelay},
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasUsableRelayCandidate(tt.candidates)
			if got != tt.want {
				t.Errorf("hasUsableRelayCandidate = %v, want %v", got, tt.want)
			}
		})
	}
}
