package scheduler

import "time"

// PathClass identifies the class of a data-plane path candidate.
// These classes correspond directly to the v1 path classes defined in
// spec/v1-data-plane.md section 7.
type PathClass string

const (
	PathClassDirectPublic     PathClass = "direct-public"
	PathClassDirectIntranet   PathClass = "direct-intranet"
	PathClassCoordinatorRelay PathClass = "coordinator-relay"
	PathClassNodeRelay        PathClass = "node-relay"
)

// IsRelay returns true when the path class uses a relay hop.
func (c PathClass) IsRelay() bool {
	return c == PathClassCoordinatorRelay || c == PathClassNodeRelay
}

// HealthState describes a path candidate's current health classification.
// The spec (v1-data-plane.md section 16.2) defines these states.
type HealthState string

const (
	// HealthStateCandidate: path is known but not yet confirmed active.
	HealthStateCandidate HealthState = "candidate"
	// HealthStateActive: path is healthy and in active use.
	HealthStateActive HealthState = "active"
	// HealthStateDegraded: path is below preferred quality, but still eligible.
	HealthStateDegraded HealthState = "degraded"
	// HealthStateStandby: path is healthy but not currently preferred.
	HealthStateStandby HealthState = "standby"
	// HealthStateProbeOnly: path is only used for lightweight probing.
	HealthStateProbeOnly HealthState = "probe-only"
	// HealthStateFailed: path is not usable for live traffic.
	HealthStateFailed HealthState = "failed"
	// HealthStateAdminDisabled: path is disabled by administrative policy.
	HealthStateAdminDisabled HealthState = "admin-disabled"
)

// IsEligible reports whether this health state makes a path eligible for
// normal live traffic. Failed, admin-disabled, and probe-only paths must not
// carry live traffic.
//
// Degraded paths are eligible but scored lower: they remain available when
// no better path exists but are not preferred over healthy paths.
func (h HealthState) IsEligible() bool {
	switch h {
	case HealthStateActive, HealthStateCandidate, HealthStateDegraded, HealthStateStandby:
		return true
	default:
		return false
	}
}

// PathQuality holds the measurable quality inputs for a path candidate.
// All values are optional: zero values mean "unmeasured" rather than "zero quality."
//
// The scheduler uses these as scoring inputs. Unmeasured paths are treated
// conservatively: they can be used but cannot qualify for per-packet striping
// because striping requires measured parity across all eligible paths.
type PathQuality struct {
	// RTT is the round-trip time estimate. Zero means unmeasured.
	RTT time.Duration

	// Jitter is the RTT variation estimate. Zero means unmeasured.
	Jitter time.Duration

	// LossFraction is the estimated packet loss fraction in [0.0, 1.0].
	// Zero means no observed loss or unmeasured.
	LossFraction float64

	// Confidence is the measurement confidence in [0.0, 1.0].
	// Zero means unmeasured; 1.0 means high confidence in the above values.
	// When Confidence is zero, the scheduler must not rely on RTT/Jitter/Loss
	// for fine-grained decisions like per-packet striping eligibility.
	Confidence float64
}

// Measured returns true when this quality snapshot has meaningful measurements
// (confidence > 0 and RTT > 0). An unmeasured path cannot qualify for
// per-packet striping eligibility checks.
func (q PathQuality) Measured() bool {
	return q.Confidence > 0 && q.RTT > 0
}

// PathCandidate represents one candidate data-plane path for an association.
// It holds identity, path class, quality inputs, and policy metadata.
//
// PathCandidate is the scheduler's input for deciding which paths to use.
// It is explicitly distinct from:
//   - RelayCandidate: the relay intermediate itself (not a resolved path)
//   - ForwardingEntry: installed direct-carriage forwarding state (internal/dataplane)
//   - RelayForwardingEntry: installed relay-carriage forwarding state (internal/dataplane)
//   - SchedulerDecision: the scheduler's output choosing from candidates
//
// This distinction is required by the object model (spec/v1-object-model.md
// section 16-17): PathCandidate and RelayCandidate must not be collapsed.
//
// Scheduling is association-bound: a PathCandidate without a valid
// AssociationID must not be used by the scheduler.
type PathCandidate struct {
	// ID uniquely identifies this candidate within its association.
	// Must be non-empty for a candidate to be eligible.
	ID string

	// AssociationID is the association this candidate belongs to.
	// The scheduler filters out candidates that do not match the target
	// association ID. This enforces association-bound scheduling.
	AssociationID string

	// Class is the path class (see PathClass constants).
	Class PathClass

	// Quality holds measurable inputs. All values may be zero (unmeasured).
	Quality PathQuality

	// AdminWeight is the policy-assigned preference weight in [1, 100].
	// Higher weight means more traffic preference. Zero is treated as 100
	// (full weight). AdminWeight lets operators prefer some paths over others
	// without relying on measured quality alone.
	AdminWeight uint8

	// IsMetered marks this path as metered (expensive or limited bandwidth).
	// Metered paths should not receive unnecessary traffic above their quality-
	// justified share, and probing on them should be lightweight.
	IsMetered bool

	// Health is the current health classification of this candidate.
	Health HealthState

	// PathGroup is an optional operator-assigned uplink or path group label.
	// It identifies which WAN uplink or network interface this candidate uses.
	// Examples: "wan0", "fiber", "lte-backup".
	//
	// PathGroup is an input for multi-WAN policy decisions: the stickiness
	// policy may prefer paths within the same group or may apply group-specific
	// weights. Group-based scheduling policy is future work; this field reserves
	// the semantic space without implementing full group-based routing in v1.
	//
	// Empty string means no group assigned (all candidates treated as ungrouped).
	PathGroup string
}

// effectiveAdminWeight returns the admin weight, using 100 when unset.
func (c PathCandidate) effectiveAdminWeight() uint8 {
	if c.AdminWeight == 0 {
		return 100
	}
	return c.AdminWeight
}

// RelayCandidate represents a relay-capable intermediate participant
// (coordinator or node relay) that may be used for relay-assisted carriage.
//
// RelayCandidate is explicitly distinct from PathCandidate:
//   - RelayCandidate = the relay participant itself (identity, capability, health)
//   - PathCandidate = a resolved path that may or may not use a relay
//
// This distinction preserves the object-model boundary defined in
// spec/v1-object-model.md section 17. A relay is not a path; a path that
// uses a relay is not the same object as the relay itself.
//
// In v1, only coordinator relay is implemented. Node relay candidates are
// defined here for completeness with the object model but are not yet
// operationally used.
type RelayCandidate struct {
	// ID uniquely identifies this relay candidate.
	ID string

	// Class is the relay class. Only relay classes are valid here:
	// PathClassCoordinatorRelay or PathClassNodeRelay.
	Class PathClass

	// NodeID is the coordinator or node ID of the relay participant.
	NodeID string

	// Health is the relay's current health classification.
	Health HealthState

	// AdminWeight is the relay's policy-assigned preference weight [1, 100].
	// Zero is treated as 100.
	AdminWeight uint8

	// IsMetered marks the relay link as metered.
	IsMetered bool
}
