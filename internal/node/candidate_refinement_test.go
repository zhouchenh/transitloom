package node

import (
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/scheduler"
	"github.com/zhouchenh/transitloom/internal/transport"
)

// makeDistributed builds a minimal DistributedPathCandidate for testing.
func makeDistributed(id, assocID, class, remoteEndpoint string, isRelay bool) controlplane.DistributedPathCandidate {
	return controlplane.DistributedPathCandidate{
		CandidateID:     id,
		AssociationID:   assocID,
		Class:           class,
		IsRelayAssisted: isRelay,
		RemoteEndpoint:  remoteEndpoint,
		AdminWeight:     100,
	}
}

// makeRegistry builds an EndpointRegistry with one entry for host:port at the
// given verification state.
func makeRegistry(host string, port uint16, state transport.VerificationState) *transport.EndpointRegistry {
	reg := transport.NewEndpointRegistry()
	ep := transport.ExternalEndpoint{
		Host:         host,
		Port:         port,
		Source:       transport.EndpointSourceConfigured,
		Verification: state,
		RecordedAt:   time.Now(),
	}
	if state == transport.VerificationStateStale || state == transport.VerificationStateFailed {
		ep.StaleAt = time.Now()
	}
	reg.Add(ep)
	return reg
}

// makeQualityStore builds a PathQualityStore with a fresh measurement for
// the given path ID.
func makeQualityStore(pathID string, rtt time.Duration) *scheduler.PathQualityStore {
	store := scheduler.NewPathQualityStore(scheduler.DefaultQualityMaxAge)
	store.RecordProbeResult(pathID, rtt, true)
	return store
}

func TestRefineCandidates_EmptyInput(t *testing.T) {
	result := RefineCandidates(nil, nil, nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
	result = RefineCandidates([]controlplane.DistributedPathCandidate{}, nil, nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestRefineCandidates_InformationalExcluded(t *testing.T) {
	// A candidate without RemoteEndpoint is informational only and must be excluded.
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "", false),
	}
	refined := RefineCandidates(candidates, nil, nil)
	if len(refined) != 1 {
		t.Fatalf("expected 1 refined candidate, got %d", len(refined))
	}
	rc := refined[0]
	if rc.Usable {
		t.Error("informational candidate (no remote endpoint) must be Usable=false")
	}
	if rc.ExcludeReason == "" {
		t.Error("ExcludeReason must be non-empty when Usable=false")
	}
	if rc.DistributedID != "c1" {
		t.Errorf("DistributedID must be preserved, got %q", rc.DistributedID)
	}
}

func TestRefineCandidates_UsableWithoutRegistry(t *testing.T) {
	// A candidate with RemoteEndpoint and nil registry must be usable.
	// EndpointState should be Unknown (no registry data available).
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
	}
	refined := RefineCandidates(candidates, nil, nil)
	if len(refined) != 1 {
		t.Fatalf("expected 1 refined candidate, got %d", len(refined))
	}
	rc := refined[0]
	if !rc.Usable {
		t.Errorf("candidate with valid endpoint and nil registry must be Usable=true, got ExcludeReason=%q", rc.ExcludeReason)
	}
	if rc.EndpointState != CandidateEndpointUnknown {
		t.Errorf("expected CandidateEndpointUnknown with nil registry, got %q", rc.EndpointState)
	}
}

func TestRefineCandidates_EndpointFailed_ExcludesCandidate(t *testing.T) {
	// When the registry marks the endpoint as failed, the candidate must be excluded.
	// Failed = probe confirmed unreachable; we must not schedule to it.
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
	}
	registry := makeRegistry("1.2.3.4", 4500, transport.VerificationStateFailed)

	refined := RefineCandidates(candidates, registry, nil)
	if len(refined) != 1 {
		t.Fatalf("expected 1 refined candidate, got %d", len(refined))
	}
	rc := refined[0]
	if rc.Usable {
		t.Error("candidate with failed endpoint must be Usable=false")
	}
	if rc.ExcludeReason == "" {
		t.Error("ExcludeReason must be non-empty for failed endpoint exclusion")
	}
	if rc.EndpointState != CandidateEndpointFailed {
		t.Errorf("expected CandidateEndpointFailed, got %q", rc.EndpointState)
	}
}

func TestRefineCandidates_EndpointStale_DegradedNotExcluded(t *testing.T) {
	// Stale endpoint: candidate is still usable as a fallback but health is degraded.
	// Stale ≠ failed: we degrade, not exclude. The scheduler can still pick it
	// if it is the only available path.
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
	}
	registry := makeRegistry("1.2.3.4", 4500, transport.VerificationStateStale)

	refined := RefineCandidates(candidates, registry, nil)
	if len(refined) != 1 {
		t.Fatalf("expected 1 refined candidate, got %d", len(refined))
	}
	rc := refined[0]
	if !rc.Usable {
		t.Errorf("stale endpoint must still be Usable=true (degraded, not excluded): ExcludeReason=%q", rc.ExcludeReason)
	}
	if rc.Candidate.Health != scheduler.HealthStateDegraded {
		t.Errorf("stale endpoint must degrade health to HealthStateDegraded, got %q", rc.Candidate.Health)
	}
	if rc.DegradedReason == "" {
		t.Error("DegradedReason must be non-empty when health was downgraded")
	}
	if rc.EndpointState != CandidateEndpointStale {
		t.Errorf("expected CandidateEndpointStale, got %q", rc.EndpointState)
	}
}

func TestRefineCandidates_EndpointVerified_FullHealth(t *testing.T) {
	// Verified endpoint: no health penalty, EndpointState=Usable.
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
	}
	registry := makeRegistry("1.2.3.4", 4500, transport.VerificationStateVerified)

	refined := RefineCandidates(candidates, registry, nil)
	rc := refined[0]

	if !rc.Usable {
		t.Errorf("verified endpoint must be Usable=true: %q", rc.ExcludeReason)
	}
	if rc.Candidate.Health != scheduler.HealthStateActive {
		t.Errorf("verified endpoint must have HealthStateActive, got %q", rc.Candidate.Health)
	}
	if rc.DegradedReason != "" {
		t.Errorf("no degraded reason expected for verified endpoint, got %q", rc.DegradedReason)
	}
	if rc.EndpointState != CandidateEndpointUsable {
		t.Errorf("expected CandidateEndpointUsable, got %q", rc.EndpointState)
	}
}

func TestRefineCandidates_EndpointUnverified_StillUsable(t *testing.T) {
	// Unverified = operator configured but not yet probed. Still usable per spec:
	// configured endpoints represent explicit operator intent.
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
	}
	registry := makeRegistry("1.2.3.4", 4500, transport.VerificationStateUnverified)

	refined := RefineCandidates(candidates, registry, nil)
	rc := refined[0]

	if !rc.Usable {
		t.Errorf("unverified endpoint must still be Usable=true: %q", rc.ExcludeReason)
	}
	if rc.EndpointState != CandidateEndpointUsable {
		t.Errorf("expected CandidateEndpointUsable for unverified state, got %q", rc.EndpointState)
	}
}

func TestRefineCandidates_EndpointNotInRegistry_Unknown(t *testing.T) {
	// Endpoint not in registry: state is Unknown, candidate is usable.
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
	}
	// Registry exists but has a different endpoint.
	registry := makeRegistry("5.6.7.8", 9000, transport.VerificationStateVerified)

	refined := RefineCandidates(candidates, registry, nil)
	rc := refined[0]

	if !rc.Usable {
		t.Errorf("candidate with no registry entry must be Usable=true: %q", rc.ExcludeReason)
	}
	if rc.EndpointState != CandidateEndpointUnknown {
		t.Errorf("expected CandidateEndpointUnknown when endpoint not in registry, got %q", rc.EndpointState)
	}
}

func TestRefineCandidates_QualityEnriched_WhenFresh(t *testing.T) {
	// Fresh quality in the store must be applied to the candidate.
	// QualityFresh must be true and Candidate.Quality must be non-zero.
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
	}
	store := makeQualityStore("c1", 20*time.Millisecond)

	refined := RefineCandidates(candidates, nil, store)
	rc := refined[0]

	if !rc.Usable {
		t.Errorf("expected Usable=true, got %q", rc.ExcludeReason)
	}
	if !rc.QualityFresh {
		t.Error("QualityFresh must be true when store has a fresh measurement")
	}
	if !rc.Candidate.Quality.Measured() {
		t.Error("Candidate.Quality must be non-zero (measured) when fresh quality is applied")
	}
	if rc.Candidate.Quality.RTT == 0 {
		t.Error("expected non-zero RTT in quality after probe recording")
	}
}

func TestRefineCandidates_QualityUnmeasured_WhenNoStore(t *testing.T) {
	// nil quality store: quality stays zero (unmeasured, conservative behavior).
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
	}

	refined := RefineCandidates(candidates, nil, nil)
	rc := refined[0]

	if rc.QualityFresh {
		t.Error("QualityFresh must be false when quality store is nil")
	}
	if rc.Candidate.Quality.Measured() {
		t.Error("quality must be unmeasured (zero) when quality store is nil")
	}
}

func TestRefineCandidates_QualityUnmeasured_WhenNoEntry(t *testing.T) {
	// Quality store exists but has no entry for this path: quality stays zero.
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
	}
	store := makeQualityStore("other-path-id", 20*time.Millisecond) // different ID

	refined := RefineCandidates(candidates, nil, store)
	rc := refined[0]

	if rc.QualityFresh {
		t.Error("QualityFresh must be false when store has no entry for this path ID")
	}
	if rc.Candidate.Quality.Measured() {
		t.Error("quality must remain unmeasured when no store entry exists for this path")
	}
}

func TestRefineCandidates_StaleQuality_TreatedAsUnmeasured(t *testing.T) {
	// Stale quality (older than MaxAge) must be treated as unmeasured.
	// FreshQuality returns false for stale entries, so quality stays zero.
	// This verifies that stale measurements don't silently appear as current.
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
	}
	// Use a very short MaxAge so the measurement is immediately stale.
	store := scheduler.NewPathQualityStore(1 * time.Nanosecond)
	store.RecordProbeResult("c1", 20*time.Millisecond, true)
	// Sleep briefly to ensure the measurement is older than 1ns.
	time.Sleep(1 * time.Millisecond)

	refined := RefineCandidates(candidates, nil, store)
	rc := refined[0]

	if rc.QualityFresh {
		t.Error("QualityFresh must be false when quality measurement is stale")
	}
	if rc.Candidate.Quality.Measured() {
		t.Error("stale quality must not appear as measured: it must be treated as unmeasured (confidence=0)")
	}
}

func TestRefineCandidates_ClassConversion(t *testing.T) {
	// Each distributed class must convert to the correct scheduler class.
	// Direct and relay classes must remain distinct after conversion.
	cases := []struct {
		distributedClass string
		isRelayAssisted  bool
		expectedClass    scheduler.PathClass
		expectRelay      bool
	}{
		{controlplane.DistributedPathClassDirectPublic, false, scheduler.PathClassDirectPublic, false},
		{controlplane.DistributedPathClassDirectIntranet, false, scheduler.PathClassDirectIntranet, false},
		{controlplane.DistributedPathClassCoordinatorRelay, true, scheduler.PathClassCoordinatorRelay, true},
		{controlplane.DistributedPathClassNodeRelay, true, scheduler.PathClassNodeRelay, true},
	}
	for _, tc := range cases {
		candidates := []controlplane.DistributedPathCandidate{
			{
				CandidateID:     "c1",
				AssociationID:   "assoc1",
				Class:           tc.distributedClass,
				IsRelayAssisted: tc.isRelayAssisted,
				RemoteEndpoint:  "1.2.3.4:4500",
				RelayNodeID: func() string {
					if tc.isRelayAssisted {
						return "relay-node-1"
					}
					return ""
				}(),
				AdminWeight: 100,
			},
		}
		refined := RefineCandidates(candidates, nil, nil)
		rc := refined[0]
		if !rc.Usable {
			t.Errorf("class %q: expected Usable=true", tc.distributedClass)
			continue
		}
		if rc.Candidate.Class != tc.expectedClass {
			t.Errorf("class %q: expected scheduler class %q, got %q",
				tc.distributedClass, tc.expectedClass, rc.Candidate.Class)
		}
		if got := rc.Candidate.Class.IsRelay(); got != tc.expectRelay {
			t.Errorf("class %q: IsRelay() expected %v, got %v",
				tc.distributedClass, tc.expectRelay, got)
		}
	}
}

func TestUsableSchedulerCandidates(t *testing.T) {
	// UsableSchedulerCandidates must return only Usable=true candidates,
	// preserving the scheduler's input contract.
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
		makeDistributed("c2", "assoc1", controlplane.DistributedPathClassDirectPublic, "", false),       // informational → excluded
		makeDistributed("c3", "assoc1", controlplane.DistributedPathClassCoordinatorRelay, "5.6.7.8:5000", true), // usable relay
	}
	// c2's CandidateID requires a relay node ID for relay candidates; fix it:
	candidates[2].RelayNodeID = "relay-1"

	refined := RefineCandidates(candidates, nil, nil)
	schedulerCandidates := UsableSchedulerCandidates(refined)

	if len(schedulerCandidates) != 2 {
		t.Fatalf("expected 2 usable scheduler candidates (c1, c3), got %d", len(schedulerCandidates))
	}

	ids := map[string]bool{}
	for _, sc := range schedulerCandidates {
		ids[sc.ID] = true
	}
	if !ids["c1"] {
		t.Error("c1 (direct, usable) must appear in scheduler candidates")
	}
	if ids["c2"] {
		t.Error("c2 (informational, excluded) must NOT appear in scheduler candidates")
	}
	if !ids["c3"] {
		t.Error("c3 (relay, usable) must appear in scheduler candidates")
	}
}

func TestRefinedCandidates_DirectRelayDistinct(t *testing.T) {
	// Direct and relay candidates must remain architecturally distinct.
	// After refinement: direct → IsRelay()=false, relay → IsRelay()=true.
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
		{
			CandidateID:     "c2",
			AssociationID:   "assoc1",
			Class:           controlplane.DistributedPathClassCoordinatorRelay,
			IsRelayAssisted: true,
			RemoteEndpoint:  "9.9.9.9:7000",
			RelayNodeID:     "relay-coord",
			AdminWeight:     100,
		},
	}

	refined := RefineCandidates(candidates, nil, nil)
	schedulerCandidates := UsableSchedulerCandidates(refined)

	if len(schedulerCandidates) != 2 {
		t.Fatalf("expected 2 scheduler candidates, got %d", len(schedulerCandidates))
	}

	for _, sc := range schedulerCandidates {
		if sc.ID == "c1" && sc.Class.IsRelay() {
			t.Error("direct candidate c1 must have IsRelay()=false")
		}
		if sc.ID == "c2" && !sc.Class.IsRelay() {
			t.Error("relay candidate c2 must have IsRelay()=true")
		}
	}
}

func TestRefinedCandidates_ExcludeReason_InspectableForTests(t *testing.T) {
	// ExcludeReason must be non-empty and inspectable for every excluded candidate.
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "", false),
	}
	registry := makeRegistry("1.2.3.4", 4500, transport.VerificationStateFailed)
	candidates2 := []controlplane.DistributedPathCandidate{
		makeDistributed("c2", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
	}

	refined1 := RefineCandidates(candidates, nil, nil)
	if refined1[0].ExcludeReason == "" {
		t.Error("no-endpoint exclusion: ExcludeReason must be non-empty")
	}

	refined2 := RefineCandidates(candidates2, registry, nil)
	if refined2[0].ExcludeReason == "" {
		t.Error("failed-endpoint exclusion: ExcludeReason must be non-empty")
	}
}

func TestRefineCandidates_AdminWeightPreserved(t *testing.T) {
	// AdminWeight from the distributed candidate must be preserved in the
	// scheduler.PathCandidate. The scheduler uses AdminWeight for scoring.
	c := makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false)
	c.AdminWeight = 50
	refined := RefineCandidates([]controlplane.DistributedPathCandidate{c}, nil, nil)
	rc := refined[0]
	if !rc.Usable {
		t.Fatalf("expected Usable=true: %q", rc.ExcludeReason)
	}
	if rc.Candidate.AdminWeight != 50 {
		t.Errorf("AdminWeight must be preserved as 50, got %d", rc.Candidate.AdminWeight)
	}
}

func TestRefineCandidates_MeteredFlagPreserved(t *testing.T) {
	c := makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false)
	c.IsMetered = true
	refined := RefineCandidates([]controlplane.DistributedPathCandidate{c}, nil, nil)
	rc := refined[0]
	if !rc.Usable {
		t.Fatalf("expected Usable=true: %q", rc.ExcludeReason)
	}
	if !rc.Candidate.IsMetered {
		t.Error("IsMetered must be preserved from distributed candidate")
	}
}

func TestRefineCandidates_AssociationIDPreserved(t *testing.T) {
	// The scheduler requires Candidate.AssociationID to match the target
	// association. Verify it is correctly preserved.
	c := makeDistributed("c1", "my-assoc-id", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false)
	refined := RefineCandidates([]controlplane.DistributedPathCandidate{c}, nil, nil)
	rc := refined[0]
	if rc.Candidate.AssociationID != "my-assoc-id" {
		t.Errorf("AssociationID must be preserved, got %q", rc.Candidate.AssociationID)
	}
}

func TestRefineCandidates_MultipleEntries_WorstState(t *testing.T) {
	// When multiple registry entries match the same host:port, the most severe
	// state must be used. A mix of verified + stale must produce stale.
	reg := transport.NewEndpointRegistry()
	now := time.Now()

	// Add one verified and one stale record for the same address.
	reg.Add(transport.ExternalEndpoint{
		Host:         "1.2.3.4",
		Port:         4500,
		Source:       transport.EndpointSourceConfigured,
		Verification: transport.VerificationStateVerified,
		RecordedAt:   now,
		VerifiedAt:   now,
	})
	reg.Add(transport.ExternalEndpoint{
		Host:         "1.2.3.4",
		Port:         4500,
		Source:       transport.EndpointSourceCoordinatorObserved,
		Verification: transport.VerificationStateStale,
		RecordedAt:   now,
		StaleAt:      now,
	})

	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
	}
	refined := RefineCandidates(candidates, reg, nil)
	rc := refined[0]

	// The most severe state (stale) must win.
	if rc.EndpointState != CandidateEndpointStale {
		t.Errorf("multiple entries: expected CandidateEndpointStale (worst of verified+stale), got %q", rc.EndpointState)
	}
	if rc.Candidate.Health != scheduler.HealthStateDegraded {
		t.Errorf("stale endpoint from multiple entries must degrade health, got %q", rc.Candidate.Health)
	}
}

func TestRefinedCandidates_RefinedDistinctFromChosenPath(t *testing.T) {
	// This test verifies the architectural separation: refined candidates are
	// inputs to the scheduler; the scheduler decision (chosen path) is separate.
	//
	// We verify that:
	//   1. RefinedCandidate.Candidate is NOT a SchedulerDecision
	//   2. UsableSchedulerCandidates returns PathCandidates, not decisions
	//   3. Only after Scheduler.Decide() is a path actually chosen
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
		{
			CandidateID:     "c2",
			AssociationID:   "assoc1",
			Class:           controlplane.DistributedPathClassCoordinatorRelay,
			IsRelayAssisted: true,
			RemoteEndpoint:  "5.6.7.8:5000",
			RelayNodeID:     "relay-1",
			AdminWeight:     100,
		},
	}

	refined := RefineCandidates(candidates, nil, nil)
	schedulerInputs := UsableSchedulerCandidates(refined)

	// Refined candidates are scheduler.PathCandidate, not SchedulerDecision.
	// Type system enforces this, but verify the slice feeds into Scheduler.Decide().
	sched := scheduler.NewScheduler(scheduler.DefaultStripeMatchThresholds())
	decision := sched.Decide("assoc1", schedulerInputs)

	// The decision is produced separately — it is not embedded in RefinedCandidate.
	if decision.Mode == scheduler.ModeNoEligiblePath {
		t.Error("scheduler must have chosen a path from 2 usable refined candidates")
	}
	if len(decision.ChosenPaths) == 0 {
		t.Error("ChosenPaths must be non-empty after deciding from refined candidates")
	}
}

// BenchmarkRefineCandidates measures the cost of refining a small set of
// candidates with a populated registry and quality store. This is the
// typical repeated path in scheduler integration.
func BenchmarkRefineCandidates(b *testing.B) {
	candidates := []controlplane.DistributedPathCandidate{
		makeDistributed("c1", "assoc1", controlplane.DistributedPathClassDirectPublic, "1.2.3.4:4500", false),
		{
			CandidateID:     "c2",
			AssociationID:   "assoc1",
			Class:           controlplane.DistributedPathClassCoordinatorRelay,
			IsRelayAssisted: true,
			RemoteEndpoint:  "9.9.9.9:7000",
			RelayNodeID:     "relay-1",
			AdminWeight:     100,
		},
		makeDistributed("c3", "assoc1", controlplane.DistributedPathClassDirectIntranet, "192.168.1.5:4500", false),
	}

	reg := makeRegistry("1.2.3.4", 4500, transport.VerificationStateVerified)
	reg.Add(transport.ExternalEndpoint{
		Host:         "192.168.1.5",
		Port:         4500,
		Source:       transport.EndpointSourceConfigured,
		Verification: transport.VerificationStateStale,
		RecordedAt:   time.Now(),
		StaleAt:      time.Now(),
	})

	store := scheduler.NewPathQualityStore(scheduler.DefaultQualityMaxAge)
	store.RecordProbeResult("c1", 15*time.Millisecond, true)
	store.RecordProbeResult("c2", 30*time.Millisecond, true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RefineCandidates(candidates, reg, store)
	}
}
