package status

import (
	"fmt"
	"time"

	"github.com/zhouchenh/transitloom/internal/scheduler"
)

// PathQualitySummary is the observability surface for measured path quality.
// It makes the live measurement layer visible to operators: what is measured,
// what is stale, and what is unknown.
//
// This is explicitly NOT:
//   - PathCandidate state (candidate existence, health, class — those live in the
//     scheduler's PathCandidate type and are driven by control-plane distribution)
//   - ScheduledEgressSummary (applied carrier behavior at runtime)
//   - Scheduling decisions (Scheduler.Decide() output)
//
// It shows only the quality measurement inputs that the scheduler may consume.
// An operator reading this can determine whether the scheduler has fresh quality
// data for each path, which directly affects whether per-packet striping can be
// activated (striping requires Confidence >= StripeMatchThresholds.MinConfidence).
type PathQualitySummary struct {
	TotalMeasured int // entries with fresh (non-stale) measurements
	TotalStale    int // entries with measurements older than MaxAge
	Entries       []PathQualityEntry
}

// PathQualityEntry describes the measured quality for one path candidate.
//
// Stale=true means the scheduler receives zero confidence for this path (via
// PathQualityStore.FreshQuality returning false). The path is still eligible for
// carriage but cannot qualify for per-packet striping until a fresh measurement
// is recorded.
type PathQualityEntry struct {
	PathID       string
	RTT          time.Duration
	Jitter       time.Duration
	LossFraction float64
	Confidence   float64
	MeasuredAt   time.Time
	SampleCount  uint32

	// Stale is true when the measurement is older than the store's MaxAge.
	// Stale entries have zero effective confidence for the scheduler.
	Stale bool
}

// ReportLines produces human-readable status lines for operator review.
// Stale entries are labeled explicitly so operators can identify paths that
// need fresh measurement before fine-grained scheduling decisions can be made.
func (s PathQualitySummary) ReportLines() []string {
	lines := make([]string, 0, len(s.Entries)+2)
	lines = append(lines, fmt.Sprintf(
		"path-quality: measured=%d stale=%d",
		s.TotalMeasured, s.TotalStale,
	))
	for _, e := range s.Entries {
		freshLabel := "[fresh]"
		if e.Stale {
			freshLabel = "[stale/needs-remeasurement]"
		}
		lines = append(lines, fmt.Sprintf(
			"  path=%s %s rtt=%s jitter=%s loss=%.2f%% confidence=%.2f samples=%d measured-at=%s",
			e.PathID, freshLabel,
			e.RTT, e.Jitter,
			e.LossFraction*100,
			e.Confidence,
			e.SampleCount,
			e.MeasuredAt.Format(time.RFC3339),
		))
	}
	if len(s.Entries) == 0 {
		lines = append(lines, "  (no path quality measurements recorded)")
	}
	return lines
}

// MakePathQualitySummary constructs a PathQualitySummary from a PathQualityStore
// snapshot. This is the bridge between the scheduler's measurement store and the
// status package.
//
// The internal/status package may import internal/scheduler because scheduler is a
// leaf package with no import of status or node — there is no cycle risk.
func MakePathQualitySummary(snapshots []scheduler.MeasuredPathQuality) PathQualitySummary {
	entries := make([]PathQualityEntry, 0, len(snapshots))
	var totalMeasured, totalStale int

	for _, snap := range snapshots {
		entry := PathQualityEntry{
			PathID:       snap.PathID,
			RTT:          snap.Quality.RTT,
			Jitter:       snap.Quality.Jitter,
			LossFraction: snap.Quality.LossFraction,
			Confidence:   snap.Quality.Confidence,
			MeasuredAt:   snap.MeasuredAt,
			SampleCount:  snap.SampleCount,
			Stale:        snap.Stale,
		}
		entries = append(entries, entry)
		if snap.Stale {
			totalStale++
		} else {
			totalMeasured++
		}
	}

	return PathQualitySummary{
		TotalMeasured: totalMeasured,
		TotalStale:    totalStale,
		Entries:       entries,
	}
}
