package scheduler

import (
	"testing"
	"time"
)

// TestPathQualityStore_Update verifies that Update sets quality directly and
// marks the entry as fresh.
func TestPathQualityStore_Update(t *testing.T) {
	store := NewPathQualityStore(10 * time.Second)

	q := PathQuality{RTT: 20 * time.Millisecond, Jitter: 2 * time.Millisecond, LossFraction: 0.01, Confidence: 0.8}
	store.Update("path-a", q)

	got, ok := store.FreshQuality("path-a")
	if !ok {
		t.Fatal("FreshQuality: expected ok=true after Update")
	}
	if got.RTT != q.RTT {
		t.Errorf("RTT: got %v want %v", got.RTT, q.RTT)
	}
	if got.LossFraction != q.LossFraction {
		t.Errorf("LossFraction: got %v want %v", got.LossFraction, q.LossFraction)
	}
	if got.Confidence != q.Confidence {
		t.Errorf("Confidence: got %v want %v", got.Confidence, q.Confidence)
	}
}

// TestPathQualityStore_FreshQuality_Unknown verifies that FreshQuality returns
// (zero, false) for a path ID that has never been recorded.
func TestPathQualityStore_FreshQuality_Unknown(t *testing.T) {
	store := NewPathQualityStore(10 * time.Second)

	q, ok := store.FreshQuality("nonexistent")
	if ok {
		t.Error("FreshQuality: expected ok=false for unknown path ID")
	}
	if q.Confidence != 0 || q.RTT != 0 {
		t.Errorf("FreshQuality: expected zero quality for unknown path, got %+v", q)
	}
}

// TestPathQualityStore_Stale verifies that FreshQuality returns (zero, false)
// once the measurement is older than MaxAge. This is the key freshness invariant:
// stale measurements must not silently appear as current quality.
func TestPathQualityStore_Stale(t *testing.T) {
	// Use a very short maxAge so we can trigger staleness reliably.
	store := NewPathQualityStore(1 * time.Millisecond)

	store.Update("path-b", PathQuality{RTT: 10 * time.Millisecond, Confidence: 0.9})

	// Sleep longer than maxAge.
	time.Sleep(5 * time.Millisecond)

	q, ok := store.FreshQuality("path-b")
	if ok {
		t.Error("FreshQuality: expected ok=false for stale measurement")
	}
	if q.Confidence != 0 {
		t.Errorf("FreshQuality: stale path should have zero confidence, got %v", q.Confidence)
	}
}

// TestPathQualityStore_RecordProbeResult_FirstSample verifies first-probe behavior:
// values are set directly without EWMA-blending against zero.
func TestPathQualityStore_RecordProbeResult_FirstSample(t *testing.T) {
	tests := []struct {
		name       string
		rtt        time.Duration
		success    bool
		wantRTT    time.Duration
		wantLoss   float64
		wantConfGT float64 // confidence should be > this
	}{
		{
			name:       "successful first probe",
			rtt:        30 * time.Millisecond,
			success:    true,
			wantRTT:    30 * time.Millisecond,
			wantLoss:   0.0,
			wantConfGT: 0.0, // confidence > 0 after success
		},
		{
			name:       "failed first probe",
			rtt:        0,
			success:    false,
			wantRTT:    0,
			wantLoss:   1.0,
			wantConfGT: -1, // confidence should be 0.0 (>= -1 always)
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := NewPathQualityStore(30 * time.Second)
			store.RecordProbeResult("path-x", tc.rtt, tc.success)

			q, ok := store.FreshQuality("path-x")
			if !ok {
				t.Fatal("FreshQuality: expected ok=true after RecordProbeResult")
			}
			if q.RTT != tc.wantRTT {
				t.Errorf("RTT: got %v want %v", q.RTT, tc.wantRTT)
			}
			if q.LossFraction != tc.wantLoss {
				t.Errorf("LossFraction: got %v want %v", q.LossFraction, tc.wantLoss)
			}
			if q.Confidence <= tc.wantConfGT {
				t.Errorf("Confidence: got %v, expected > %v", q.Confidence, tc.wantConfGT)
			}
		})
	}
}

// TestPathQualityStore_RecordProbeResult_Accumulation verifies EWMA behavior:
// multiple successful probes should increase confidence toward 1.0 and produce
// a smoothed RTT estimate. RTT should not jump to the last value.
func TestPathQualityStore_RecordProbeResult_Accumulation(t *testing.T) {
	store := NewPathQualityStore(30 * time.Second)

	// Record 10 successful probes at 20ms RTT.
	for i := 0; i < 10; i++ {
		store.RecordProbeResult("path-c", 20*time.Millisecond, true)
	}

	q, ok := store.FreshQuality("path-c")
	if !ok {
		t.Fatal("FreshQuality: expected ok=true after 10 probes")
	}

	// After 10 successful probes at the same RTT, the EWMA should have converged
	// close to 20ms, and confidence should be above MinConfidence (0.3).
	if q.RTT < 15*time.Millisecond || q.RTT > 25*time.Millisecond {
		t.Errorf("RTT after 10 probes at 20ms: got %v, expected ~20ms", q.RTT)
	}
	if q.Confidence < 0.3 {
		t.Errorf("Confidence after 10 successful probes: got %v, expected >= 0.3", q.Confidence)
	}
	if q.LossFraction > 0.01 {
		t.Errorf("LossFraction after 10 successful probes: got %v, expected ~0", q.LossFraction)
	}
}

// TestPathQualityStore_RecordProbeResult_LossEffect verifies that probe failures
// increase loss fraction and decrease confidence. This ensures the EWMA properly
// reflects observed link quality degradation.
func TestPathQualityStore_RecordProbeResult_LossEffect(t *testing.T) {
	store := NewPathQualityStore(30 * time.Second)

	// Build up a good baseline.
	for i := 0; i < 5; i++ {
		store.RecordProbeResult("path-d", 20*time.Millisecond, true)
	}
	q0, _ := store.FreshQuality("path-d")

	// Record a loss.
	store.RecordProbeResult("path-d", 0, false)

	q1, ok := store.FreshQuality("path-d")
	if !ok {
		t.Fatal("FreshQuality: expected ok=true after mixed probes")
	}

	// Confidence should have decreased after the failure.
	if q1.Confidence >= q0.Confidence {
		t.Errorf("Confidence should decrease after a loss: before=%v after=%v", q0.Confidence, q1.Confidence)
	}

	// LossFraction should have increased.
	if q1.LossFraction <= q0.LossFraction {
		t.Errorf("LossFraction should increase after a loss: before=%v after=%v", q0.LossFraction, q1.LossFraction)
	}
}

// TestPathQualityStore_ConfidenceClamp verifies that confidence is clamped to [0, 1].
// This ensures the EWMA never produces out-of-range confidence values.
func TestPathQualityStore_ConfidenceClamp(t *testing.T) {
	store := NewPathQualityStore(30 * time.Second)

	// Many successes: confidence should approach but not exceed 1.0.
	for i := 0; i < 100; i++ {
		store.RecordProbeResult("path-e", 10*time.Millisecond, true)
	}
	q, _ := store.FreshQuality("path-e")
	if q.Confidence > 1.0 {
		t.Errorf("Confidence exceeds 1.0: %v", q.Confidence)
	}

	// Many failures: confidence should reach but not go below 0.0.
	for i := 0; i < 100; i++ {
		store.RecordProbeResult("path-e", 0, false)
	}
	q, _ = store.FreshQuality("path-e")
	if q.Confidence < 0.0 {
		t.Errorf("Confidence below 0.0: %v", q.Confidence)
	}
}

// TestPathQualityStore_ApplyCandidates verifies that ApplyCandidates enriches
// candidates that have fresh measurements and leaves others unchanged.
// This tests the integration point between the measurement store and scheduler inputs.
func TestPathQualityStore_ApplyCandidates(t *testing.T) {
	store := NewPathQualityStore(30 * time.Second)

	// Record quality for one of two candidates.
	store.Update("assoc1:direct", PathQuality{RTT: 15 * time.Millisecond, Confidence: 0.7})

	candidates := []PathCandidate{
		{
			ID:            "assoc1:direct",
			AssociationID: "assoc1",
			Class:         PathClassDirectPublic,
			Health:        HealthStateActive,
		},
		{
			ID:            "assoc1:relay",
			AssociationID: "assoc1",
			Class:         PathClassCoordinatorRelay,
			Health:        HealthStateActive,
		},
	}

	enriched := store.ApplyCandidates(candidates)

	if len(enriched) != 2 {
		t.Fatalf("ApplyCandidates: expected 2 results, got %d", len(enriched))
	}

	// The direct candidate should have quality applied.
	if enriched[0].Quality.RTT != 15*time.Millisecond {
		t.Errorf("direct candidate RTT: got %v want 15ms", enriched[0].Quality.RTT)
	}
	if enriched[0].Quality.Confidence != 0.7 {
		t.Errorf("direct candidate Confidence: got %v want 0.7", enriched[0].Quality.Confidence)
	}

	// The relay candidate has no measurement — Quality should remain zero.
	if enriched[1].Quality.RTT != 0 {
		t.Errorf("relay candidate RTT: expected 0 (unmeasured), got %v", enriched[1].Quality.RTT)
	}
	if enriched[1].Quality.Confidence != 0 {
		t.Errorf("relay candidate Confidence: expected 0 (unmeasured), got %v", enriched[1].Quality.Confidence)
	}
}

// TestPathQualityStore_ApplyCandidates_OriginalUnchanged verifies that
// ApplyCandidates does not modify the original slice. The store's quality layer
// must not silently mutate caller-owned data.
func TestPathQualityStore_ApplyCandidates_OriginalUnchanged(t *testing.T) {
	store := NewPathQualityStore(30 * time.Second)
	store.Update("path-f", PathQuality{RTT: 25 * time.Millisecond, Confidence: 0.5})

	original := []PathCandidate{
		{ID: "path-f", AssociationID: "assoc2", Class: PathClassDirectPublic, Health: HealthStateActive},
	}
	originalRTT := original[0].Quality.RTT

	_ = store.ApplyCandidates(original)

	if original[0].Quality.RTT != originalRTT {
		t.Error("ApplyCandidates modified the original slice — must not mutate caller data")
	}
}

// TestPathQualityStore_Snapshot_FreshnessLabels verifies that Snapshot correctly
// marks stale entries. An operator reading the snapshot must be able to distinguish
// fresh measurements from stale ones.
func TestPathQualityStore_Snapshot_FreshnessLabels(t *testing.T) {
	store := NewPathQualityStore(5 * time.Millisecond)

	store.Update("fresh-path", PathQuality{RTT: 10 * time.Millisecond, Confidence: 0.9})
	store.Update("stale-path", PathQuality{RTT: 20 * time.Millisecond, Confidence: 0.8})

	// Wait for stale-path to age out. We immediately overwrite fresh-path to
	// keep it fresh, then sleep to make stale-path old.
	time.Sleep(10 * time.Millisecond)
	store.Update("fresh-path", PathQuality{RTT: 10 * time.Millisecond, Confidence: 0.9})

	snaps := store.Snapshot()

	found := map[string]MeasuredPathQuality{}
	for _, s := range snaps {
		found[s.PathID] = s
	}

	freshSnap, ok := found["fresh-path"]
	if !ok {
		t.Fatal("fresh-path not in Snapshot")
	}
	if freshSnap.Stale {
		t.Error("fresh-path should not be marked Stale")
	}

	staleSnap, ok := found["stale-path"]
	if !ok {
		t.Fatal("stale-path not in Snapshot")
	}
	if !staleSnap.Stale {
		t.Error("stale-path should be marked Stale after MaxAge elapsed")
	}
}

// TestPathQualityStore_MeasurementDistinctFromCandidateExistence verifies that
// the quality store knows nothing about PathCandidate existence, health, or class.
// Measurement state and candidate existence are separate layers.
func TestPathQualityStore_MeasurementDistinctFromCandidateExistence(t *testing.T) {
	store := NewPathQualityStore(30 * time.Second)

	// Record quality for a path ID.
	store.Update("assoc99:direct", PathQuality{RTT: 5 * time.Millisecond, Confidence: 0.6})

	// Build a candidate for a DIFFERENT association — quality must not bleed across.
	candidates := []PathCandidate{
		{ID: "assocXX:direct", AssociationID: "assocXX", Class: PathClassDirectPublic, Health: HealthStateActive},
	}
	enriched := store.ApplyCandidates(candidates)
	if enriched[0].Quality.Confidence != 0 {
		t.Error("quality from assoc99 leaked into assocXX candidate — measurement must be ID-scoped")
	}

	// A candidate for the measured path does get quality applied.
	candidates2 := []PathCandidate{
		{ID: "assoc99:direct", AssociationID: "assoc99", Class: PathClassDirectPublic, Health: HealthStateActive},
	}
	enriched2 := store.ApplyCandidates(candidates2)
	if enriched2[0].Quality.Confidence != 0.6 {
		t.Errorf("expected quality applied to assoc99:direct candidate, got confidence=%v", enriched2[0].Quality.Confidence)
	}

	// The store itself cannot tell you whether a candidate is healthy or what class it is.
	// Those are PathCandidate fields, not measurement store concerns.
	snap := store.Snapshot()
	if len(snap) != 1 || snap[0].PathID != "assoc99:direct" {
		t.Errorf("snapshot should contain only measured paths, got %+v", snap)
	}
}

// TestPathQualityStore_UpdateOverride verifies that Update overwrites prior
// EWMA-based state. This tests the "replace, don't blend" contract of Update.
func TestPathQualityStore_UpdateOverride(t *testing.T) {
	store := NewPathQualityStore(30 * time.Second)

	// Build up some EWMA state.
	for i := 0; i < 5; i++ {
		store.RecordProbeResult("path-g", 100*time.Millisecond, true)
	}

	// Now override with a fresh direct observation.
	store.Update("path-g", PathQuality{RTT: 5 * time.Millisecond, Confidence: 1.0})

	q, ok := store.FreshQuality("path-g")
	if !ok {
		t.Fatal("FreshQuality: expected ok=true after Update")
	}
	if q.RTT != 5*time.Millisecond {
		t.Errorf("RTT after Update override: got %v want 5ms", q.RTT)
	}
	if q.Confidence != 1.0 {
		t.Errorf("Confidence after Update override: got %v want 1.0", q.Confidence)
	}
}

// TestPathQualityStore_SampleCount verifies that the sample counter increments
// on each RecordProbeResult and Update call. This makes the measurement history
// inspectable in the snapshot.
func TestPathQualityStore_SampleCount(t *testing.T) {
	store := NewPathQualityStore(30 * time.Second)

	store.RecordProbeResult("path-h", 10*time.Millisecond, true)
	store.RecordProbeResult("path-h", 12*time.Millisecond, true)
	store.RecordProbeResult("path-h", 0, false)

	snaps := store.Snapshot()
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot entry, got %d", len(snaps))
	}
	if snaps[0].SampleCount != 3 {
		t.Errorf("SampleCount: expected 3, got %d", snaps[0].SampleCount)
	}
}

// BenchmarkApplyCandidates measures the cost of enriching candidates from
// the quality store. This path is called before each Scheduler.Decide() call,
// so it should remain fast.
func BenchmarkApplyCandidates(b *testing.B) {
	store := NewPathQualityStore(30 * time.Second)

	// Pre-populate with quality for the candidates we'll use.
	for i := 0; i < 4; i++ {
		store.Update(
			pathID(i),
			PathQuality{RTT: time.Duration(10+i*5) * time.Millisecond, Confidence: 0.8},
		)
	}

	candidates := make([]PathCandidate, 4)
	for i := range candidates {
		candidates[i] = PathCandidate{
			ID:            pathID(i),
			AssociationID: "bench-assoc",
			Class:         PathClassDirectPublic,
			Health:        HealthStateActive,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.ApplyCandidates(candidates)
	}
}

// pathID generates a deterministic path ID string for benchmark use.
func pathID(i int) string {
	return "bench-assoc:path-" + string(rune('0'+i))
}
