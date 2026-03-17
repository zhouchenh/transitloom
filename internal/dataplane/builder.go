package dataplane

import (
	"fmt"
	"net"

	"github.com/zhouchenh/transitloom/internal/service"
)

// BuildDirectForwardingEntry constructs a ForwardingEntry from association and
// service records plus a direct peer endpoint address. This function is the
// explicit bridge between control-plane association state and data-plane
// forwarding state.
//
// It requires:
//   - a valid association record (from the coordinator's association store)
//   - the source service record (from the coordinator's service registry)
//   - the destination service record (from the coordinator's service registry)
//   - a direct endpoint address string for the peer's mesh-facing UDP endpoint
//   - a local ingress address for the source-side local ingress binding
//   - a mesh listen address for the destination-side mesh listener
//
// This function does not install the entry into any forwarding table — the
// caller must do that explicitly. It also does not imply relay support,
// scheduler logic, or multi-WAN behavior.
func BuildDirectForwardingEntry(
	assoc service.AssociationRecord,
	sourceRecord service.Record,
	destRecord service.Record,
	directEndpoint string,
	localIngressAddr string,
	meshListenAddr string,
) (*ForwardingEntry, error) {
	if assoc.AssociationID == "" {
		return nil, fmt.Errorf("association record has no association ID")
	}

	remoteAddr, err := net.ResolveUDPAddr("udp", directEndpoint)
	if err != nil {
		return nil, fmt.Errorf("resolve direct endpoint %q: %w", directEndpoint, err)
	}

	ingressAddr, err := net.ResolveUDPAddr("udp", localIngressAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve local ingress address %q: %w", localIngressAddr, err)
	}

	meshAddr, err := net.ResolveUDPAddr("udp", meshListenAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve mesh listen address %q: %w", meshListenAddr, err)
	}

	// The local target comes from the destination service's binding.
	targetAddr, err := net.ResolveUDPAddr("udp",
		fmt.Sprintf("%s:%d", destRecord.Binding.LocalTarget.Address, destRecord.Binding.LocalTarget.Port))
	if err != nil {
		return nil, fmt.Errorf("resolve local target from destination service binding: %w", err)
	}

	entry := &ForwardingEntry{
		AssociationID:    assoc.AssociationID,
		SourceNode:       assoc.SourceNode,
		SourceService:    assoc.SourceService,
		DestNode:         assoc.DestinationNode,
		DestService:      assoc.DestinationService,
		LocalIngressAddr: ingressAddr,
		RemoteAddr:       remoteAddr,
		LocalTargetAddr:  targetAddr,
		MeshAddr:         meshAddr,
		DirectOnly:       true,
	}

	if err := entry.Validate(); err != nil {
		return nil, fmt.Errorf("built entry failed validation: %w", err)
	}

	return entry, nil
}

// DirectCarriageStatus describes what the current direct carriage
// implementation supports and does not support. This is placeholder reporting
// that makes the implementation boundaries explicit.
type DirectCarriageStatus struct {
	Implemented    []string
	NotImplemented []string
}

// ReportDirectCarriageStatus returns a structured description of what the
// current direct raw UDP carriage implementation covers. This makes it clear
// to callers and operators what is and is not yet available.
func ReportDirectCarriageStatus() DirectCarriageStatus {
	return DirectCarriageStatus{
		Implemented: []string{
			"direct raw UDP carriage for association-bound forwarding entries",
			"local ingress listener (source side): receives from local app, forwards to remote peer",
			"local target delivery (destination side): receives from remote peer, delivers to local service target",
			"zero in-band overhead: raw UDP packet bytes forwarded unchanged",
			"association-bound forwarding: carriage requires valid installed association context",
			"forwarding table with install, lookup, and remove operations",
			"per-association packet and byte counters",
		},
		NotImplemented: []string{
			"relay forwarding (coordinator relay or node relay)",
			"scheduler logic (weighted burst/flowlet-aware or per-packet striping)",
			"multi-WAN path selection or aggregation",
			"encrypted carriage",
			"TCP carriage",
			"path health measurement or scoring",
			"path candidate discovery",
			"production transport hardening",
			"bidirectional return-path state learning",
			"local ingress port allocation from range or persisted-auto mode",
		},
	}
}
