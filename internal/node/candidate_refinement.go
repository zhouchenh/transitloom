package node

import (
	"fmt"
	"net"
	"strconv"

	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/scheduler"
	"github.com/zhouchenh/transitloom/internal/transport"
)

// CandidateEndpointState describes the freshness/reachability state of a
// candidate's remote endpoint as found in the EndpointRegistry.
//
// This is explicitly separate from measured path quality (PathQuality):
//   - CandidateEndpointState reflects whether the remote address is known to
//     be reachable or stale based on probe results and registry state.
//   - PathQuality reflects RTT/jitter/loss measurements accumulated by the
//     PathQualityStore from active probes or passive observation.
//
// Both inform candidate refinement but must not be collapsed into one value.
// Collapsing them would hide why a candidate was downgraded — whether due to
// endpoint unreachability or measured quality degradation — making operator
// diagnosis much harder.
type CandidateEndpointState string

const (
	// CandidateEndpointUnknown: no entry in the EndpointRegistry matches this
	// candidate's remote endpoint address. No external freshness information is
	// available. The candidate remains usable but endpoint health is unconfirmed.
	CandidateEndpointUnknown CandidateEndpointState = "unknown"

	// CandidateEndpointUsable: the registry has an entry for this endpoint in
	// unverified or verified state. Unverified represents operator intent (still
	// usable); verified means a probe confirmed reachability.
	CandidateEndpointUsable CandidateEndpointState = "usable"

	// CandidateEndpointStale: the registry has an entry for this endpoint in
	// stale state. The endpoint was valid but a path-down or IP-change event
	// invalidated it. Revalidation by probing is needed before trusting this
	// endpoint for full direct-path use.
	//
	// Stale candidates are health-degraded (HealthStateDegraded) but not
	// excluded. They remain available as a last-resort path while revalidation
	// is pending — the scheduler will prefer fresher candidates when they exist.
	CandidateEndpointStale CandidateEndpointState = "stale"

	// CandidateEndpointFailed: the registry has an entry for this endpoint in
	// failed state. A targeted probe actively found the remote address unreachable.
	//
	// Failed candidates are excluded from scheduler inputs (Usable=false). The
	// scheduler must not select a path that is known to be unreachable. Exclusion
	// is the correct behavior here — stale paths may still work, failed paths
	// are confirmed broken.
	CandidateEndpointFailed CandidateEndpointState = "failed"
)

// RefinedCandidate is a coordinator-distributed path candidate after
// endpoint-freshness checking and quality enrichment. It is the intermediate
// form between coordinator-distributed candidate data and the
// scheduler.PathCandidate inputs passed to Scheduler.Decide().
//
// Architectural boundaries preserved by this type:
//   - Candidate existence (DistributedID non-empty) is NOT the same as
//     candidate usability (Usable). A coordinator-distributed candidate exists
//     in the CandidateStore; a usable candidate has a real endpoint and was not
//     excluded by endpoint-failure or missing-endpoint checks.
//   - Candidate usability (Usable=true) is NOT the same as chosen runtime path.
//     Scheduler.Decide() makes the actual path selection from usable candidates;
//     that decision is separate and endpoint-owned.
//   - Endpoint state (EndpointState) is NOT the same as measured path quality.
//     EndpointState reflects address-level reachability from the EndpointRegistry.
//     QualityFresh / Candidate.Quality reflect RTT/jitter/loss from the
//     PathQualityStore. Both are distinct inputs kept explicitly separate.
//   - Direct and relay candidates remain distinct via Candidate.Class — the
//     relay/direct distinction is never collapsed here.
//
// Reasons for exclusion or degradation are explicit and inspectable so that
// future agents, operators, and tests can verify refinement behavior without
// reading through opaque scoring logic.
type RefinedCandidate struct {
	// Candidate is the scheduler.PathCandidate ready for Scheduler.Decide().
	// Only meaningful when Usable=true.
	Candidate scheduler.PathCandidate

	// DistributedID is the original CandidateID from the coordinator-distributed
	// candidate. It links this refined candidate back to the distributed source.
	DistributedID string

	// Usable is true when this candidate should be included in scheduler inputs.
	// False means this candidate was excluded. Excluded candidates must not be
	// passed to Scheduler.Decide().
	Usable bool

	// ExcludeReason explains why Usable=false. Non-empty only when Usable=false.
	// Inspectable for diagnostics and tests.
	ExcludeReason string

	// DegradedReason explains why Candidate.Health was downgraded below
	// HealthStateActive. Non-empty when health degradation was applied.
	// May coexist with Usable=true (degraded but still eligible).
	DegradedReason string

	// EndpointState describes the freshness state of the candidate's remote
	// endpoint as found in the EndpointRegistry. Distinct from quality.
	EndpointState CandidateEndpointState

	// QualityFresh is true when the PathQualityStore contained a fresh
	// measurement for this candidate. When false, Candidate.Quality is zero
	// (unmeasured, confidence=0) — the scheduler treats it conservatively.
	QualityFresh bool
}

// ReportLine returns a single human-readable line summarizing this refined
// candidate's state. Intended for status output and log reporting.
func (rc RefinedCandidate) ReportLine() string {
	if !rc.Usable {
		return fmt.Sprintf("candidate %s [excluded]: %s", rc.DistributedID, rc.ExcludeReason)
	}
	parts := fmt.Sprintf("candidate %s [usable] class=%s endpoint=%s quality=%s",
		rc.DistributedID,
		rc.Candidate.Class,
		rc.EndpointState,
		qualityLabel(rc.Candidate.Quality, rc.QualityFresh),
	)
	if rc.DegradedReason != "" {
		parts += fmt.Sprintf(" degraded: %s", rc.DegradedReason)
	}
	return parts
}

// qualityLabel returns a compact quality description for reporting.
func qualityLabel(q scheduler.PathQuality, fresh bool) string {
	if !fresh || !q.Measured() {
		return "unmeasured"
	}
	return fmt.Sprintf("rtt=%v conf=%.2f loss=%.3f", q.RTT, q.Confidence, q.LossFraction)
}

// RefineCandidates applies endpoint-freshness checking and quality enrichment
// to a set of coordinator-distributed path candidates, returning one
// RefinedCandidate per input.
//
// Each candidate is processed in three explicit steps:
//  1. Usability check: candidates without a RemoteEndpoint are informational
//     only and excluded (Usable=false, ExcludeReason set).
//  2. Endpoint-freshness check: if registry is non-nil, the candidate's remote
//     endpoint (host:port) is looked up in the registry snapshot.
//     - Failed endpoint → candidate excluded (Usable=false).
//     - Stale endpoint → health downgraded to HealthStateDegraded, candidate
//       remains usable as a last-resort path while revalidation is pending.
//     - Usable/unknown endpoint → no health penalty applied.
//  3. Quality enrichment: if qualityStore is non-nil, a fresh measurement
//     for this candidate ID is looked up. When fresh, Candidate.Quality is
//     set and QualityFresh=true. When stale or absent, Quality stays zero
//     (unmeasured, confidence=0).
//
// Endpoint freshness and measured quality are kept as distinct inputs throughout:
//   - EndpointState reflects registry-based address reachability.
//   - QualityFresh / Candidate.Quality reflect path-quality measurements.
//
// The returned slice always has the same length as the input. Callers use
// UsableSchedulerCandidates to extract only the scheduler-ready candidates.
//
// nil registry → endpoint state is CandidateEndpointUnknown for all candidates.
// nil qualityStore → quality stays zero (unmeasured) for all candidates.
func RefineCandidates(
	candidates []controlplane.DistributedPathCandidate,
	registry *transport.EndpointRegistry,
	qualityStore *scheduler.PathQualityStore,
) []RefinedCandidate {
	if len(candidates) == 0 {
		return nil
	}

	// Build endpoint snapshot once for O(n*m) lookup across all candidates.
	// This avoids holding the registry lock for each individual lookup.
	var endpointSnapshot []transport.ExternalEndpoint
	if registry != nil {
		endpointSnapshot = registry.Snapshot()
	}

	result := make([]RefinedCandidate, len(candidates))

	for i, c := range candidates {
		rc := RefinedCandidate{
			DistributedID: c.CandidateID,
			EndpointState: CandidateEndpointUnknown,
		}

		// Step 1: Usability check.
		// Candidates without a RemoteEndpoint are informational only — the
		// coordinator knows the path class exists but has no usable address yet.
		// These must not be passed to the scheduler.
		if !c.IsUsable() {
			rc.Usable = false
			rc.ExcludeReason = "no remote endpoint: informational candidate only"
			result[i] = rc
			continue
		}

		// Convert to scheduler.PathCandidate with active health as the initial
		// state. Health may be degraded below if endpoint freshness requires it.
		rc.Candidate = scheduler.PathCandidate{
			ID:            c.CandidateID,
			AssociationID: c.AssociationID,
			Class:         convertDistributedClass(c.Class),
			Health:        scheduler.HealthStateActive,
			AdminWeight:   c.AdminWeight,
			IsMetered:     c.IsMetered,
		}

		// Step 2: Endpoint-freshness check.
		// Check the registry for any known freshness state for this remote endpoint.
		// Endpoint freshness is about address-level reachability — distinct from
		// measured RTT/jitter/loss quality.
		if len(endpointSnapshot) > 0 {
			rc.EndpointState = checkEndpointFreshness(c.RemoteEndpoint, endpointSnapshot)
		}

		switch rc.EndpointState {
		case CandidateEndpointFailed:
			// A targeted probe actively found this remote address unreachable.
			// Exclude the candidate: the scheduler must not select a confirmed-
			// unreachable path. Stale candidates are degraded (not excluded)
			// because they *may* still work; failed candidates are confirmed broken.
			rc.Usable = false
			rc.ExcludeReason = "endpoint failed: probe confirmed remote address unreachable"
			result[i] = rc
			continue

		case CandidateEndpointStale:
			// The endpoint was valid but a path-down or IP-change event made it
			// suspect. Downgrade health so the scheduler prefers fresher candidates
			// when available, but keep the candidate usable as a fallback while
			// revalidation is pending. Stale reachability data must not silently
			// appear healthy — the degraded health makes the staleness visible to
			// the scheduler and to observability tooling.
			rc.Candidate.Health = scheduler.HealthStateDegraded
			rc.DegradedReason = "endpoint stale: needs revalidation before trusted use"
		}

		// Step 3: Quality enrichment.
		// Look up fresh quality measurements for this candidate ID. When fresh,
		// enrich the Candidate.Quality field so the scheduler has real RTT/jitter/
		// loss/confidence data for fine-grained scoring and striping eligibility.
		//
		// Stale or absent measurements leave quality at zero (unmeasured,
		// confidence=0). The scheduler treats unmeasured candidates conservatively:
		// eligible for carriage, but cannot qualify for per-packet striping.
		//
		// Quality enrichment is done here, inside RefineCandidates, rather than
		// by a separate QualityStore.ApplyCandidates call, so that the QualityFresh
		// flag is visible on the RefinedCandidate for diagnostics. The quality
		// has already been applied when UsableSchedulerCandidates is called —
		// callers must not call ApplyCandidates again on the result.
		if qualityStore != nil {
			if q, fresh := qualityStore.FreshQuality(c.CandidateID); fresh {
				rc.Candidate.Quality = q
				rc.QualityFresh = true
			}
			// No fresh measurement: Quality stays zero-value.
			// Stale measurement is correctly represented as "not fresh" by
			// FreshQuality (returns false when older than MaxAge), which means
			// the scheduler sees unmeasured rather than old stale data.
		}

		rc.Usable = true
		result[i] = rc
	}

	return result
}

// UsableSchedulerCandidates extracts the scheduler.PathCandidate inputs from
// a refined candidate set. Only Usable=true candidates are included.
//
// The returned slice is the correct input for Scheduler.Decide(). Excluded
// candidates (failed endpoint, informational-only, etc.) are omitted because
// the scheduler must not select paths that are known to be unusable.
//
// Quality enrichment has already been applied during RefineCandidates. Callers
// must not call QualityStore.ApplyCandidates on the returned slice — that would
// incorrectly zero out already-enriched quality fields for candidates without
// a second store entry, and would duplicate work for those with one.
//
// This function is the boundary between the refinement layer and the scheduler:
// refined candidates → usable scheduler inputs → Scheduler.Decide() → chosen path.
// The chosen path is produced separately; it is not stored or tracked here.
func UsableSchedulerCandidates(refined []RefinedCandidate) []scheduler.PathCandidate {
	candidates := make([]scheduler.PathCandidate, 0, len(refined))
	for _, rc := range refined {
		if rc.Usable {
			candidates = append(candidates, rc.Candidate)
		}
	}
	return candidates
}

// checkEndpointFreshness looks up the endpoint state for a "host:port" address
// in the provided registry snapshot. It returns the most severe state found
// across all matching entries (Failed > Stale > Usable > Unknown).
//
// Returning the most severe state is conservative: if any record for an
// address is failed or stale, the candidate should be treated accordingly.
// This prevents a situation where one stale record is hidden by a usable
// record for the same host:port.
//
// Returns CandidateEndpointUnknown when:
//   - addrPort cannot be parsed as "host:port"
//   - no registry entry matches the host and port
func checkEndpointFreshness(addrPort string, snapshot []transport.ExternalEndpoint) CandidateEndpointState {
	host, portStr, err := net.SplitHostPort(addrPort)
	if err != nil {
		return CandidateEndpointUnknown
	}
	port64, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil || port64 == 0 {
		return CandidateEndpointUnknown
	}
	port := uint16(port64)

	found := false
	worst := CandidateEndpointUsable

	for _, ep := range snapshot {
		if ep.Host != host || ep.Port != port {
			continue
		}
		found = true
		state := verificationToEndpointState(ep.Verification)
		if endpointStateSeverity(state) > endpointStateSeverity(worst) {
			worst = state
		}
	}

	if !found {
		return CandidateEndpointUnknown
	}
	return worst
}

// verificationToEndpointState converts a transport.VerificationState to a
// CandidateEndpointState for use in refinement decisions.
//
// Unverified and verified both map to CandidateEndpointUsable because both are
// appropriate for direct-path attempts. The distinction between them (operator
// intent vs probe-confirmed) is preserved in the registry but does not require
// different treatment at the refinement level.
func verificationToEndpointState(v transport.VerificationState) CandidateEndpointState {
	switch v {
	case transport.VerificationStateFailed:
		return CandidateEndpointFailed
	case transport.VerificationStateStale:
		return CandidateEndpointStale
	default:
		// VerificationStateUnverified and VerificationStateVerified: both usable.
		return CandidateEndpointUsable
	}
}

// endpointStateSeverity returns a numeric severity for CandidateEndpointState.
// Higher severity means worse freshness state. Used to find the worst state
// across multiple registry entries for the same host:port.
func endpointStateSeverity(s CandidateEndpointState) int {
	switch s {
	case CandidateEndpointFailed:
		return 3
	case CandidateEndpointStale:
		return 2
	case CandidateEndpointUsable:
		return 1
	default: // CandidateEndpointUnknown
		return 0
	}
}

// convertDistributedClass converts a DistributedPathCandidateClass (wire format)
// to a scheduler.PathClass (local runtime input). The two types use the same
// string values by design, but are kept distinct so that wire-format changes
// do not accidentally affect scheduler semantics.
//
// The relay/direct distinction is preserved exactly: coordinator-relay maps to
// PathClassCoordinatorRelay, direct-public maps to PathClassDirectPublic, etc.
// This function must never collapse relay and direct classes.
func convertDistributedClass(class controlplane.DistributedPathCandidateClass) scheduler.PathClass {
	switch class {
	case controlplane.DistributedPathClassDirectPublic:
		return scheduler.PathClassDirectPublic
	case controlplane.DistributedPathClassDirectIntranet:
		return scheduler.PathClassDirectIntranet
	case controlplane.DistributedPathClassNodeRelay:
		return scheduler.PathClassNodeRelay
	default:
		// coordinator-relay is the default for unknown relay-class values.
		return scheduler.PathClassCoordinatorRelay
	}
}
