package scheduler

import "sync/atomic"

// AssociationCounters holds atomic per-association scheduling decision counters.
//
// These counters make scheduling behavior observable for debugging and operational
// review. An operator or future agent should be able to inspect these counters
// to understand how the scheduler has been behaving for each association without
// needing to instrument the decision logic itself.
//
// All fields are atomic so counters can be updated from multiple goroutines
// without holding a lock.
type AssociationCounters struct {
	TotalDecisions    atomic.Uint64
	WeightedBurst     atomic.Uint64
	PerPacketStripe   atomic.Uint64
	SinglePath        atomic.Uint64
	NoEligiblePath    atomic.Uint64
	StripingActivated atomic.Uint64
}

// AssociationCountersSnapshot is a point-in-time copy of counters for one
// association. It is safe to read and pass around after creation.
type AssociationCountersSnapshot struct {
	AssociationID     string
	TotalDecisions    uint64
	WeightedBurst     uint64
	PerPacketStripe   uint64
	SinglePath        uint64
	NoEligiblePath    uint64
	StripingActivated uint64
}

// SchedulerStatus is a point-in-time summary of scheduler state.
//
// It is intended for status display, logging, and debugging. The combination
// of Thresholds and Counters gives an operator enough information to understand
// whether the scheduler is behaving as configured.
//
// Scheduler behavior must be observable (spec/v1-data-plane.md section 22):
// the Reason field on individual decisions explains per-decision choices, while
// Counters here give the aggregate picture.
type SchedulerStatus struct {
	Thresholds StripeMatchThresholds
	Counters   []AssociationCountersSnapshot
}

// Status returns a current snapshot of scheduler state.
func (s *Scheduler) Status() SchedulerStatus {
	return SchedulerStatus{
		Thresholds: s.thresholds,
		Counters:   s.CountersSnapshot(),
	}
}

// SchedulerCarriageStatus describes what the current scheduler implementation
// supports and does not support.
//
// This is returned by ReportSchedulerStatus and is intended for operator review
// and for keeping future agents informed of implementation boundaries.
type SchedulerCarriageStatus struct {
	Implemented    []string
	NotImplemented []string
}

// ReportSchedulerStatus returns a structured description of the current
// scheduler implementation scope. This makes the implementation boundaries
// explicit and prevents false assumptions about what is available.
func ReportSchedulerStatus() SchedulerCarriageStatus {
	return SchedulerCarriageStatus{
		Implemented: []string{
			"endpoint-owned scheduling: Scheduler runs at source endpoint only; relays do not schedule",
			"association-bound decisions: each Decide() call is scoped to one association ID",
			"PathCandidate model: explicit type, distinct from RelayCandidate and ForwardingEntry",
			"RelayCandidate model: explicit type, distinct from PathCandidate",
			"PathClass: direct-public, direct-intranet, coordinator-relay, node-relay",
			"HealthState: candidate, active, degraded, standby, probe-only, failed, admin-disabled",
			"PathQuality: RTT, jitter, loss fraction, confidence inputs",
			"weighted burst/flowlet mode (ModeWeightedBurstFlowlet): default when paths are not closely matched",
			"per-packet striping mode (ModePerPacketStripe): conditional on RTT/jitter/loss spread thresholds",
			"striping match gate: all-or-nothing; blocks striping if any path is unmeasured or outside thresholds",
			"relay path scoring penalty: direct paths preferred over relay paths when quality is similar",
			"degraded health penalty: degraded paths are lower-priority but still eligible",
			"loss scoring penalty: applied when quality is measured",
			"RTT scoring penalty: applied above 50ms base threshold when quality is measured",
			"observable counters: per-association decision/mode/striping counts (AssociationCounters)",
			"decision reasoning: every SchedulerDecision.Reason is non-empty and human-readable",
			"SchedulerStatus snapshot: thresholds and counters readable at any time",
			"scheduler-to-carrier integration: Decide() results wired into ScheduledEgressRuntime; governs DirectCarrier vs RelayEgressCarrier activation per association",
			"live path quality measurement basics: PathQualityStore (EWMA RTT/jitter/loss, confidence, freshness-aware staleness); RecordProbeResult for probe-driven updates; Update for direct quality injection; ApplyCandidates wired into ScheduledEgressRuntime before Decide(); QualitySnapshot for operator observability; MeasuredPathQuality distinct from PathCandidate existence",
		},
		NotImplemented: []string{
			"active probing integration (PathQualityStore accepts probe results; probe scheduling loop not yet wired into runtime)",
			"hysteresis for path switching (preventing oscillation on noisy quality measurements)",
			"multi-path carrier load balancing (per-packet delivery split not yet implemented at carrier level)",
			"node relay scheduling (only coordinator relay defined in PathClass; node relay not yet active)",
			"metered path bandwidth limiting or reduced-weight treatment",
			"goodput-based scoring (only RTT/loss/jitter scalar inputs used; goodput not measured)",
			"coordinator-distributed path candidates (candidates currently local config only; not pushed by coordinator)",
			"dynamic relay candidate selection from RelayCandidate pool",
			"failover signaling (switching carrier socket on path health change not yet implemented)",
		},
	}
}
