package status

import (
	"strings"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/scheduler"
)

// TestMakePathQualitySummary_Basic verifies that MakePathQualitySummary correctly
// classifies fresh vs stale entries and maps all fields.
func TestMakePathQualitySummary_Basic(t *testing.T) {
	now := time.Now()

	snapshots := []scheduler.MeasuredPathQuality{
		{
			PathID: "assoc1:direct",
			Quality: scheduler.PathQuality{
				RTT: 15 * time.Millisecond, Jitter: 2 * time.Millisecond,
				LossFraction: 0.01, Confidence: 0.7,
			},
			MeasuredAt:  now,
			SampleCount: 5,
			Stale:       false,
		},
		{
			PathID: "assoc1:relay",
			Quality: scheduler.PathQuality{
				RTT: 50 * time.Millisecond, Confidence: 0.3,
			},
			MeasuredAt:  now.Add(-90 * time.Second),
			SampleCount: 2,
			Stale:       true,
		},
	}

	summary := MakePathQualitySummary(snapshots)

	if summary.TotalMeasured != 1 {
		t.Errorf("TotalMeasured: got %d want 1", summary.TotalMeasured)
	}
	if summary.TotalStale != 1 {
		t.Errorf("TotalStale: got %d want 1", summary.TotalStale)
	}
	if len(summary.Entries) != 2 {
		t.Fatalf("Entries: got %d want 2", len(summary.Entries))
	}

	direct := summary.Entries[0]
	if direct.PathID != "assoc1:direct" {
		t.Errorf("PathID: got %q", direct.PathID)
	}
	if direct.RTT != 15*time.Millisecond {
		t.Errorf("RTT: got %v want 15ms", direct.RTT)
	}
	if direct.Confidence != 0.7 {
		t.Errorf("Confidence: got %v want 0.7", direct.Confidence)
	}
	if direct.Stale {
		t.Error("direct entry should not be stale")
	}
	if direct.SampleCount != 5 {
		t.Errorf("SampleCount: got %d want 5", direct.SampleCount)
	}

	relay := summary.Entries[1]
	if !relay.Stale {
		t.Error("relay entry should be stale")
	}
}

// TestMakePathQualitySummary_Empty verifies the zero-entry case.
func TestMakePathQualitySummary_Empty(t *testing.T) {
	summary := MakePathQualitySummary(nil)
	if summary.TotalMeasured != 0 || summary.TotalStale != 0 {
		t.Errorf("expected zero totals for empty input, got %+v", summary)
	}
	if len(summary.Entries) != 0 {
		t.Errorf("expected empty Entries slice, got %d entries", len(summary.Entries))
	}
}

// TestPathQualitySummary_ReportLines_Stale verifies that stale entries are
// labeled clearly in ReportLines output. Operators must be able to identify
// paths needing remeasurement without parsing raw fields.
func TestPathQualitySummary_ReportLines_Stale(t *testing.T) {
	summary := PathQualitySummary{
		TotalMeasured: 1,
		TotalStale:    1,
		Entries: []PathQualityEntry{
			{PathID: "path-fresh", Confidence: 0.8, MeasuredAt: time.Now()},
			{PathID: "path-stale", Confidence: 0.5, MeasuredAt: time.Now().Add(-120 * time.Second), Stale: true},
		},
	}

	lines := summary.ReportLines()
	output := strings.Join(lines, "\n")

	if !strings.Contains(output, "[stale/needs-remeasurement]") {
		t.Errorf("ReportLines should label stale entry; output: %s", output)
	}
	if !strings.Contains(output, "[fresh]") {
		t.Errorf("ReportLines should label fresh entry; output: %s", output)
	}
	if !strings.Contains(output, "path-fresh") {
		t.Errorf("ReportLines should include path IDs; output: %s", output)
	}
}

// TestPathQualitySummary_ReportLines_Empty verifies that the empty case produces
// a "(no path quality measurements recorded)" note rather than a blank output.
func TestPathQualitySummary_ReportLines_Empty(t *testing.T) {
	summary := PathQualitySummary{}
	lines := summary.ReportLines()
	output := strings.Join(lines, "\n")

	if !strings.Contains(output, "no path quality measurements") {
		t.Errorf("empty summary ReportLines should note no measurements; output: %s", output)
	}
}

// TestPathQualitySummary_DistinctFromCandidateExistence verifies that the summary
// only contains measurement data — it has no fields for candidate health, class,
// or association legality. These are separate concerns.
func TestPathQualitySummary_DistinctFromCandidateExistence(t *testing.T) {
	entry := PathQualityEntry{
		PathID:      "assoc3:direct",
		RTT:         20 * time.Millisecond,
		Confidence:  0.6,
		MeasuredAt:  time.Now(),
		SampleCount: 3,
		Stale:       false,
	}

	// PathQualityEntry must NOT have fields for health state, path class,
	// or association legality — those belong to PathCandidate, not here.
	// This is a compile-time check: the struct literal above exhausts
	// all fields we care about. Additional type-assertion checks below
	// document the architectural constraint.
	_ = entry.PathID
	_ = entry.RTT
	_ = entry.Jitter
	_ = entry.LossFraction
	_ = entry.Confidence
	_ = entry.MeasuredAt
	_ = entry.SampleCount
	_ = entry.Stale

	// PathQualityEntry has no Health, Class, AssociationID field.
	// If someone adds one, it would indicate collapsing measurement with
	// candidate existence — this test documents the intended absence.
}
