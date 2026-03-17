package scheduler

import (
	"math"
	"sync"
	"time"
)

// DefaultQualityMaxAge is the default freshness window for measured path quality.
// Measurements older than this are treated as stale: FreshQuality returns (zero, false),
// and the scheduler treats the path as unmeasured (confidence=0, no striping).
const DefaultQualityMaxAge = 60 * time.Second

// ewmaRTTAlpha is the EWMA smoothing factor for RTT updates.
// Follows RFC 6298 / 6298bis guidance (0.125 = 1/8).
const ewmaRTTAlpha = 0.125

// ewmaJitterAlpha is the EWMA smoothing factor for jitter (RTT variation) estimation.
const ewmaJitterAlpha = 0.25

// ewmaLossAlpha is the EWMA smoothing factor for packet loss fraction.
const ewmaLossAlpha = 0.1

// confidenceStep is how much confidence changes per probe result.
// Successful probes increase confidence by one step.
// Failed probes decrease confidence by two steps (loss is penalized more than
// a single success improves things, reflecting the asymmetric impact of loss).
const confidenceStep = 0.1

// PathQualityStore is a thread-safe, freshness-aware store of per-path measured
// quality. It is the live measurement input layer between probe/observation
// signals and the scheduler's PathCandidate quality inputs.
//
// Architectural separation (what this store is NOT):
//   - Not scheduling: the store provides quality inputs; scheduling decisions are
//     made by Scheduler.Decide() which reads PathCandidate.Quality.
//   - Not candidate existence: a path ID in this store does not imply a
//     PathCandidate with that ID exists or is healthy. The two are separate objects.
//   - Not applied runtime behavior: what carrier is actually running is tracked
//     separately by ScheduledEgressActivation.CarrierActivated.
//
// Confidence and freshness are explicit: once a measurement is older than MaxAge,
// FreshQuality returns (zero, false). Callers must treat the path as unmeasured.
// The scheduler handles unmeasured paths conservatively — they are eligible for
// carriage but cannot qualify for per-packet striping (which requires Confidence >=
// StripeMatchThresholds.MinConfidence across all paths).
//
// This store is intentionally bounded: it stores and ages per-path measurements
// but does not perform active probing itself. Probe loops or passive observers
// drive updates via RecordProbeResult or Update.
type PathQualityStore struct {
	mu      sync.RWMutex
	entries map[string]*measuredEntry
	maxAge  time.Duration
}

// measuredEntry holds the per-path quality sample and tracking state.
type measuredEntry struct {
	quality     PathQuality
	measuredAt  time.Time
	sampleCount uint32
	initialized bool // false until at least one sample has been recorded
}

// MeasuredPathQuality is a point-in-time snapshot of one path's measured quality,
// including freshness state. This is the observability-facing representation.
//
// Stale=true means FreshQuality would return (zero, false) for this path: the
// measurement is too old to trust for fine-grained scheduling decisions. The
// scheduler treats stale paths as unmeasured (confidence=0).
type MeasuredPathQuality struct {
	PathID      string
	Quality     PathQuality
	MeasuredAt  time.Time
	SampleCount uint32
	Stale       bool // true when MeasuredAt is older than the store's MaxAge
}

// NewPathQualityStore creates a new PathQualityStore with the given freshness window.
// Use DefaultQualityMaxAge for a sensible v1 default.
func NewPathQualityStore(maxAge time.Duration) *PathQualityStore {
	return &PathQualityStore{
		entries: make(map[string]*measuredEntry),
		maxAge:  maxAge,
	}
}

// Update records a complete quality snapshot for a path, replacing any prior value.
//
// Use this when you have a fully computed quality snapshot (e.g., from passive
// traffic observation or an external measurement source). Unlike RecordProbeResult,
// Update does not blend with prior measurements via EWMA — it replaces them.
//
// Calling Update resets the measurement timestamp to now, making the entry fresh
// regardless of prior state.
func (s *PathQualityStore) Update(pathID string, q PathQuality) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[pathID]
	if !ok {
		e = &measuredEntry{}
		s.entries[pathID] = e
	}
	e.quality = q
	e.measuredAt = time.Now()
	e.initialized = true
	e.sampleCount++
}

// RecordProbeResult updates stored quality for a path from one probe sample
// (RTT + success/failure). Multiple calls accumulate via EWMA to produce
// smoothed RTT, jitter, and loss fraction estimates.
//
// RTT and Jitter use EWMA (RFC 6298 style: alpha=0.125 for RTT, 0.25 for jitter
// deviation from the prior RTT estimate). LossFraction uses EWMA (alpha=0.1)
// over {0.0 on success, 1.0 on failure}. Confidence changes by confidenceStep
// on success, and by -2*confidenceStep on failure, clamped to [0.0, 1.0].
//
// The measurement timestamp is updated to now on each call, keeping the entry
// fresh. Staleness is determined in FreshQuality based on time elapsed since the
// most recent call.
//
// This is the expected entry point for lightweight active probe integration.
// It is safe for concurrent use.
func (s *PathQualityStore) RecordProbeResult(pathID string, rtt time.Duration, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[pathID]
	if !ok {
		e = &measuredEntry{}
		s.entries[pathID] = e
	}
	e.measuredAt = time.Now()
	e.sampleCount++

	if !e.initialized {
		// First sample: set values directly without EWMA-blending against zero.
		if success && rtt > 0 {
			e.quality.RTT = rtt
			e.quality.Jitter = 0
			e.quality.LossFraction = 0.0
			e.quality.Confidence = confidenceStep
		} else {
			// First probe failed: no RTT estimate yet; record initial loss.
			e.quality.RTT = 0
			e.quality.Jitter = 0
			e.quality.LossFraction = 1.0
			e.quality.Confidence = 0.0
		}
		e.initialized = true
		return
	}

	// EWMA RTT and Jitter update (only on successful probe with valid RTT).
	if success && rtt > 0 {
		prev := e.quality.RTT
		if prev == 0 {
			// No prior RTT: bootstrap directly from this sample.
			e.quality.RTT = rtt
		} else {
			// EWMA smoothed RTT: smoothedRTT = alpha*sample + (1-alpha)*smoothedRTT.
			e.quality.RTT = time.Duration(ewmaRTTAlpha*float64(rtt) + (1-ewmaRTTAlpha)*float64(prev))

			// Jitter: EWMA of absolute RTT deviation from prior estimate.
			dev := rtt - prev
			if dev < 0 {
				dev = -dev
			}
			e.quality.Jitter = time.Duration(
				ewmaJitterAlpha*float64(dev) + (1-ewmaJitterAlpha)*float64(e.quality.Jitter),
			)
		}
	}

	// EWMA loss fraction: success → observed=0.0, failure → observed=1.0.
	observed := 0.0
	if !success {
		observed = 1.0
	}
	e.quality.LossFraction = ewmaLossAlpha*observed + (1-ewmaLossAlpha)*e.quality.LossFraction

	// Confidence update.
	if success {
		e.quality.Confidence = math.Min(1.0, e.quality.Confidence+confidenceStep)
	} else {
		// Failures penalize confidence more than successes improve it.
		e.quality.Confidence = math.Max(0.0, e.quality.Confidence-2*confidenceStep)
	}
}

// FreshQuality returns the stored quality for a path if it is fresh (within MaxAge).
// Returns (zero PathQuality, false) when:
//   - no entry exists for pathID (path unknown to the store)
//   - the entry has never been initialized
//   - the entry is older than MaxAge (stale)
//
// When false is returned, callers must treat the path as unmeasured (confidence=0).
// The scheduler handles unmeasured paths conservatively: they are eligible for
// carriage but cannot qualify for per-packet striping.
//
// This explicit freshness check prevents old measurements from silently passing
// as current, which would falsely inflate confidence and potentially misgate
// the per-packet striping threshold.
func (s *PathQualityStore) FreshQuality(pathID string) (PathQuality, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entries[pathID]
	if !ok || !e.initialized {
		return PathQuality{}, false
	}
	if time.Since(e.measuredAt) > s.maxAge {
		// Stale: measurement is too old. Do not return it as current quality.
		// Returning false ensures the scheduler sees unmeasured (confidence=0).
		return PathQuality{}, false
	}
	return e.quality, true
}

// ApplyCandidates returns a new slice of PathCandidates with Quality fields
// enriched from fresh measurements in the store. Candidates without fresh
// measurements are returned unchanged (Quality remains zero / unmeasured).
//
// The returned slice is always a new allocation — originals are not modified.
// Only Quality is updated; candidate identity fields (ID, Class, Health, etc.)
// are preserved unchanged. This keeps measurement and candidate layers separate.
//
// Call ApplyCandidates before Scheduler.Decide() to give the scheduler current
// quality data. The scheduler still works correctly with unmeasured candidates —
// it just cannot enable per-packet striping for them.
func (s *PathQualityStore) ApplyCandidates(candidates []PathCandidate) []PathCandidate {
	if len(candidates) == 0 {
		return candidates
	}
	result := make([]PathCandidate, len(candidates))
	copy(result, candidates)
	for i := range result {
		if q, ok := s.FreshQuality(result[i].ID); ok {
			result[i].Quality = q
		}
		// No fresh measurement: Quality stays zero (unmeasured, confidence=0).
	}
	return result
}

// Snapshot returns a point-in-time view of all stored quality entries,
// including freshness state. This is the observability surface for the store.
//
// Entries with Stale=true have measurements too old for fine-grained scheduling.
// The scheduler receives zero confidence for stale paths via FreshQuality.
//
// Snapshot is safe for concurrent use.
func (s *PathQualityStore) Snapshot() []MeasuredPathQuality {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	result := make([]MeasuredPathQuality, 0, len(s.entries))
	for id, e := range s.entries {
		if !e.initialized {
			continue
		}
		stale := now.Sub(e.measuredAt) > s.maxAge
		result = append(result, MeasuredPathQuality{
			PathID:      id,
			Quality:     e.quality,
			MeasuredAt:  e.measuredAt,
			SampleCount: e.sampleCount,
			Stale:       stale,
		})
	}
	return result
}
