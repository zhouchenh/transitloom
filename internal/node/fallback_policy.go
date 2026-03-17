package node

import (
	"fmt"
	"sync"
	"time"

	"github.com/zhouchenh/transitloom/internal/scheduler"
)

// FallbackState describes the current direct-vs-relay fallback policy state
// for one association.
//
// The policy is explicit, bounded, and observable:
//   - FallbackStatePreferDirect is the normal state. Direct candidates pass
//     through to the scheduler, which prefers direct over relay via the relay
//     penalty in scoring. This is the spec-correct default (v1-data-plane.md
//     section 8.1: prefer direct when healthy and competitively useful).
//   - FallbackStateRelay is entered when direct becomes unusable and relay is
//     available. A minimum relay dwell time prevents rapid oscillation: the
//     policy stays on relay even if direct briefly re-appears, until the dwell
//     timer expires. This protects against flapping on a marginal or
//     intermittently failing direct path.
//   - FallbackStateRecovering is entered when direct appears usable again after
//     the relay dwell expires. The policy stays in this state — still filtering
//     direct — until direct has been continuously usable for the full recovery
//     confirmation window. If direct drops again during the window, the policy
//     returns to FallbackStateRelay immediately. This separates the detection
//     of "direct is back" from the decision to "use direct again."
//
// The three-state design is the minimum needed to preserve these boundaries:
//   - candidate presence != candidate usability
//   - candidate usability != chosen runtime path
//   - direct/relay path distinction is preserved (never merged)
//   - fallback policy is separate from candidate generation and measurement
type FallbackState string

const (
	// FallbackStatePreferDirect: direct is preferred. Either direct is usable
	// and in active use, or both direct and relay are unavailable (in which
	// case the scheduler returns ModeNoEligiblePath). This is the initial state.
	FallbackStatePreferDirect FallbackState = "prefer-direct"

	// FallbackStateRelay: direct is unusable; relay is in active use. Recovery
	// to direct is gated by MinRelayDwell: the policy stays here until the dwell
	// timer expires even if direct re-appears, preventing oscillation.
	FallbackStateRelay FallbackState = "fallen-back-to-relay"

	// FallbackStateRecovering: direct has become usable again after the relay
	// dwell expired. The policy is confirming direct stability. Direct candidates
	// remain filtered (relay still used). If direct stays usable for the full
	// RecoveryConfirmWindow, the policy returns to FallbackStatePreferDirect.
	// If direct drops again during the window, returns to FallbackStateRelay.
	FallbackStateRecovering FallbackState = "recovering-to-direct"
)

// FallbackConfig configures the timing thresholds for the fallback state machine.
//
// These thresholds bound the fallback/recovery behavior explicitly:
//   - MinRelayDwell prevents rapid oscillation (switching back to direct before
//     the relay has had a chance to stabilize the situation)
//   - RecoveryConfirmWindow prevents premature return to a flapping direct path
//
// Both thresholds are bounded and observable: the FallbackEval.Reason field
// always states how much of the timer has elapsed, so operators can see exactly
// where in the policy cycle the system is.
type FallbackConfig struct {
	// MinRelayDwell is the minimum time the policy must spend in FallenBackToRelay
	// before recovery evaluation can begin. This prevents oscillation: if direct
	// briefly appears usable and then fails again before the dwell expires, the
	// policy stays on relay rather than flapping.
	//
	// Default: 30 seconds. Tune conservatively: longer values reduce oscillation
	// risk at the cost of slower recovery when direct genuinely recovers.
	MinRelayDwell time.Duration

	// RecoveryConfirmWindow is the time direct must remain continuously usable
	// before the policy returns to FallbackStatePreferDirect. During this window,
	// relay is still used. If direct drops before the window completes, the
	// policy returns to FallenBackToRelay immediately (resetting the dwell timer).
	//
	// Default: 15 seconds. A shorter window means faster recovery; a longer
	// window reduces the risk of returning to a transiently recovered path.
	RecoveryConfirmWindow time.Duration
}

// DefaultFallbackConfig returns conservative v1 defaults for the fallback policy.
func DefaultFallbackConfig() FallbackConfig {
	return FallbackConfig{
		MinRelayDwell:         30 * time.Second,
		RecoveryConfirmWindow: 15 * time.Second,
	}
}

// FallbackEval is the result of one fallback policy evaluation.
//
// It describes the current policy state, whether to filter direct candidates
// from the scheduler's input, and why. The Reason field is always non-empty:
// fallback/recovery behavior is never opaque. An operator reading the Reason
// field can understand why the system is on relay, how long it has been there,
// and how much of the recovery window remains.
//
// FallbackEval is the boundary between the fallback policy and the scheduler:
// the policy produces FallbackEval; the scheduler consumes the filtered
// candidate list that results from applying FilterDirect.
type FallbackEval struct {
	// State is the policy's current state after this evaluation.
	State FallbackState

	// FilterDirect is true when direct candidates should be excluded from the
	// scheduler's candidate input for this evaluation epoch.
	//
	// When true (FallenBackToRelay or RecoveringToDirect), the scheduler only
	// sees relay candidates, preventing it from selecting direct even if direct
	// is technically usable from the candidate refinement perspective. This
	// implements the bounded anti-flap behavior: the fallback policy, not the
	// scheduler, controls when it is safe to return to direct.
	//
	// When false (PreferDirect), all candidates pass to the scheduler. The
	// scheduler then prefers direct over relay via its relay penalty in scoring,
	// which is the spec-correct behavior (v1-data-plane.md section 8.1).
	//
	// FilterDirect is always false when State == FallbackStatePreferDirect.
	FilterDirect bool

	// Reason is the human-readable explanation of why the policy is in this
	// state and what decision was made. Always non-empty. Intended for operator
	// status output, log reporting, and test assertions.
	Reason string
}

// DirectRelayFallbackPolicy is the per-association state machine governing
// fallback and recovery between direct and single-relay-hop paths.
//
// It is explicitly separate from:
//   - candidate generation (CandidateStore, RefineCandidates): those produce
//     the usability signals the policy consumes; the policy does not alter them
//   - path quality measurement (PathQualityStore): quality is an input to
//     candidate usability, not a direct input to the policy
//   - the scheduler (Scheduler.Decide): the policy filters the candidate list
//     before scheduling; the scheduler still makes the final path selection
//
// The policy input is two boolean signals:
//   - directUsable: any usable direct-class candidate exists in the refined set
//   - relayUsable: any usable relay-class candidate exists in the refined set
//
// These signals are derived from the post-refinement candidate list, so endpoint
// freshness and quality exclusions are already reflected in them.
//
// DirectRelayFallbackPolicy is safe for concurrent use.
type DirectRelayFallbackPolicy struct {
	mu     sync.Mutex
	config FallbackConfig
	state  FallbackState

	// enteredRelayAt records when we entered FallenBackToRelay.
	// Used to enforce MinRelayDwell before recovery evaluation begins.
	// Zero when not in FallenBackToRelay (or when transitioning).
	enteredRelayAt time.Time

	// recoveryStartedAt records when we entered RecoveringToDirect.
	// Used to enforce RecoveryConfirmWindow before returning to PreferDirect.
	// Zero when not in RecoveringToDirect.
	recoveryStartedAt time.Time

	// now is the time source. Defaults to time.Now; replaceable in tests.
	now func() time.Time
}

// NewDirectRelayFallbackPolicy creates a new fallback policy starting in
// FallbackStatePreferDirect with the given configuration.
func NewDirectRelayFallbackPolicy(cfg FallbackConfig) *DirectRelayFallbackPolicy {
	return &DirectRelayFallbackPolicy{
		config: cfg,
		state:  FallbackStatePreferDirect,
		now:    time.Now,
	}
}

// Evaluate updates the policy state based on current direct/relay usability
// and returns a FallbackEval describing the resulting state and action.
//
// directUsable: true if at least one direct-class candidate (PathClassDirectPublic
// or PathClassDirectIntranet) is present and usable in the current candidate set.
// "Usable" means it passed RefineCandidates (not excluded for failed endpoint
// or missing endpoint) and has an eligible health state.
//
// relayUsable: true if at least one relay-class candidate is present and usable.
//
// The returned FallbackEval.FilterDirect indicates whether direct candidates
// should be excluded from the scheduler's input. Applying this filter is the
// caller's responsibility — the policy only computes the decision.
func (p *DirectRelayFallbackPolicy) Evaluate(directUsable, relayUsable bool) FallbackEval {
	p.mu.Lock()
	defer p.mu.Unlock()

	t := p.now()

	switch p.state {
	case FallbackStatePreferDirect:
		return p.evalPreferDirect(directUsable, relayUsable, t)
	case FallbackStateRelay:
		return p.evalRelay(directUsable, relayUsable, t)
	case FallbackStateRecovering:
		return p.evalRecovering(directUsable, relayUsable, t)
	default:
		// Unexpected state: reset rather than staying stuck.
		p.state = FallbackStatePreferDirect
		return FallbackEval{
			State:        FallbackStatePreferDirect,
			FilterDirect: false,
			Reason:       "unexpected policy state; reset to prefer-direct",
		}
	}
}

// State returns the current policy state without triggering a transition.
// Safe for concurrent use. Intended for testing and observability.
func (p *DirectRelayFallbackPolicy) State() FallbackState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// evalPreferDirect handles the FallbackStatePreferDirect state.
//
// Normal state: pass direct candidates through to the scheduler. If direct
// becomes unusable and relay is available, fall back to relay. If both become
// unavailable, stay in PreferDirect and let the scheduler return ModeNoEligiblePath
// — filtering candidates when all paths are down makes the situation worse.
func (p *DirectRelayFallbackPolicy) evalPreferDirect(directUsable, relayUsable bool, t time.Time) FallbackEval {
	if directUsable {
		return FallbackEval{
			State:        FallbackStatePreferDirect,
			FilterDirect: false,
			Reason:       "direct is usable; preferred over relay",
		}
	}

	if relayUsable {
		// Direct is unusable and relay is available: fall back to relay.
		// Start the relay dwell timer so recovery evaluation cannot begin
		// immediately — we need to confirm relay is stable before attempting
		// to return to direct.
		p.state = FallbackStateRelay
		p.enteredRelayAt = t
		return FallbackEval{
			State:        FallbackStateRelay,
			FilterDirect: true,
			Reason:       "direct unusable; falling back to relay",
		}
	}

	// Both unavailable: stay in PreferDirect. The scheduler receives all
	// (currently empty or ineligible) candidates and returns ModeNoEligiblePath.
	// FilterDirect=false because filtering direct when relay is also unavailable
	// would not help — there is nothing to fall back to.
	return FallbackEval{
		State:        FallbackStatePreferDirect,
		FilterDirect: false,
		Reason:       "direct and relay both unusable; no eligible path",
	}
}

// evalRelay handles the FallbackStateRelay state.
//
// Anti-flap enforcement: even if direct appears usable, the policy stays on relay
// until MinRelayDwell expires. This prevents oscillation on marginal or
// intermittently-failing direct paths. When the dwell expires, the policy
// transitions to RecoveringToDirect to confirm direct stability before switching.
func (p *DirectRelayFallbackPolicy) evalRelay(directUsable, relayUsable bool, t time.Time) FallbackEval {
	if !relayUsable {
		// Relay itself became unavailable. There is no point holding FilterDirect=true
		// when relay is also gone. Return to PreferDirect and let the scheduler
		// determine the outcome (likely ModeNoEligiblePath if direct is also down).
		p.state = FallbackStatePreferDirect
		p.enteredRelayAt = time.Time{}
		return FallbackEval{
			State:        FallbackStatePreferDirect,
			FilterDirect: false,
			Reason:       "relay also became unavailable; returning to direct (both paths down)",
		}
	}

	if !directUsable {
		return FallbackEval{
			State:        FallbackStateRelay,
			FilterDirect: true,
			Reason:       "direct still unusable; remaining on relay",
		}
	}

	// Direct appears usable. Check if the relay dwell timer has expired.
	// This is the anti-flap gate: direct must not be selected immediately
	// when it re-appears after a failure, because brief re-appearance followed
	// by another failure would cause visible path oscillation.
	elapsed := t.Sub(p.enteredRelayAt)
	if elapsed < p.config.MinRelayDwell {
		return FallbackEval{
			State:        FallbackStateRelay,
			FilterDirect: true,
			Reason: fmt.Sprintf(
				"direct appears usable but relay dwell not satisfied (%s of %s elapsed); preventing oscillation",
				elapsed.Round(time.Second),
				p.config.MinRelayDwell,
			),
		}
	}

	// Relay dwell satisfied: start the recovery confirmation window.
	// Direct is still filtered during RecoveringToDirect — we confirm stability
	// before switching back, rather than switching immediately on dwell expiry.
	p.state = FallbackStateRecovering
	p.recoveryStartedAt = t
	return FallbackEval{
		State:        FallbackStateRecovering,
		FilterDirect: true,
		Reason: fmt.Sprintf(
			"direct appears usable; relay dwell satisfied (%s); starting recovery confirmation window",
			elapsed.Round(time.Second),
		),
	}
}

// evalRecovering handles the FallbackStateRecovering state.
//
// The policy is confirming that direct is stable before returning to PreferDirect.
// Direct candidates remain filtered (relay still used) throughout this window.
// If direct drops before RecoveryConfirmWindow elapses, the policy returns to
// FallenBackToRelay — resetting the dwell timer so the full cycle must repeat.
func (p *DirectRelayFallbackPolicy) evalRecovering(directUsable, relayUsable bool, t time.Time) FallbackEval {
	if !directUsable {
		// Direct became unusable again during the recovery window. Abort recovery
		// and return to relay. Reset the dwell timer: the next recovery attempt
		// must wait the full MinRelayDwell again. This prevents a fast path where
		// repeated brief direct re-appearances accumulate toward recovery.
		p.state = FallbackStateRelay
		p.enteredRelayAt = t
		p.recoveryStartedAt = time.Time{}
		return FallbackEval{
			State:        FallbackStateRelay,
			FilterDirect: true,
			Reason:       "direct became unusable during recovery window; returning to relay",
		}
	}

	// Direct remains usable. Check whether the confirmation window has elapsed.
	elapsed := t.Sub(p.recoveryStartedAt)
	if elapsed < p.config.RecoveryConfirmWindow {
		return FallbackEval{
			State:        FallbackStateRecovering,
			FilterDirect: true,
			Reason: fmt.Sprintf(
				"direct confirmed usable for %s; awaiting %s recovery confirmation window",
				elapsed.Round(time.Second),
				p.config.RecoveryConfirmWindow,
			),
		}
	}

	// Recovery confirmed: direct has been usable for the full window.
	// Return to PreferDirect and clear all timers.
	p.state = FallbackStatePreferDirect
	p.enteredRelayAt = time.Time{}
	p.recoveryStartedAt = time.Time{}
	return FallbackEval{
		State:        FallbackStatePreferDirect,
		FilterDirect: false,
		Reason: fmt.Sprintf(
			"direct confirmed usable for %s; recovery complete; preferring direct",
			elapsed.Round(time.Second),
		),
	}
}

// --- AssociationFallbackStore ---

// AssociationFallbackStore manages per-association DirectRelayFallbackPolicy
// instances, isolating fallback/recovery state per association.
//
// Each association has its own state machine. A direct path failure on one
// association must not affect the fallback state of another — associations are
// independent data-plane flows, and per-association isolation prevents one
// failing path from masking healthy paths on other associations.
//
// Policies are created lazily on the first Evaluate call for each association.
// AssociationFallbackStore is safe for concurrent use.
type AssociationFallbackStore struct {
	mu       sync.Mutex
	config   FallbackConfig
	policies map[string]*DirectRelayFallbackPolicy
}

// NewAssociationFallbackStore creates a store that applies the given config to
// all policies it creates.
func NewAssociationFallbackStore(cfg FallbackConfig) *AssociationFallbackStore {
	return &AssociationFallbackStore{
		config:   cfg,
		policies: make(map[string]*DirectRelayFallbackPolicy),
	}
}

// Evaluate runs the fallback policy for the given association, creating a new
// policy lazily if none exists yet.
func (s *AssociationFallbackStore) Evaluate(associationID string, directUsable, relayUsable bool) FallbackEval {
	s.mu.Lock()
	p, ok := s.policies[associationID]
	if !ok {
		p = NewDirectRelayFallbackPolicy(s.config)
		s.policies[associationID] = p
	}
	s.mu.Unlock()

	return p.Evaluate(directUsable, relayUsable)
}

// PolicyState returns the current state for an association, or
// FallbackStatePreferDirect if no policy has been created yet.
func (s *AssociationFallbackStore) PolicyState(associationID string) FallbackState {
	s.mu.Lock()
	p, ok := s.policies[associationID]
	s.mu.Unlock()
	if !ok {
		return FallbackStatePreferDirect
	}
	return p.State()
}

// Remove deletes the policy for an association, reclaiming memory when an
// association is torn down.
func (s *AssociationFallbackStore) Remove(associationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.policies, associationID)
}

// FallbackPolicySnapshot holds the observable fallback policy state for one
// association. Used by Snapshot and status reporting.
type FallbackPolicySnapshot struct {
	AssociationID string
	State         FallbackState
}

// Snapshot returns a copy of all current per-association fallback policy states.
// Useful for operator status views.
func (s *AssociationFallbackStore) Snapshot() []FallbackPolicySnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]FallbackPolicySnapshot, 0, len(s.policies))
	for assocID, p := range s.policies {
		result = append(result, FallbackPolicySnapshot{
			AssociationID: assocID,
			State:         p.State(),
		})
	}
	return result
}

// --- candidate filtering helpers ---

// hasUsableDirectCandidate reports whether any candidate in the list is a
// direct-class path (not relay-assisted). Used to derive the directUsable
// signal for the fallback policy evaluation.
//
// Candidates in this list have already passed the refinement layer: stale/failed
// endpoints have been excluded, health has been adjusted. So "present in the
// list" accurately represents "usable from a candidate perspective."
func hasUsableDirectCandidate(candidates []scheduler.PathCandidate) bool {
	for _, c := range candidates {
		if !c.Class.IsRelay() {
			return true
		}
	}
	return false
}

// hasUsableRelayCandidate reports whether any candidate in the list is a
// relay-class path. Used to derive the relayUsable signal for the fallback
// policy evaluation.
func hasUsableRelayCandidate(candidates []scheduler.PathCandidate) bool {
	for _, c := range candidates {
		if c.Class.IsRelay() {
			return true
		}
	}
	return false
}

// applyFallbackFilter removes direct-class candidates from the list when the
// fallback policy has set FilterDirect=true.
//
// This is the boundary between the fallback policy's decision and the scheduler's
// input: when the policy is in FallenBackToRelay or RecoveringToDirect, direct
// candidates are excluded here before Scheduler.Decide() is called. The
// scheduler then only sees relay candidates and picks the best relay path.
//
// The filter is explicit and narrow: it only acts when FilterDirect=true. When
// FilterDirect=false, candidates are returned unchanged. This preserves the
// scheduler's authority over path selection in the normal (PreferDirect) case.
func applyFallbackFilter(candidates []scheduler.PathCandidate, eval FallbackEval) []scheduler.PathCandidate {
	if !eval.FilterDirect {
		return candidates
	}
	// Keep only relay candidates.
	result := candidates[:0:0] // allocate fresh; do not modify input
	for _, c := range candidates {
		if c.Class.IsRelay() {
			result = append(result, c)
		}
	}
	return result
}
