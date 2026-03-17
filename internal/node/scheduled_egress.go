package node

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/scheduler"
	"github.com/zhouchenh/transitloom/internal/service"
	"github.com/zhouchenh/transitloom/internal/status"
	"github.com/zhouchenh/transitloom/internal/transport"
)

// ScheduledEgressRuntime holds the scheduler and path runtimes for
// scheduler-guided egress activation.
//
// Scheduler integration: Scheduler.Decide() is called at the egress activation
// decision point to determine which path (direct or relay-assisted) to use for
// each association. The scheduler remains endpoint-owned — only this source
// endpoint's Scheduler runs Decide(); relay nodes do not schedule.
//
// Direct and relay-assisted runtimes remain architecturally distinct: the
// scheduler chooses between them but does not blend them into one carrier.
//
// QualityStore: when non-nil, the quality store enriches PathCandidates with
// fresh measured quality before Scheduler.Decide() is called. This is the live
// measurement input layer. The store is optional — without it the scheduler
// operates on unmeasured (confidence=0) candidates, which is the prior behavior.
//
// CandidateStore: when non-nil, coordinator-distributed candidates are loaded
// for each association and run through the refinement layer (RefineCandidates)
// before scheduling. Refined distributed candidates are merged with any
// config-derived candidates already on the ScheduledActivationInput.
// The CandidateStore is optional — without it the runtime falls back to
// config-only candidates (prior behavior).
//
// EndpointRegistry: when non-nil, used by the refinement layer to check
// endpoint freshness (stale/failed state) for distributed candidates. A
// nil EndpointRegistry means endpoint freshness is unknown for all candidates
// (no probe data available). Distinct from QualityStore: the registry tracks
// address-level reachability; the quality store tracks RTT/jitter/loss.
//
// The mu/lastActivations fields enable Snapshot() to return the applied carrier
// state after activation. lastActivations records what the scheduler decided
// and which carrier was actually started for each association.
type ScheduledEgressRuntime struct {
	Scheduler    *scheduler.Scheduler
	Direct       *DirectPathRuntime
	Relay        *RelayPathRuntime
	QualityStore *scheduler.PathQualityStore // optional; nil means no live measurement

	// CandidateStore holds coordinator-distributed path candidates.
	// When non-nil, distributed candidates are refined and merged into scheduler
	// inputs for each association. Optional: nil disables distributed candidate use.
	CandidateStore *CandidateStore

	// EndpointRegistry is the endpoint freshness store used by the refinement
	// layer to check whether distributed candidate remote endpoints are usable,
	// stale, or failed. Optional: nil means endpoint freshness is unknown.
	// Distinct from QualityStore — they track different concerns.
	EndpointRegistry *transport.EndpointRegistry

	// FallbackStore holds the per-association direct-vs-relay fallback policy
	// state machines. When non-nil, the fallback policy gates recovery back to
	// direct after a relay fallback, preventing rapid oscillation. The store is
	// evaluated after candidate refinement and before Scheduler.Decide(), so the
	// fallback policy can filter direct candidates when anti-flap is active.
	//
	// The fallback policy is separate from candidate generation and measurement:
	//   - CandidateStore/RefineCandidates produce the usability signals it consumes
	//   - PathQualityStore influences usability via candidate refinement, not directly
	//   - Scheduler.Decide() receives the filtered candidate set after policy applies
	//
	// When nil, no fallback policy is applied (direct candidates always pass to
	// the scheduler; the scheduler prefers direct via its relay penalty, which is
	// the prior behavior). NewScheduledEgressRuntime creates a FallbackStore with
	// DefaultFallbackConfig automatically.
	FallbackStore *AssociationFallbackStore

	// EventHistory tracks recent bounded path change events.
	// Optional: if nil, events are not recorded.
	EventHistory *status.EventHistory

	// mu protects lastActivations from concurrent reads and writes.
	mu sync.RWMutex
	// lastActivations holds the results from the most recent ActivateScheduledEgress
	// call. It is the primary source for Snapshot() to report applied carrier state.
	lastActivations []ScheduledEgressActivation
}

// NewScheduledEgressRuntime creates a ScheduledEgressRuntime with a new
// scheduler (using default stripe thresholds), a new direct-path runtime,
// a new relay-path runtime, and a new PathQualityStore (using the default
// freshness window).
//
// The QualityStore starts empty — no measurements recorded. Callers populate it
// via QualityStore.RecordProbeResult or QualityStore.Update as probe or observation
// results become available. Until measurements are present, the scheduler operates
// on unmeasured (confidence=0) candidates, which is conservative and correct.
func NewScheduledEgressRuntime() *ScheduledEgressRuntime {
	return &ScheduledEgressRuntime{
		Scheduler:     scheduler.NewScheduler(scheduler.DefaultStripeMatchThresholds()),
		Direct:        NewDirectPathRuntime(),
		Relay:         NewRelayPathRuntime(),
		QualityStore:  scheduler.NewPathQualityStore(scheduler.DefaultQualityMaxAge),
		FallbackStore: NewAssociationFallbackStore(DefaultFallbackConfig()),
		EventHistory:  status.NewEventHistory(100), // Bounded to last 100 path events
	}
}

// ScheduledActivationInput holds the context needed to activate scheduler-guided
// egress for one association. Both DirectEndpoint and RelayEndpoint are optional
// — the scheduler chooses from whichever are present and eligible.
//
// This type replaces the separate AssociationActivationInput (direct-only) and
// RelayEgressActivationInput (relay-only) for the scheduler-integrated activation
// path. Both path types may be present simultaneously, giving the scheduler a
// meaningful choice between direct and relay-assisted carriage.
type ScheduledActivationInput struct {
	AssociationID string
	SourceNode    string
	SourceService service.Identity
	DestNode      string
	DestService   service.Identity

	// DirectEndpoint is the peer node's direct mesh-facing UDP address.
	// When non-empty, a direct PathCandidate is offered to the scheduler.
	DirectEndpoint string

	// MeshListenPort is the local port for inbound direct-path delivery.
	// Only used when DirectEndpoint is chosen and delivery is active.
	MeshListenPort uint16

	// RelayEndpoint is the coordinator relay's per-association listen address.
	// When non-empty, a relay PathCandidate is offered to the scheduler.
	RelayEndpoint string

	// PathCandidates are the scheduler inputs for this association.
	// Derived from DirectEndpoint and RelayEndpoint by BuildScheduledActivationInputs.
	// Callers may provide them directly (useful in tests).
	PathCandidates []scheduler.PathCandidate
}

// ScheduledEgressActivation describes one association's scheduler decision
// and the resulting carrier activation.
//
// The scheduler decision is always recorded (Decision.Mode and Decision.Reason),
// even when activation fails or no eligible path exists. This makes the
// integration between scheduler output and runtime behavior directly observable:
// an operator can compare Decision.Mode/Reason with CarrierActivated to confirm
// that runtime behavior is aligned with the scheduler decision.
type ScheduledEgressActivation struct {
	AssociationID string
	SourceService string
	DestNode      string
	DestService   string

	// Decision is the scheduler's output for this association. Always set.
	// Decision.Reason explains why this mode and path were chosen.
	Decision scheduler.SchedulerDecision

	// CarrierActivated is the carrier started as a result of the scheduler decision.
	//   "direct" — DirectCarrier started for this association
	//   "relay"  — RelayEgressCarrier started for this association
	//   "none"   — no carrier started (ModeNoEligiblePath or activation error)
	//
	// Runtime behavior must match this field: an operator can verify that the
	// carrier reported here matches what the scheduler's decision said to use.
	CarrierActivated string

	// ActivationError records any carrier startup error. Empty on success.
	ActivationError string

	// Candidates records the diagnostics for all candidates considered
	// for this association's decision.
	Candidates []status.PathCandidateStatus

	// FallbackState is the fallback policy state at the time of this activation.
	// One of the FallbackState constants: "prefer-direct", "fallen-back-to-relay",
	// "recovering-to-direct", or "" when FallbackStore is nil.
	FallbackState string

	// FallbackReason is the human-readable explanation of the fallback policy
	// state. Non-empty when FallbackState is non-empty.
	FallbackReason string
}

// ScheduledEgressResult summarizes all scheduler-guided egress activation outcomes.
type ScheduledEgressResult struct {
	Activations     []ScheduledEgressActivation
	TotalActive     int
	TotalFailed     int
	TotalNoEligible int
}

// ReportLines produces human-readable log lines for the scheduled egress result.
func (r ScheduledEgressResult) ReportLines() []string {
	lines := make([]string, 0, len(r.Activations)+4)

	lines = append(lines, fmt.Sprintf(
		"scheduled-egress activation: active=%d failed=%d no-eligible=%d (scheduler-guided carrier selection; per-packet striping deferred to carrier level)",
		r.TotalActive, r.TotalFailed, r.TotalNoEligible,
	))

	for _, a := range r.Activations {
		if a.Decision.Mode == scheduler.ModeNoEligiblePath {
			lines = append(lines, fmt.Sprintf(
				"  association %s: no-eligible-path; reason: %s",
				a.AssociationID, a.Decision.Reason,
			))
			continue
		}
		if a.ActivationError != "" {
			lines = append(lines, fmt.Sprintf(
				"  association %s: FAILED (scheduler=%s carrier=%s): %s",
				a.AssociationID, a.Decision.Mode, a.CarrierActivated, a.ActivationError,
			))
			continue
		}
		lines = append(lines, fmt.Sprintf(
			"  association %s: carrier=%s scheduler=%s",
			a.AssociationID, a.CarrierActivated, a.Decision.Mode,
		))
		lines = append(lines, fmt.Sprintf(
			"    decision: %s", a.Decision.Reason,
		))
	}

	return lines
}

// BuildScheduledActivationInputs constructs scheduled activation inputs from
// node config and coordinator association results.
//
// Both DirectEndpoint and RelayEndpoint are read when present, allowing the
// scheduler to choose between them for the same association. Associations with
// neither endpoint configured are skipped (control-plane records only).
//
// PathCandidates are derived automatically from the endpoints, using
// HealthStateActive with unmeasured quality (confidence=0). The scheduler
// produces valid single-path or weighted-burst decisions from these static
// candidates. Quality-based refinement will improve decisions when live path
// measurement is added.
func BuildScheduledActivationInputs(
	cfg config.NodeConfig,
	assocResults []AssociationResultEntry,
) []ScheduledActivationInput {
	type configKey struct {
		sourceService string
		destNode      string
		destService   string
	}
	configMap := make(map[configKey]config.AssociationConfig, len(cfg.Associations))
	for _, ac := range cfg.Associations {
		k := configKey{
			sourceService: strings.TrimSpace(ac.SourceService),
			destNode:      strings.TrimSpace(ac.DestinationNode),
			destService:   strings.TrimSpace(ac.DestinationService),
		}
		configMap[k] = ac
	}

	var inputs []ScheduledActivationInput
	for _, ar := range assocResults {
		if ar.AssociationID == "" || !ar.Accepted {
			continue
		}
		k := configKey{
			sourceService: strings.TrimSpace(ar.SourceServiceName),
			destNode:      strings.TrimSpace(ar.DestinationNode),
			destService:   strings.TrimSpace(ar.DestinationService),
		}
		ac, exists := configMap[k]
		if !exists {
			continue
		}

		direct := strings.TrimSpace(ac.DirectEndpoint)
		relay := strings.TrimSpace(ac.RelayEndpoint)
		if direct == "" && relay == "" {
			continue // no data-plane path configured; control-plane record only
		}

		// Resolve source service type from local config.
		var sourceType config.ServiceType
		for _, svc := range cfg.Services {
			if svc.Name == ar.SourceServiceName {
				sourceType = svc.Type
				break
			}
		}
		if sourceType == "" {
			sourceType = config.ServiceTypeRawUDP
		}

		input := ScheduledActivationInput{
			AssociationID:  ar.AssociationID,
			SourceNode:     cfg.Identity.Name,
			SourceService:  service.Identity{Name: ar.SourceServiceName, Type: sourceType},
			DestNode:       ar.DestinationNode,
			DestService:    service.Identity{Name: ar.DestinationService, Type: config.ServiceTypeRawUDP},
			DirectEndpoint: direct,
			MeshListenPort: ac.MeshListenPort,
			RelayEndpoint:  relay,
		}

		// Derive PathCandidates from the configured endpoints.
		// These are the scheduler's inputs for path selection.
		input.PathCandidates = buildPathCandidatesFromEndpoints(ar.AssociationID, direct, relay)

		inputs = append(inputs, input)
	}
	return inputs
}

// buildPathCandidatesFromEndpoints constructs PathCandidates from endpoint strings.
//
// Direct endpoint  → PathClassDirectPublic    with HealthStateActive
// Relay endpoint   → PathClassCoordinatorRelay with HealthStateActive
//
// Quality is intentionally zero-value (unmeasured, confidence=0): live path
// measurement is not yet implemented. The scheduler still produces useful
// decisions because:
//   - direct paths are preferred over relay via the relay penalty in scoring
//   - the striping gate conservatively blocks per-packet striping for
//     unmeasured paths (confidence < MinConfidence threshold)
//
// This is the static-config bridge from endpoint strings to scheduler inputs.
// When live path measurement is added, callers will populate Quality with
// real RTT/jitter/loss data from probes or passive observation.
func buildPathCandidatesFromEndpoints(associationID, directEndpoint, relayEndpoint string) []scheduler.PathCandidate {
	var candidates []scheduler.PathCandidate

	if directEndpoint != "" {
		// Direct public path candidate. The scheduler prefers direct paths over
		// relay because relay paths incur a scoring penalty (see relayPenalty in
		// scheduler.go). Direct is the correct preference per spec/v1-data-plane.md
		// section 8.1: "prefer direct paths when they are legal, healthy enough,
		// and competitively useful."
		candidates = append(candidates, scheduler.PathCandidate{
			ID:            associationID + ":direct",
			AssociationID: associationID,
			Class:         scheduler.PathClassDirectPublic,
			Health:        scheduler.HealthStateActive,
			AdminWeight:   100,
			// Quality zero-value: unmeasured. Scheduler uses AdminWeight and
			// class-based scoring only (no quality penalties applied for zero Quality).
		})
	}

	if relayEndpoint != "" {
		// Coordinator relay path candidate. The relay penalty (-10) in the scorer
		// ensures direct paths are preferred when both are available and quality
		// is comparable, matching spec/v1-data-plane.md section 8.1.
		candidates = append(candidates, scheduler.PathCandidate{
			ID:            associationID + ":relay",
			AssociationID: associationID,
			Class:         scheduler.PathClassCoordinatorRelay,
			Health:        scheduler.HealthStateActive,
			AdminWeight:   100,
			// Quality zero-value: unmeasured.
		})
	}

	return candidates
}

// ActivateScheduledEgress activates egress carriage for each association guided
// by the scheduler's decision.
//
// For each input, this function:
//  1. Calls Scheduler.Decide() with the available PathCandidates — this is the
//     egress decision point where the endpoint-owned scheduler runs.
//  2. If ModeNoEligiblePath: skips carrier activation; reason recorded in Decision.
//  3. Otherwise: activates the best-scoring carrier (direct or relay-assisted)
//     based on the scheduler's chosen path class.
//  4. Records the decision and activation result for observability.
//
// Per-packet striping (ModePerPacketStripe): when this mode is decided, the
// best path is activated. Full per-packet striping across multiple carriers
// requires a multi-path ingress handle not yet implemented at the carrier level.
// The scheduler decision is still recorded so the gap is observable.
//
// Endpoint-owned scheduling: Scheduler.Decide() runs here only, at the source
// endpoint. Relay nodes follow installed forwarding context and must not
// independently reschedule end-to-end traffic.
func ActivateScheduledEgress(
	ctx context.Context,
	cfg config.NodeConfig,
	runtime *ScheduledEgressRuntime,
	inputs []ScheduledActivationInput,
) ScheduledEgressResult {
	var result ScheduledEgressResult

	for _, input := range inputs {
		activation := activateSingleScheduledEgress(ctx, cfg, runtime, input)
		result.Activations = append(result.Activations, activation)
		switch {
		case activation.Decision.Mode == scheduler.ModeNoEligiblePath:
			result.TotalNoEligible++
		case activation.ActivationError != "":
			result.TotalFailed++
		default:
			result.TotalActive++
		}
	}

	// Store activation results so Snapshot() can report applied carrier state.
	// This makes it possible to inspect what the scheduler decided and which
	// carrier was actually started, even after the initial startup log is gone.
	runtime.mu.Lock()
	oldActivations := runtime.lastActivations
	runtime.lastActivations = append([]ScheduledEgressActivation(nil), result.Activations...)

	if runtime.EventHistory != nil {
		recordActivationEvents(runtime.EventHistory, oldActivations, result.Activations)
	}
	runtime.mu.Unlock()

	return result
}

func recordActivationEvents(history *status.EventHistory, oldActs, newActs []ScheduledEgressActivation) {
	oldByAssoc := make(map[string]ScheduledEgressActivation)
	for _, a := range oldActs {
		oldByAssoc[a.AssociationID] = a
	}

	for _, newA := range newActs {
		oldA, exists := oldByAssoc[newA.AssociationID]
		if !exists {
			continue // skip new associations
		}

		// 1. Carrier change events (fallback / recovery / path change)
		if oldA.CarrierActivated != newA.CarrierActivated {
			if oldA.CarrierActivated == "direct" && newA.CarrierActivated == "relay" {
				history.Record(status.Event{
					Type:          status.EventFallbackToRelay,
					AssociationID: newA.AssociationID,
					Message:       fmt.Sprintf("fallback from direct to relay: %s", newA.Decision.Reason),
				})
			} else if oldA.CarrierActivated == "relay" && newA.CarrierActivated == "direct" {
				history.Record(status.Event{
					Type:          status.EventRecoveryToDirect,
					AssociationID: newA.AssociationID,
					Message:       fmt.Sprintf("recovery from relay to direct: %s", newA.Decision.Reason),
				})
			} else {
				history.Record(status.Event{
					Type:          status.EventChosenPathChanged,
					AssociationID: newA.AssociationID,
					Message:       fmt.Sprintf("carrier changed %s -> %s: %s", oldA.CarrierActivated, newA.CarrierActivated, newA.Decision.Reason),
				})
			}
		} else {
			// Check if primary path changed within the same carrier
			oldPrimary := ""
			if len(oldA.Decision.ChosenPaths) > 0 {
				oldPrimary = oldA.Decision.ChosenPaths[0].CandidateID
			}
			newPrimary := ""
			if len(newA.Decision.ChosenPaths) > 0 {
				newPrimary = newA.Decision.ChosenPaths[0].CandidateID
			}
			if oldPrimary != "" && newPrimary != "" && oldPrimary != newPrimary {
				history.Record(status.Event{
					Type:          status.EventChosenPathChanged,
					AssociationID: newA.AssociationID,
					CandidateID:   newPrimary,
					Message:       fmt.Sprintf("primary path changed %s -> %s: %s", oldPrimary, newPrimary, newA.Decision.Reason),
				})
			}
		}

		// 2. Candidate state change events (excluded/restored, endpoint state)
		oldCands := make(map[string]status.PathCandidateStatus)
		for _, c := range oldA.Candidates {
			oldCands[c.ID] = c
		}

		for _, newC := range newA.Candidates {
			oldC, cExists := oldCands[newC.ID]
			if !cExists {
				continue
			}

			// Excluded / Restored
			if oldC.Usable && !newC.Usable {
				history.Record(status.Event{
					Type:          status.EventCandidateExcluded,
					AssociationID: newA.AssociationID,
					CandidateID:   newC.ID,
					Message:       fmt.Sprintf("candidate excluded: %s", newC.ExcludeReason),
				})
			} else if !oldC.Usable && newC.Usable {
				history.Record(status.Event{
					Type:          status.EventCandidateRestored,
					AssociationID: newA.AssociationID,
					CandidateID:   newC.ID,
					Message:       "candidate restored to usable state",
				})
			}

			// Endpoint state
			if oldC.EndpointState != newC.EndpointState {
				switch newC.EndpointState {
				case "stale":
					history.Record(status.Event{
						Type:          status.EventEndpointStale,
						AssociationID: newA.AssociationID,
						CandidateID:   newC.ID,
						Message:       fmt.Sprintf("endpoint marked stale (was %s)", oldC.EndpointState),
					})
				case "failed":
					history.Record(status.Event{
						Type:          status.EventEndpointFailed,
						AssociationID: newA.AssociationID,
						CandidateID:   newC.ID,
						Message:       fmt.Sprintf("endpoint marked failed (was %s)", oldC.EndpointState),
					})
				case "usable":
					if oldC.EndpointState == "stale" || oldC.EndpointState == "failed" {
						history.Record(status.Event{
							Type:          status.EventEndpointVerified,
							AssociationID: newA.AssociationID,
							CandidateID:   newC.ID,
							Message:       fmt.Sprintf("endpoint verified usable (was %s)", oldC.EndpointState),
						})
					}
				}
			}
		}
	}
}

// Snapshot returns a current point-in-time snapshot of the scheduled egress
// runtime state, combining the last activation results with live carrier counters.
//
// This is the primary way to inspect actual applied carrier behavior after
// startup. Each entry shows:
//   - CarrierActivated: what is actually running ("direct", "relay", or "none")
//   - SchedulerMode: what the scheduler decided (may differ from carrier state)
//   - SchedulerReason: plain-text explanation of the scheduler's decision
//   - ActivationError: non-empty if the carrier failed to start
//   - Live traffic counters for the active carrier
//
// The distinction between SchedulerMode and CarrierActivated is intentional:
// a "per-packet-stripe" decision with a "direct" carrier means the best single
// path is running because multi-carrier striping is not yet implemented at the
// carrier level. The snapshot makes this gap observable without hiding it.
//
// Snapshot is safe for concurrent use.
func (r *ScheduledEgressRuntime) Snapshot() status.ScheduledEgressSummary {
	r.mu.RLock()
	activations := append([]ScheduledEgressActivation(nil), r.lastActivations...)
	r.mu.RUnlock()

	entries := make([]status.ScheduledEgressEntry, 0, len(activations))
	var totalActive, totalFailed, totalNoEligible int

	for _, a := range activations {
		entry := status.ScheduledEgressEntry{
			AssociationID:    a.AssociationID,
			SourceService:    a.SourceService,
			DestNode:         a.DestNode,
			DestService:      a.DestService,
			CarrierActivated: a.CarrierActivated,
			SchedulerMode:    string(a.Decision.Mode),
			SchedulerReason:  a.Decision.Reason,
			ActivationError:  a.ActivationError,
			Candidates:       a.Candidates,
			FallbackState:    a.FallbackState,
			FallbackReason:   a.FallbackReason,
		}

		// Read live traffic counters from the carrier that is actually running.
		// Direct and relay counters are kept separate — mixing them would erase
		// the architectural distinction between direct and relay-assisted carriage.
		switch a.CarrierActivated {
		case "direct":
			entry.IngressPackets, entry.IngressBytes, _ = r.Direct.Carrier.IngressStats(a.AssociationID)
		case "relay":
			entry.EgressPackets, entry.EgressBytes, _ = r.Relay.Carrier.EgressStats(a.AssociationID)
		}

		// Tally outcome counters using the same logic as ActivateScheduledEgress.
		switch {
		case a.Decision.Mode == scheduler.ModeNoEligiblePath:
			totalNoEligible++
		case a.ActivationError != "":
			totalFailed++
		default:
			totalActive++
		}

		entries = append(entries, entry)
	}

	summary := status.ScheduledEgressSummary{
		TotalActive:     totalActive,
		TotalFailed:     totalFailed,
		TotalNoEligible: totalNoEligible,
		Entries:         entries,
	}

	if r.EventHistory != nil {
		summary.RecentEvents = r.EventHistory.Snapshot()
	}

	return summary
}

// QualitySnapshot returns a point-in-time view of all measured path quality
// stored in the QualityStore. This surface makes the live measurement layer
// directly inspectable: operators can see which paths have been measured, what
// the current RTT/loss/confidence estimates are, and which entries are stale.
//
// Returns nil when QualityStore is nil (live measurement not configured).
//
// This is intentionally a separate method from Snapshot(): carrier state and
// measurement state are different concerns and must not be conflated.
// Snapshot() shows applied runtime behavior; QualitySnapshot() shows measurement inputs.
func (r *ScheduledEgressRuntime) QualitySnapshot() status.PathQualitySummary {
	if r.QualityStore == nil {
		return status.PathQualitySummary{}
	}
	return status.MakePathQualitySummary(r.QualityStore.Snapshot())
}

// activateSingleScheduledEgress handles one association's scheduler-guided
// activation. The scheduler runs here, at the source endpoint, before any
// carrier is started.
func activateSingleScheduledEgress(
	ctx context.Context,
	cfg config.NodeConfig,
	runtime *ScheduledEgressRuntime,
	input ScheduledActivationInput,
) ScheduledEgressActivation {
	activation := ScheduledEgressActivation{
		AssociationID:    input.AssociationID,
		SourceService:    input.SourceService.Name,
		DestNode:         input.DestNode,
		DestService:      input.DestService.Name,
		CarrierActivated: "none",
	}

	// Build the scheduler candidate list for this association.
	//
	// Two candidate sources may be present:
	//   1. Config-derived candidates (from DirectEndpoint / RelayEndpoint strings).
	//      These are already on input.PathCandidates. Quality is applied via
	//      QualityStore.ApplyCandidates below when no distributed candidates replace them.
	//   2. Coordinator-distributed candidates (from CandidateStore).
	//      These are run through RefineCandidates, which checks endpoint freshness
	//      (via EndpointRegistry) and applies quality (via QualityStore) in one step.
	//
	// When distributed candidates exist for this association, they are merged
	// with the config-derived candidates. The scheduler sees all candidates and
	// picks the best one according to its scoring policy. When no distributed
	// candidates are available, the config-derived candidates are used as before.
	//
	// Endpoint freshness (EndpointRegistry) and measured quality (QualityStore)
	// are distinct inputs kept separate through the refinement step:
	//   - Endpoint state affects candidate usability and health classification.
	//   - Quality enriches RTT/jitter/loss/confidence for fine-grained scoring.
	// Neither source replaces the other; both are visible in RefinedCandidate fields.
	candidates := input.PathCandidates
	var refined []RefinedCandidate

	if runtime.CandidateStore != nil {
		distributed := runtime.CandidateStore.Lookup(input.AssociationID)
		if len(distributed) > 0 {
			// Run the refinement layer: endpoint freshness check + quality enrichment.
			// RefineCandidates returns one RefinedCandidate per input, with explicit
			// Usable/ExcludeReason/DegradedReason/EndpointState/QualityFresh fields
			// so that refinement decisions are inspectable rather than opaque.
			//
			// Quality enrichment is done inside RefineCandidates (not via a separate
			// ApplyCandidates call below), so we must NOT call ApplyCandidates again
			// on the distributed candidates — that would incorrectly zero out the
			// already-enriched quality for candidates without a quality-store entry.
			refined = RefineCandidates(distributed, runtime.EndpointRegistry, runtime.QualityStore)
			distributedCandidates := UsableSchedulerCandidates(refined)

			// Merge distributed candidates into the candidate list.
			// Config-derived candidates remain as a base; distributed candidates
			// supplement them. The scheduler sees the full picture and picks the best.
			// Duplicates by ID cannot occur: config IDs use "assocID:direct/relay"
			// while distributed IDs use the coordinator-assigned CandidateID.
			candidates = append(candidates, distributedCandidates...)
		}
	}

	// Apply quality enrichment to config-derived candidates (input.PathCandidates).
	// Distributed candidates have already had quality applied by RefineCandidates above.
	// We only apply ApplyCandidates to the config-derived portion to avoid double-applying.
	//
	// When QualityStore is nil, no enrichment occurs and quality stays zero
	// (unmeasured, conservative behavior) for all candidates.
	if runtime.QualityStore != nil && len(input.PathCandidates) > 0 {
		// Enrich only the config-derived candidates (first len(input.PathCandidates) entries).
		enriched := runtime.QualityStore.ApplyCandidates(input.PathCandidates)
		// Replace the config-derived prefix in the merged slice.
		copy(candidates[:len(input.PathCandidates)], enriched)
	}

	// Fallback policy evaluation: determine whether to filter direct candidates.
	//
	// The fallback policy runs after candidate refinement (so usability signals
	// are accurate) and before Scheduler.Decide() (so the scheduler only sees
	// candidates allowed by the policy). This preserves the layering:
	//
	//   candidate generation → refinement → fallback policy → scheduler → carrier
	//
	// The policy is separate from all three neighbor layers:
	//   - It does not generate or alter candidates (that is refinement's job).
	//   - It does not score or select paths (that is the scheduler's job).
	//   - It does not start or stop carriers (that is the activation layer's job).
	//
	// When FallbackStore is nil (not configured), all candidates pass through to
	// the scheduler unchanged. The scheduler still prefers direct via its relay
	// penalty, which is the correct baseline behavior.
	directUsable := hasUsableDirectCandidate(candidates)
	relayUsable := hasUsableRelayCandidate(candidates)

	var fallbackEval FallbackEval
	if runtime.FallbackStore != nil {
		fallbackEval = runtime.FallbackStore.Evaluate(input.AssociationID, directUsable, relayUsable)
	} else {
		fallbackEval = FallbackEval{
			State:        FallbackStatePreferDirect,
			FilterDirect: false,
			Reason:       "fallback policy not configured; direct preferred by default",
		}
	}

	// Record the fallback policy outcome on the activation before applying the filter.
	// This makes the policy decision visible regardless of what the scheduler decides:
	// an operator can compare FallbackState with CarrierActivated to understand
	// why the system is on relay even when direct candidates technically exist.
	activation.FallbackState = string(fallbackEval.State)
	activation.FallbackReason = fallbackEval.Reason

	// Apply the fallback filter: when the policy is in FallenBackToRelay or
	// RecoveringToDirect, direct candidates are excluded from the scheduler's
	// input. The scheduler then selects among relay candidates only.
	schedulerCandidates := applyFallbackFilter(candidates, fallbackEval)

	// Egress decision point: the scheduler runs here, at the source endpoint.
	// This is the primary integration between scheduler decisions and carrier
	// activation. Endpoint-owned: only source endpoints call Decide(); relays
	// must not call Decide() for end-to-end path selection.
	decision := runtime.Scheduler.Decide(input.AssociationID, schedulerCandidates)
	activation.Decision = decision

	// Record diagnostics for all candidates considered.
	// This preserves the "why" for both included and excluded candidates.
	activation.Candidates = buildCandidateStatus(input.PathCandidates, refined)

	// No eligible path: skip carrier activation. The reason is in Decision.Reason.
	if decision.Mode == scheduler.ModeNoEligiblePath {
		return activation
	}

	if len(decision.ChosenPaths) == 0 {
		// Should not happen for any non-NoEligiblePath mode. Guard defensively.
		activation.ActivationError = "internal: scheduler returned non-empty mode with no chosen paths"
		return activation
	}

	// Select the carrier based on the scheduler's best-scoring path class.
	//
	// For ModeWeightedBurstFlowlet and ModeSinglePath, ChosenPaths[0] is the
	// single best path. For ModePerPacketStripe, ChosenPaths contains all paths
	// ordered by weight — we activate the best path because per-packet striping
	// across multiple carriers requires a multi-path ingress handle not yet
	// implemented at the carrier level. The Decision field records the mode,
	// making the implementation gap observable without hiding it.
	best := decision.ChosenPaths[0]

	if !best.Class.IsRelay() {
		// Scheduler chose a direct path. Activate the direct carrier.
		// Direct carriage remains architecturally distinct from relay-assisted.
		if err := activateDirectFromScheduledInput(ctx, cfg, runtime.Direct, input); err != nil {
			activation.ActivationError = fmt.Sprintf("activate direct carrier: %v", err)
			return activation
		}
		activation.CarrierActivated = "direct"
	} else {
		// Scheduler chose a relay path. Activate the relay egress carrier.
		// Relay-assisted carriage remains architecturally distinct from direct.
		if err := activateRelayFromScheduledInput(ctx, cfg, runtime.Relay, input); err != nil {
			activation.ActivationError = fmt.Sprintf("activate relay carrier: %v", err)
			return activation
		}
		activation.CarrierActivated = "relay"
	}

	return activation
}

// buildCandidateStatus converts both config-derived and distributed candidates
// into unified diagnostic status for operator reporting.
func buildCandidateStatus(configCandidates []scheduler.PathCandidate, refined []RefinedCandidate) []status.PathCandidateStatus {
	res := make([]status.PathCandidateStatus, 0, len(configCandidates)+len(refined))

	// Config-derived candidates (from static config).
	for _, c := range configCandidates {
		res = append(res, status.PathCandidateStatus{
			ID:            c.ID,
			Class:         string(c.Class),
			Usable:        true,
			Health:        string(c.Health),
			EndpointState: string(CandidateEndpointUnknown),
			RTT:           c.Quality.RTT,
			Jitter:        c.Quality.Jitter,
			LossFraction:  c.Quality.LossFraction,
			Confidence:    c.Quality.Confidence,
		})
	}

	// Coordinator-distributed candidates (refined).
	for _, rc := range refined {
		res = append(res, status.PathCandidateStatus{
			ID:             rc.DistributedID,
			Class:          string(rc.Candidate.Class),
			Usable:         rc.Usable,
			ExcludeReason:  rc.ExcludeReason,
			Health:         string(rc.Candidate.Health),
			DegradedReason: rc.DegradedReason,
			EndpointState:  string(rc.EndpointState),
			RTT:            rc.Candidate.Quality.RTT,
			Jitter:         rc.Candidate.Quality.Jitter,
			LossFraction:   rc.Candidate.Quality.LossFraction,
			Confidence:     rc.Candidate.Quality.Confidence,
		})
	}

	return res
}

// activateDirectFromScheduledInput bridges a ScheduledActivationInput to the
// existing direct-path activation machinery. Direct and relay activation remain
// distinct code paths; this function converts the input type without merging the
// concepts.
func activateDirectFromScheduledInput(
	ctx context.Context,
	cfg config.NodeConfig,
	directRuntime *DirectPathRuntime,
	input ScheduledActivationInput,
) error {
	converted := AssociationActivationInput{
		AssociationID:  input.AssociationID,
		SourceNode:     input.SourceNode,
		SourceService:  input.SourceService,
		DestNode:       input.DestNode,
		DestService:    input.DestService,
		DirectEndpoint: input.DirectEndpoint,
		MeshListenPort: input.MeshListenPort,
	}

	result := ActivateDirectPaths(ctx, cfg, directRuntime, []AssociationActivationInput{converted})
	if len(result.Activations) == 0 {
		return fmt.Errorf("no activation result from direct path")
	}
	if result.Activations[0].Error != "" {
		return fmt.Errorf("%s", result.Activations[0].Error)
	}
	return nil
}

// activateRelayFromScheduledInput bridges a ScheduledActivationInput to the
// existing relay egress activation machinery. Relay and direct activation remain
// distinct code paths; this function converts the input type without merging the
// concepts.
func activateRelayFromScheduledInput(
	ctx context.Context,
	cfg config.NodeConfig,
	relayRuntime *RelayPathRuntime,
	input ScheduledActivationInput,
) error {
	// Resolve local ingress address from config. This is the port where the
	// local application sends traffic into the mesh — the same concept as in
	// direct carriage, but here the outbound target is the relay, not the peer.
	localIngressAddr, err := resolveLocalIngressAddr(cfg, input.SourceService.Name)
	if err != nil {
		return fmt.Errorf("resolve local ingress: %v", err)
	}

	relayInput := RelayEgressActivationInput{
		AssociationID:    input.AssociationID,
		SourceNode:       input.SourceNode,
		SourceService:    input.SourceService,
		DestNode:         input.DestNode,
		DestService:      input.DestService,
		LocalIngressAddr: localIngressAddr,
		RelayEndpoint:    input.RelayEndpoint,
	}

	result := ActivateRelayEgressPaths(ctx, relayRuntime, []RelayEgressActivationInput{relayInput})
	if len(result.Activations) == 0 {
		return fmt.Errorf("no activation result from relay egress")
	}
	if result.Activations[0].Error != "" {
		return fmt.Errorf("%s", result.Activations[0].Error)
	}
	return nil
}
