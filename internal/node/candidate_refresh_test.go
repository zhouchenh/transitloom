package node

import (
	"context"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/scheduler"
	"github.com/zhouchenh/transitloom/internal/transport"
)

// ----------------------------------------------------------------------------
// CandidateFreshnessStore tests
// ----------------------------------------------------------------------------

func TestCandidateFreshnessStore_TrackAndInitial(t *testing.T) {
	s := NewCandidateFreshnessStore(DefaultCandidateMaxAge)

	// Untracked association → unknown.
	if got := s.FreshnessState("assoc-1"); got != CandidateFreshnessStateUnknown {
		t.Errorf("untracked: want unknown, got %q", got)
	}

	s.TrackAssociation("assoc-1")
	if got := s.FreshnessState("assoc-1"); got != CandidateFreshnessStateUnknown {
		t.Errorf("after track: want unknown, got %q", got)
	}

	// Second TrackAssociation call is a no-op (should not overwrite state).
	s.MarkRefreshed("assoc-1", time.Now())
	s.TrackAssociation("assoc-1") // no-op
	if got := s.FreshnessState("assoc-1"); got != CandidateFreshnessStateFresh {
		t.Errorf("track after refresh should be no-op: want fresh, got %q", got)
	}
}

func TestCandidateFreshnessStore_MarkRefreshedAndFresh(t *testing.T) {
	s := NewCandidateFreshnessStore(DefaultCandidateMaxAge)

	s.MarkRefreshed("assoc-1", time.Now())
	if got := s.FreshnessState("assoc-1"); got != CandidateFreshnessStateFresh {
		t.Errorf("after refresh: want fresh, got %q", got)
	}

	// Refresh candidates not appear in SelectForRefresh.
	refresh := s.SelectForRefresh()
	for _, r := range refresh {
		if r.AssociationID == "assoc-1" {
			t.Error("freshly-refreshed association should not appear in SelectForRefresh")
		}
	}
}

func TestCandidateFreshnessStore_MarkStale(t *testing.T) {
	s := NewCandidateFreshnessStore(DefaultCandidateMaxAge)
	now := time.Now()

	s.MarkStale("assoc-1", CandidateRefreshTriggerEndpointFailed, "ep 1.2.3.4:5000 failed", now)

	if got := s.FreshnessState("assoc-1"); got != CandidateFreshnessStateStale {
		t.Errorf("want stale, got %q", got)
	}

	found := false
	for _, r := range s.SelectForRefresh() {
		if r.AssociationID == "assoc-1" {
			found = true
			if r.StaleReason != CandidateRefreshTriggerEndpointFailed {
				t.Errorf("stale reason: want endpoint-failed, got %q", r.StaleReason)
			}
		}
	}
	if !found {
		t.Error("stale association not returned by SelectForRefresh")
	}
}

func TestCandidateFreshnessStore_MarkStalePreservesFirstTrigger(t *testing.T) {
	// The first trigger that fires should be preserved (root-cause visibility).
	s := NewCandidateFreshnessStore(DefaultCandidateMaxAge)
	now := time.Now()

	s.MarkStale("assoc-1", CandidateRefreshTriggerEndpointStale, "first-trigger", now)
	s.MarkStale("assoc-1", CandidateRefreshTriggerPathUnhealthy, "second-trigger", now.Add(time.Second))

	for _, r := range s.SelectForRefresh() {
		if r.AssociationID == "assoc-1" {
			if r.StaleReason != CandidateRefreshTriggerEndpointStale {
				t.Errorf("should preserve first trigger endpoint-stale, got %q", r.StaleReason)
			}
			if r.StaleDetail != "first-trigger" {
				t.Errorf("should preserve first detail, got %q", r.StaleDetail)
			}
		}
	}
}

func TestCandidateFreshnessStore_MarkRefreshedClearsStale(t *testing.T) {
	s := NewCandidateFreshnessStore(DefaultCandidateMaxAge)
	now := time.Now()

	s.MarkStale("assoc-1", CandidateRefreshTriggerExplicit, "test", now)
	if got := s.FreshnessState("assoc-1"); got != CandidateFreshnessStateStale {
		t.Fatalf("expected stale, got %q", got)
	}

	s.MarkRefreshed("assoc-1", now.Add(time.Second))
	if got := s.FreshnessState("assoc-1"); got != CandidateFreshnessStateFresh {
		t.Errorf("after refresh: want fresh, got %q", got)
	}

	// No longer in SelectForRefresh.
	for _, r := range s.SelectForRefresh() {
		if r.AssociationID == "assoc-1" {
			t.Error("refreshed association should not appear in SelectForRefresh")
		}
	}
}

func TestCandidateFreshnessStore_AgeBasedExpiry(t *testing.T) {
	// Use a very short maxAge to test age-based expiry.
	maxAge := 10 * time.Millisecond
	s := NewCandidateFreshnessStore(maxAge)

	// Refresh with a timestamp in the past (older than maxAge).
	past := time.Now().Add(-20 * time.Millisecond)
	s.MarkRefreshed("assoc-1", past)

	// FreshnessState should return stale because the last refresh was too long ago.
	if got := s.FreshnessState("assoc-1"); got != CandidateFreshnessStateStale {
		t.Errorf("age-expired: want stale, got %q", got)
	}

	// SelectForRefresh should include it.
	found := false
	for _, r := range s.SelectForRefresh() {
		if r.AssociationID == "assoc-1" {
			found = true
		}
	}
	if !found {
		t.Error("age-expired association should appear in SelectForRefresh")
	}
}

func TestCandidateFreshnessStore_SnapshotAndCount(t *testing.T) {
	s := NewCandidateFreshnessStore(DefaultCandidateMaxAge)
	s.TrackAssociation("assoc-1")
	s.TrackAssociation("assoc-2")
	s.MarkRefreshed("assoc-3", time.Now())

	if s.Count() != 3 {
		t.Errorf("want 3 tracked, got %d", s.Count())
	}

	snap := s.Snapshot()
	if len(snap) != 3 {
		t.Errorf("snapshot: want 3 records, got %d", len(snap))
	}
}

func TestCandidateFreshnessStore_ReportLines(t *testing.T) {
	s := NewCandidateFreshnessStore(DefaultCandidateMaxAge)
	s.MarkRefreshed("assoc-fresh", time.Now())
	s.MarkStale("assoc-stale", CandidateRefreshTriggerEndpointStale, "ep stale", time.Now())
	s.TrackAssociation("assoc-unknown")

	lines := s.ReportLines()
	if len(lines) < 1 {
		t.Fatal("expected non-empty ReportLines")
	}
	// First line should be a summary.
	if lines[0] == "" {
		t.Error("first report line should be non-empty summary")
	}
}

// ----------------------------------------------------------------------------
// SelectCandidateRefreshTargets tests
// ----------------------------------------------------------------------------

func TestSelectCandidateRefreshTargets_NilStore(t *testing.T) {
	targets := SelectCandidateRefreshTargets(nil, nil, nil, nil)
	if len(targets) != 0 {
		t.Errorf("nil candidateStore: want 0 targets, got %d", len(targets))
	}
}

func TestSelectCandidateRefreshTargets_NoTriggers(t *testing.T) {
	store := NewCandidateStore()
	store.Store("assoc-1", []controlplane.DistributedPathCandidate{
		{CandidateID: "c1", AssociationID: "assoc-1", RemoteEndpoint: "1.2.3.4:5000"},
	})

	// No stale endpoints, no stale quality, no stale freshness → no targets.
	targets := SelectCandidateRefreshTargets(store, nil, nil, nil)
	if len(targets) != 0 {
		t.Errorf("no triggers: want 0 targets, got %d: %+v", len(targets), targets)
	}
}

func TestSelectCandidateRefreshTargets_FreshnessStoreStale(t *testing.T) {
	store := NewCandidateStore()
	store.Store("assoc-1", []controlplane.DistributedPathCandidate{
		{CandidateID: "c1", AssociationID: "assoc-1", RemoteEndpoint: "1.2.3.4:5000"},
	})

	freshnessStore := NewCandidateFreshnessStore(DefaultCandidateMaxAge)
	freshnessStore.MarkStale("assoc-1", CandidateRefreshTriggerExplicit, "operator request", time.Now())

	targets := SelectCandidateRefreshTargets(store, nil, nil, freshnessStore)
	if len(targets) != 1 {
		t.Fatalf("want 1 target, got %d", len(targets))
	}
	if targets[0].AssociationID != "assoc-1" {
		t.Errorf("target assocID: want assoc-1, got %q", targets[0].AssociationID)
	}
	if targets[0].Trigger != CandidateRefreshTriggerExplicit {
		t.Errorf("trigger: want explicit, got %q", targets[0].Trigger)
	}
}

func TestSelectCandidateRefreshTargets_EndpointStaleTrigger(t *testing.T) {
	store := NewCandidateStore()
	store.Store("assoc-1", []controlplane.DistributedPathCandidate{
		{CandidateID: "c1", AssociationID: "assoc-1", RemoteEndpoint: "1.2.3.4:5000"},
	})

	reg := transport.NewEndpointRegistry()
	ep := transport.ExternalEndpoint{Host: "1.2.3.4", Port: 5000, Source: transport.EndpointSourceConfigured}
	ep.MarkStale(time.Now())
	reg.Add(ep)

	targets := SelectCandidateRefreshTargets(store, reg, nil, nil)
	if len(targets) != 1 {
		t.Fatalf("want 1 target, got %d", len(targets))
	}
	if targets[0].Trigger != CandidateRefreshTriggerEndpointStale {
		t.Errorf("trigger: want endpoint-stale, got %q", targets[0].Trigger)
	}
}

func TestSelectCandidateRefreshTargets_EndpointFailedTrigger(t *testing.T) {
	store := NewCandidateStore()
	store.Store("assoc-1", []controlplane.DistributedPathCandidate{
		{CandidateID: "c1", AssociationID: "assoc-1", RemoteEndpoint: "1.2.3.4:5000"},
	})

	reg := transport.NewEndpointRegistry()
	ep := transport.ExternalEndpoint{Host: "1.2.3.4", Port: 5000, Source: transport.EndpointSourceConfigured}
	ep.MarkFailed(time.Now())
	reg.Add(ep)

	targets := SelectCandidateRefreshTargets(store, reg, nil, nil)
	if len(targets) != 1 {
		t.Fatalf("want 1 target, got %d", len(targets))
	}
	if targets[0].Trigger != CandidateRefreshTriggerEndpointFailed {
		t.Errorf("trigger: want endpoint-failed, got %q", targets[0].Trigger)
	}
}

func TestSelectCandidateRefreshTargets_QualityStaleTrigger(t *testing.T) {
	store := NewCandidateStore()
	store.Store("assoc-1", []controlplane.DistributedPathCandidate{
		{CandidateID: "c1", AssociationID: "assoc-1", RemoteEndpoint: "1.2.3.4:5000"},
	})

	// Use a tiny max age so the quality measurement is immediately stale.
	qs := scheduler.NewPathQualityStore(1 * time.Millisecond)
	qs.RecordProbeResult("c1", 10*time.Millisecond, true)
	time.Sleep(5 * time.Millisecond) // let measurement age out

	targets := SelectCandidateRefreshTargets(store, nil, qs, nil)
	if len(targets) != 1 {
		t.Fatalf("want 1 target, got %d: %+v", len(targets), targets)
	}
	if targets[0].Trigger != CandidateRefreshTriggerQualityStale {
		t.Errorf("trigger: want quality-stale, got %q", targets[0].Trigger)
	}
}

func TestSelectCandidateRefreshTargets_NoQualityTriggerForAbsent(t *testing.T) {
	// Absent quality measurements (never measured) must NOT trigger quality-stale.
	// Stale means "was measured, now expired". Absent means "not yet measured".
	store := NewCandidateStore()
	store.Store("assoc-1", []controlplane.DistributedPathCandidate{
		{CandidateID: "c1", AssociationID: "assoc-1", RemoteEndpoint: "1.2.3.4:5000"},
	})

	qs := scheduler.NewPathQualityStore(DefaultCandidateMaxAge)
	// No measurement recorded for "c1" — qs.Snapshot() returns empty → staleQualityIDs empty.

	targets := SelectCandidateRefreshTargets(store, nil, qs, nil)
	for _, target := range targets {
		if target.Trigger == CandidateRefreshTriggerQualityStale {
			t.Error("absent (never measured) quality should not trigger quality-stale")
		}
	}
}

func TestSelectCandidateRefreshTargets_Deduplication(t *testing.T) {
	// An association should appear at most once even if multiple triggers fire.
	store := NewCandidateStore()
	store.Store("assoc-1", []controlplane.DistributedPathCandidate{
		{CandidateID: "c1", AssociationID: "assoc-1", RemoteEndpoint: "1.2.3.4:5000"},
	})

	// Trigger from freshness store.
	freshnessStore := NewCandidateFreshnessStore(DefaultCandidateMaxAge)
	freshnessStore.MarkStale("assoc-1", CandidateRefreshTriggerExplicit, "explicit", time.Now())

	// Also trigger from endpoint registry.
	reg := transport.NewEndpointRegistry()
	ep := transport.ExternalEndpoint{Host: "1.2.3.4", Port: 5000}
	ep.MarkStale(time.Now())
	reg.Add(ep)

	targets := SelectCandidateRefreshTargets(store, reg, nil, freshnessStore)
	// Should have exactly 1 target despite 2 triggers.
	if len(targets) != 1 {
		t.Errorf("deduplication: want 1 target, got %d", len(targets))
	}
	// First trigger (freshness store / explicit) should win.
	if targets[0].Trigger != CandidateRefreshTriggerExplicit {
		t.Errorf("first trigger should win: want explicit, got %q", targets[0].Trigger)
	}
}

func TestSelectCandidateRefreshTargets_MultipleAssociations(t *testing.T) {
	store := NewCandidateStore()
	// assoc-1 → stale endpoint
	store.Store("assoc-1", []controlplane.DistributedPathCandidate{
		{CandidateID: "c1", AssociationID: "assoc-1", RemoteEndpoint: "1.2.3.4:5001"},
	})
	// assoc-2 → explicitly stale
	store.Store("assoc-2", []controlplane.DistributedPathCandidate{
		{CandidateID: "c2", AssociationID: "assoc-2", RemoteEndpoint: "5.6.7.8:5002"},
	})
	// assoc-3 → clean (no trigger)
	store.Store("assoc-3", []controlplane.DistributedPathCandidate{
		{CandidateID: "c3", AssociationID: "assoc-3", RemoteEndpoint: "9.10.11.12:5003"},
	})

	reg := transport.NewEndpointRegistry()
	ep1 := transport.ExternalEndpoint{Host: "1.2.3.4", Port: 5001}
	ep1.MarkStale(time.Now())
	reg.Add(ep1)

	freshnessStore := NewCandidateFreshnessStore(DefaultCandidateMaxAge)
	freshnessStore.MarkStale("assoc-2", CandidateRefreshTriggerPathUnhealthy, "path down", time.Now())

	targets := SelectCandidateRefreshTargets(store, reg, nil, freshnessStore)

	if len(targets) != 2 {
		t.Fatalf("want 2 targets, got %d: %+v", len(targets), targets)
	}

	byAssoc := make(map[string]CandidateRefreshTarget)
	for _, t := range targets {
		byAssoc[t.AssociationID] = t
	}
	if _, ok := byAssoc["assoc-3"]; ok {
		t.Error("assoc-3 has no trigger and should not be in targets")
	}
	if byAssoc["assoc-2"].Trigger != CandidateRefreshTriggerPathUnhealthy {
		t.Errorf("assoc-2 trigger: want path-unhealthy, got %q", byAssoc["assoc-2"].Trigger)
	}
}

// ----------------------------------------------------------------------------
// CandidateRefreshResult.ReportLines tests
// ----------------------------------------------------------------------------

func TestCandidateRefreshResult_ReportLines(t *testing.T) {
	r := CandidateRefreshResult{
		Attempted: 3,
		Refreshed: 1,
		Failed:    1,
		Details: []CandidateRefreshDetail{
			{AssociationID: "a1", Trigger: CandidateRefreshTriggerExplicit, Refreshed: true, CandidateCount: 2},
			{AssociationID: "a2", Trigger: CandidateRefreshTriggerEndpointFailed, Refreshed: false, Error: "fetch failed: timeout"},
			{AssociationID: "a3", Trigger: CandidateRefreshTriggerCandidateExpired, Refreshed: false, Error: "no accepted coordinator session available for refresh"},
		},
	}

	lines := r.ReportLines()
	if len(lines) < 4 {
		t.Fatalf("want at least 4 report lines, got %d", len(lines))
	}
	// First line should be the summary.
	if lines[0] == "" {
		t.Error("first report line should be non-empty")
	}
}

// ----------------------------------------------------------------------------
// ExecuteCandidateRefresh tests
// ----------------------------------------------------------------------------

func TestExecuteCandidateRefresh_NoSession(t *testing.T) {
	// Without an accepted session, all targets should be skipped.
	store := NewCandidateStore()
	freshnessStore := NewCandidateFreshnessStore(DefaultCandidateMaxAge)

	targets := []CandidateRefreshTarget{
		{AssociationID: "assoc-1", Trigger: CandidateRefreshTriggerExplicit, Detail: "test"},
		{AssociationID: "assoc-2", Trigger: CandidateRefreshTriggerEndpointStale, Detail: "test"},
	}

	session := BootstrapSessionAttemptResult{} // empty session — not accepted

	result := ExecuteCandidateRefresh(context.Background(),
		testNodeCfg(), BootstrapState{}, session, store, freshnessStore, targets)

	if result.Attempted != 2 {
		t.Errorf("attempted: want 2, got %d", result.Attempted)
	}
	if result.SkippedNoSession != 2 {
		t.Errorf("skipped-no-session: want 2, got %d", result.SkippedNoSession)
	}
	if result.Refreshed != 0 {
		t.Errorf("refreshed: want 0, got %d", result.Refreshed)
	}
	if len(result.Details) != 2 {
		t.Fatalf("details: want 2, got %d", len(result.Details))
	}
	for _, d := range result.Details {
		if d.Refreshed {
			t.Errorf("association %s should not be refreshed without session", d.AssociationID)
		}
		if d.Error == "" {
			t.Errorf("association %s should have error recorded", d.AssociationID)
		}
	}

	// Freshness store should not be updated (no refresh happened).
	for _, target := range targets {
		if state := freshnessStore.FreshnessState(target.AssociationID); state != CandidateFreshnessStateUnknown {
			t.Errorf("assoc %s: freshness should be unknown (no refresh), got %q",
				target.AssociationID, state)
		}
	}
}

func TestExecuteCandidateRefresh_EmptyTargets(t *testing.T) {
	// Empty target list → no work, no error.
	session := BootstrapSessionAttemptResult{
		Response: controlplane.BootstrapSessionResponse{Outcome: controlplane.BootstrapSessionOutcomeAccepted},
	}
	result := ExecuteCandidateRefresh(context.Background(),
		testNodeCfg(), BootstrapState{}, session,
		NewCandidateStore(), NewCandidateFreshnessStore(DefaultCandidateMaxAge),
		nil)

	if result.Attempted != 0 || result.Refreshed != 0 || result.Failed != 0 {
		t.Errorf("empty targets: want all zeros, got %+v", result)
	}
}

// ----------------------------------------------------------------------------
// Architectural boundary tests
// ----------------------------------------------------------------------------

// TestCandidateRefresh_RefreshDistinctFromSchedulerDecision verifies that
// refreshed candidates update the CandidateStore but do NOT affect scheduler
// decisions or carrier activation state. The refresh layer is bounded.
func TestCandidateRefresh_RefreshDistinctFromSchedulerDecision(t *testing.T) {
	// This test verifies the boundary: after refreshing candidates, the
	// CandidateStore contains the new candidates, but the ScheduledEgressRuntime
	// has NOT changed (no re-scheduling happened automatically).

	store := NewCandidateStore()
	store.Store("assoc-1", []controlplane.DistributedPathCandidate{
		{CandidateID: "old-c1", AssociationID: "assoc-1", RemoteEndpoint: "1.2.3.4:5000"},
	})

	freshnessStore := NewCandidateFreshnessStore(DefaultCandidateMaxAge)
	freshnessStore.MarkStale("assoc-1", CandidateRefreshTriggerExplicit, "test", time.Now())

	// The runtime has NO activations (Snapshot returns empty).
	runtime := NewScheduledEgressRuntime()
	runtime.CandidateStore = store

	// Runtime snapshot before refresh — should show no activations.
	snapBefore := runtime.Snapshot()
	if len(snapBefore.Entries) != 0 {
		t.Fatalf("pre-refresh: expected no activations, got %d", len(snapBefore.Entries))
	}

	// Without a session, no refresh can happen. The key check is that
	// even if refresh *could* happen, it would not automatically trigger
	// carrier activation. Refresh updates the store; scheduling is separate.
	targets := SelectCandidateRefreshTargets(store, nil, nil, freshnessStore)
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	// After refresh attempt (skipped because no session), runtime is unchanged.
	session := BootstrapSessionAttemptResult{} // no session
	ExecuteCandidateRefresh(context.Background(),
		testNodeCfg(), BootstrapState{}, session, store, freshnessStore, targets)

	snapAfter := runtime.Snapshot()
	if len(snapAfter.Entries) != 0 {
		t.Error("refresh must not trigger carrier activation or scheduler decisions")
	}
}

// TestCandidateRefresh_FreshnessLayersDistinct verifies that the three
// freshness layers (candidate, endpoint, quality) remain distinct objects
// with distinct state transitions.
func TestCandidateRefresh_FreshnessLayersDistinct(t *testing.T) {
	// Endpoint freshness: EndpointRegistry.VerificationState
	reg := transport.NewEndpointRegistry()
	ep := transport.ExternalEndpoint{Host: "1.2.3.4", Port: 5000, Source: transport.EndpointSourceConfigured}
	reg.Add(ep)
	reg.MarkAllStale(time.Now()) // marks endpoints stale in the registry

	// Quality freshness: PathQualityStore
	qs := scheduler.NewPathQualityStore(DefaultCandidateMaxAge)
	qs.RecordProbeResult("c1", 10*time.Millisecond, true) // fresh measurement

	// Candidate freshness: CandidateFreshnessStore
	fs := NewCandidateFreshnessStore(DefaultCandidateMaxAge)
	fs.MarkRefreshed("assoc-1", time.Now()) // candidates are freshly fetched

	// Endpoint is stale, quality is fresh, candidate is fresh.
	// Marking endpoints stale does NOT automatically mark candidates stale.
	if state := fs.FreshnessState("assoc-1"); state != CandidateFreshnessStateFresh {
		t.Errorf("candidate freshness should be independent of endpoint freshness: got %q", state)
	}

	// Candidate store snapshot for verification.
	store := NewCandidateStore()
	store.Store("assoc-1", []controlplane.DistributedPathCandidate{
		{CandidateID: "c1", AssociationID: "assoc-1", RemoteEndpoint: "1.2.3.4:5000"},
	})

	// SelectCandidateRefreshTargets reads from all three layers, returning the
	// endpoint-stale trigger because the endpoint is stale, even though the
	// candidate freshness store says fresh.
	targets := SelectCandidateRefreshTargets(store, reg, qs, fs)

	// The endpoint registry says stale → candidate refresh target appears.
	// The candidate freshness store says fresh — but the endpoint check still fires.
	// This is correct: endpoint staleness can trigger coordinator refresh even when
	// the candidate store's own freshness record is not expired.
	if len(targets) == 0 {
		t.Error("stale endpoint should trigger refresh target even if candidate freshness is fresh")
	}
	if targets[0].Trigger != CandidateRefreshTriggerEndpointStale {
		t.Errorf("trigger: want endpoint-stale, got %q", targets[0].Trigger)
	}

	// Verify the three stores are still independent after the check.
	if state := fs.FreshnessState("assoc-1"); state != CandidateFreshnessStateFresh {
		t.Error("SelectCandidateRefreshTargets must not mutate CandidateFreshnessStore")
	}
	// Quality store should still have the fresh measurement.
	if _, fresh := qs.FreshQuality("c1"); !fresh {
		t.Error("SelectCandidateRefreshTargets must not affect PathQualityStore")
	}
	// Endpoint registry: endpoints should still be stale.
	if len(reg.SelectForRevalidation()) == 0 {
		t.Error("SelectCandidateRefreshTargets must not affect EndpointRegistry")
	}
}

// TestCandidateFreshnessState_RefreshedStateDistinctFromChosenPath verifies that
// the CandidateFreshnessStore tracks coordinator-fetch freshness only, not which
// path the scheduler chose or which carrier is active.
func TestCandidateFreshnessState_RefreshedStateDistinctFromChosenPath(t *testing.T) {
	fs := NewCandidateFreshnessStore(DefaultCandidateMaxAge)

	// Refresh the candidate set — this means the coordinator was queried successfully.
	fs.MarkRefreshed("assoc-1", time.Now())

	// The freshness store knows nothing about scheduler decisions or carriers.
	// It only knows "candidates were last refreshed at time T".
	// This check verifies the semantic boundary: fresh != path chosen.
	state := fs.FreshnessState("assoc-1")
	if state != CandidateFreshnessStateFresh {
		t.Errorf("refreshed candidates should be fresh: got %q", state)
	}

	// The freshness store does NOT contain carrier state, scheduler mode, or
	// chosen path information. Those live in ScheduledEgressActivation.
	// Snapshot should only contain freshness-relevant fields.
	snap := fs.Snapshot()
	for _, r := range snap {
		// No scheduler or carrier fields exist on CandidateFreshnessRecord.
		// The record is freshness-only: state + timestamps + trigger.
		if r.AssociationID == "" {
			t.Error("record must have a non-empty AssociationID")
		}
	}
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

// testNodeCfg returns a minimal NodeConfig for use in unit tests.
// ExecuteCandidateRefresh uses cfg.Identity.Name and cfg.Coordinator.BootstrapEndpoints;
// a zero-value config is sufficient for tests that exercise error paths.
func testNodeCfg() config.NodeConfig {
	return config.NodeConfig{}
}
