package scheduler

// Mode identifies the scheduling mode chosen for an association.
type Mode string

const (
	// ModeWeightedBurstFlowlet is the v1 default scheduling mode.
	//
	// The scheduler assigns traffic to the best-scoring path for a burst or
	// flowlet. A flowlet is a short burst of packets; keeping a flowlet on
	// one path avoids unnecessary reordering amplification, which degrades
	// performance for UDP sessions including WireGuard.
	//
	// This mode is chosen when:
	//   - multiple eligible paths exist, AND
	//   - paths are NOT closely enough matched to allow per-packet striping
	//
	// Why this is the v1 default: per-packet striping on mismatched paths causes
	// reordering that hurts application-layer performance. Burst/flowlet-aware
	// scheduling gives aggregation benefit while avoiding the worst reordering.
	// See spec/v1-data-plane.md section 14.
	ModeWeightedBurstFlowlet Mode = "weighted-burst-flowlet"

	// ModePerPacketStripe enables per-packet distribution across multiple paths.
	//
	// Per-packet striping is only activated when all eligible paths are closely
	// matched within configured quality thresholds (RTT, jitter, loss spread).
	// On mismatched paths, per-packet striping amplifies reordering and hurts
	// application performance.
	//
	// Endpoint-owned: only the source endpoint's scheduler can enable this mode.
	// Relays must not independently reshape traffic distribution decisions.
	// See spec/v1-data-plane.md section 15.
	ModePerPacketStripe Mode = "per-packet-stripe"

	// ModeSinglePath is used when exactly one eligible path exists.
	// There is no traffic distribution decision to make.
	ModeSinglePath Mode = "single-path"

	// ModeNoEligiblePath means no eligible path was found for this association.
	// The association cannot carry traffic until a path becomes available.
	ModeNoEligiblePath Mode = "no-eligible-path"
)

// ChosenPath is a selected path candidate with its effective weight for
// traffic distribution.
type ChosenPath struct {
	// CandidateID is the ID of the selected PathCandidate.
	CandidateID string

	// Class is the path class of the selected candidate.
	Class PathClass

	// Weight is the effective scheduling weight [1, 100] for this path.
	// Used for traffic distribution when multiple paths are chosen.
	Weight uint8
}

// SchedulerDecision is the output of one endpoint-owned scheduling decision
// for a single association.
//
// Scheduling is association-bound: each decision is scoped to one association
// and must not be applied to another. The scheduler must not mix path candidates
// across associations. AssociationID must always be set.
//
// Scheduling is endpoint-owned: this decision is made entirely by the source
// endpoint's Scheduler. Relay nodes follow installed forwarding context and
// must not override or reinterpret this decision independently.
//
// The Reason field is required to make scheduling observable: operators and
// future agents must be able to understand why a particular mode and path set
// was chosen without reverse-engineering the scoring logic.
type SchedulerDecision struct {
	// AssociationID is the association this decision applies to.
	// Must not be empty. Association-bound scheduling is non-negotiable.
	AssociationID string

	// Mode is the scheduling mode selected for this association.
	Mode Mode

	// ChosenPaths is the set of selected path candidates and their weights.
	// Empty only when Mode is ModeNoEligiblePath.
	ChosenPaths []ChosenPath

	// StripingAllowed is true when per-packet striping was activated
	// (Mode == ModePerPacketStripe). When false, traffic should stay on
	// a single path for a burst/flowlet duration.
	StripingAllowed bool

	// Reason explains in plain text why this mode and path set was chosen.
	// Always non-empty. Required for observability: scheduling decisions must
	// not be invisible black boxes. A future agent or operator should be able
	// to read Reason and understand the decision without re-reading the scorer.
	Reason string
}
