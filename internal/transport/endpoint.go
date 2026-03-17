package transport

import (
	"fmt"
	"net/netip"
	"time"
)

// EndpointSource identifies how an external endpoint was learned.
//
// The intended precedence order for endpoint sources is:
//  1. Configured   — explicit operator knowledge (highest precedence)
//  2. RouterDiscovered — UPnP/PCP/NAT-PMP mapping reported by a router
//  3. ProbeDiscovered  — verified by targeted external probing
//  4. CoordinatorObserved — coordinator-reported public IP + configured forwarded port
//
// This ordering reflects the trust hierarchy: explicit operator config is the
// most reliable source; router protocol reports are more targeted than probing;
// coordinator observation of a source address alone is the least specific
// because it cannot infer which inbound ports are actually forwarded.
type EndpointSource string

const (
	// EndpointSourceConfigured means the operator explicitly declared this
	// external endpoint in node config. This is the highest-confidence source
	// and takes precedence over all discovered or observed sources.
	EndpointSourceConfigured EndpointSource = "configured"

	// EndpointSourceRouterDiscovered means a router protocol (UPnP, PCP, or
	// NAT-PMP) reported this external mapping. The router itself knows the
	// active forwarding rule, making this more reliable than blind probing.
	//
	// Not yet implemented. This constant reserves the semantic space so that
	// future router-discovery code places its results in the correct source
	// category rather than overloading local service binding fields.
	EndpointSourceRouterDiscovered EndpointSource = "router-discovered"

	// EndpointSourceProbeDiscovered means targeted external probing has
	// verified that this endpoint is reachable from outside. The probe target
	// must have been chosen deliberately — not through blind full-range scanning.
	//
	// Not yet implemented. This constant reserves the semantic space so that
	// future probe-verification code places its results in the correct source
	// category.
	EndpointSourceProbeDiscovered EndpointSource = "probe-discovered"

	// EndpointSourceCoordinatorObserved means the coordinator observed this
	// node's source address (public IP), and the operator separately configured
	// the forwarded external port. The IP comes from coordinator observation;
	// the port comes from explicit config.
	//
	// This is lower-confidence than pure configured endpoints because the
	// observed IP may differ from what is actually reachable by peers, and
	// coordinator observation of a source address alone is insufficient to
	// infer which inbound ports are usable.
	EndpointSourceCoordinatorObserved EndpointSource = "coordinator-observed"
)

// VerificationState captures what is currently known about an external
// endpoint's reachability from outside the local network.
//
// Verification state is separate from endpoint source: a configured endpoint
// starts as Unverified even though its source is Configured. A
// router-discovered endpoint may start as Verified because the router itself
// confirmed the mapping exists.
//
// Endpoint knowledge must become Stale after unhealthy/down events and must
// be revalidated before being used again for direct-path decisions. This
// prevents stale DNAT mappings or changed public IPs from silently causing
// connection failures in real deployments where dynamic public IPs and
// DNAT-configured inbound ports are common.
type VerificationState string

const (
	// VerificationStateUnverified means this endpoint has been recorded but
	// external reachability has not been tested. Configured endpoints start
	// in this state.
	//
	// Unverified configured endpoints are still usable for direct-path
	// attempts: they represent explicit operator intent and are more reliable
	// than inference-based approaches.
	VerificationStateUnverified VerificationState = "unverified"

	// VerificationStateVerified means a targeted probe or router confirmation
	// has verified that this endpoint is currently reachable from outside.
	VerificationStateVerified VerificationState = "verified"

	// VerificationStateStale means this endpoint was previously verified (or
	// configured and assumed valid) but should be revalidated before use.
	//
	// Staleness is triggered by:
	//   - path health transitioning to unhealthy or down
	//   - explicit invalidation (e.g., public IP change observed)
	//   - time-based expiry (implementation-defined; not yet enforced here)
	//
	// Stale endpoints must not be treated as timeless truth. A stale endpoint
	// may represent a public IP that changed or a DNAT rule that was removed.
	VerificationStateStale VerificationState = "stale"

	// VerificationStateFailed means a targeted probe or discovery attempt
	// actively found this endpoint to be unreachable. Failed endpoints must
	// not be used for direct-path attempts until revalidated.
	VerificationStateFailed VerificationState = "failed"
)

// IsUsableForDirectPath reports whether this verification state allows the
// endpoint to be used for direct-path reachability attempts.
//
// Configured (unverified) and verified endpoints are usable. Stale and
// failed endpoints must be revalidated first — stale because the endpoint
// may no longer be valid after a network event, failed because a probe
// actively found it unreachable.
func (v VerificationState) IsUsableForDirectPath() bool {
	switch v {
	case VerificationStateUnverified, VerificationStateVerified:
		return true
	default:
		// Stale and failed endpoints require revalidation before use.
		// Treating them as usable would allow stale DNAT mappings or
		// changed public IPs to silently poison direct-path decisions.
		return false
	}
}

// ExternalEndpoint represents an externally reachable endpoint for a
// Transitloom node's data plane.
//
// This is explicitly distinct from:
//   - local target: where the local service receives carried traffic
//     (e.g., 127.0.0.1:51820 for a local WireGuard listener)
//   - local ingress: where local apps send traffic into the mesh
//     (a Transitloom-provided loopback port per association)
//   - mesh/runtime port: the local UDP port where Transitloom listens
//     for inbound mesh traffic on this node
//
// An ExternalEndpoint is specifically what a remote node should use to reach
// this node from outside the local network, potentially through DNAT rules
// on a router. The external host:port may differ from the local mesh
// listener address when a DNAT rule is in effect.
//
// Full NAT traversal is intentionally out of scope. ExternalEndpoint models
// only what is explicitly known or discovered about external reachability.
// Transitloom does not assume it can auto-discover unknown DNAT rules from
// local observation alone.
type ExternalEndpoint struct {
	// Host is the externally reachable IP address or hostname.
	//
	// For DNAT cases, this is the public-facing address (e.g., the router's
	// WAN IP), not the node's local interface address.
	//
	// Must not be empty for a valid endpoint.
	Host string

	// Port is the externally reachable UDP port number.
	//
	// For DNAT cases, this is the port number on the public-facing address
	// that the router forwards to LocalPort on this node. The local
	// Transitloom mesh listener is on LocalPort, not Port.
	//
	// Must be nonzero for a valid endpoint.
	Port uint16

	// LocalPort is the local UDP port that the Transitloom mesh data plane
	// is actually listening on. Inbound packets arriving at Host:Port are
	// forwarded by the router to this local port.
	//
	// When zero, the external port and local port are assumed to be the same
	// (no DNAT in effect). When nonzero and different from Port, this records
	// explicit DNAT knowledge: the router is forwarding ExternalPort→LocalPort.
	//
	// This field must not be collapsed with Port. Treating them as the same
	// value in a DNAT deployment would silently break inbound reachability
	// because the router would need to forward to the correct local port.
	LocalPort uint16

	// Source identifies how this endpoint was learned.
	// See EndpointSource constants for the precedence order.
	Source EndpointSource

	// Verification is the current verification state of this endpoint.
	// See VerificationState constants and the MarkStale/MarkVerified/
	// MarkFailed methods for state transitions.
	Verification VerificationState

	// RecordedAt is when this endpoint record was first created or last
	// refreshed. Used for age-based staleness checks.
	RecordedAt time.Time

	// VerifiedAt is when this endpoint was last confirmed reachable.
	// Zero if never verified (e.g., for configured endpoints that start
	// as Unverified).
	VerifiedAt time.Time

	// StaleAt is when this endpoint transitioned to VerificationStateStale
	// or VerificationStateFailed. Zero if the endpoint has never been stale.
	//
	// When nonzero, this records the event that triggered staleness (e.g.,
	// path going unhealthy) so operators can diagnose why an endpoint is
	// not being used for direct-path attempts.
	StaleAt time.Time
}

// Validate checks that the ExternalEndpoint fields are internally consistent
// and meaningful.
//
// Returns an error if:
//   - Host is empty
//   - Port is zero
//   - Source is not a known EndpointSource value
//   - Verification is not a known VerificationState value
func (e ExternalEndpoint) Validate() error {
	if e.Host == "" {
		return fmt.Errorf("external endpoint: host must be set")
	}
	if e.Port == 0 {
		return fmt.Errorf("external endpoint: port must be greater than zero")
	}
	switch e.Source {
	case EndpointSourceConfigured,
		EndpointSourceRouterDiscovered,
		EndpointSourceProbeDiscovered,
		EndpointSourceCoordinatorObserved:
	default:
		return fmt.Errorf("external endpoint: unknown source %q", e.Source)
	}
	switch e.Verification {
	case VerificationStateUnverified,
		VerificationStateVerified,
		VerificationStateStale,
		VerificationStateFailed:
	default:
		return fmt.Errorf("external endpoint: unknown verification state %q", e.Verification)
	}
	return nil
}

// IsUsable reports whether this endpoint is appropriate for use in direct-path
// reachability decisions.
//
// Configured and unverified endpoints are considered usable because they
// represent explicit operator intent. Stale and failed endpoints must be
// revalidated before use.
func (e ExternalEndpoint) IsUsable() bool {
	return e.Verification.IsUsableForDirectPath()
}

// MarkStale transitions this endpoint to VerificationStateStale.
//
// This should be called when the associated path becomes unhealthy or down,
// when a public IP change is observed, or when another event makes the
// current endpoint knowledge suspect.
//
// Stale endpoints must be revalidated before being used for direct-path
// decisions. Endpoint knowledge must not be treated as timeless truth.
func (e *ExternalEndpoint) MarkStale(at time.Time) {
	e.Verification = VerificationStateStale
	e.StaleAt = at
}

// MarkVerified transitions this endpoint to VerificationStateVerified.
//
// This should be called after a targeted probe confirms external reachability,
// or after a router-protocol confirmation. This also serves as revalidation
// for previously stale or failed endpoints.
func (e *ExternalEndpoint) MarkVerified(at time.Time) {
	e.Verification = VerificationStateVerified
	e.VerifiedAt = at
}

// MarkFailed transitions this endpoint to VerificationStateFailed.
//
// This should be called after a targeted probe actively finds the endpoint
// unreachable. Failed endpoints must be revalidated before use.
func (e *ExternalEndpoint) MarkFailed(at time.Time) {
	e.Verification = VerificationStateFailed
	e.StaleAt = at
}

// EffectiveLocalPort returns the local mesh listener port for this endpoint.
//
// When LocalPort is nonzero, it is returned (DNAT case: the router forwards
// the external port to a different local port). When LocalPort is zero, Port
// is returned (no DNAT: the external port and local port are the same).
//
// Callers should use this instead of inspecting Port and LocalPort directly
// to avoid accidentally collapsing the DNAT distinction.
func (e ExternalEndpoint) EffectiveLocalPort() uint16 {
	if e.LocalPort != 0 {
		return e.LocalPort
	}
	return e.Port
}

// HasDNAT reports whether this endpoint has explicit DNAT knowledge,
// meaning the external port differs from the local mesh listener port.
//
// When true, a router is forwarding traffic from Port on the public-facing
// address to LocalPort on this node's local interface. The two ports must
// not be collapsed into one.
func (e ExternalEndpoint) HasDNAT() bool {
	return e.LocalPort != 0 && e.LocalPort != e.Port
}

// RouterDiscoveryHint is a placeholder type for future UPnP/PCP/NAT-PMP
// router-protocol discovery results.
//
// This type is not yet operationally used. It exists to:
//  1. Reserve the semantic space so future agents do not place router
//     discovery data in local service binding fields.
//  2. Define the intended shape of router discovery inputs before any
//     implementation exists.
//
// When router-protocol discovery is implemented, it will populate
// ExternalEndpoint records with EndpointSourceRouterDiscovered.
//
// Full UPnP/PCP/NAT-PMP implementation is intentionally out of scope
// for this task.
type RouterDiscoveryHint struct {
	// Protocol is the discovery protocol that reported this mapping.
	// Expected values: "upnp", "pcp", "nat-pmp".
	Protocol string

	// ExternalHost is the public-facing IP reported by the router.
	ExternalHost string

	// ExternalPort is the public-facing port reported by the router.
	ExternalPort uint16

	// InternalPort is the local port the router is forwarding to.
	InternalPort uint16

	// LeaseDuration is the forwarding rule lease duration reported by the
	// router, if any. Zero means no lease duration was reported.
	LeaseDuration time.Duration

	// RecordedAt is when this hint was received.
	RecordedAt time.Time
}

// ToExternalEndpoint converts a RouterDiscoveryHint to an ExternalEndpoint.
//
// The result starts as VerificationStateVerified because a router-protocol
// report provides first-hand knowledge that the forwarding rule is active.
// The endpoint should still be marked stale after path health events, since
// the router mapping may be removed or the public IP may change.
func (h RouterDiscoveryHint) ToExternalEndpoint() ExternalEndpoint {
	return ExternalEndpoint{
		Host:         h.ExternalHost,
		Port:         h.ExternalPort,
		LocalPort:    h.InternalPort,
		Source:       EndpointSourceRouterDiscovered,
		Verification: VerificationStateVerified,
		RecordedAt:   h.RecordedAt,
		VerifiedAt:   h.RecordedAt,
	}
}

// ProbeResult is a placeholder type for future targeted external probe results.
//
// This type is not yet operationally used. It exists to:
//  1. Reserve the semantic space so future probe-verification code places
//     results in the correct source category.
//  2. Define the intended shape of probe results before any implementation
//     exists.
//
// When probe-based verification is implemented, it will use ProbeResult to
// update ExternalEndpoint records to VerificationStateVerified or
// VerificationStateFailed.
//
// Blind full-range port probing is intentionally not the default behavior.
// Probing should be targeted: verify a specific candidate endpoint rather
// than scanning for unknown mappings.
type ProbeResult struct {
	// TargetHost is the probed external address.
	TargetHost string

	// TargetPort is the probed external port.
	TargetPort uint16

	// Reachable indicates whether the probe confirmed reachability.
	Reachable bool

	// RoundTripTime is the measured RTT if the probe succeeded.
	// Zero when Reachable is false.
	RoundTripTime time.Duration

	// ProbedAt is when this probe was executed.
	ProbedAt time.Time
}

// ApplyToEndpoint updates the ExternalEndpoint verification state based on
// this ProbeResult.
//
// If the probe found the endpoint reachable, the endpoint is marked Verified.
// If the probe found the endpoint unreachable, the endpoint is marked Failed.
func (r ProbeResult) ApplyToEndpoint(e *ExternalEndpoint) {
	if r.Reachable {
		e.MarkVerified(r.ProbedAt)
	} else {
		e.MarkFailed(r.ProbedAt)
	}
}

// NewConfiguredEndpoint creates an ExternalEndpoint from explicit operator
// configuration.
//
// The endpoint starts as VerificationStateUnverified. Configured endpoints
// are usable for direct-path attempts despite being unverified because they
// represent explicit operator intent (the highest-precedence source).
//
// For DNAT cases, pass the public-facing port as externalPort and the local
// mesh listener port as localPort. When no DNAT is in effect and the external
// port equals the local listener port, pass zero for localPort.
func NewConfiguredEndpoint(host string, externalPort, localPort uint16) ExternalEndpoint {
	return ExternalEndpoint{
		Host:         host,
		Port:         externalPort,
		LocalPort:    localPort,
		Source:       EndpointSourceConfigured,
		Verification: VerificationStateUnverified,
		RecordedAt:   time.Now(),
	}
}

// ValidateAddrPort checks whether a host and port combination is structurally
// valid for use as an external endpoint address.
//
// It accepts both IP addresses and hostnames for the host. Port must be
// nonzero. This is a lightweight syntactic check; it does not test actual
// network reachability.
func ValidateAddrPort(host string, port uint16) error {
	if host == "" {
		return fmt.Errorf("host must be set")
	}
	if port == 0 {
		return fmt.Errorf("port must be greater than zero")
	}
	// Try parsing as an IP address. If that fails, the value may still be
	// a valid hostname. Apply a minimal sanity check: no control characters
	// or spaces, which are never valid in a hostname or IP literal.
	if _, err := netip.ParseAddr(host); err != nil {
		for _, c := range host {
			if c < 0x21 || c == 0x7F {
				return fmt.Errorf("host contains invalid character %q", c)
			}
		}
	}
	return nil
}
