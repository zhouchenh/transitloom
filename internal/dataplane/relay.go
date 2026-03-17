package dataplane

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/zhouchenh/transitloom/internal/service"
)

// --- Coordinator relay side ---

// RelayForwardingEntry holds the installed relay-forwarding context for one
// association on the coordinator relay side.
//
// The coordinator relay is the single intermediate hop in the path:
//
//	source node → [coordinator relay] → destination node
//
// v1 single-hop constraint: this entry has no "next relay" or "chain"
// field. The only outbound destination is DestMeshAddr, which must be a
// terminal destination-node endpoint. There is no mechanism to create relay
// chains. This structural absence enforces the v1 single-hop limit without
// needing a runtime check.
//
// Zero in-band overhead: the relay identifies which association a packet
// belongs to by which RelayListenAddr it arrived on, not by inspecting packet
// content. Each association gets its own per-association relay listen port.
//
// Association-bound: a RelayForwardingEntry must be installed before the
// coordinator will relay for an association. There is no open forwarding.
type RelayForwardingEntry struct {
	AssociationID string
	SourceNode    string
	SourceService service.Identity
	DestNode      string
	DestService   service.Identity

	// RelayListenAddr is the coordinator's per-association relay listen port.
	// Each association that uses this relay gets a unique port, which is how
	// the coordinator knows which association each arriving packet belongs to
	// without any in-band header.
	RelayListenAddr *net.UDPAddr

	// DestMeshAddr is the destination node's mesh-facing UDP address.
	// The coordinator forwards received packets here.
	//
	// This is a terminal endpoint — not another relay — which enforces
	// the v1 single-hop constraint structurally.
	DestMeshAddr *net.UDPAddr
}

// Validate checks that the relay forwarding entry has the required fields.
func (e *RelayForwardingEntry) Validate() error {
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
	if e.RelayListenAddr == nil {
		return fmt.Errorf("relay_listen_addr must be set")
	}
	if e.DestMeshAddr == nil {
		return fmt.Errorf("dest_mesh_addr must be set")
	}
	return nil
}

// RelayForwardingTable is the coordinator's association-bound relay lookup.
// It stores the relay forwarding context for each active relay association.
// Like ForwardingTable, it enforces association-bound carriage: there is no
// relay forwarding outside of installed association context.
type RelayForwardingTable struct {
	mu      sync.RWMutex
	entries map[string]*RelayForwardingEntry // keyed by AssociationID
}

// NewRelayForwardingTable creates an empty relay forwarding table.
func NewRelayForwardingTable() *RelayForwardingTable {
	return &RelayForwardingTable{
		entries: make(map[string]*RelayForwardingEntry),
	}
}

// Install adds a validated relay forwarding entry to the table.
func (t *RelayForwardingTable) Install(entry *RelayForwardingEntry) error {
	if entry == nil {
		return fmt.Errorf("relay forwarding entry must not be nil")
	}
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("invalid relay forwarding entry: %w", err)
	}
	clone := *entry
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries[clone.AssociationID] = &clone
	return nil
}

// Lookup returns the relay forwarding entry for the given association ID.
func (t *RelayForwardingTable) Lookup(associationID string) (*RelayForwardingEntry, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	entry, exists := t.entries[associationID]
	if !exists {
		return nil, false
	}
	clone := *entry
	return &clone, true
}

// Remove removes the relay forwarding entry for the given association ID.
func (t *RelayForwardingTable) Remove(associationID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, existed := t.entries[associationID]
	delete(t.entries, associationID)
	return existed
}

// Count returns the number of installed relay forwarding entries.
func (t *RelayForwardingTable) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.entries)
}

// Snapshot returns a sorted copy of all installed relay forwarding entries.
func (t *RelayForwardingTable) Snapshot() []*RelayForwardingEntry {
	t.mu.RLock()
	defer t.mu.RUnlock()
	entries := make([]*RelayForwardingEntry, 0, len(t.entries))
	for _, entry := range t.entries {
		clone := *entry
		entries = append(entries, &clone)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].AssociationID < entries[j].AssociationID
	})
	return entries
}

// RelayCarrier manages coordinator relay forwarding for association-bound
// single-hop raw UDP relay paths.
//
// The coordinator relay is the intermediate hop:
//
//	source node → [RelayCarrier listens on RelayListenAddr]
//	           → [RelayCarrier forwards to DestMeshAddr]
//	           → destination node
//
// Relay-assisted carriage properties:
//   - association-bound: relay requires an installed RelayForwardingEntry
//   - single-hop: forwards to DestMeshAddr only; no chaining mechanism exists
//   - zero in-band overhead: packet bytes forwarded unchanged
//
// RelayCarrier does not implement:
//   - scheduler logic or multi-WAN path selection
//   - encrypted carriage or TCP carriage
//   - arbitrary multi-hop forwarding
//   - relay health tracking or dynamic relay selection
type RelayCarrier struct {
	table    *RelayForwardingTable
	mu       sync.Mutex
	handlers map[string]*relayHandle // keyed by association ID
}

// NewRelayCarrier creates a relay carrier backed by the given forwarding table.
func NewRelayCarrier(table *RelayForwardingTable) *RelayCarrier {
	return &RelayCarrier{
		table:    table,
		handlers: make(map[string]*relayHandle),
	}
}

// StartRelay starts the relay handler for an association.
//
// It binds a UDP listener on the entry's RelayListenAddr and forwards
// received packets to the entry's DestMeshAddr (the destination node's mesh
// listener). The association must be installed in the relay forwarding table.
func (c *RelayCarrier) StartRelay(ctx context.Context, associationID string) error {
	entry, exists := c.table.Lookup(associationID)
	if !exists {
		return fmt.Errorf("association %q: no installed relay forwarding entry; relay carriage requires valid association context", associationID)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.handlers[associationID]; exists {
		return fmt.Errorf("association %q: relay already running", associationID)
	}

	// Bind the relay listen socket where source node sends packets.
	relayConn, err := net.ListenUDP("udp", entry.RelayListenAddr)
	if err != nil {
		return fmt.Errorf("association %q: bind relay listener %s: %w", associationID, entry.RelayListenAddr, err)
	}

	// Connected send socket to the destination node's mesh address.
	// Using DialUDP gives a connected socket for efficient forwarding.
	destConn, err := net.DialUDP("udp", nil, entry.DestMeshAddr)
	if err != nil {
		relayConn.Close()
		return fmt.Errorf("association %q: connect to destination mesh %s: %w", associationID, entry.DestMeshAddr, err)
	}

	childCtx, cancel := context.WithCancel(ctx)
	handle := &relayHandle{
		associationID: associationID,
		relayConn:     relayConn,
		destConn:      destConn,
		cancel:        cancel,
	}

	c.handlers[associationID] = handle
	go handle.run(childCtx)

	return nil
}

// StopRelay stops the relay handler for the given association.
func (c *RelayCarrier) StopRelay(associationID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if handle, exists := c.handlers[associationID]; exists {
		handle.stop()
		delete(c.handlers, associationID)
	}
}

// StopAll stops all running relay handlers.
func (c *RelayCarrier) StopAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, handle := range c.handlers {
		handle.stop()
		delete(c.handlers, id)
	}
}

// RelayStats returns packet/byte counters for a relay handler.
func (c *RelayCarrier) RelayStats(associationID string) (packets, bytes uint64, running bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	handle, exists := c.handlers[associationID]
	if !exists {
		return 0, 0, false
	}
	return handle.packetsRelayed.Load(), handle.bytesRelayed.Load(), true
}

// relayHandle manages the coordinator relay for one association.
// It receives packets from the source node on RelayListenAddr and forwards
// them to the destination node's DestMeshAddr.
type relayHandle struct {
	associationID  string
	relayConn      *net.UDPConn // receives from source node
	destConn       *net.UDPConn // sends to destination node's mesh addr
	cancel         context.CancelFunc
	packetsRelayed atomic.Uint64
	bytesRelayed   atomic.Uint64
}

func (h *relayHandle) run(ctx context.Context) {
	buf := make([]byte, maxUDPPayload)

	go func() {
		<-ctx.Done()
		h.relayConn.Close()
		h.destConn.Close()
	}()

	for {
		n, _, err := h.relayConn.ReadFromUDP(buf)
		if err != nil {
			return // closed or context done
		}

		// Zero in-band overhead: forward raw packet bytes unchanged.
		// Single-hop: we send to destConn only — no further relay step.
		// There is no mechanism here for a second hop, which enforces
		// the v1 single relay hop constraint at the operational level.
		_, err = h.destConn.Write(buf[:n])
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue // transient send error, keep going
		}

		h.packetsRelayed.Add(1)
		h.bytesRelayed.Add(uint64(n))
	}
}

func (h *relayHandle) stop() {
	h.cancel()
}

// --- Source node relay egress side ---

// RelayEgressEntry holds the source node's installed forwarding context for
// relay-assisted carriage through a coordinator relay.
//
// This entry represents the source node's outbound relay egress path:
//
//	local app → [local ingress] → coordinator relay (RelayAddr)
//
// Relay-assisted egress is explicitly distinct from direct carriage:
//   - direct carriage (ForwardingEntry): sends to destination node directly
//   - relay-assisted egress (RelayEgressEntry): sends to coordinator relay
//
// The source node knows it is sending via a relay. This distinction is
// preserved architecturally by using a separate entry type — RelayEgressEntry
// must not be confused with ForwardingEntry (direct) even though the outbound
// send behavior is similar at the socket level.
//
// The local ingress role is the same as in direct carriage: the local
// application sends traffic to LocalIngressAddr, and this carrier forwards it
// to RelayAddr (the coordinator relay's per-association listen port).
type RelayEgressEntry struct {
	AssociationID string
	SourceNode    string
	SourceService service.Identity
	DestNode      string
	DestService   service.Identity

	// LocalIngressAddr is where the local application sends traffic into
	// the mesh. Same concept as in direct carriage (ForwardingEntry), but
	// here the outbound destination is the coordinator relay, not a peer.
	LocalIngressAddr *net.UDPAddr

	// RelayAddr is the coordinator relay's per-association listen address.
	// The source sends outbound packets here instead of to the destination
	// node directly. The coordinator then forwards to the destination.
	RelayAddr *net.UDPAddr
}

// Validate checks that the relay egress entry has the required fields.
func (e *RelayEgressEntry) Validate() error {
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
	return nil
}

// RelayEgressTable provides association-bound relay egress lookup for the
// source node's RelayEgressCarrier.
type RelayEgressTable struct {
	mu      sync.RWMutex
	entries map[string]*RelayEgressEntry // keyed by AssociationID
}

// NewRelayEgressTable creates an empty relay egress table.
func NewRelayEgressTable() *RelayEgressTable {
	return &RelayEgressTable{
		entries: make(map[string]*RelayEgressEntry),
	}
}

// Install adds a validated relay egress entry to the table.
func (t *RelayEgressTable) Install(entry *RelayEgressEntry) error {
	if entry == nil {
		return fmt.Errorf("relay egress entry must not be nil")
	}
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("invalid relay egress entry: %w", err)
	}
	clone := *entry
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries[clone.AssociationID] = &clone
	return nil
}

// Lookup returns the relay egress entry for the given association ID.
func (t *RelayEgressTable) Lookup(associationID string) (*RelayEgressEntry, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	entry, exists := t.entries[associationID]
	if !exists {
		return nil, false
	}
	clone := *entry
	return &clone, true
}

// Remove removes the relay egress entry for the given association ID.
func (t *RelayEgressTable) Remove(associationID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, existed := t.entries[associationID]
	delete(t.entries, associationID)
	return existed
}

// Count returns the number of installed relay egress entries.
func (t *RelayEgressTable) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.entries)
}

// Snapshot returns a sorted copy of all installed relay egress entries.
func (t *RelayEgressTable) Snapshot() []*RelayEgressEntry {
	t.mu.RLock()
	defer t.mu.RUnlock()
	entries := make([]*RelayEgressEntry, 0, len(t.entries))
	for _, entry := range t.entries {
		clone := *entry
		entries = append(entries, &clone)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].AssociationID < entries[j].AssociationID
	})
	return entries
}

// RelayEgressCarrier manages the source node's relay-assisted egress for
// association-bound forwarding through a coordinator relay.
//
// The source node's relay egress path:
//
//	local app → [local ingress] → coordinator relay (RelayAddr)
//
// Relay-assisted egress is explicitly distinct from direct carriage.
// This carrier does not implement delivery (inbound path). For destination-
// side relay delivery, the existing DirectCarrier.StartDelivery suffices
// because delivery behavior is the same regardless of whether packets
// arrived via direct path or coordinator relay.
//
// RelayEgressCarrier does not implement:
//   - scheduler logic or multi-WAN path selection
//   - encrypted carriage or TCP carriage
//   - relay-path scoring or dynamic relay selection
type RelayEgressCarrier struct {
	table    *RelayEgressTable
	mu       sync.Mutex
	handlers map[string]*relayEgressHandle // keyed by association ID
}

// NewRelayEgressCarrier creates a relay egress carrier backed by the given table.
func NewRelayEgressCarrier(table *RelayEgressTable) *RelayEgressCarrier {
	return &RelayEgressCarrier{
		table:    table,
		handlers: make(map[string]*relayEgressHandle),
	}
}

// StartEgress starts the relay egress handler for an association.
//
// It binds a UDP listener on the entry's LocalIngressAddr and forwards
// received packets to the entry's RelayAddr (the coordinator relay's
// per-association listen port). The association must be installed in the
// relay egress table.
func (c *RelayEgressCarrier) StartEgress(ctx context.Context, associationID string) error {
	entry, exists := c.table.Lookup(associationID)
	if !exists {
		return fmt.Errorf("association %q: no installed relay egress entry; relay egress requires valid association context", associationID)
	}
	if entry.LocalIngressAddr == nil {
		return fmt.Errorf("association %q: local ingress address not configured for relay egress", associationID)
	}
	if entry.RelayAddr == nil {
		return fmt.Errorf("association %q: relay address not configured", associationID)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.handlers[associationID]; exists {
		return fmt.Errorf("association %q: relay egress already running", associationID)
	}

	ingressConn, err := net.ListenUDP("udp", entry.LocalIngressAddr)
	if err != nil {
		return fmt.Errorf("association %q: bind local ingress %s: %w", associationID, entry.LocalIngressAddr, err)
	}

	sendConn, err := net.DialUDP("udp", nil, entry.RelayAddr)
	if err != nil {
		ingressConn.Close()
		return fmt.Errorf("association %q: connect to relay %s: %w", associationID, entry.RelayAddr, err)
	}

	childCtx, cancel := context.WithCancel(ctx)
	handle := &relayEgressHandle{
		associationID: associationID,
		ingressConn:   ingressConn,
		sendConn:      sendConn,
		cancel:        cancel,
	}

	c.handlers[associationID] = handle
	go handle.run(childCtx)

	return nil
}

// StopEgress stops the relay egress handler for the given association.
func (c *RelayEgressCarrier) StopEgress(associationID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if handle, exists := c.handlers[associationID]; exists {
		handle.stop()
		delete(c.handlers, associationID)
	}
}

// StopAll stops all running relay egress handlers.
func (c *RelayEgressCarrier) StopAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, handle := range c.handlers {
		handle.stop()
		delete(c.handlers, id)
	}
}

// EgressStats returns packet/byte counters for a relay egress handler.
func (c *RelayEgressCarrier) EgressStats(associationID string) (packets, bytes uint64, running bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	handle, exists := c.handlers[associationID]
	if !exists {
		return 0, 0, false
	}
	return handle.packetsSent.Load(), handle.bytesSent.Load(), true
}

// relayEgressHandle manages the relay egress for one association on the source
// node. It receives packets from the local application on LocalIngressAddr and
// forwards them to the coordinator relay (RelayAddr).
type relayEgressHandle struct {
	associationID string
	ingressConn   *net.UDPConn // receives from local app
	sendConn      *net.UDPConn // sends to coordinator relay
	cancel        context.CancelFunc
	packetsSent   atomic.Uint64
	bytesSent     atomic.Uint64
}

func (h *relayEgressHandle) run(ctx context.Context) {
	buf := make([]byte, maxUDPPayload)

	go func() {
		<-ctx.Done()
		h.ingressConn.Close()
		h.sendConn.Close()
	}()

	for {
		n, _, err := h.ingressConn.ReadFromUDP(buf)
		if err != nil {
			return // closed or context done
		}

		// Zero in-band overhead: forward raw packet bytes unchanged to relay.
		_, err = h.sendConn.Write(buf[:n])
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue // transient send error, keep going
		}

		h.packetsSent.Add(1)
		h.bytesSent.Add(uint64(n))
	}
}

func (h *relayEgressHandle) stop() {
	h.cancel()
}

// --- Builders ---

// BuildRelayForwardingEntry constructs a RelayForwardingEntry for the
// coordinator relay side. It is the bridge between a control-plane association
// record and the coordinator's relay forwarding state.
//
// Parameters:
//   - assoc: control-plane association record
//   - relayListenAddr: the address string the coordinator will bind for this association
//   - destMeshAddr: the destination node's mesh-facing UDP address string
func BuildRelayForwardingEntry(
	assoc service.AssociationRecord,
	relayListenAddr string,
	destMeshAddr string,
) (*RelayForwardingEntry, error) {
	if assoc.AssociationID == "" {
		return nil, fmt.Errorf("association record has no association ID")
	}

	relayAddr, err := net.ResolveUDPAddr("udp", relayListenAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve relay listen address %q: %w", relayListenAddr, err)
	}

	meshAddr, err := net.ResolveUDPAddr("udp", destMeshAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve destination mesh address %q: %w", destMeshAddr, err)
	}

	entry := &RelayForwardingEntry{
		AssociationID:   assoc.AssociationID,
		SourceNode:      assoc.SourceNode,
		SourceService:   assoc.SourceService,
		DestNode:        assoc.DestinationNode,
		DestService:     assoc.DestinationService,
		RelayListenAddr: relayAddr,
		DestMeshAddr:    meshAddr,
	}

	if err := entry.Validate(); err != nil {
		return nil, fmt.Errorf("built relay forwarding entry failed validation: %w", err)
	}

	return entry, nil
}

// BuildRelayEgressEntry constructs a RelayEgressEntry for the source node.
// It is the bridge between a control-plane association record and the source
// node's relay egress forwarding state.
//
// Parameters:
//   - assoc: control-plane association record
//   - localIngressAddr: the source node's local ingress address string
//   - relayAddr: the coordinator relay's per-association listen address string
func BuildRelayEgressEntry(
	assoc service.AssociationRecord,
	localIngressAddr string,
	relayAddr string,
) (*RelayEgressEntry, error) {
	if assoc.AssociationID == "" {
		return nil, fmt.Errorf("association record has no association ID")
	}

	ingressAddr, err := net.ResolveUDPAddr("udp", localIngressAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve local ingress address %q: %w", localIngressAddr, err)
	}

	relayUDPAddr, err := net.ResolveUDPAddr("udp", relayAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve relay address %q: %w", relayAddr, err)
	}

	entry := &RelayEgressEntry{
		AssociationID:    assoc.AssociationID,
		SourceNode:       assoc.SourceNode,
		SourceService:    assoc.SourceService,
		DestNode:         assoc.DestinationNode,
		DestService:      assoc.DestinationService,
		LocalIngressAddr: ingressAddr,
		RelayAddr:        relayUDPAddr,
	}

	if err := entry.Validate(); err != nil {
		return nil, fmt.Errorf("built relay egress entry failed validation: %w", err)
	}

	return entry, nil
}

// --- Status reporting ---

// RelayCarriageStatus describes what the current relay-assisted carriage
// implementation supports and does not support.
type RelayCarriageStatus struct {
	Implemented    []string
	NotImplemented []string
}

// ReportRelayCarriageStatus returns a structured description of what the
// current coordinator relay-assisted carriage implementation covers.
func ReportRelayCarriageStatus() RelayCarriageStatus {
	return RelayCarriageStatus{
		Implemented: []string{
			"coordinator relay-assisted raw UDP carriage: source → coordinator → destination",
			"RelayCarrier (coordinator side): binds per-association relay port, forwards to destination mesh addr",
			"RelayEgressCarrier (source side): binds local ingress, forwards to coordinator relay port",
			"zero in-band overhead: raw UDP packet bytes forwarded unchanged through relay",
			"association-bound relay: relay requires installed RelayForwardingEntry or RelayEgressEntry",
			"single relay hop enforced structurally: RelayForwardingEntry has no next-relay or chain field",
			"direct and relay-assisted carriage are distinct entry types and carriers; they must not be conflated",
			"per-association relay listen port: association identity comes from port, not packet content",
			"destination-side delivery unchanged: DirectCarrier.StartDelivery works for relay-arrived packets",
		},
		NotImplemented: []string{
			"node relay (only coordinator relay is implemented in v1 T-0010)",
			"relay-path scoring, health-based relay selection, or relay failover",
			"scheduler logic (weighted burst/flowlet-aware or per-packet striping)",
			"multi-WAN path selection or aggregation",
			"encrypted carriage or TCP carriage",
			"relay path liveness probing or NAT keep-alive",
			"dynamic relay discovery or coordinator-distributed relay candidates",
			"full production relay hardening",
		},
	}
}
