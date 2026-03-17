package node

import (
	"fmt"
	"sync"
	"time"

	"github.com/zhouchenh/transitloom/internal/scheduler"
)

// MultiWANStickinessConfig configures the path-switching thresholds for the
// multi-WAN stickiness policy.
//
// The policy prevents unnecessary path oscillation when multiple eligible
// paths exist (e.g., two direct WAN uplinks with similar quality scores).
// A path switch requires the alternative to be "clearly better" rather than
// marginally better. This is the threshold layer: "not all improvements are
// worth switching for."
//
// This is explicitly distinct from DirectRelayFallbackPolicy (T-0023), which
// governs direct↔relay fallback and recovery behavior based on path usability.
// The stickiness policy governs within-class and cross-class quality-based
// switching when multiple eligible candidates are present.
//
// Hysteresis layering:
//
//	candidate generation → refinement → fallback policy → stickiness policy → scheduler → carrier
//
// The stickiness policy sits between the fallback filter and the scheduler.
// It does not generate, exclude, or re-order candidates; it only removes
// non-current candidates when suppressing a switch.
type MultiWANStickinessConfig struct {
	// StickinessThreshold is the minimum score advantage (in points, same
	// scale as AdminWeight/quality penalties) that an alternative path must
	// have over the current path before a switch is allowed. Score comparison
	// uses ScoreCandidate() from the scheduler package.
	//
	// Default: 3. Requires the alternative to score strictly more than 3
	// points better than the current path. This blocks trivial switches on
	// small measurement fluctuations while allowing switches when the quality
	// difference is genuinely meaningful (e.g., 3%+ loss improvement or
	// 150ms+ RTT improvement above the 50ms baseline).
	//
	// A value of 0 disables the threshold check (always switch to best path,
	// no quality-based stickiness).
	StickinessThreshold int

	// HoldDownDuration is the minimum time between path switches.
	//
	// After a switch occurs, any further switch is suppressed for this
	// duration regardless of quality differences. This prevents oscillation
	// caused by transient quality improvements that would not persist after
	// switching. During hold-down, only the current path is passed to the
	// scheduler unless the current path becomes unavailable (excluded by
	// refinement or fallback filter), in which case switching is allowed
	// immediately.
	//
	// Default: 30 seconds. Tune conservatively: longer values reduce
	// oscillation risk; shorter values allow faster recovery to a better path.
	HoldDownDuration time.Duration
}

// DefaultMultiWANStickinessConfig returns conservative v1 defaults.
//
// These defaults balance switch stability against recovery responsiveness:
//   - StickinessThreshold=3: block trivial improvements; allow clear quality gains
//   - HoldDownDuration=30s: prevent rapid re-switching after a path change
func DefaultMultiWANStickinessConfig() MultiWANStickinessConfig {
	return MultiWANStickinessConfig{
		StickinessThreshold: 3,
		HoldDownDuration:    30 * time.Second,
	}
}

// StickinessEval is the result of one stickiness policy evaluation.
//
// It describes whether a switch was suppressed, whether hold-down was active,
// and why. The Reason field is always non-empty: switching behavior must never
// be opaque. An operator reading Reason can understand whether hold-down was
// active, what the score comparison was, and whether a switch occurred.
//
// StickinessEval is the boundary between the stickiness policy and the scheduler:
// the policy produces StickinessEval + adjusted candidates; the scheduler
// consumes the adjusted candidate list and makes the final path selection.
type StickinessEval struct {
	// CurrentCandidateID is the ID of the currently preferred path before
	// this evaluation. Empty if no path has been selected yet for this
	// association (first selection).
	CurrentCandidateID string

	// SwitchSuppressed is true when the policy removed non-current candidates
	// from the scheduler's input, forcing the scheduler to keep the current path.
	// False when all candidates were passed through unchanged.
	//
	// SwitchSuppressed=true with SwitchOccurred=false means the policy held.
	// SwitchSuppressed=false with SwitchOccurred=false means no better path
	// existed (scheduler picked the current path naturally).
	// SwitchSuppressed=false with SwitchOccurred=true means a clear improvement
	// overcame the threshold (or the current path disappeared).
	SwitchSuppressed bool

	// HoldDownActive is true when the hold-down timer was running during this
	// evaluation. Hold-down suppresses switching regardless of quality differences.
	HoldDownActive bool

	// SwitchOccurred is true when RecordSelection detected a path change.
	// Set only after RecordSelection is called; always false from AdjustCandidates.
	SwitchOccurred bool

	// Reason is always non-empty. Explains: whether hold-down is active,
	// what the score comparison showed, whether switching was suppressed, and
	// what happened. Intended for operator status output and test assertions.
	Reason string
}

// MultiWANStickinessPolicy is the per-association state machine for path
// switching stability.
//
// It tracks the currently selected path and the last switch time. It provides
// an adjusted candidate list where non-current candidates are removed when the
// policy determines a switch should be suppressed. The scheduler then picks
// naturally: it sees only the current path (forced hold) or all candidates
// (switch allowed).
//
// The policy makes a switch/no-switch decision by comparing scores:
//   - If the best alternative scores > StickinessThreshold points higher than
//     the current path: switch allowed (pass all candidates to scheduler).
//   - Otherwise: switch suppressed (pass only current path to scheduler).
//   - During hold-down: always suppress (pass only current path) unless the
//     current path is no longer in the candidate list.
//   - First selection (no current path): all candidates passed through.
//
// Score comparison uses scheduler.ScoreCandidate(), which is the single
// authoritative scoring formula. The policy does not re-implement scoring.
//
// MultiWANStickinessPolicy is safe for concurrent use.
type MultiWANStickinessPolicy struct {
	mu        sync.Mutex
	config    MultiWANStickinessConfig
	currentID string    // ID of the currently selected path; empty if none
	switchedAt time.Time // when we last switched; zero if never switched
	now       func() time.Time
}

// NewMultiWANStickinessPolicy creates a new policy with no current path.
// The first call to AdjustCandidates will pass all candidates unchanged
// (first-selection epoch).
func NewMultiWANStickinessPolicy(cfg MultiWANStickinessConfig) *MultiWANStickinessPolicy {
	return &MultiWANStickinessPolicy{
		config: cfg,
		now:    time.Now,
	}
}

// AdjustCandidates returns a (possibly filtered) copy of candidates with the
// stickiness policy applied.
//
// The caller must call RecordSelection after Scheduler.Decide() to update the
// policy state with the scheduler's actual choice.
//
// Return semantics:
//   - All candidates unchanged: either first selection, current path gone, or
//     switch allowed (alternative score exceeds threshold or hold-down expired).
//   - Only current candidate: switch suppressed (below threshold or hold-down active).
func (p *MultiWANStickinessPolicy) AdjustCandidates(candidates []scheduler.PathCandidate) ([]scheduler.PathCandidate, StickinessEval) {
	p.mu.Lock()
	currentID := p.currentID
	switchedAt := p.switchedAt
	p.mu.Unlock()

	// First selection: no current path yet.
	if currentID == "" {
		return candidates, StickinessEval{
			Reason: "no current path; first selection is free",
		}
	}

	// Find the current candidate in the provided list.
	currentIdx := -1
	var currentCandidate scheduler.PathCandidate
	for i, c := range candidates {
		if c.ID == currentID {
			currentIdx = i
			currentCandidate = c
			break
		}
	}

	if currentIdx < 0 {
		// Current path is not in the candidate list. It was excluded by the
		// refinement layer or the fallback filter. Allow free selection so the
		// scheduler can pick the best available path. RecordSelection will
		// update the current path after the scheduler decides.
		return candidates, StickinessEval{
			CurrentCandidateID: currentID,
			Reason:             fmt.Sprintf("current path %q not in candidate list; switch allowed", currentID),
		}
	}

	t := p.now()
	holdDownActive := !switchedAt.IsZero() && t.Sub(switchedAt) < p.config.HoldDownDuration

	if holdDownActive {
		// Hold-down suppresses any switch unconditionally. Pass only the current
		// candidate to the scheduler. The scheduler sees a single-element list
		// and returns ModeSinglePath for the current path.
		elapsed := t.Sub(switchedAt)
		return []scheduler.PathCandidate{currentCandidate}, StickinessEval{
			CurrentCandidateID: currentID,
			SwitchSuppressed:   true,
			HoldDownActive:     true,
			Reason: fmt.Sprintf(
				"hold-down active (%s of %s elapsed after switching to %q); switch suppressed",
				elapsed.Round(time.Second), p.config.HoldDownDuration, currentID,
			),
		}
	}

	// No hold-down: compare the current path's score to the best alternative.
	// A switch is allowed only when the best alternative scores strictly more
	// than StickinessThreshold points above the current path.
	if p.config.StickinessThreshold > 0 {
		currentScore := scheduler.ScoreCandidate(currentCandidate)
		bestAltScore := 0
		bestAltID := ""
		for _, c := range candidates {
			if c.ID == currentID {
				continue
			}
			s := scheduler.ScoreCandidate(c)
			if s > bestAltScore {
				bestAltScore = s
				bestAltID = c.ID
			}
		}

		improvement := bestAltScore - currentScore
		if improvement <= p.config.StickinessThreshold {
			// Improvement is within threshold: suppress switch.
			reason := fmt.Sprintf(
				"stickiness threshold=%d; current path %q score=%d; best alternative %q score=%d (improvement=%d ≤ threshold); switch suppressed",
				p.config.StickinessThreshold, currentID, currentScore, bestAltID, bestAltScore, improvement,
			)
			if bestAltID == "" {
				reason = fmt.Sprintf(
					"stickiness threshold=%d; current path %q score=%d; no alternative candidate; current path maintained",
					p.config.StickinessThreshold, currentID, currentScore,
				)
			}
			return []scheduler.PathCandidate{currentCandidate}, StickinessEval{
				CurrentCandidateID: currentID,
				SwitchSuppressed:   true,
				Reason:             reason,
			}
		}

		// Improvement exceeds threshold: allow switch.
		return candidates, StickinessEval{
			CurrentCandidateID: currentID,
			Reason: fmt.Sprintf(
				"stickiness threshold=%d; current path %q score=%d; best alternative %q score=%d (improvement=%d > threshold); switch allowed",
				p.config.StickinessThreshold, currentID, currentScore, bestAltID, bestAltScore, improvement,
			),
		}
	}

	// StickinessThreshold == 0: disabled, pass all candidates unchanged.
	return candidates, StickinessEval{
		CurrentCandidateID: currentID,
		Reason:             "stickiness threshold=0; threshold check disabled; all candidates passed through",
	}
}

// RecordSelection updates the stickiness state with the scheduler's chosen
// path ID. Returns a StickinessEval that includes whether a switch occurred.
//
// chosenID is SchedulerDecision.ChosenPaths[0].CandidateID, or empty when
// ModeNoEligiblePath was returned.
//
// When chosenID is empty (no eligible path), the current path state is
// preserved unchanged. A path choice can return only after a candidate
// re-appears in the list.
func (p *MultiWANStickinessPolicy) RecordSelection(chosenID string) StickinessEval {
	p.mu.Lock()
	defer p.mu.Unlock()

	if chosenID == "" {
		// Scheduler found no eligible path; keep current state.
		return StickinessEval{
			CurrentCandidateID: p.currentID,
			Reason:             "no eligible path chosen; current selection state unchanged",
		}
	}

	if p.currentID == "" {
		// First selection.
		p.currentID = chosenID
		return StickinessEval{
			CurrentCandidateID: chosenID,
			SwitchOccurred:     false,
			Reason:             fmt.Sprintf("first path selection; current path set to %q", chosenID),
		}
	}

	if p.currentID == chosenID {
		// No switch; current path maintained.
		return StickinessEval{
			CurrentCandidateID: chosenID,
			SwitchOccurred:     false,
			Reason:             fmt.Sprintf("current path %q maintained; no switch", chosenID),
		}
	}

	// Switch occurred: the scheduler chose a different path (which means the
	// alternative cleared the stickiness threshold, or the current path was
	// absent from the candidate list). Start the hold-down timer to prevent
	// immediate re-switching.
	prevID := p.currentID
	p.currentID = chosenID
	p.switchedAt = p.now()
	return StickinessEval{
		CurrentCandidateID: chosenID,
		SwitchOccurred:     true,
		Reason: fmt.Sprintf(
			"switched from %q to %q; hold-down started (will last %s)",
			prevID, chosenID, p.config.HoldDownDuration,
		),
	}
}

// State returns the current candidate ID and last switch time.
// Intended for testing and operator observability.
func (p *MultiWANStickinessPolicy) State() (currentID string, switchedAt time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.currentID, p.switchedAt
}

// --- AssociationStickinessStore ---

// AssociationStickinessStore manages per-association MultiWANStickinessPolicy
// instances. Each association has its own stickiness state machine.
//
// Per-association isolation ensures that a path quality change for one
// association does not affect switching decisions for other associations.
// Associations are independent data-plane flows with independent path choices.
//
// Policies are created lazily on the first AdjustCandidates call per association.
// AssociationStickinessStore is safe for concurrent use.
type AssociationStickinessStore struct {
	mu       sync.Mutex
	config   MultiWANStickinessConfig
	policies map[string]*MultiWANStickinessPolicy
}

// NewAssociationStickinessStore creates a store that applies the given config
// to all per-association policies it creates.
func NewAssociationStickinessStore(cfg MultiWANStickinessConfig) *AssociationStickinessStore {
	return &AssociationStickinessStore{
		config:   cfg,
		policies: make(map[string]*MultiWANStickinessPolicy),
	}
}

// policyFor returns the MultiWANStickinessPolicy for the given association ID,
// creating a new one lazily if none exists.
func (s *AssociationStickinessStore) policyFor(associationID string) *MultiWANStickinessPolicy {
	s.mu.Lock()
	p, ok := s.policies[associationID]
	if !ok {
		p = NewMultiWANStickinessPolicy(s.config)
		s.policies[associationID] = p
	}
	s.mu.Unlock()
	return p
}

// AdjustCandidates returns candidates with the stickiness policy applied for
// the given association. See MultiWANStickinessPolicy.AdjustCandidates.
func (s *AssociationStickinessStore) AdjustCandidates(associationID string, candidates []scheduler.PathCandidate) ([]scheduler.PathCandidate, StickinessEval) {
	return s.policyFor(associationID).AdjustCandidates(candidates)
}

// RecordSelection updates the stickiness state for the given association with
// the scheduler's chosen path. See MultiWANStickinessPolicy.RecordSelection.
func (s *AssociationStickinessStore) RecordSelection(associationID, chosenID string) StickinessEval {
	return s.policyFor(associationID).RecordSelection(chosenID)
}

// Remove deletes the per-association policy, reclaiming memory when an
// association is torn down.
func (s *AssociationStickinessStore) Remove(associationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.policies, associationID)
}

// PolicySnapshot describes the observable stickiness policy state for one association.
type StickinessSnapshot struct {
	AssociationID      string
	CurrentCandidateID string
	SwitchedAt         time.Time
}

// Snapshot returns a copy of all current per-association stickiness states.
// Useful for operator status views.
func (s *AssociationStickinessStore) Snapshot() []StickinessSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]StickinessSnapshot, 0, len(s.policies))
	for assocID, p := range s.policies {
		currentID, switchedAt := p.State()
		result = append(result, StickinessSnapshot{
			AssociationID:      assocID,
			CurrentCandidateID: currentID,
			SwitchedAt:         switchedAt,
		})
	}
	return result
}
