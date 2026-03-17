package controlplane

import (
	"fmt"
	"time"
)

// CoordinatorProbeRequest asks the coordinator to probe a specific external
// endpoint on behalf of the requesting node.
//
// Coordinator-assisted probing is the primary mechanism for a node to verify
// its own external reachability: the coordinator sits outside the node's local
// network and can probe the externally advertised address from a different
// vantage point.
//
// The coordinator cannot infer which inbound ports are forwarded by observing
// the node's source address alone. TargetPort and EffectiveLocalPort must come
// from explicit operator configuration or router-protocol discovery, never from
// port-range guessing. Probing an arbitrary range of ports at coordinator
// request is out of scope.
//
// The node must be running a transport.ProbeResponder on EffectiveLocalPort
// (after DNAT forwarding, if any) so that the coordinator's probe datagram
// can be answered and the round-trip confirmed.
type CoordinatorProbeRequest struct {
	// TargetHost is the external address to probe. This should be the
	// externally reachable address, not the node's local interface address.
	TargetHost string `json:"target_host"`

	// TargetPort is the external UDP port to probe. For DNAT cases this is
	// the public-facing port on the router, not the local mesh listener port.
	TargetPort uint16 `json:"target_port"`

	// EffectiveLocalPort is the local UDP port that will receive the probe
	// after any DNAT forwarding. When there is no DNAT, EffectiveLocalPort
	// equals TargetPort.
	//
	// The coordinator must not assume these are always the same value.
	// Collapsing TargetPort and EffectiveLocalPort breaks DNAT-aware
	// deployments where the router forwards an external port to a different
	// local port.
	EffectiveLocalPort uint16 `json:"effective_local_port"`

	// TimeoutMs is the per-probe deadline in milliseconds. When zero,
	// the coordinator uses its default probe timeout.
	TimeoutMs int `json:"timeout_ms,omitempty"`
}

// CoordinatorProbeResponse reports the result of a coordinator-assisted probe.
//
// A non-empty Error means the probe could not be attempted (e.g., target
// address refused by coordinator policy, or a network error before the probe
// was sent). An empty Error with Reachable=false means the probe was sent but
// the endpoint did not respond within the timeout.
type CoordinatorProbeResponse struct {
	// TargetHost and TargetPort echo the original request for correlation.
	TargetHost string `json:"target_host"`
	TargetPort uint16 `json:"target_port"`

	// Reachable is true if the probe received a valid response within the timeout.
	Reachable bool `json:"reachable"`

	// RoundTripMs is the measured RTT in milliseconds when Reachable is true.
	// Zero when Reachable is false.
	RoundTripMs int `json:"round_trip_ms,omitempty"`

	// Error is non-empty when the probe could not be attempted. An empty Error
	// with Reachable=false means the probe ran but found no response.
	Error string `json:"error,omitempty"`

	// ProbedAt is when the coordinator executed the probe attempt.
	ProbedAt time.Time `json:"probed_at"`
}

// ValidateCoordinatorProbeRequest validates a CoordinatorProbeRequest.
// Returns a descriptive error if required fields are absent or malformed.
func ValidateCoordinatorProbeRequest(req CoordinatorProbeRequest) error {
	if req.TargetHost == "" {
		return fmt.Errorf("coordinator probe request: target_host must be set")
	}
	if req.TargetPort == 0 {
		return fmt.Errorf("coordinator probe request: target_port must be greater than zero")
	}
	if req.EffectiveLocalPort == 0 {
		return fmt.Errorf("coordinator probe request: effective_local_port must be greater than zero")
	}
	if req.TimeoutMs < 0 {
		return fmt.Errorf("coordinator probe request: timeout_ms must be non-negative")
	}
	return nil
}
