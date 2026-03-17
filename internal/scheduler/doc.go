// Package scheduler implements the Transitloom v1 endpoint-owned path scheduler.
//
// The scheduler is the source endpoint's authority for deciding which eligible
// path(s) to use for raw UDP association-bound carriage and how to distribute
// traffic across them.
//
// # Core architectural constraints
//
// Endpoint-owned scheduling: the Scheduler runs only at the source endpoint.
// Relay nodes follow installed forwarding context (dataplane.RelayCarrier,
// dataplane.RelayEgressCarrier) and must not override scheduling decisions.
// See spec/v1-data-plane.md section 13.
//
// Association-bound decisions: each scheduling decision is scoped to one
// association. The Scheduler never mixes path candidates across associations.
// A PathCandidate without a valid AssociationID is silently excluded.
//
// # Default scheduling mode
//
// Weighted burst/flowlet-aware (ModeWeightedBurstFlowlet) is the v1 default.
// The scheduler assigns traffic to the best-scoring path for a burst or
// flowlet. This avoids the reordering amplification that unconstrained
// per-packet striping would cause on mismatched paths.
// See spec/v1-data-plane.md section 14.
//
// # Per-packet striping
//
// Per-packet striping (ModePerPacketStripe) is only activated when all eligible
// paths are closely matched within configured quality thresholds (RTT spread,
// jitter spread, loss spread). On mismatched paths, per-packet striping causes
// harmful reordering. The conservative StripeMatchThresholds gate blocks striping
// when paths differ significantly in quality or when measurements are absent.
// See spec/v1-data-plane.md section 15.
//
// # Key types
//
//   - PathCandidate: one candidate data-plane path for an association (score input)
//   - RelayCandidate: a relay-capable intermediate participant (distinct from PathCandidate)
//   - SchedulerDecision: the scheduler's output (chosen paths, mode, reason)
//   - Scheduler: the decision engine (Decide() method is the main entry point)
//   - StripeMatchThresholds: configures the per-packet striping eligibility gate
//   - SchedulerStatus: a point-in-time snapshot of counters and thresholds
//
// # Object-model boundaries preserved
//
// PathCandidate, RelayCandidate, and ForwardingEntry (from internal/dataplane)
// are distinct types and must not be collapsed. See spec/v1-object-model.md
// sections 16-17. The scheduler operates on path candidates; the dataplane
// package operates on installed forwarding entries.
//
// # Observability
//
// Every SchedulerDecision.Reason is non-empty and explains the decision in
// plain text. Per-association counters (AssociationCounters) track decision
// history. The SchedulerStatus snapshot exposes both for operator review.
package scheduler
