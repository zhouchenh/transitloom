package scheduler

import (
	"testing"
	"time"
)

// --- PathCandidate helpers for test brevity ---

func directCandidate(id, assocID string, health HealthState) PathCandidate {
	return PathCandidate{
		ID:            id,
		AssociationID: assocID,
		Class:         PathClassDirectPublic,
		Health:        health,
		AdminWeight:   100,
	}
}

func relayCandidate(id, assocID string, health HealthState) PathCandidate {
	return PathCandidate{
		ID:            id,
		AssociationID: assocID,
		Class:         PathClassCoordinatorRelay,
		Health:        health,
		AdminWeight:   100,
	}
}

func withQuality(c PathCandidate, rtt, jitter time.Duration, loss, confidence float64) PathCandidate {
	c.Quality = PathQuality{
		RTT:          rtt,
		Jitter:       jitter,
		LossFraction: loss,
		Confidence:   confidence,
	}
	return c
}

// --- Tests ---

func TestHealthStateIsEligible(t *testing.T) {
	eligible := []HealthState{
		HealthStateActive, HealthStateCandidate, HealthStateDegraded, HealthStateStandby,
	}
	for _, h := range eligible {
		if !h.IsEligible() {
			t.Errorf("expected %s to be eligible", h)
		}
	}

	ineligible := []HealthState{
		HealthStateFailed, HealthStateAdminDisabled, HealthStateProbeOnly,
	}
	for _, h := range ineligible {
		if h.IsEligible() {
			t.Errorf("expected %s to be ineligible", h)
		}
	}
}

func TestPathClassIsRelay(t *testing.T) {
	relay := []PathClass{PathClassCoordinatorRelay, PathClassNodeRelay}
	for _, c := range relay {
		if !c.IsRelay() {
			t.Errorf("expected %s to be relay", c)
		}
	}

	direct := []PathClass{PathClassDirectPublic, PathClassDirectIntranet}
	for _, c := range direct {
		if c.IsRelay() {
			t.Errorf("expected %s to not be relay", c)
		}
	}
}

func TestPathQualityMeasured(t *testing.T) {
	cases := []struct {
		q        PathQuality
		measured bool
	}{
		{PathQuality{}, false},
		{PathQuality{RTT: 10 * time.Millisecond}, false}, // no confidence
		{PathQuality{Confidence: 0.8}, false},             // no RTT
		{PathQuality{RTT: 10 * time.Millisecond, Confidence: 0.8}, true},
	}
	for _, tc := range cases {
		got := tc.q.Measured()
		if got != tc.measured {
			t.Errorf("Measured() = %v, want %v for %+v", got, tc.measured, tc.q)
		}
	}
}

func TestDecideEmptyAssociationID(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	dec := s.Decide("", nil)
	if dec.Mode != ModeNoEligiblePath {
		t.Errorf("empty association ID: got mode %s, want %s", dec.Mode, ModeNoEligiblePath)
	}
	if dec.Reason == "" {
		t.Error("expected non-empty Reason for empty association ID")
	}
}

func TestDecideNoCandidates(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	dec := s.Decide("assoc-1", nil)
	if dec.Mode != ModeNoEligiblePath {
		t.Errorf("no candidates: got mode %s, want %s", dec.Mode, ModeNoEligiblePath)
	}
	if dec.AssociationID != "assoc-1" {
		t.Errorf("expected AssociationID=assoc-1, got %s", dec.AssociationID)
	}
	if dec.Reason == "" {
		t.Error("expected non-empty Reason")
	}
}

func TestDecideAllCandidatesFailed(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	candidates := []PathCandidate{
		directCandidate("p1", "assoc-1", HealthStateFailed),
		directCandidate("p2", "assoc-1", HealthStateAdminDisabled),
		directCandidate("p3", "assoc-1", HealthStateProbeOnly),
	}
	dec := s.Decide("assoc-1", candidates)
	if dec.Mode != ModeNoEligiblePath {
		t.Errorf("all failed: got mode %s, want %s", dec.Mode, ModeNoEligiblePath)
	}
	if len(dec.ChosenPaths) != 0 {
		t.Errorf("expected no chosen paths, got %d", len(dec.ChosenPaths))
	}
}

func TestDecideSingleEligible(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	candidates := []PathCandidate{
		directCandidate("p1", "assoc-1", HealthStateActive),
		directCandidate("p2", "assoc-1", HealthStateFailed), // filtered out
	}
	dec := s.Decide("assoc-1", candidates)
	if dec.Mode != ModeSinglePath {
		t.Errorf("single eligible: got mode %s, want %s", dec.Mode, ModeSinglePath)
	}
	if len(dec.ChosenPaths) != 1 {
		t.Fatalf("expected 1 chosen path, got %d", len(dec.ChosenPaths))
	}
	if dec.ChosenPaths[0].CandidateID != "p1" {
		t.Errorf("expected chosen path p1, got %s", dec.ChosenPaths[0].CandidateID)
	}
	if dec.Reason == "" {
		t.Error("expected non-empty Reason")
	}
}

// TestDecideAssociationBound verifies that candidates for different associations
// do not influence decisions for the target association.
func TestDecideAssociationBound(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	candidates := []PathCandidate{
		directCandidate("p1", "assoc-A", HealthStateActive), // wrong association
		directCandidate("p2", "assoc-B", HealthStateActive), // correct association
		directCandidate("p3", "assoc-A", HealthStateActive), // wrong association
	}
	dec := s.Decide("assoc-B", candidates)
	if dec.Mode != ModeSinglePath {
		t.Errorf("association-bound: got mode %s, want %s", dec.Mode, ModeSinglePath)
	}
	if len(dec.ChosenPaths) != 1 || dec.ChosenPaths[0].CandidateID != "p2" {
		t.Errorf("expected only p2 chosen, got %+v", dec.ChosenPaths)
	}
}

// TestDecideDirectPreferredOverRelay verifies that a direct path scores higher
// than a relay path of the same admin weight and quality.
func TestDecideDirectPreferredOverRelay(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	candidates := []PathCandidate{
		relayCandidate("relay-1", "assoc-1", HealthStateActive),
		directCandidate("direct-1", "assoc-1", HealthStateActive),
	}
	dec := s.Decide("assoc-1", candidates)

	// With two candidates and no measurements, paths are not closely matched
	// (unmeasured paths block striping), so we expect burst/flowlet mode.
	if dec.Mode != ModeWeightedBurstFlowlet {
		t.Errorf("expected ModeWeightedBurstFlowlet, got %s", dec.Mode)
	}
	if len(dec.ChosenPaths) == 0 {
		t.Fatal("expected at least one chosen path")
	}
	// The direct path should be chosen (higher score due to no relay penalty).
	if dec.ChosenPaths[0].CandidateID != "direct-1" {
		t.Errorf("expected direct-1 chosen over relay-1, got %s", dec.ChosenPaths[0].CandidateID)
	}
	if dec.StripingAllowed {
		t.Error("striping must not be allowed for unmeasured paths")
	}
}

// TestDecideWeightedBurstFlowletForMismatchedPaths verifies that clearly
// mismatched paths (large RTT spread) yield burst/flowlet mode, not striping.
func TestDecideWeightedBurstFlowletForMismatchedPaths(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	candidates := []PathCandidate{
		withQuality(
			directCandidate("p1", "assoc-1", HealthStateActive),
			10*time.Millisecond, 2*time.Millisecond, 0.0, 0.9,
		),
		withQuality(
			directCandidate("p2", "assoc-1", HealthStateActive),
			// RTT spread = 90ms >> 20ms threshold → striping blocked
			100*time.Millisecond, 5*time.Millisecond, 0.0, 0.9,
		),
	}
	dec := s.Decide("assoc-1", candidates)
	if dec.Mode != ModeWeightedBurstFlowlet {
		t.Errorf("mismatched paths: got mode %s, want %s", dec.Mode, ModeWeightedBurstFlowlet)
	}
	if dec.StripingAllowed {
		t.Error("striping must not be allowed for mismatched paths")
	}
	// Best path (lower RTT = higher score) should be chosen.
	if len(dec.ChosenPaths) == 0 {
		t.Fatal("expected a chosen path")
	}
	if dec.ChosenPaths[0].CandidateID != "p1" {
		t.Errorf("expected p1 (lower RTT) chosen, got %s", dec.ChosenPaths[0].CandidateID)
	}
	if dec.Reason == "" {
		t.Error("expected non-empty Reason")
	}
}

// TestDecidePerPacketStripeForCloselyMatchedPaths verifies that closely matched
// paths (within all thresholds) yield per-packet striping mode.
func TestDecidePerPacketStripeForCloselyMatchedPaths(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	candidates := []PathCandidate{
		withQuality(
			directCandidate("p1", "assoc-1", HealthStateActive),
			// Both paths well within thresholds: RTT spread 5ms < 20ms,
			// jitter spread 0ms < 10ms, loss spread 0 < 1%.
			20*time.Millisecond, 3*time.Millisecond, 0.005, 0.9,
		),
		withQuality(
			directCandidate("p2", "assoc-1", HealthStateActive),
			25*time.Millisecond, 3*time.Millisecond, 0.005, 0.9,
		),
	}
	dec := s.Decide("assoc-1", candidates)
	if dec.Mode != ModePerPacketStripe {
		t.Errorf("closely matched paths: got mode %s, want %s", dec.Mode, ModePerPacketStripe)
	}
	if !dec.StripingAllowed {
		t.Error("striping must be allowed for closely matched paths")
	}
	if len(dec.ChosenPaths) != 2 {
		t.Errorf("expected 2 chosen paths, got %d", len(dec.ChosenPaths))
	}
	if dec.Reason == "" {
		t.Error("expected non-empty Reason")
	}
}

// TestDecideStripingBlockedByLossSpread verifies that high loss spread blocks
// per-packet striping even when RTT and jitter are closely matched.
func TestDecideStripingBlockedByLossSpread(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	candidates := []PathCandidate{
		withQuality(
			directCandidate("p1", "assoc-1", HealthStateActive),
			20*time.Millisecond, 3*time.Millisecond,
			0.001, // 0.1% loss
			0.9,
		),
		withQuality(
			directCandidate("p2", "assoc-1", HealthStateActive),
			22*time.Millisecond, 3*time.Millisecond,
			0.05, // 5% loss — spread = 4.9% >> 1% threshold
			0.9,
		),
	}
	dec := s.Decide("assoc-1", candidates)
	if dec.Mode != ModeWeightedBurstFlowlet {
		t.Errorf("high loss spread: got mode %s, want %s", dec.Mode, ModeWeightedBurstFlowlet)
	}
	if dec.StripingAllowed {
		t.Error("striping must not be allowed when loss spread exceeds threshold")
	}
}

// TestDecideStripingBlockedByLowConfidence verifies that unmeasured paths
// (low/zero confidence) block per-packet striping.
func TestDecideStripingBlockedByLowConfidence(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	candidates := []PathCandidate{
		withQuality(
			directCandidate("p1", "assoc-1", HealthStateActive),
			20*time.Millisecond, 3*time.Millisecond, 0.0, 0.9,
		),
		withQuality(
			directCandidate("p2", "assoc-1", HealthStateActive),
			// Confidence below MinConfidence threshold (0.3)
			22*time.Millisecond, 3*time.Millisecond, 0.0, 0.1,
		),
	}
	dec := s.Decide("assoc-1", candidates)
	if dec.Mode != ModeWeightedBurstFlowlet {
		t.Errorf("low confidence: got mode %s, want %s", dec.Mode, ModeWeightedBurstFlowlet)
	}
	if dec.StripingAllowed {
		t.Error("striping must not be allowed when confidence is below threshold")
	}
}

// TestDecideStripingBlockedForUnmeasuredPaths verifies that paths with no
// quality measurements at all block per-packet striping.
func TestDecideStripingBlockedForUnmeasuredPaths(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	candidates := []PathCandidate{
		directCandidate("p1", "assoc-1", HealthStateActive),
		directCandidate("p2", "assoc-1", HealthStateActive),
	}
	dec := s.Decide("assoc-1", candidates)
	if dec.Mode != ModeWeightedBurstFlowlet {
		t.Errorf("unmeasured paths: got mode %s, want %s", dec.Mode, ModeWeightedBurstFlowlet)
	}
	if dec.StripingAllowed {
		t.Error("striping must not be allowed for unmeasured paths")
	}
}

// TestDecideDegradedPathLowerPriority verifies that a degraded path scores
// below an active path of identical quality.
func TestDecideDegradedPathLowerPriority(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	degraded := PathCandidate{
		ID:            "degraded",
		AssociationID: "assoc-1",
		Class:         PathClassDirectPublic,
		Health:        HealthStateDegraded,
		AdminWeight:   100,
	}
	active := PathCandidate{
		ID:            "active",
		AssociationID: "assoc-1",
		Class:         PathClassDirectPublic,
		Health:        HealthStateActive,
		AdminWeight:   100,
	}
	candidates := []PathCandidate{degraded, active}
	dec := s.Decide("assoc-1", candidates)
	if dec.Mode != ModeWeightedBurstFlowlet {
		t.Errorf("degraded+active: got mode %s, want %s", dec.Mode, ModeWeightedBurstFlowlet)
	}
	if len(dec.ChosenPaths) == 0 {
		t.Fatal("expected a chosen path")
	}
	if dec.ChosenPaths[0].CandidateID != "active" {
		t.Errorf("expected active path chosen over degraded, got %s", dec.ChosenPaths[0].CandidateID)
	}
}

// TestDecideAdminWeightRespected verifies that higher AdminWeight wins when
// health and quality are otherwise equal.
func TestDecideAdminWeightRespected(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	low := PathCandidate{
		ID:            "low-weight",
		AssociationID: "assoc-1",
		Class:         PathClassDirectPublic,
		Health:        HealthStateActive,
		AdminWeight:   30,
	}
	high := PathCandidate{
		ID:            "high-weight",
		AssociationID: "assoc-1",
		Class:         PathClassDirectPublic,
		Health:        HealthStateActive,
		AdminWeight:   90,
	}
	candidates := []PathCandidate{low, high}
	dec := s.Decide("assoc-1", candidates)
	if dec.Mode != ModeWeightedBurstFlowlet {
		t.Errorf("expected ModeWeightedBurstFlowlet, got %s", dec.Mode)
	}
	if len(dec.ChosenPaths) == 0 {
		t.Fatal("expected a chosen path")
	}
	if dec.ChosenPaths[0].CandidateID != "high-weight" {
		t.Errorf("expected high-weight chosen, got %s", dec.ChosenPaths[0].CandidateID)
	}
}

// TestDecideCountersAccumulate verifies that counters increment correctly
// across multiple Decide calls.
func TestDecideCountersAccumulate(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	assocID := "assoc-counter"
	single := []PathCandidate{directCandidate("p1", assocID, HealthStateActive)}
	none := []PathCandidate{directCandidate("p1", assocID, HealthStateFailed)}

	s.Decide(assocID, single) // single-path
	s.Decide(assocID, single) // single-path
	s.Decide(assocID, none)   // no-eligible-path

	snaps := s.CountersSnapshot()
	if len(snaps) == 0 {
		t.Fatal("expected at least one counter snapshot")
	}
	var got *AssociationCountersSnapshot
	for i := range snaps {
		if snaps[i].AssociationID == assocID {
			got = &snaps[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("no counter snapshot for association %s", assocID)
	}
	if got.TotalDecisions != 3 {
		t.Errorf("TotalDecisions = %d, want 3", got.TotalDecisions)
	}
	if got.SinglePath != 2 {
		t.Errorf("SinglePath = %d, want 2", got.SinglePath)
	}
	if got.NoEligiblePath != 1 {
		t.Errorf("NoEligiblePath = %d, want 1", got.NoEligiblePath)
	}
}

// TestDecideStripingCounters verifies that striping counter increments when
// per-packet striping is activated.
func TestDecideStripingCounters(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	assocID := "assoc-stripe"
	matched := []PathCandidate{
		withQuality(directCandidate("p1", assocID, HealthStateActive),
			20*time.Millisecond, 3*time.Millisecond, 0.0, 0.9),
		withQuality(directCandidate("p2", assocID, HealthStateActive),
			22*time.Millisecond, 3*time.Millisecond, 0.0, 0.9),
	}

	s.Decide(assocID, matched) // should produce ModePerPacketStripe
	s.Decide(assocID, matched)

	snaps := s.CountersSnapshot()
	var got *AssociationCountersSnapshot
	for i := range snaps {
		if snaps[i].AssociationID == assocID {
			got = &snaps[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("no counter snapshot for %s", assocID)
	}
	if got.PerPacketStripe != 2 {
		t.Errorf("PerPacketStripe = %d, want 2", got.PerPacketStripe)
	}
	if got.StripingActivated != 2 {
		t.Errorf("StripingActivated = %d, want 2", got.StripingActivated)
	}
}

// TestDecideDecisionReason verifies that every decision has a non-empty Reason.
func TestDecideDecisionReason(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	assocID := "assoc-reason"
	cases := [][]PathCandidate{
		nil, // no-eligible
		{directCandidate("p1", assocID, HealthStateFailed)}, // no-eligible (ineligible)
		{directCandidate("p1", assocID, HealthStateActive)}, // single-path
		{directCandidate("p1", assocID, HealthStateActive),
			directCandidate("p2", assocID, HealthStateActive)}, // burst/flowlet (unmeasured)
		{withQuality(directCandidate("p1", assocID, HealthStateActive),
			20*time.Millisecond, 2*time.Millisecond, 0.0, 0.9),
			withQuality(directCandidate("p2", assocID, HealthStateActive),
				22*time.Millisecond, 2*time.Millisecond, 0.0, 0.9)}, // stripe
	}

	for i, tc := range cases {
		dec := s.Decide(assocID, tc)
		if dec.Reason == "" {
			t.Errorf("case %d: Reason must not be empty (mode=%s)", i, dec.Mode)
		}
		if dec.AssociationID != assocID {
			t.Errorf("case %d: AssociationID mismatch: got %s", i, dec.AssociationID)
		}
	}
}

// TestDecideRelayVsDirectWithQuality verifies that a direct path is preferred
// over a relay path when relay has higher RTT penalty.
func TestDecideRelayVsDirectWithQuality(t *testing.T) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	// Both measured, but relay gets both relay penalty and higher RTT.
	direct := withQuality(
		directCandidate("direct", "assoc-1", HealthStateActive),
		30*time.Millisecond, 2*time.Millisecond, 0.0, 0.9,
	)
	relay := withQuality(
		relayCandidate("relay", "assoc-1", HealthStateActive),
		60*time.Millisecond, 3*time.Millisecond, 0.0, 0.9,
	)
	dec := s.Decide("assoc-1", []PathCandidate{relay, direct})
	if len(dec.ChosenPaths) == 0 {
		t.Fatal("expected a chosen path")
	}
	if dec.ChosenPaths[0].CandidateID != "direct" {
		t.Errorf("expected direct preferred over relay, got %s", dec.ChosenPaths[0].CandidateID)
	}
}

// TestSchedulerStatus verifies that Status returns a snapshot with the
// configured thresholds.
func TestSchedulerStatus(t *testing.T) {
	thresholds := StripeMatchThresholds{
		MaxRTTSpread:    15 * time.Millisecond,
		MaxJitterSpread: 8 * time.Millisecond,
		MaxLossSpread:   0.02,
		MinConfidence:   0.5,
	}
	s := NewScheduler(thresholds)
	status := s.Status()
	if status.Thresholds.MaxRTTSpread != thresholds.MaxRTTSpread {
		t.Errorf("MaxRTTSpread mismatch")
	}
	if status.Thresholds.MaxLossSpread != thresholds.MaxLossSpread {
		t.Errorf("MaxLossSpread mismatch")
	}
}

// TestReportSchedulerStatus verifies the report is non-empty.
func TestReportSchedulerStatus(t *testing.T) {
	r := ReportSchedulerStatus()
	if len(r.Implemented) == 0 {
		t.Error("expected non-empty Implemented list")
	}
	if len(r.NotImplemented) == 0 {
		t.Error("expected non-empty NotImplemented list")
	}
}

// TestFilterEligibleExcludesWrongAssociation is a unit test for filterEligible.
func TestFilterEligibleExcludesWrongAssociation(t *testing.T) {
	candidates := []PathCandidate{
		directCandidate("p1", "assoc-A", HealthStateActive),
		directCandidate("p2", "assoc-B", HealthStateActive),
		directCandidate("p3", "assoc-A", HealthStateFailed),
	}
	result := filterEligible("assoc-A", candidates)
	if len(result) != 1 || result[0].ID != "p1" {
		t.Errorf("expected only p1, got %+v", result)
	}
}

// TestScoreCandidatesDirectOverRelay verifies scoring ranking.
func TestScoreCandidatesDirectOverRelay(t *testing.T) {
	candidates := []PathCandidate{
		relayCandidate("relay", "a", HealthStateActive),
		directCandidate("direct", "a", HealthStateActive),
	}
	scored := scoreCandidates(candidates)
	if len(scored) != 2 {
		t.Fatalf("expected 2 scored candidates, got %d", len(scored))
	}
	if scored[0].ID != "direct" {
		t.Errorf("expected direct to score higher, got %s first", scored[0].ID)
	}
	if scored[0].score <= scored[1].score {
		t.Errorf("direct score %d should be > relay score %d",
			scored[0].score, scored[1].score)
	}
}

// TestScoreCandidatesDegradedLower verifies degraded health reduces score.
func TestScoreCandidatesDegradedLower(t *testing.T) {
	candidates := []PathCandidate{
		{ID: "degraded", AssociationID: "a", Class: PathClassDirectPublic,
			Health: HealthStateDegraded, AdminWeight: 100},
		{ID: "active", AssociationID: "a", Class: PathClassDirectPublic,
			Health: HealthStateActive, AdminWeight: 100},
	}
	scored := scoreCandidates(candidates)
	if scored[0].ID != "active" {
		t.Errorf("expected active to score higher, got %s first", scored[0].ID)
	}
}

// --- Benchmark ---

// BenchmarkDecideTwoCandidates measures the cost of a typical scheduler
// decision with two path candidates. This is the hot path for multi-WAN
// associations.
func BenchmarkDecideTwoCandidates(b *testing.B) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	assocID := "bench-assoc"
	candidates := []PathCandidate{
		withQuality(directCandidate("direct", assocID, HealthStateActive),
			20*time.Millisecond, 3*time.Millisecond, 0.001, 0.9),
		withQuality(relayCandidate("relay", assocID, HealthStateActive),
			50*time.Millisecond, 5*time.Millisecond, 0.002, 0.8),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Decide(assocID, candidates)
	}
}

// BenchmarkDecideFourCandidates measures decision cost with four candidates.
func BenchmarkDecideFourCandidates(b *testing.B) {
	s := NewScheduler(DefaultStripeMatchThresholds())
	assocID := "bench-assoc-4"
	candidates := []PathCandidate{
		withQuality(directCandidate("d1", assocID, HealthStateActive),
			20*time.Millisecond, 3*time.Millisecond, 0.001, 0.9),
		withQuality(directCandidate("d2", assocID, HealthStateActive),
			25*time.Millisecond, 4*time.Millisecond, 0.002, 0.85),
		withQuality(relayCandidate("r1", assocID, HealthStateActive),
			50*time.Millisecond, 5*time.Millisecond, 0.003, 0.8),
		withQuality(relayCandidate("r2", assocID, HealthStateDegraded),
			80*time.Millisecond, 10*time.Millisecond, 0.01, 0.7),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Decide(assocID, candidates)
	}
}
