package transport

import (
	"sync"
	"time"
)

// RevalidationTrigger identifies the cause of a staleness event.
//
// Recording the trigger when endpoints are marked stale lets operators
// understand why endpoint knowledge was invalidated and what probe results
// would mean for their deployment.
type RevalidationTrigger string

const (
	// RevalidationTriggerPathDown means the path associated with this
	// endpoint went down. Endpoint knowledge is suspect because the public IP
	// or DNAT rule may have changed during the outage.
	RevalidationTriggerPathDown RevalidationTrigger = "path-down"

	// RevalidationTriggerPathUnhealthy means the path became unhealthy
	// (high loss, latency spike, etc.). The external endpoint may still be
	// reachable, but path health evidence is degraded.
	RevalidationTriggerPathUnhealthy RevalidationTrigger = "path-unhealthy"

	// RevalidationTriggerIPChanged means the node's observed public IP
	// changed. All endpoints with the old IP are now stale.
	RevalidationTriggerIPChanged RevalidationTrigger = "ip-changed"

	// RevalidationTriggerExplicit means an operator or coordinator explicitly
	// requested revalidation. Used for forced-refresh workflows.
	RevalidationTriggerExplicit RevalidationTrigger = "explicit"
)

// EndpointRegistry tracks a collection of ExternalEndpoints and their
// verification and freshness states.
//
// The registry is the node-side authority for managing external endpoint
// knowledge during runtime. It maintains the conceptual distinction between:
//   - configured: operator-declared, not yet verified
//   - unverified: recorded but not probed
//   - verified: confirmed reachable by probe or router protocol
//   - stale: was valid but needs revalidation after a health or IP event
//   - failed: probe actively found unreachable
//
// Key invariants:
//   - Stale and failed endpoints are not used for direct-path decisions
//     without revalidation.
//   - Probe candidates come only from endpoints already in the registry;
//     the registry never generates new host:port combinations.
//
// Thread-safe.
type EndpointRegistry struct {
	mu        sync.RWMutex
	endpoints []*ExternalEndpoint
}

// NewEndpointRegistry creates an empty EndpointRegistry.
func NewEndpointRegistry() *EndpointRegistry {
	return &EndpointRegistry{}
}

// Add inserts a new endpoint into the registry.
func (r *EndpointRegistry) Add(ep ExternalEndpoint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := ep
	r.endpoints = append(r.endpoints, &copied)
}

// Count returns the total number of endpoints in the registry.
func (r *EndpointRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.endpoints)
}

// MarkAllStale marks all non-stale, non-failed endpoints as stale.
//
// Call this when path health events make current endpoint knowledge suspect:
// for example, when the WAN link goes down, when the observed public IP
// changes, or when another condition suggests that externally reachable
// addresses may have changed.
//
// Stale endpoints require revalidation by probing before being used for
// direct-path decisions. Treating stale endpoint data as timeless truth
// would allow stale DNAT mappings or changed public IPs to silently cause
// direct-path failures in real deployments.
//
// Endpoints already stale or failed are left unchanged to preserve existing
// staleness timestamps for operator observability.
func (r *EndpointRegistry) MarkAllStale(at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ep := range r.endpoints {
		if ep.Verification != VerificationStateStale && ep.Verification != VerificationStateFailed {
			ep.MarkStale(at)
		}
	}
}

// SelectForRevalidation returns a copy of endpoints in VerificationStateStale
// or VerificationStateFailed — those that need probing before use.
//
// These form the targeted candidate set for the next probe run. The returned
// endpoints do not include any port-range expansions: the registry only
// surfaces what was explicitly added.
func (r *EndpointRegistry) SelectForRevalidation() []ExternalEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []ExternalEndpoint
	for _, ep := range r.endpoints {
		if ep.Verification == VerificationStateStale || ep.Verification == VerificationStateFailed {
			result = append(result, *ep)
		}
	}
	return result
}

// SelectForInitialVerification returns a copy of endpoints in
// VerificationStateUnverified — newly added endpoints that have not yet
// been probed.
func (r *EndpointRegistry) SelectForInitialVerification() []ExternalEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []ExternalEndpoint
	for _, ep := range r.endpoints {
		if ep.Verification == VerificationStateUnverified {
			result = append(result, *ep)
		}
	}
	return result
}

// UsableEndpoints returns a copy of endpoints that are appropriate for
// direct-path reachability decisions (unverified or verified, not stale
// or failed).
//
// Callers must not use stale or failed endpoints for direct-path decisions
// without probing them first.
func (r *EndpointRegistry) UsableEndpoints() []ExternalEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []ExternalEndpoint
	for _, ep := range r.endpoints {
		if ep.IsUsable() {
			result = append(result, *ep)
		}
	}
	return result
}

// Snapshot returns a copy of all endpoints in the registry regardless of state.
// Useful for status reporting and observability.
func (r *EndpointRegistry) Snapshot() []ExternalEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ExternalEndpoint, 0, len(r.endpoints))
	for _, ep := range r.endpoints {
		result = append(result, *ep)
	}
	return result
}

// ApplyProbeResult updates the verification state of all endpoints in the
// registry matching the result's TargetHost and TargetPort.
//
// Multiple endpoints may match the same host:port (e.g., the same external
// address recorded from both a configured endpoint and a coordinator-observed
// candidate). All matching endpoints are updated because the probe result
// reflects the address-level reachability, not a specific source record.
func (r *EndpointRegistry) ApplyProbeResult(result ProbeResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ep := range r.endpoints {
		if ep.Host == result.TargetHost && ep.Port == result.TargetPort {
			result.ApplyToEndpoint(ep)
		}
	}
}
