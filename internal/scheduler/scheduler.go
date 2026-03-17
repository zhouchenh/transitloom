package scheduler

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// relayPenalty is the scoring penalty applied to relay-class paths.
//
// Relay paths add latency, introduce a potential relay failure point, and
// incur extra forwarding cost. Direct paths are preferred when quality is
// comparable. This constant penalty keeps direct paths preferred unless their
// quality is significantly worse than a relay alternative.
//
// Value is out of a max effective score of 100 (from AdminWeight).
const relayPenalty = 10

// degradedPenalty is the scoring penalty applied to degraded-health paths.
// Degraded paths are still eligible but lower-priority.
const degradedPenalty = 20

// StripeMatchThresholds defines the quality spread thresholds for per-packet
// striping eligibility. Striping is only allowed when ALL eligible paths are
// within ALL of these thresholds.
//
// These are conservative defaults. On paths with poor RTT parity or significant
// loss difference, per-packet striping causes reordering that degrades effective
// throughput. It is safer to stay on the best path (burst/flowlet mode) than
// to spray packets across mismatched paths.
//
// See spec/v1-data-plane.md section 15 for the architectural basis.
type StripeMatchThresholds struct {
	// MaxRTTSpread is the maximum allowed RTT spread across eligible candidates.
	// Default: 20ms. Paths with larger RTT spread are too mismatched for safe
	// per-packet striping — the reordering buffer requirement at the receiver
	// grows proportionally to the RTT spread.
	MaxRTTSpread time.Duration

	// MaxJitterSpread is the maximum allowed jitter spread across candidates.
	// Default: 10ms. High jitter spread means paths behave unpredictably
	// relative to each other, making per-packet striping unstable.
	MaxJitterSpread time.Duration

	// MaxLossSpread is the maximum allowed loss fraction spread [0.0, 1.0].
	// Default: 0.01 (1%). A significant loss difference between paths means
	// one path is delivering packets reliably while the other is not — striping
	// onto the lossy path hurts overall delivery quality.
	MaxLossSpread float64

	// MinConfidence is the minimum measurement confidence required for any
	// candidate to be considered in the striping match gate.
	// Default: 0.3. Unmeasured or very low-confidence paths block striping
	// because the match thresholds cannot be evaluated without measurements.
	MinConfidence float64
}

// DefaultStripeMatchThresholds returns conservative v1 defaults for the
// striping match gate. These defaults preserve transport quality on mismatched
// paths and require meaningful measurement before striping is allowed.
func DefaultStripeMatchThresholds() StripeMatchThresholds {
	return StripeMatchThresholds{
		MaxRTTSpread:    20 * time.Millisecond,
		MaxJitterSpread: 10 * time.Millisecond,
		MaxLossSpread:   0.01,
		MinConfidence:   0.3,
	}
}

// Scheduler makes endpoint-owned path selection and scheduling mode decisions
// for association-bound raw UDP carriage.
//
// The Scheduler is the single authority for deciding:
//   - which eligible path(s) to use for an association
//   - whether to use weighted burst/flowlet or per-packet striping
//   - how to weight traffic across multiple paths
//
// Endpoint-owned scheduling: the Scheduler runs at the source endpoint only.
// Relays do not run a Scheduler; they follow installed forwarding context.
// This preserves the architectural invariant from spec/v1-data-plane.md
// section 13: endpoints own end-to-end split decisions; relays do not.
//
// Association-bound: all decisions are scoped to one association at a time.
// The Scheduler never mixes candidates across associations.
//
// v1 default mode: ModeWeightedBurstFlowlet. Per-packet striping is only
// activated when all eligible paths are within the configured StripeMatchThresholds.
// See spec/v1-data-plane.md sections 14-15.
type Scheduler struct {
	thresholds StripeMatchThresholds

	mu       sync.Mutex
	counters map[string]*AssociationCounters // keyed by association ID
}

// ScoreCandidate computes the composite score for a single path candidate.
// The result uses the same scoring logic as Scheduler.Decide(): AdminWeight
// as base, relay penalty for relay-class paths, degraded-health penalty,
// and quality penalties for RTT and loss when measurements are available.
// The score is clamped to [0, 100].
//
// This function is provided so external callers (e.g., the multi-WAN
// stickiness policy) can compare candidate scores without running a full
// scheduling decision. It is the single authoritative score formula; callers
// must not duplicate the scoring logic independently.
func ScoreCandidate(c PathCandidate) int {
	scored := scoreCandidates([]PathCandidate{c})
	if len(scored) == 0 {
		return 0
	}
	return scored[0].score
}

// NewScheduler creates a new Scheduler with the given stripe match thresholds.
// Use DefaultStripeMatchThresholds for conservative v1 defaults.
func NewScheduler(thresholds StripeMatchThresholds) *Scheduler {
	return &Scheduler{
		thresholds: thresholds,
		counters:   make(map[string]*AssociationCounters),
	}
}

// Decide computes the endpoint-owned scheduling decision for the given
// association and set of path candidates.
//
// Decide filters candidates by association ID and health state, scores them,
// and picks the scheduling mode:
//   - ModeNoEligiblePath: no healthy candidates for this association
//   - ModeSinglePath: exactly one eligible candidate
//   - ModeWeightedBurstFlowlet: multiple candidates, not closely matched
//   - ModePerPacketStripe: multiple candidates, closely matched within thresholds
//
// The returned SchedulerDecision is always association-bound and always has
// a non-empty Reason field explaining the decision.
//
// Decide is safe for concurrent use.
func (s *Scheduler) Decide(associationID string, candidates []PathCandidate) SchedulerDecision {
	if associationID == "" {
		// Association-bound scheduling requires a valid association ID.
		// Returning no-eligible-path is the safest response for an invalid call.
		return SchedulerDecision{
			AssociationID: "",
			Mode:          ModeNoEligiblePath,
			Reason:        "association ID is empty; scheduling requires valid association context",
		}
	}

	// Filter to candidates that belong to this association and are health-eligible.
	// This enforces association-bound scheduling: candidates from other associations
	// must not influence this decision.
	eligible := filterEligible(associationID, candidates)

	var decision SchedulerDecision
	decision.AssociationID = associationID

	switch len(eligible) {
	case 0:
		decision.Mode = ModeNoEligiblePath
		decision.Reason = buildNoEligibleReason(associationID, candidates)
	case 1:
		decision.Mode = ModeSinglePath
		decision.ChosenPaths = []ChosenPath{{
			CandidateID: eligible[0].ID,
			Class:       eligible[0].Class,
			Weight:      100,
		}}
		decision.Reason = fmt.Sprintf(
			"single eligible path: id=%s class=%s health=%s",
			eligible[0].ID, eligible[0].Class, eligible[0].Health,
		)
	default:
		decision = s.decideMultiple(associationID, eligible)
	}

	s.recordDecision(associationID, decision)
	return decision
}

// decideMultiple handles scheduling when 2 or more eligible candidates exist.
// It scores all candidates, then picks weighted-burst-flowlet or per-packet-stripe
// based on whether the paths are closely matched.
func (s *Scheduler) decideMultiple(associationID string, eligible []PathCandidate) SchedulerDecision {
	scored := scoreCandidates(eligible)

	matched, mismatchReason := s.pathsCloselyMatched(scored)
	if matched {
		paths := buildChosenPaths(scored)
		return SchedulerDecision{
			AssociationID:   associationID,
			Mode:            ModePerPacketStripe,
			ChosenPaths:     paths,
			StripingAllowed: true,
			Reason:          buildStripeReason(scored, s.thresholds),
		}
	}

	// Default: weighted burst/flowlet — direct traffic to the best-scoring path.
	// The flowlet/burst detection itself is the carrier's concern; the scheduler
	// simply declares which path to use for the current decision epoch.
	best := scored[0]
	return SchedulerDecision{
		AssociationID: associationID,
		Mode:          ModeWeightedBurstFlowlet,
		ChosenPaths: []ChosenPath{{
			CandidateID: best.ID,
			Class:       best.Class,
			Weight:      100,
		}},
		Reason: buildBurstReason(scored, mismatchReason),
	}
}

// pathsCloselyMatched returns true when all scored candidates are within the
// configured stripe match thresholds for RTT, jitter, and loss spread.
//
// If any candidate is unmeasured or has insufficient confidence, striping is
// blocked. This is the conservative gate required by spec/v1-data-plane.md
// section 15.2: "Closely matched" requires actual measurements, not assumptions.
func (s *Scheduler) pathsCloselyMatched(scored []scoredCandidate) (bool, string) {
	if len(scored) < 2 {
		return false, "fewer than 2 candidates"
	}

	// All candidates must have sufficient measurement confidence.
	// Per-packet striping on unmeasured paths is unsafe: we cannot verify
	// that the paths are actually similar without measurement data.
	for _, c := range scored {
		if !c.Quality.Measured() {
			return false, fmt.Sprintf("candidate %s is unmeasured", c.ID)
		}
		if c.Quality.Confidence < s.thresholds.MinConfidence {
			return false, fmt.Sprintf("candidate %s confidence %.2f below threshold %.2f", c.ID, c.Quality.Confidence, s.thresholds.MinConfidence)
		}
	}

	// Compute the spread (max - min) across all candidates for each metric.
	minRTT, maxRTT := scored[0].Quality.RTT, scored[0].Quality.RTT
	minJitter, maxJitter := scored[0].Quality.Jitter, scored[0].Quality.Jitter
	minLoss, maxLoss := scored[0].Quality.LossFraction, scored[0].Quality.LossFraction

	for _, c := range scored[1:] {
		if c.Quality.RTT < minRTT {
			minRTT = c.Quality.RTT
		}
		if c.Quality.RTT > maxRTT {
			maxRTT = c.Quality.RTT
		}
		if c.Quality.Jitter < minJitter {
			minJitter = c.Quality.Jitter
		}
		if c.Quality.Jitter > maxJitter {
			maxJitter = c.Quality.Jitter
		}
		if c.Quality.LossFraction < minLoss {
			minLoss = c.Quality.LossFraction
		}
		if c.Quality.LossFraction > maxLoss {
			maxLoss = c.Quality.LossFraction
		}
	}

	rttSpread := maxRTT - minRTT
	jitterSpread := maxJitter - minJitter
	lossSpread := maxLoss - minLoss

	if rttSpread > s.thresholds.MaxRTTSpread {
		return false, fmt.Sprintf("rtt spread %v exceeds threshold %v", rttSpread, s.thresholds.MaxRTTSpread)
	}
	if jitterSpread > s.thresholds.MaxJitterSpread {
		return false, fmt.Sprintf("jitter spread %v exceeds threshold %v", jitterSpread, s.thresholds.MaxJitterSpread)
	}
	if lossSpread > s.thresholds.MaxLossSpread {
		return false, fmt.Sprintf("loss spread %.2f%% exceeds threshold %.2f%%", lossSpread*100, s.thresholds.MaxLossSpread*100)
	}

	return true, ""
}

// CountersSnapshot returns a sorted copy of all per-association decision counters.
// These counters make scheduling behavior observable without requiring access to
// individual SchedulerDecision values.
func (s *Scheduler) CountersSnapshot() []AssociationCountersSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]AssociationCountersSnapshot, 0, len(s.counters))
	for assocID, c := range s.counters {
		result = append(result, AssociationCountersSnapshot{
			AssociationID:     assocID,
			TotalDecisions:    c.TotalDecisions.Load(),
			WeightedBurst:     c.WeightedBurst.Load(),
			PerPacketStripe:   c.PerPacketStripe.Load(),
			SinglePath:        c.SinglePath.Load(),
			NoEligiblePath:    c.NoEligiblePath.Load(),
			StripingActivated: c.StripingActivated.Load(),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].AssociationID < result[j].AssociationID
	})
	return result
}

// recordDecision updates per-association atomic counters.
// The map entry is created under the mutex; counter increments use atomics.
func (s *Scheduler) recordDecision(associationID string, decision SchedulerDecision) {
	s.mu.Lock()
	c, exists := s.counters[associationID]
	if !exists {
		c = &AssociationCounters{}
		s.counters[associationID] = c
	}
	s.mu.Unlock()

	c.TotalDecisions.Add(1)
	switch decision.Mode {
	case ModeWeightedBurstFlowlet:
		c.WeightedBurst.Add(1)
	case ModePerPacketStripe:
		c.PerPacketStripe.Add(1)
		if decision.StripingAllowed {
			c.StripingActivated.Add(1)
		}
	case ModeSinglePath:
		c.SinglePath.Add(1)
	case ModeNoEligiblePath:
		c.NoEligiblePath.Add(1)
	}
}

// --- internal helpers ---

// scoredCandidate pairs a PathCandidate with its computed composite score.
type scoredCandidate struct {
	PathCandidate
	score int // composite score; higher is better; clamped to [0, 100]
}

// filterEligible returns only those candidates that:
//  1. have a matching AssociationID (association-bound scheduling)
//  2. have an eligible HealthState (not failed or admin-disabled)
//  3. have a non-empty ID (invalid candidates are excluded)
func filterEligible(associationID string, candidates []PathCandidate) []PathCandidate {
	eligible := candidates[:0:0] // reuse backing array memory; len=0
	for _, c := range candidates {
		if c.AssociationID != associationID {
			continue // not for this association; scheduling is association-bound
		}
		if !c.Health.IsEligible() {
			continue // failed, admin-disabled, or probe-only: cannot carry live traffic
		}
		if c.ID == "" {
			continue // invalid candidate; skip
		}
		eligible = append(eligible, c)
	}
	return eligible
}

// scoreCandidates computes composite scores and returns a sorted slice,
// highest score first. Ties are broken by candidate ID (lexicographic) for
// determinism.
//
// Score composition (max base = AdminWeight, which defaults to 100):
//   - relayPenalty (-10) for coordinator/node relay paths: direct paths preferred
//     when quality is comparable; relay adds latency and a failure point
//   - degradedPenalty (-20) for degraded health: degraded paths are lower-priority
//   - quality penalties (applied only when Quality.Measured() is true):
//   - loss: -1 per 1% loss fraction (i.e., LossFraction * 100)
//   - RTT: -1 per 50ms above a 50ms base threshold
//
// Score is clamped to [0, 100].
func scoreCandidates(candidates []PathCandidate) []scoredCandidate {
	scored := make([]scoredCandidate, len(candidates))
	for i, c := range candidates {
		s := int(c.effectiveAdminWeight())

		// Relay paths are penalized to preserve direct-path preference.
		// The architecture requires: direct preferred over relay when quality
		// is comparable (spec/v1-data-plane.md section 8.1).
		if c.Class.IsRelay() {
			s -= relayPenalty
		}

		// Degraded paths are still eligible but scored lower.
		if c.Health == HealthStateDegraded {
			s -= degradedPenalty
		}

		// Quality penalties are only applied when measurements are available.
		// Applying penalties to unmeasured paths would be misleading.
		if c.Quality.Measured() {
			// Loss penalty: -1 per 1% loss fraction.
			lossPenalty := int(c.Quality.LossFraction * 100)
			s -= lossPenalty

			// RTT penalty: -1 per 50ms above a 50ms base threshold.
			// Paths with RTT <= 50ms receive no RTT penalty.
			const rttBase = 50 * time.Millisecond
			if c.Quality.RTT > rttBase {
				excess := c.Quality.RTT - rttBase
				s -= int(excess / (50 * time.Millisecond))
			}
		}

		// Clamp to [0, 100].
		if s < 0 {
			s = 0
		}
		if s > 100 {
			s = 100
		}

		scored[i] = scoredCandidate{PathCandidate: c, score: s}
	}

	// Sort descending by score; ties broken by ID for determinism.
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].ID < scored[j].ID
	})

	return scored
}

// buildChosenPaths builds the ChosenPath slice from scored candidates.
// Weights are proportional to scores with a minimum of 1.
func buildChosenPaths(scored []scoredCandidate) []ChosenPath {
	paths := make([]ChosenPath, len(scored))
	for i, c := range scored {
		w := uint8(c.score)
		if w < 1 {
			w = 1
		}
		paths[i] = ChosenPath{
			CandidateID: c.ID,
			Class:       c.Class,
			Weight:      w,
		}
	}
	return paths
}

// buildNoEligibleReason explains why no eligible paths exist for an association.
func buildNoEligibleReason(associationID string, all []PathCandidate) string {
	if len(all) == 0 {
		return fmt.Sprintf("association %s: no path candidates provided", associationID)
	}

	var reasons []string
	matchCount := 0
	for _, c := range all {
		if c.AssociationID != associationID {
			continue
		}
		matchCount++
		if !c.Health.IsEligible() {
			reasons = append(reasons, fmt.Sprintf("candidate %s health=%s", c.ID, c.Health))
		} else if c.ID == "" {
			reasons = append(reasons, "candidate with empty ID skipped")
		}
	}

	if matchCount == 0 {
		return fmt.Sprintf("association %s: no candidates with matching association ID", associationID)
	}
	if len(reasons) == 0 {
		return fmt.Sprintf("association %s: candidates exist but all filtered for unknown reason", associationID)
	}
	return fmt.Sprintf("association %s: no eligible paths; ineligible: %s",
		associationID, strings.Join(reasons, "; "))
}

// buildBurstReason explains why weighted burst/flowlet mode was chosen.
func buildBurstReason(scored []scoredCandidate, mismatchReason string) string {
	best := scored[0]
	msg := fmt.Sprintf("mode=weighted-burst-flowlet; best path: id=%s class=%s score=%d",
		best.ID, best.Class, best.score)
	if len(scored) > 1 {
		runner := scored[1]
		msg += fmt.Sprintf("; runner-up: id=%s class=%s score=%d; striping-blocked: %s",
			runner.ID, runner.Class, runner.score, mismatchReason)
	}
	return msg
}

// buildStripeReason explains why per-packet striping was activated.
func buildStripeReason(scored []scoredCandidate, thresholds StripeMatchThresholds) string {
	ids := make([]string, len(scored))
	for i, c := range scored {
		ids[i] = fmt.Sprintf("%s(score=%d)", c.ID, c.score)
	}
	return fmt.Sprintf(
		"mode=per-packet-stripe; %d paths within match thresholds (rtt_spread<%s jitter_spread<%s loss_spread<%.2f%%); paths: %s",
		len(scored),
		thresholds.MaxRTTSpread,
		thresholds.MaxJitterSpread,
		thresholds.MaxLossSpread*100,
		strings.Join(ids, ", "),
	)
}
