package dataplane

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
)

// maxUDPPayload is the maximum raw UDP payload we handle. This is generous
// enough for any practical v1 use case including full-MTU WireGuard packets.
const maxUDPPayload = 65535

// DirectCarrier manages direct raw UDP carriage for association-bound
// forwarding entries.
//
// It handles two distinct roles that must remain conceptually separate:
//
//  1. Local ingress (source side): receives UDP from the local application
//     on a Transitloom-provided local ingress port, then forwards the raw
//     packet to the remote peer's direct mesh endpoint.
//
//  2. Local target delivery (destination side): receives UDP from a remote
//     peer on a mesh-facing port, then delivers the raw packet to the local
//     service's target port.
//
// The local ingress port is NOT the same as the local target port:
//   - Local ingress: where the local app sends traffic into the mesh
//   - Local target: where Transitloom delivers inbound traffic to the service
//   - Service binding: the service's own runtime endpoint (maps to local target)
//
// Carriage is association-bound: the DirectCarrier refuses to start carriage
// for an association that is not installed in the ForwardingTable.
//
// Zero in-band overhead: raw UDP packet bytes are forwarded unchanged.
// No Transitloom headers are added or removed.
//
// This carrier is direct-only. It does not implement:
//   - relay forwarding
//   - scheduler logic
//   - multi-WAN path selection
//   - encrypted carriage
//   - TCP carriage
type DirectCarrier struct {
	table *ForwardingTable

	mu       sync.Mutex
	ingress  map[string]*ingressHandle  // by association ID
	delivery map[string]*deliveryHandle // by association ID
}

// NewDirectCarrier creates a direct carrier backed by the given forwarding
// table. The table enforces association-bound carriage: only associations
// installed in the table can be started.
func NewDirectCarrier(table *ForwardingTable) *DirectCarrier {
	return &DirectCarrier{
		table:    table,
		ingress:  make(map[string]*ingressHandle),
		delivery: make(map[string]*deliveryHandle),
	}
}

// StartIngress starts the local ingress listener for an association.
//
// It binds a UDP listener on the entry's LocalIngressAddr and forwards
// received packets to the entry's RemoteAddr. The association must be
// installed in the forwarding table — carriage outside valid association
// context is not allowed.
//
// The local ingress address is where the local application (e.g., WireGuard)
// sends traffic into the mesh. It is NOT the service's local target.
func (c *DirectCarrier) StartIngress(ctx context.Context, associationID string) error {
	entry, exists := c.table.Lookup(associationID)
	if !exists {
		return fmt.Errorf("association %q: no installed forwarding entry; carriage requires valid association context", associationID)
	}
	if entry.LocalIngressAddr == nil {
		return fmt.Errorf("association %q: local ingress address not configured", associationID)
	}
	if entry.RemoteAddr == nil {
		return fmt.Errorf("association %q: remote address not configured for direct carriage", associationID)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.ingress[associationID]; exists {
		return fmt.Errorf("association %q: ingress already running", associationID)
	}

	// Bind the local ingress listener where the local app sends packets.
	ingressConn, err := net.ListenUDP("udp", entry.LocalIngressAddr)
	if err != nil {
		return fmt.Errorf("association %q: bind local ingress %s: %w", associationID, entry.LocalIngressAddr, err)
	}

	// Create a send socket to the remote peer's direct mesh endpoint.
	// Using DialUDP gives a connected socket for efficient sends.
	sendConn, err := net.DialUDP("udp", nil, entry.RemoteAddr)
	if err != nil {
		ingressConn.Close()
		return fmt.Errorf("association %q: connect to remote %s: %w", associationID, entry.RemoteAddr, err)
	}

	childCtx, cancel := context.WithCancel(ctx)
	handle := &ingressHandle{
		associationID: associationID,
		ingressConn:   ingressConn,
		sendConn:      sendConn,
		cancel:        cancel,
	}

	c.ingress[associationID] = handle
	go handle.run(childCtx)

	return nil
}

// StartDelivery starts the mesh-facing delivery handler for an association.
//
// It binds a UDP listener on the entry's MeshAddr and delivers received
// packets to the entry's LocalTargetAddr. The association must be installed
// in the forwarding table.
//
// The local target address is where inbound carried traffic is delivered to
// the local service. It is NOT the local ingress.
func (c *DirectCarrier) StartDelivery(ctx context.Context, associationID string) error {
	entry, exists := c.table.Lookup(associationID)
	if !exists {
		return fmt.Errorf("association %q: no installed forwarding entry; carriage requires valid association context", associationID)
	}
	if entry.MeshAddr == nil {
		return fmt.Errorf("association %q: mesh listen address not configured", associationID)
	}
	if entry.LocalTargetAddr == nil {
		return fmt.Errorf("association %q: local target address not configured for delivery", associationID)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.delivery[associationID]; exists {
		return fmt.Errorf("association %q: delivery already running", associationID)
	}

	// Bind the mesh-facing listener where remote peer packets arrive.
	meshConn, err := net.ListenUDP("udp", entry.MeshAddr)
	if err != nil {
		return fmt.Errorf("association %q: bind mesh listener %s: %w", associationID, entry.MeshAddr, err)
	}

	// Create a delivery socket to the local service target.
	deliverConn, err := net.DialUDP("udp", nil, entry.LocalTargetAddr)
	if err != nil {
		meshConn.Close()
		return fmt.Errorf("association %q: connect to local target %s: %w", associationID, entry.LocalTargetAddr, err)
	}

	childCtx, cancel := context.WithCancel(ctx)
	handle := &deliveryHandle{
		associationID: associationID,
		meshConn:      meshConn,
		deliverConn:   deliverConn,
		cancel:        cancel,
	}

	c.delivery[associationID] = handle
	go handle.run(childCtx)

	return nil
}

// StopIngress stops the ingress listener for the given association.
func (c *DirectCarrier) StopIngress(associationID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if handle, exists := c.ingress[associationID]; exists {
		handle.stop()
		delete(c.ingress, associationID)
	}
}

// StopDelivery stops the delivery handler for the given association.
func (c *DirectCarrier) StopDelivery(associationID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if handle, exists := c.delivery[associationID]; exists {
		handle.stop()
		delete(c.delivery, associationID)
	}
}

// StopAll stops all running ingress listeners and delivery handlers.
func (c *DirectCarrier) StopAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for id, handle := range c.ingress {
		handle.stop()
		delete(c.ingress, id)
	}
	for id, handle := range c.delivery {
		handle.stop()
		delete(c.delivery, id)
	}
}

// IngressStats returns packet/byte counters for an ingress handle.
func (c *DirectCarrier) IngressStats(associationID string) (packets, bytes uint64, running bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	handle, exists := c.ingress[associationID]
	if !exists {
		return 0, 0, false
	}
	return handle.packetsSent.Load(), handle.bytesSent.Load(), true
}

// DeliveryStats returns packet/byte counters for a delivery handle.
func (c *DirectCarrier) DeliveryStats(associationID string) (packets, bytes uint64, running bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	handle, exists := c.delivery[associationID]
	if !exists {
		return 0, 0, false
	}
	return handle.packetsRecv.Load(), handle.bytesRecv.Load(), true
}

// ingressHandle manages the local ingress listener for one association.
// It receives packets from the local application and forwards them to the
// remote peer's direct mesh endpoint.
type ingressHandle struct {
	associationID string
	ingressConn   *net.UDPConn // receives from local app
	sendConn      *net.UDPConn // sends to remote peer
	cancel        context.CancelFunc
	packetsSent   atomic.Uint64
	bytesSent     atomic.Uint64
}

func (h *ingressHandle) run(ctx context.Context) {
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

		// Zero in-band overhead: forward raw packet bytes unchanged.
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

func (h *ingressHandle) stop() {
	h.cancel()
}

// deliveryHandle manages the mesh-facing delivery for one association.
// It receives packets from the remote peer and delivers them to the local
// service's target address.
type deliveryHandle struct {
	associationID string
	meshConn      *net.UDPConn // receives from remote peer
	deliverConn   *net.UDPConn // sends to local target
	cancel        context.CancelFunc
	packetsRecv   atomic.Uint64
	bytesRecv     atomic.Uint64
}

func (h *deliveryHandle) run(ctx context.Context) {
	buf := make([]byte, maxUDPPayload)

	go func() {
		<-ctx.Done()
		h.meshConn.Close()
		h.deliverConn.Close()
	}()

	for {
		n, _, err := h.meshConn.ReadFromUDP(buf)
		if err != nil {
			return // closed or context done
		}

		// Zero in-band overhead: deliver raw packet bytes unchanged.
		_, err = h.deliverConn.Write(buf[:n])
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue // transient delivery error, keep going
		}

		h.packetsRecv.Add(1)
		h.bytesRecv.Add(uint64(n))
	}
}

func (h *deliveryHandle) stop() {
	h.cancel()
}
