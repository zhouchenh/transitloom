// Package dataplane implements Transitloom's data-plane runtime behavior.
//
// This package contains direct raw UDP carriage and coordinator relay-assisted
// one-hop raw UDP carriage. It does not implement scheduler, multi-WAN,
// encrypted, or TCP carriage behavior.
//
// Key architectural constraints preserved here:
//
//   - Zero in-band overhead: no Transitloom headers are added to carried
//     UDP packets. Forwarding identity comes from installed association
//     state and binding context, not from packet content.
//
//   - Association-bound forwarding: packets flow only within valid,
//     installed association context. There is no unrestricted forwarding.
//
//   - Local ingress and local target are separate concepts. The local
//     ingress is a Transitloom-provided port where the local application
//     sends traffic into the mesh. The local target is the service
//     binding's port where inbound carried traffic is delivered.
//
//   - Direct and relay-assisted carriage are distinct modes. DirectCarrier
//     handles direct paths; RelayCarrier handles coordinator relay paths.
//     These must not be conflated.
//
//   - Single relay hop: relay-assisted carriage allows exactly one relay
//     hop (source → relay → destination). No relay chains are allowed in
//     v1. This constraint is enforced by the relay type model: there is
//     no mechanism in RelayForwardingEntry for a second forwarding step.
package dataplane

import (
	"fmt"
	"net"
	"sort"
	"sync"

	"github.com/zhouchenh/transitloom/internal/service"
)

// ForwardingEntry holds the installed forwarding state for one direction of
// direct raw UDP association-bound carriage.
//
// This entry is the data-plane manifestation of a control-plane association
// record. It does not represent the association itself — it represents the
// minimum forwarding context that makes direct carriage legal and operational
// for that association direction.
//
// Zero in-band overhead: no Transitloom headers are added to carried packets.
// Forwarding identity comes entirely from this installed state.
//
// Why association-bound: the v1 data plane must not invent forwarding behavior
// outside association context. A ForwardingEntry without a valid association
// must not exist in the forwarding table.
//
// Why direct-only: this task implements only direct carriage. Relay support,
// scheduler logic, multi-WAN optimization, and encrypted carriage are future
// work and must not be implied by this implementation.
type ForwardingEntry struct {
	AssociationID string
	SourceNode    string
	SourceService service.Identity
	DestNode      string
	DestService   service.Identity

	// LocalIngressAddr is where the local application sends traffic into the
	// mesh. This is a Transitloom-provided local ingress port, NOT the
	// service's own local target. These must remain separate concepts:
	// service binding (local target) ≠ local ingress binding.
	LocalIngressAddr *net.UDPAddr

	// RemoteAddr is the peer's direct mesh-facing UDP endpoint. For
	// direct-only carriage, this is where outbound packets are sent.
	RemoteAddr *net.UDPAddr

	// LocalTargetAddr is where inbound carried traffic is delivered to the
	// local service. This is the service binding's local target, NOT the
	// local ingress. These must remain separate concepts.
	LocalTargetAddr *net.UDPAddr

	// MeshAddr is the local mesh-facing UDP address where this node receives
	// inbound carried packets from the remote peer for this association.
	MeshAddr *net.UDPAddr

	// DirectOnly is always true in this task. Direct raw UDP carriage does
	// not imply relay support, scheduler logic, or multi-WAN behavior.
	DirectOnly bool
}

// Validate checks that the forwarding entry has the minimum required fields
// for installation into the forwarding table.
func (e *ForwardingEntry) Validate() error {
	if e.AssociationID == "" {
		return fmt.Errorf("association_id must be set")
	}
	if e.SourceNode == "" {
		return fmt.Errorf("source_node must be set")
	}
	if e.DestNode == "" {
		return fmt.Errorf("dest_node must be set")
	}
	if err := e.SourceService.Validate(); err != nil {
		return fmt.Errorf("source_service: %w", err)
	}
	if err := e.DestService.Validate(); err != nil {
		return fmt.Errorf("dest_service: %w", err)
	}
	if !e.DirectOnly {
		return fmt.Errorf("ForwardingEntry is for direct carriage only; for relay-assisted carriage use RelayForwardingEntry or RelayEgressEntry")
	}
	return nil
}

// ForwardingTable provides association-bound forwarding lookup for direct raw
// UDP carriage.
//
// Forwarding is strictly association-bound: packets can only flow within
// installed association context. This table does not support unrestricted
// forwarding — attempting to look up a nonexistent association returns a
// clear miss, not a fallback.
//
// This table is direct-only. It does not contain relay state, scheduler
// state, multi-WAN logic, or path scoring.
type ForwardingTable struct {
	mu      sync.RWMutex
	entries map[string]*ForwardingEntry // keyed by AssociationID
}

// NewForwardingTable creates an empty forwarding table.
func NewForwardingTable() *ForwardingTable {
	return &ForwardingTable{
		entries: make(map[string]*ForwardingEntry),
	}
}

// Install adds a validated forwarding entry to the table. The entry must
// pass validation and have a unique AssociationID. If an entry with the
// same AssociationID already exists, it is replaced.
func (t *ForwardingTable) Install(entry *ForwardingEntry) error {
	if entry == nil {
		return fmt.Errorf("forwarding entry must not be nil")
	}
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("invalid forwarding entry: %w", err)
	}

	clone := *entry
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries[clone.AssociationID] = &clone
	return nil
}

// Lookup returns the forwarding entry for the given association ID, or false
// if no entry is installed. A miss means the association has no installed
// forwarding context — carriage must not proceed.
func (t *ForwardingTable) Lookup(associationID string) (*ForwardingEntry, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	entry, exists := t.entries[associationID]
	if !exists {
		return nil, false
	}
	clone := *entry
	return &clone, true
}

// LookupByIngress finds the forwarding entry whose LocalIngressAddr matches
// the given address string. Returns false if no match exists.
func (t *ForwardingTable) LookupByIngress(ingressAddr string) (*ForwardingEntry, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, entry := range t.entries {
		if entry.LocalIngressAddr != nil && entry.LocalIngressAddr.String() == ingressAddr {
			clone := *entry
			return &clone, true
		}
	}
	return nil, false
}

// LookupByMeshAddr finds the forwarding entry whose MeshAddr matches the
// given address string. Returns false if no match exists.
func (t *ForwardingTable) LookupByMeshAddr(meshAddr string) (*ForwardingEntry, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, entry := range t.entries {
		if entry.MeshAddr != nil && entry.MeshAddr.String() == meshAddr {
			clone := *entry
			return &clone, true
		}
	}
	return nil, false
}

// Remove removes the forwarding entry for the given association ID. Returns
// true if an entry was removed, false if none existed.
func (t *ForwardingTable) Remove(associationID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	_, existed := t.entries[associationID]
	delete(t.entries, associationID)
	return existed
}

// Snapshot returns a sorted copy of all installed forwarding entries.
func (t *ForwardingTable) Snapshot() []*ForwardingEntry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	entries := make([]*ForwardingEntry, 0, len(t.entries))
	for _, entry := range t.entries {
		clone := *entry
		entries = append(entries, &clone)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].AssociationID < entries[j].AssociationID
	})
	return entries
}

// Count returns the number of installed forwarding entries.
func (t *ForwardingTable) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.entries)
}
