package coordinator

import (
	"context"
	"fmt"

	"github.com/zhouchenh/transitloom/internal/dataplane"
	"github.com/zhouchenh/transitloom/internal/service"
)

// CoordinatorRelayRuntime manages the coordinator's relay-forwarding state.
//
// The coordinator relay is the single intermediate hop in the path:
//
//	source node → [coordinator relay] → destination node
//
// v1 constraints this runtime enforces:
//   - relay forwarding is association-bound only
//   - exactly one hop: source → coordinator → destination
//   - zero in-band overhead: packet bytes forwarded unchanged
//   - no relay chains: RelayForwardingEntry has no next-relay field
//
// This runtime does not implement:
//   - node relay (only coordinator relay is implemented in T-0010)
//   - scheduler logic, multi-WAN, or encrypted carriage
//   - relay health tracking or dynamic relay selection
//   - relay-path scoring or relay failover
type CoordinatorRelayRuntime struct {
	Table   *dataplane.RelayForwardingTable
	Carrier *dataplane.RelayCarrier
}

// NewCoordinatorRelayRuntime creates a new relay runtime with an empty
// forwarding table and carrier.
func NewCoordinatorRelayRuntime() *CoordinatorRelayRuntime {
	table := dataplane.NewRelayForwardingTable()
	return &CoordinatorRelayRuntime{
		Table:   table,
		Carrier: dataplane.NewRelayCarrier(table),
	}
}

// RelayActivation describes the result of activating relay forwarding for
// a single association.
type RelayActivation struct {
	AssociationID   string
	SourceNode      string
	DestNode        string
	RelayListenAddr string // coordinator's per-association relay listen address
	DestMeshAddr    string // destination node's mesh-facing address
	Active          bool
	Error           string
}

// RelayActivationResult summarizes all relay activation outcomes for a batch.
type RelayActivationResult struct {
	Activations []RelayActivation
	TotalActive int
	TotalFailed int
}

// ReportLines produces human-readable log lines for the relay activation result.
func (r RelayActivationResult) ReportLines() []string {
	lines := make([]string, 0, len(r.Activations)+3)

	lines = append(lines, fmt.Sprintf(
		"coordinator relay activation: active=%d failed=%d (coordinator relay only; single-hop; no scheduler, no multi-WAN)",
		r.TotalActive, r.TotalFailed,
	))

	for _, a := range r.Activations {
		if a.Error != "" {
			lines = append(lines, fmt.Sprintf(
				"  association %s: FAILED: %s",
				a.AssociationID, a.Error,
			))
			continue
		}
		lines = append(lines, fmt.Sprintf(
			"  association %s: relay active: listen=%s -> dest=%s",
			a.AssociationID, a.RelayListenAddr, a.DestMeshAddr,
		))
	}

	return lines
}

// ActivateRelayForAssociation installs relay forwarding context and starts the
// relay carrier for a single association.
//
// Parameters:
//   - assoc: control-plane association record (must have AssociationID set)
//   - relayListenAddr: the coordinator's relay listen address for this association
//   - destMeshAddr: the destination node's mesh-facing UDP address
func ActivateRelayForAssociation(
	ctx context.Context,
	runtime *CoordinatorRelayRuntime,
	assoc service.AssociationRecord,
	relayListenAddr string,
	destMeshAddr string,
) RelayActivation {
	activation := RelayActivation{
		AssociationID:   assoc.AssociationID,
		SourceNode:      assoc.SourceNode,
		DestNode:        assoc.DestinationNode,
		RelayListenAddr: relayListenAddr,
		DestMeshAddr:    destMeshAddr,
	}

	entry, err := dataplane.BuildRelayForwardingEntry(assoc, relayListenAddr, destMeshAddr)
	if err != nil {
		activation.Error = fmt.Sprintf("build relay forwarding entry: %v", err)
		return activation
	}

	if err := runtime.Table.Install(entry); err != nil {
		activation.Error = fmt.Sprintf("install relay forwarding entry: %v", err)
		return activation
	}

	if err := runtime.Carrier.StartRelay(ctx, assoc.AssociationID); err != nil {
		activation.Error = fmt.Sprintf("start relay: %v", err)
		return activation
	}
	activation.Active = true

	return activation
}
