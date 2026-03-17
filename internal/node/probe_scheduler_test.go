package node

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/scheduler"
	"github.com/zhouchenh/transitloom/internal/transport"
)

// --- fake probe executor ---

// fakeProbeExecutor is a configurable in-memory probe executor for tests.
// It returns preset results keyed by "host:port" and never does network I/O.
type fakeProbeExecutor struct {
	results map[string]transport.ProbeResult
	err     map[string]error
}

func newFakeProbeExecutor() *fakeProbeExecutor {
	return &fakeProbeExecutor{
		results: make(map[string]transport.ProbeResult),
		err:     make(map[string]error),
	}
}

func (f *fakeProbeExecutor) setResult(host string, port uint16, reachable bool, rtt time.Duration) {
	key := fmt.Sprintf("%s:%d", host, port)
	f.results[key] = transport.ProbeResult{
		TargetHost:    host,
		TargetPort:    port,
		Reachable:     reachable,
		RoundTripTime: rtt,
		ProbedAt:      time.Now(),
	}
}

func (f *fakeProbeExecutor) setError(host string, port uint16, err error) {
	key := fmt.Sprintf("%s:%d", host, port)
	f.err[key] = err
}

func (f *fakeProbeExecutor) Execute(_ context.Context, c transport.ProbeCandidate) (transport.ProbeResult, error) {
	key := fmt.Sprintf("%s:%d", c.Host, c.Port)
	if err := f.err[key]; err != nil {
		return transport.ProbeResult{}, err
	}
	if r, ok := f.results[key]; ok {
		return r, nil
	}
	// Default: not reachable.
	return transport.ProbeResult{
		TargetHost: c.Host,
		TargetPort: c.Port,
		Reachable:  false,
		ProbedAt:   time.Now(),
	}, nil
}

// --- helpers ---

// newRegistry builds an EndpointRegistry populated with the given endpoints.
func newRegistry(eps []transport.ExternalEndpoint) *transport.EndpointRegistry {
	r := transport.NewEndpointRegistry()
	for _, ep := range eps {
		r.Add(ep)
	}
	return r
}

// configuredEP creates a configured, unverified ExternalEndpoint.
func configuredEP(host string, port uint16) transport.ExternalEndpoint {
	ep := transport.NewConfiguredEndpoint(host, port, 0)
	return ep
}

// staleEP creates a configured endpoint that was previously verified and is now stale.
// VerifiedAt is set (non-zero) so BuildCandidatesFromEndpoints assigns
// CandidateReasonPreviouslyVerified to it.
func staleEP(host string, port uint16) transport.ExternalEndpoint {
	ep := configuredEP(host, port)
	ep.MarkVerified(time.Now().Add(-2 * time.Hour)) // was verified in the past
	ep.MarkStale(time.Now())
	return ep
}

// failedEP creates a configured endpoint that was previously verified and is now failed.
// VerifiedAt is set (non-zero) so BuildCandidatesFromEndpoints assigns
// CandidateReasonPreviouslyVerified to it.
func failedEP(host string, port uint16) transport.ExternalEndpoint {
	ep := configuredEP(host, port)
	ep.MarkVerified(time.Now().Add(-2 * time.Hour)) // was verified in the past
	ep.MarkFailed(time.Now())
	return ep
}

// verifiedEP creates a configured endpoint that has been marked verified.
func verifiedEP(host string, port uint16) transport.ExternalEndpoint {
	ep := configuredEP(host, port)
	ep.MarkVerified(time.Now())
	return ep
}

// --- TestSelectProbeTargets ---

// TestSelectProbeTargets_UnverifiedFirst verifies that unverified endpoints are
// selected before stale/failed endpoints, and that no new host:port combinations
// are invented (targeted-first discipline).
func TestSelectProbeTargets_UnverifiedFirst(t *testing.T) {
	registry := newRegistry([]transport.ExternalEndpoint{
		staleEP("10.0.0.1", 4500),
		configuredEP("10.0.0.2", 4501), // unverified
		failedEP("10.0.0.3", 4502),
		configuredEP("10.0.0.4", 4503), // unverified
	})

	targets := SelectProbeTargets(registry, nil, 10)

	// Unverified (10.0.0.2 and 10.0.0.4) should come before stale/failed.
	if len(targets) != 4 {
		t.Fatalf("expected 4 targets, got %d", len(targets))
	}

	// First two should be unverified.
	unverifiedCount := 0
	for _, tgt := range targets[:2] {
		if tgt.Reason == transport.CandidateReasonConfigured {
			unverifiedCount++
		}
	}
	if unverifiedCount != 2 {
		t.Errorf("expected first 2 targets to be unverified (configured reason), got %+v", targets[:2])
	}

	// Last two should be stale/failed (previously-verified).
	for _, tgt := range targets[2:] {
		if tgt.Reason != transport.CandidateReasonPreviouslyVerified {
			t.Errorf("expected stale/failed targets to have PreviouslyVerified reason, got %q", tgt.Reason)
		}
	}
}

// TestSelectProbeTargets_Bounded verifies that maxTargets is enforced and
// excess candidates are dropped rather than appended unboundedly.
func TestSelectProbeTargets_Bounded(t *testing.T) {
	eps := make([]transport.ExternalEndpoint, 20)
	for i := range eps {
		eps[i] = configuredEP(fmt.Sprintf("10.0.0.%d", i+1), uint16(4500+i))
	}
	registry := newRegistry(eps)

	maxTargets := 5
	targets := SelectProbeTargets(registry, nil, maxTargets)

	if len(targets) > maxTargets {
		t.Errorf("expected at most %d targets, got %d", maxTargets, len(targets))
	}
}

// TestSelectProbeTargets_OnlyFromRegistry verifies that SelectProbeTargets only
// returns endpoints already in the registry — no new host:port combinations.
func TestSelectProbeTargets_OnlyFromRegistry(t *testing.T) {
	registry := newRegistry([]transport.ExternalEndpoint{
		configuredEP("192.168.1.1", 5000),
		staleEP("192.168.1.2", 5001),
	})

	targets := SelectProbeTargets(registry, nil, 10)

	knownAddrs := map[string]bool{
		"192.168.1.1:5000": true,
		"192.168.1.2:5001": true,
	}
	for _, tgt := range targets {
		key := fmt.Sprintf("%s:%d", tgt.Host, tgt.Port)
		if !knownAddrs[key] {
			t.Errorf("target %q not in registry — targeted-first discipline violated", key)
		}
	}
}

// TestSelectProbeTargets_VerifiedExcluded verifies that fully verified endpoints
// are excluded from probe targets (they do not need probing).
func TestSelectProbeTargets_VerifiedExcluded(t *testing.T) {
	registry := newRegistry([]transport.ExternalEndpoint{
		verifiedEP("10.0.0.1", 4500),
		configuredEP("10.0.0.2", 4501), // unverified
	})

	targets := SelectProbeTargets(registry, nil, 10)

	// Only the unverified endpoint should be selected.
	if len(targets) != 1 {
		t.Fatalf("expected 1 target (unverified only), got %d: %+v", len(targets), targets)
	}
	if targets[0].Host != "10.0.0.2" || targets[0].Port != 4501 {
		t.Errorf("expected unverified endpoint 10.0.0.2:4501, got %s:%d", targets[0].Host, targets[0].Port)
	}
}

// TestSelectProbeTargets_DeduplicatesSameAddress verifies that the same host:port
// appearing in both unverified and stale sets is not duplicated in targets.
func TestSelectProbeTargets_DeduplicatesSameAddress(t *testing.T) {
	// Simulate same address appearing twice (e.g., configured + router-discovered).
	ep1 := configuredEP("10.0.0.1", 4500) // unverified
	ep2 := transport.ExternalEndpoint{
		Host:         "10.0.0.1",
		Port:         4500,
		Source:       transport.EndpointSourceRouterDiscovered,
		Verification: transport.VerificationStateStale,
	}
	ep2.MarkStale(time.Now())

	registry := transport.NewEndpointRegistry()
	registry.Add(ep1)
	registry.Add(ep2)

	targets := SelectProbeTargets(registry, nil, 10)

	// Should only appear once despite being in both sets.
	count := 0
	for _, tgt := range targets {
		if tgt.Host == "10.0.0.1" && tgt.Port == 4500 {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected address 10.0.0.1:4500 exactly once, got %d targets: %+v", count, targets)
	}
}

// TestSelectProbeTargets_PathIDs verifies that path IDs are wired into targets
// from the pathIDMap.
func TestSelectProbeTargets_PathIDs(t *testing.T) {
	registry := newRegistry([]transport.ExternalEndpoint{
		configuredEP("10.0.0.1", 4500),
		configuredEP("10.0.0.2", 4501),
	})

	pathIDMap := map[string][]string{
		"10.0.0.1:4500": {"assoc-1:direct"},
		"10.0.0.2:4501": {"assoc-2:direct", "cand-xyz"},
	}

	targets := SelectProbeTargets(registry, pathIDMap, 10)

	found := make(map[string][]string)
	for _, tgt := range targets {
		key := fmt.Sprintf("%s:%d", tgt.Host, tgt.Port)
		found[key] = tgt.PathIDs
	}

	if ids := found["10.0.0.1:4500"]; len(ids) != 1 || ids[0] != "assoc-1:direct" {
		t.Errorf("expected PathIDs [assoc-1:direct] for 10.0.0.1:4500, got %v", ids)
	}
	if ids := found["10.0.0.2:4501"]; len(ids) != 2 {
		t.Errorf("expected 2 PathIDs for 10.0.0.2:4501, got %v", ids)
	}
}

// TestSelectProbeTargets_NilRegistry verifies graceful handling of nil input.
func TestSelectProbeTargets_NilRegistry(t *testing.T) {
	targets := SelectProbeTargets(nil, nil, 10)
	if len(targets) != 0 {
		t.Errorf("expected empty targets for nil registry, got %d", len(targets))
	}
}

// --- TestExecuteProbeRound ---

// TestExecuteProbeRound_UpdatesEndpointFreshness verifies that a successful probe
// marks the endpoint as verified in the registry.
func TestExecuteProbeRound_UpdatesEndpointFreshness(t *testing.T) {
	ep := configuredEP("10.0.0.1", 4500)
	registry := newRegistry([]transport.ExternalEndpoint{ep})

	executor := newFakeProbeExecutor()
	executor.setResult("10.0.0.1", 4500, true, 10*time.Millisecond)

	targets := []ProbeTarget{{Host: "10.0.0.1", Port: 4500, Reason: transport.CandidateReasonConfigured}}
	result := ExecuteProbeRound(context.Background(), targets, executor, registry, nil)

	if result.Reachable != 1 {
		t.Errorf("expected 1 reachable, got %d", result.Reachable)
	}

	// Endpoint should now be verified in the registry.
	usable := registry.UsableEndpoints()
	verified := false
	for _, u := range usable {
		if u.Host == "10.0.0.1" && u.Port == 4500 && u.Verification == transport.VerificationStateVerified {
			verified = true
		}
	}
	if !verified {
		t.Error("expected 10.0.0.1:4500 to be verified after successful probe")
	}
}

// TestExecuteProbeRound_UpdatesEndpointFreshness_Failed verifies that a failed probe
// marks the endpoint as failed in the registry.
func TestExecuteProbeRound_UpdatesEndpointFreshness_Failed(t *testing.T) {
	ep := configuredEP("10.0.0.1", 4500)
	registry := newRegistry([]transport.ExternalEndpoint{ep})

	executor := newFakeProbeExecutor()
	executor.setResult("10.0.0.1", 4500, false, 0) // not reachable

	targets := []ProbeTarget{{Host: "10.0.0.1", Port: 4500, Reason: transport.CandidateReasonConfigured}}
	result := ExecuteProbeRound(context.Background(), targets, executor, registry, nil)

	if result.Unreachable != 1 {
		t.Errorf("expected 1 unreachable, got %d", result.Unreachable)
	}

	// Endpoint should now be failed in the registry.
	snap := registry.Snapshot()
	failed := false
	for _, e := range snap {
		if e.Host == "10.0.0.1" && e.Port == 4500 && e.Verification == transport.VerificationStateFailed {
			failed = true
		}
	}
	if !failed {
		t.Error("expected 10.0.0.1:4500 to be failed after unsuccessful probe")
	}
}

// TestExecuteProbeRound_UpdatesQualityStore verifies that a successful probe
// updates quality measurements in the path-quality store for all linked path IDs.
func TestExecuteProbeRound_UpdatesQualityStore(t *testing.T) {
	qualityStore := scheduler.NewPathQualityStore(scheduler.DefaultQualityMaxAge)
	executor := newFakeProbeExecutor()
	executor.setResult("10.0.0.1", 4500, true, 20*time.Millisecond)

	targets := []ProbeTarget{
		{
			Host:    "10.0.0.1",
			Port:    4500,
			Reason:  transport.CandidateReasonConfigured,
			PathIDs: []string{"assoc-1:direct", "cand-xyz"},
		},
	}
	result := ExecuteProbeRound(context.Background(), targets, executor, nil, qualityStore)

	if result.Reachable != 1 {
		t.Errorf("expected 1 reachable, got %d", result.Reachable)
	}

	// Quality should now be measured for both path IDs.
	for _, pathID := range []string{"assoc-1:direct", "cand-xyz"} {
		q, fresh := qualityStore.FreshQuality(pathID)
		if !fresh {
			t.Errorf("expected fresh quality for path ID %q after successful probe", pathID)
		}
		if q.Confidence <= 0 {
			t.Errorf("expected positive confidence for path ID %q after successful probe, got %f", pathID, q.Confidence)
		}
	}
}

// TestExecuteProbeRound_QualityFailure verifies that a failed probe records
// failure in the quality store (confidence decreases, loss fraction rises).
func TestExecuteProbeRound_QualityFailure(t *testing.T) {
	qualityStore := scheduler.NewPathQualityStore(scheduler.DefaultQualityMaxAge)
	executor := newFakeProbeExecutor()
	executor.setResult("10.0.0.1", 4500, false, 0) // not reachable

	targets := []ProbeTarget{
		{
			Host:    "10.0.0.1",
			Port:    4500,
			Reason:  transport.CandidateReasonConfigured,
			PathIDs: []string{"assoc-1:direct"},
		},
	}
	ExecuteProbeRound(context.Background(), targets, executor, nil, qualityStore)

	// After a failure, quality store should have an entry with high loss fraction.
	q, fresh := qualityStore.FreshQuality("assoc-1:direct")
	if !fresh {
		t.Error("expected fresh quality entry for failed probe (failure IS a measurement)")
	}
	if q.LossFraction <= 0 {
		t.Errorf("expected positive loss fraction after failed probe, got %f", q.LossFraction)
	}
	if q.Confidence != 0 {
		t.Errorf("expected zero confidence after first failed probe, got %f", q.Confidence)
	}
}

// TestAbsentMeasurementVsFailedMeasurement verifies that absent measurement
// (no PathIDs or quality store entry) is explicitly distinct from failed
// measurement (Reachable=false with PathIDs recorded in quality store).
func TestAbsentMeasurementVsFailedMeasurement(t *testing.T) {
	qualityStore := scheduler.NewPathQualityStore(scheduler.DefaultQualityMaxAge)
	executor := newFakeProbeExecutor()

	// Target with PathIDs but not reachable → failed measurement.
	executor.setResult("10.0.0.1", 4500, false, 0)
	// Target without PathIDs → absent measurement.
	executor.setResult("10.0.0.2", 4501, false, 0)

	targets := []ProbeTarget{
		{
			Host:    "10.0.0.1",
			Port:    4500,
			Reason:  transport.CandidateReasonConfigured,
			PathIDs: []string{"path-with-failure"},
		},
		{
			Host:    "10.0.0.2",
			Port:    4501,
			Reason:  transport.CandidateReasonConfigured,
			PathIDs: nil, // no path ID linkage
		},
	}
	ExecuteProbeRound(context.Background(), targets, executor, nil, qualityStore)

	// "path-with-failure" should have a quality entry (failed measurement).
	_, failedFresh := qualityStore.FreshQuality("path-with-failure")
	if !failedFresh {
		t.Error("expected quality entry for 'path-with-failure' — failed probe IS a measurement")
	}

	// There should be no quality entry for the absent case (no path ID).
	// We verify this by checking the snapshot has exactly one entry.
	snap := qualityStore.Snapshot()
	if len(snap) != 1 {
		t.Errorf("expected exactly 1 quality entry (failed measurement only), got %d: %+v", len(snap), snap)
	}
	if snap[0].PathID != "path-with-failure" {
		t.Errorf("expected quality entry for 'path-with-failure', got %q", snap[0].PathID)
	}
}

// TestExecuteProbeRound_ExecutorError verifies that executor errors (context
// cancellation, internal error) do not update registry or quality store.
func TestExecuteProbeRound_ExecutorError(t *testing.T) {
	registry := transport.NewEndpointRegistry()
	registry.Add(configuredEP("10.0.0.1", 4500))
	qualityStore := scheduler.NewPathQualityStore(scheduler.DefaultQualityMaxAge)

	executor := newFakeProbeExecutor()
	executor.setError("10.0.0.1", 4500, fmt.Errorf("simulated internal executor error"))

	targets := []ProbeTarget{
		{
			Host:    "10.0.0.1",
			Port:    4500,
			Reason:  transport.CandidateReasonConfigured,
			PathIDs: []string{"path-1"},
		},
	}
	result := ExecuteProbeRound(context.Background(), targets, executor, registry, qualityStore)

	if result.Errors != 1 {
		t.Errorf("expected 1 executor error, got %d", result.Errors)
	}
	if result.Reachable != 0 || result.Unreachable != 0 {
		t.Errorf("expected no reachable/unreachable for executor error, got %+v", result)
	}

	// Registry should still be unverified (not updated on executor error).
	snap := registry.Snapshot()
	for _, ep := range snap {
		if ep.Host == "10.0.0.1" && ep.Port == 4500 {
			if ep.Verification != transport.VerificationStateUnverified {
				t.Errorf("expected endpoint to remain unverified on executor error, got %s", ep.Verification)
			}
		}
	}

	// Quality store should have no entry (not updated on executor error).
	_, fresh := qualityStore.FreshQuality("path-1")
	if fresh {
		t.Error("expected no quality entry for executor error case")
	}
}

// TestExecuteProbeRound_ContextCancellation verifies that a cancelled context
// stops the probe loop cleanly after the current in-flight probe.
func TestExecuteProbeRound_ContextCancellation(t *testing.T) {
	executor := newFakeProbeExecutor()
	executor.setResult("10.0.0.1", 4500, true, 5*time.Millisecond)
	executor.setResult("10.0.0.2", 4501, true, 5*time.Millisecond)

	// Cancel the context immediately — should stop before any probes start.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	targets := []ProbeTarget{
		{Host: "10.0.0.1", Port: 4500, Reason: transport.CandidateReasonConfigured},
		{Host: "10.0.0.2", Port: 4501, Reason: transport.CandidateReasonConfigured},
	}
	result := ExecuteProbeRound(ctx, targets, executor, nil, nil)

	// With cancelled context, no probes should complete (loop exits on first check).
	if result.Reachable != 0 || result.Unreachable != 0 {
		t.Errorf("expected no probes after cancelled context, got reachable=%d unreachable=%d",
			result.Reachable, result.Unreachable)
	}
}

// --- TestBuildPathIDMap ---

// TestBuildPathIDMap_ConfigDerived verifies that direct endpoint → assocID:direct
// mapping is built from ScheduledActivationInputs.
func TestBuildPathIDMap_ConfigDerived(t *testing.T) {
	inputs := []ScheduledActivationInput{
		{AssociationID: "assoc-1", DirectEndpoint: "10.0.0.1:4500"},
		{AssociationID: "assoc-2", DirectEndpoint: "10.0.0.2:4501"},
		{AssociationID: "assoc-3", DirectEndpoint: ""}, // no direct endpoint
	}

	m := BuildPathIDMap(inputs, nil)

	if ids := m["10.0.0.1:4500"]; len(ids) != 1 || ids[0] != "assoc-1:direct" {
		t.Errorf("expected [assoc-1:direct] for 10.0.0.1:4500, got %v", ids)
	}
	if ids := m["10.0.0.2:4501"]; len(ids) != 1 || ids[0] != "assoc-2:direct" {
		t.Errorf("expected [assoc-2:direct] for 10.0.0.2:4501, got %v", ids)
	}
	if _, exists := m[""]; exists {
		t.Error("empty endpoint should not appear in path ID map")
	}
}

// TestBuildPathIDMap_NoDuplicates verifies that the same path ID is not added
// twice even when multiple sources reference the same endpoint.
func TestBuildPathIDMap_NoDuplicates(t *testing.T) {
	inputs := []ScheduledActivationInput{
		{AssociationID: "assoc-1", DirectEndpoint: "10.0.0.1:4500"},
		{AssociationID: "assoc-1", DirectEndpoint: "10.0.0.1:4500"}, // duplicate
	}

	m := BuildPathIDMap(inputs, nil)

	ids := m["10.0.0.1:4500"]
	if len(ids) != 1 {
		t.Errorf("expected exactly 1 path ID for duplicate inputs, got %d: %v", len(ids), ids)
	}
}

// TestBuildPathIDMap_EmptyInputs verifies that nil/empty inputs produce an empty map.
func TestBuildPathIDMap_EmptyInputs(t *testing.T) {
	m := BuildPathIDMap(nil, nil)
	if len(m) != 0 {
		t.Errorf("expected empty map for nil inputs, got %v", m)
	}
}

// --- TestProbeRoundResult_ReportLines ---

// TestProbeRoundResult_ReportLines verifies that report lines include key fields.
func TestProbeRoundResult_ReportLines(t *testing.T) {
	result := ProbeRoundResult{
		TargetsSelected: 3,
		Reachable:       2,
		Unreachable:     1,
		Errors:          0,
		Details: []ProbeRoundDetail{
			{
				Host:      "10.0.0.1",
				Port:      4500,
				Reason:    transport.CandidateReasonConfigured,
				Reachable: true,
				RTT:       12 * time.Millisecond,
				PathIDs:   []string{"assoc-1:direct"},
			},
			{
				Host:      "10.0.0.2",
				Port:      4501,
				Reason:    transport.CandidateReasonPreviouslyVerified,
				Reachable: false,
				PathIDs:   []string{"assoc-2:direct"},
			},
		},
	}

	lines := result.ReportLines()

	if len(lines) == 0 {
		t.Fatal("expected non-empty report lines")
	}
	header := lines[0]
	if header == "" {
		t.Error("expected non-empty header line")
	}

	// Header should mention reachable/unreachable counts.
	for _, want := range []string{"selected=3", "reachable=2", "unreachable=1"} {
		found := false
		for _, l := range lines {
			if probeContains(l, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in report lines, got:\n%v", want, lines)
		}
	}
}

// probeContains reports whether s contains substr.
func probeContains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- TestDefaultProbeSchedulerConfig ---

// TestDefaultProbeSchedulerConfig verifies defaults are sensible.
func TestDefaultProbeSchedulerConfig(t *testing.T) {
	cfg := DefaultProbeSchedulerConfig()
	if cfg.ProbeInterval <= 0 {
		t.Error("expected positive ProbeInterval")
	}
	if cfg.MaxTargetsPerRound <= 0 {
		t.Error("expected positive MaxTargetsPerRound")
	}
}

// --- Integration: select targets → execute → observe updated state ---

// TestProbeScheduler_EndToEnd_ImprovedUsabilitySignals verifies the full flow:
// select targets → execute probes → observe updated endpoint freshness and
// quality state that downstream runtime logic (RefineCandidates, fallback)
// can consume.
func TestProbeScheduler_EndToEnd_ImprovedUsabilitySignals(t *testing.T) {
	// Set up: two unverified endpoints.
	registry := newRegistry([]transport.ExternalEndpoint{
		configuredEP("10.0.0.1", 4500), // will succeed
		configuredEP("10.0.0.2", 4501), // will fail
	})
	qualityStore := scheduler.NewPathQualityStore(scheduler.DefaultQualityMaxAge)

	executor := newFakeProbeExecutor()
	executor.setResult("10.0.0.1", 4500, true, 15*time.Millisecond)
	executor.setResult("10.0.0.2", 4501, false, 0)

	pathIDMap := map[string][]string{
		"10.0.0.1:4500": {"assoc-a:direct"},
		"10.0.0.2:4501": {"assoc-b:direct"},
	}

	// Step 1: select targets.
	targets := SelectProbeTargets(registry, pathIDMap, 10)
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	// Step 2: execute probe round.
	result := ExecuteProbeRound(context.Background(), targets, executor, registry, qualityStore)

	if result.Reachable != 1 || result.Unreachable != 1 {
		t.Errorf("expected 1 reachable 1 unreachable, got %+v", result)
	}

	// Step 3: observe improved endpoint freshness signals.
	snap := registry.Snapshot()
	stateByAddr := make(map[string]transport.VerificationState)
	for _, ep := range snap {
		key := fmt.Sprintf("%s:%d", ep.Host, ep.Port)
		stateByAddr[key] = ep.Verification
	}
	if stateByAddr["10.0.0.1:4500"] != transport.VerificationStateVerified {
		t.Errorf("expected 10.0.0.1:4500 to be verified, got %s", stateByAddr["10.0.0.1:4500"])
	}
	if stateByAddr["10.0.0.2:4501"] != transport.VerificationStateFailed {
		t.Errorf("expected 10.0.0.2:4501 to be failed, got %s", stateByAddr["10.0.0.2:4501"])
	}

	// Step 4: observe improved quality signals.
	qA, freshA := qualityStore.FreshQuality("assoc-a:direct")
	if !freshA || qA.Confidence <= 0 {
		t.Error("expected fresh quality with positive confidence for assoc-a:direct")
	}
	if qA.RTT <= 0 {
		t.Error("expected positive RTT for successfully probed path")
	}

	qB, freshB := qualityStore.FreshQuality("assoc-b:direct")
	if !freshB {
		t.Error("expected fresh quality entry for assoc-b:direct (failed probe IS a measurement)")
	}
	if qB.LossFraction <= 0 {
		t.Error("expected positive loss fraction for failed probe")
	}

	// Step 5: verify downstream consumer can now distinguish reachable from failed.
	// After the probe round, SelectForRevalidation should return 10.0.0.2:4501 (failed),
	// and UsableEndpoints should return only 10.0.0.1:4500 (verified, which is usable).
	usable := registry.UsableEndpoints()
	if len(usable) != 1 || usable[0].Host != "10.0.0.1" {
		t.Errorf("expected only 10.0.0.1:4500 in usable endpoints after probing, got %+v", usable)
	}
	revalidation := registry.SelectForRevalidation()
	if len(revalidation) != 1 || revalidation[0].Host != "10.0.0.2" {
		t.Errorf("expected only 10.0.0.2:4501 in revalidation after probing, got %+v", revalidation)
	}
}

// TestSelectProbeTargets_EmptyRegistry verifies that an empty registry returns
// no targets (nothing to probe).
func TestSelectProbeTargets_EmptyRegistry(t *testing.T) {
	registry := transport.NewEndpointRegistry()
	targets := SelectProbeTargets(registry, nil, 10)
	if len(targets) != 0 {
		t.Errorf("expected 0 targets for empty registry, got %d", len(targets))
	}
}

// TestExecuteProbeRound_NilRegistry verifies that a nil registry is gracefully
// handled (quality store still updated, no panic).
func TestExecuteProbeRound_NilRegistry(t *testing.T) {
	qualityStore := scheduler.NewPathQualityStore(scheduler.DefaultQualityMaxAge)
	executor := newFakeProbeExecutor()
	executor.setResult("10.0.0.1", 4500, true, 10*time.Millisecond)

	targets := []ProbeTarget{
		{Host: "10.0.0.1", Port: 4500, Reason: transport.CandidateReasonConfigured, PathIDs: []string{"path-1"}},
	}

	// Should not panic with nil registry.
	result := ExecuteProbeRound(context.Background(), targets, executor, nil, qualityStore)
	if result.Reachable != 1 {
		t.Errorf("expected 1 reachable with nil registry, got %d", result.Reachable)
	}

	_, fresh := qualityStore.FreshQuality("path-1")
	if !fresh {
		t.Error("expected quality to be updated even with nil registry")
	}
}
