package status

import (
	"time"

	"github.com/zhouchenh/transitloom/internal/service"
)

// BootstrapSummary captures the node's local identity and admission readiness.
//
// This is a LOCAL readiness summary only. A "ready" phase means local identity
// material and cached admission token appear coherent. It does NOT mean:
//   - the coordinator has validated this node's certificate
//   - the admission token has been verified with the coordinator
//   - the node is currently authorized for normal participation
//
// The distinction matters: identity and cached-token coherence must not be
// confused with current authorized participation. See spec/v1-pki-admission.md.
type BootstrapSummary struct {
	NodeName string

	// Phase is the computed bootstrap readiness phase. One of:
	//   identity-bootstrap-required  — no identity material found
	//   awaiting-certificate         — key present, no certificate yet
	//   admission-token-missing      — identity ready, no token cached
	//   admission-token-expired      — identity ready, cached token expired
	//   ready                        — identity and token appear locally coherent
	//
	// "ready" is a local material check, not a coordinator authorization check.
	Phase string

	// IdentityReady is true when the node certificate and key are present
	// and locally coherent. Does NOT imply coordinator certificate validation.
	IdentityReady bool

	// AdmissionTokenCached is true when a token file is present locally.
	// Does NOT imply the token is currently valid with the coordinator.
	// The local token cache is a readiness signal, not authoritative truth.
	AdmissionTokenCached bool

	// AdmissionTokenExpired is true when the cached token is expired by local clock.
	// Only meaningful when AdmissionTokenCached is true.
	AdmissionTokenExpired bool
}

// ServiceRegistrySummary captures a snapshot of the coordinator's service registry.
//
// These records are bootstrap-only placeholders unless later authenticated
// control sessions confirm them. "Registered" does not mean "authorized" or
// "discoverable to all nodes." A BootstrapOnly record is a coordinator-side
// placeholder created during bootstrap; it does not have full authenticated
// service ownership.
type ServiceRegistrySummary struct {
	TotalServices int
	Entries       []ServiceEntry
}

// ServiceEntry is one service in the service registry snapshot.
type ServiceEntry struct {
	Key         string
	NodeName    string
	ServiceName string
	ServiceType string

	// BootstrapOnly is true when this record has not been confirmed through
	// an authenticated session. It is a coordinator-side placeholder, not
	// final authenticated service state.
	BootstrapOnly bool
}

// AssociationStoreSummary captures a snapshot of the coordinator's association store.
//
// Association records at this stage are logical connectivity placeholders.
// A record existing here does NOT imply:
//   - path selection or path candidates have been computed
//   - relay eligibility has been evaluated
//   - forwarding state has been installed
//   - traffic can currently flow
//
// The "pending" state reflects that the coordinator accepted the intent but
// none of the above steps have been completed.
type AssociationStoreSummary struct {
	TotalAssociations int
	Entries           []AssociationEntry
}

// AssociationEntry is one association in the association store snapshot.
type AssociationEntry struct {
	AssociationID string
	SourceNode    string
	SourceService string
	DestNode      string
	DestService   string

	// State reflects the association lifecycle state. "pending" means the
	// coordinator accepted the intent but path selection, relay eligibility,
	// and forwarding-state installation have not been completed.
	State string

	// BootstrapOnly is true when the record is a bootstrap-only placeholder.
	BootstrapOnly bool
}

// ScheduledEgressSummary captures the applied scheduler/carrier activation state.
//
// This is the primary observability surface for confirming that runtime carrier
// behavior is aligned with scheduler decisions.
//
// "Applied" means the carrier was actually started successfully, not just that
// the scheduler computed a decision. A SchedulerMode of ModePerPacketStripe with
// a CarrierActivated of "direct" indicates the best single path is running (because
// multi-carrier striping is not yet implemented at the carrier level); the summary
// makes this gap explicit rather than hiding it.
//
// An operator reading this summary can verify:
//   - what mode the scheduler chose per association
//   - which carrier is actually running (direct/relay/none)
//   - whether any carrier failed to start
//   - live traffic counters for running carriers
type ScheduledEgressSummary struct {
	TotalActive     int
	TotalFailed     int
	TotalNoEligible int
	Entries         []ScheduledEgressEntry
	ProbeLoop       ProbeLoopSummary
	RecentEvents    []Event
}

type ProbeLoopSummary struct {
	State              string
	Reason             string
	ProbeInterval      time.Duration
	MaxTargetsPerRound int
	LastRoundAt        time.Time
	LastRound          ProbeLoopRoundSummary
}

type ProbeLoopRoundSummary struct {
	TargetsSelected int
	Reachable       int
	Unreachable     int
	Errors          int
}

// ScheduledEgressEntry describes one association's applied carrier state.
type ScheduledEgressEntry struct {
	AssociationID string
	SourceService string
	DestNode      string
	DestService   string

	// CarrierActivated is what actually started: "direct", "relay", or "none".
	// This is the applied runtime state, NOT just a scheduler computation.
	// "none" means either no eligible path was found or an activation error occurred.
	// Compare with SchedulerMode to verify alignment.
	CarrierActivated string

	// SchedulerMode is the mode the scheduler decided for this association.
	// Compare with CarrierActivated to verify alignment. If Mode is
	// "per-packet-stripe" but CarrierActivated is "direct", it means the
	// best single path was used because multi-carrier striping is not yet
	// implemented at the carrier level.
	SchedulerMode string

	// SchedulerReason is the scheduler's human-readable explanation.
	SchedulerReason string

	// ActivationError is non-empty if the carrier failed to start.
	// A non-empty ActivationError with a non-"none" SchedulerMode indicates
	// the scheduler chose a path but the carrier could not be started.
	ActivationError string

	// FallbackState is the direct-vs-relay fallback policy state for this
	// association at the time of the last activation. One of:
	//   "prefer-direct"         — direct is preferred (normal state)
	//   "fallen-back-to-relay"  — direct unusable; relay in active use
	//   "recovering-to-direct"  — relay dwell expired; confirming direct stability
	//   ""                      — fallback policy not configured
	//
	// This is distinct from CarrierActivated: FallbackState reflects the policy
	// decision layer; CarrierActivated reflects what carrier actually started.
	// Together they make the full fallback/recovery state visible to operators.
	FallbackState string

	// FallbackReason is the human-readable explanation of why the fallback
	// policy is in the current state. Non-empty when FallbackState is non-empty.
	// Shows elapsed dwell/recovery window time so operators can see exactly
	// where in the fallback cycle the system is.
	FallbackReason string

	// StickinessReason is the human-readable explanation of the stickiness
	// policy's last decision: whether it suppressed a switch (hold-down active,
	// below threshold), allowed a switch (threshold exceeded, current path gone),
	// or passed all candidates through (first selection, threshold disabled).
	// Empty when the stickiness policy is not configured.
	StickinessReason string

	// SwitchOccurred is true when the stickiness policy detected a path switch
	// during the last activation (the scheduler chose a different path than the
	// previously selected one). False on first selection, no-switch, or when
	// the stickiness policy is not configured.
	SwitchOccurred bool

	// HoldDownActive is true when the stickiness hold-down timer was active
	// during the last activation. Hold-down suppresses switching regardless of
	// quality improvement. False when hold-down is not running or the policy
	// is not configured.
	HoldDownActive bool

	// IngressPackets and IngressBytes are live counters for direct-path egress.
	// Non-zero only when CarrierActivated == "direct" and traffic has flowed.
	IngressPackets uint64
	IngressBytes   uint64

	// EgressPackets and EgressBytes are live counters for relay egress.
	// Non-zero only when CarrierActivated == "relay" and traffic has flowed.
	EgressPackets uint64
	EgressBytes   uint64

	// Candidates provides detailed diagnostics for all path candidates
	// considered by the scheduler for this association.
	//
	// This is the primary explainability surface for "why this path / why not
	// that path" questions. It includes excluded, degraded, and unmeasured
	// candidates with explicit reasons and freshness state.
	Candidates []PathCandidateStatus
}

// PathCandidateStatus provides operator-facing diagnostics for a single
// path candidate.
//
// It preserves architectural distinctions among candidate existence,
// endpoint freshness, measured quality, and scheduler eligibility.
type PathCandidateStatus struct {
	ID    string
	Class string

	// Usable is true when the candidate was eligible for scheduling.
	// False means it was excluded (e.g. endpoint failed, missing endpoint).
	Usable        bool
	ExcludeReason string

	// Health is the health state passed to the scheduler.
	Health         string
	DegradedReason string

	// EndpointState reflects address-level reachability (usable/stale/failed/unknown).
	EndpointState string

	// Quality describes the measured RTT/jitter/loss/confidence.
	RTT          time.Duration
	Jitter       time.Duration
	LossFraction float64
	Confidence   float64
}

type ControlReconciliationPhase string

const (
	ControlReconciliationPhaseDisconnected         ControlReconciliationPhase = "disconnected"
	ControlReconciliationPhaseTransportReconnected ControlReconciliationPhase = "transport-reconnected"
	ControlReconciliationPhaseSessionEstablished   ControlReconciliationPhase = "session-established"
	ControlReconciliationPhaseReconciling          ControlReconciliationPhase = "reconciling"
	ControlReconciliationPhaseReconciled           ControlReconciliationPhase = "reconciled"
	ControlReconciliationPhaseReconciliationFailed ControlReconciliationPhase = "reconciliation-failed"
)

type ControlReconciliationStep string

const (
	ControlReconciliationStepPending   ControlReconciliationStep = "pending"
	ControlReconciliationStepSkipped   ControlReconciliationStep = "skipped"
	ControlReconciliationStepSucceeded ControlReconciliationStep = "succeeded"
	ControlReconciliationStepFailed    ControlReconciliationStep = "failed"
)

type ControlReconciliationSummary struct {
	Phase                    ControlReconciliationPhase
	TransportMode            string
	CurrentCoordinator       string
	TransportConnected       bool
	SessionEstablished       bool
	SessionAuthenticated     bool
	LogicalStateReconciled   bool
	ServiceRefresh           ControlReconciliationStep
	AssociationRefresh       ControlReconciliationStep
	PathCandidateRefresh     ControlReconciliationStep
	LastFailure              string
	LastTransitionAt         time.Time
	LastTransportReconnectAt time.Time
	LastSessionEstablishedAt time.Time
	LastReconciledAt         time.Time
}

// MakeBootstrapSummary constructs a BootstrapSummary from node bootstrap state.
//
// The caller passes simple typed fields, keeping this package free of import
// cycles with internal/node. The phase string must be one of the BootstrapPhase
// constants defined in internal/node (e.g., "ready", "awaiting-certificate").
func MakeBootstrapSummary(nodeName, phase string, identityReady, tokenCached, tokenExpired bool) BootstrapSummary {
	return BootstrapSummary{
		NodeName:              nodeName,
		Phase:                 phase,
		IdentityReady:         identityReady,
		AdmissionTokenCached:  tokenCached,
		AdmissionTokenExpired: tokenExpired,
	}
}

// MakeServiceRegistrySummary constructs a ServiceRegistrySummary from a
// coordinator service.Record snapshot. The caller passes records from
// coordinator.ServiceRegistry.Snapshot().
func MakeServiceRegistrySummary(records []service.Record) ServiceRegistrySummary {
	entries := make([]ServiceEntry, 0, len(records))
	for _, r := range records {
		entries = append(entries, ServiceEntry{
			Key:           r.Key(),
			NodeName:      r.NodeName,
			ServiceName:   r.Identity.Name,
			ServiceType:   string(r.Identity.Type),
			BootstrapOnly: r.BootstrapOnly,
		})
	}
	return ServiceRegistrySummary{
		TotalServices: len(entries),
		Entries:       entries,
	}
}

// MakeAssociationStoreSummary constructs an AssociationStoreSummary from a
// coordinator service.AssociationRecord snapshot. The caller passes records
// from coordinator.AssociationStore.Snapshot().
func MakeAssociationStoreSummary(records []service.AssociationRecord) AssociationStoreSummary {
	entries := make([]AssociationEntry, 0, len(records))
	for _, r := range records {
		entries = append(entries, AssociationEntry{
			AssociationID: r.AssociationID,
			SourceNode:    r.SourceNode,
			SourceService: r.SourceService.Name,
			DestNode:      r.DestinationNode,
			DestService:   r.DestinationService.Name,
			State:         string(r.State),
			BootstrapOnly: r.BootstrapOnly,
		})
	}
	return AssociationStoreSummary{
		TotalAssociations: len(entries),
		Entries:           entries,
	}
}
