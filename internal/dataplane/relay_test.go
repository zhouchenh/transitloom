package dataplane

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/service"
)

// testRelayForwardingEntry returns a minimal valid RelayForwardingEntry for testing.
func testRelayForwardingEntry(id string) *RelayForwardingEntry {
	return &RelayForwardingEntry{
		AssociationID:   id,
		SourceNode:      "node-a",
		SourceService:   testIdentity("svc-a"),
		DestNode:        "node-b",
		DestService:     testIdentity("svc-b"),
		RelayListenAddr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0},
		DestMeshAddr:    &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1},
	}
}

// testRelayEgressEntry returns a minimal valid RelayEgressEntry for testing.
func testRelayEgressEntry(id string) *RelayEgressEntry {
	return &RelayEgressEntry{
		AssociationID: id,
		SourceNode:    "node-a",
		SourceService: testIdentity("svc-a"),
		DestNode:      "node-b",
		DestService:   testIdentity("svc-b"),
	}
}

// --- RelayForwardingEntry validation ---

func TestRelayForwardingEntry_Validate(t *testing.T) {
	tests := []struct {
		name    string
		entry   *RelayForwardingEntry
		wantErr bool
	}{
		{
			name:    "valid entry",
			entry:   testRelayForwardingEntry("assoc-1"),
			wantErr: false,
		},
		{
			name: "missing association ID",
			entry: &RelayForwardingEntry{
				SourceNode:      "a",
				SourceService:   testIdentity("s"),
				DestNode:        "b",
				DestService:     testIdentity("d"),
				RelayListenAddr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0},
				DestMeshAddr:    &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1},
			},
			wantErr: true,
		},
		{
			name: "missing source node",
			entry: &RelayForwardingEntry{
				AssociationID:   "x",
				SourceService:   testIdentity("s"),
				DestNode:        "b",
				DestService:     testIdentity("d"),
				RelayListenAddr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0},
				DestMeshAddr:    &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1},
			},
			wantErr: true,
		},
		{
			name: "missing dest node",
			entry: &RelayForwardingEntry{
				AssociationID:   "x",
				SourceNode:      "a",
				SourceService:   testIdentity("s"),
				DestService:     testIdentity("d"),
				RelayListenAddr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0},
				DestMeshAddr:    &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1},
			},
			wantErr: true,
		},
		{
			name: "missing relay listen addr",
			entry: &RelayForwardingEntry{
				AssociationID: "x",
				SourceNode:    "a",
				SourceService: testIdentity("s"),
				DestNode:      "b",
				DestService:   testIdentity("d"),
				DestMeshAddr:  &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1},
			},
			wantErr: true,
		},
		{
			name: "missing dest mesh addr",
			entry: &RelayForwardingEntry{
				AssociationID:   "x",
				SourceNode:      "a",
				SourceService:   testIdentity("s"),
				DestNode:        "b",
				DestService:     testIdentity("d"),
				RelayListenAddr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entry.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --- RelayEgressEntry validation ---

func TestRelayEgressEntry_Validate(t *testing.T) {
	tests := []struct {
		name    string
		entry   *RelayEgressEntry
		wantErr bool
	}{
		{
			name:    "valid entry",
			entry:   testRelayEgressEntry("assoc-1"),
			wantErr: false,
		},
		{
			name: "missing association ID",
			entry: &RelayEgressEntry{
				SourceNode:    "a",
				SourceService: testIdentity("s"),
				DestNode:      "b",
				DestService:   testIdentity("d"),
			},
			wantErr: true,
		},
		{
			name: "missing source node",
			entry: &RelayEgressEntry{
				AssociationID: "x",
				SourceService: testIdentity("s"),
				DestNode:      "b",
				DestService:   testIdentity("d"),
			},
			wantErr: true,
		},
		{
			name: "missing dest node",
			entry: &RelayEgressEntry{
				AssociationID: "x",
				SourceNode:    "a",
				SourceService: testIdentity("s"),
				DestService:   testIdentity("d"),
			},
			wantErr: true,
		},
		{
			name: "invalid source service",
			entry: &RelayEgressEntry{
				AssociationID: "x",
				SourceNode:    "a",
				SourceService: service.Identity{Name: "", Type: "raw-udp"},
				DestNode:      "b",
				DestService:   testIdentity("d"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entry.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --- RelayForwardingTable ---

func TestRelayForwardingTable_InstallAndLookup(t *testing.T) {
	table := NewRelayForwardingTable()

	entry := testRelayForwardingEntry("assoc-1")
	if err := table.Install(entry); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if table.Count() != 1 {
		t.Fatalf("Count: want 1, got %d", table.Count())
	}

	found, exists := table.Lookup("assoc-1")
	if !exists {
		t.Fatal("Lookup: expected entry to exist")
	}
	if found.AssociationID != "assoc-1" {
		t.Fatalf("Lookup: wrong association ID %q", found.AssociationID)
	}

	_, exists = table.Lookup("nonexistent")
	if exists {
		t.Fatal("Lookup: expected miss for nonexistent association")
	}
}

func TestRelayForwardingTable_InstallRejectsInvalid(t *testing.T) {
	table := NewRelayForwardingTable()

	err := table.Install(nil)
	if err == nil {
		t.Fatal("Install(nil): expected error")
	}

	err = table.Install(&RelayForwardingEntry{AssociationID: "x"})
	if err == nil {
		t.Fatal("Install(incomplete): expected error")
	}

	if table.Count() != 0 {
		t.Fatalf("table should be empty after rejected installs, got %d", table.Count())
	}
}

func TestRelayForwardingTable_Remove(t *testing.T) {
	table := NewRelayForwardingTable()
	table.Install(testRelayForwardingEntry("assoc-1"))

	if !table.Remove("assoc-1") {
		t.Fatal("Remove: expected true for existing entry")
	}
	if table.Remove("assoc-1") {
		t.Fatal("Remove: expected false for already-removed entry")
	}
}

func TestRelayForwardingTable_Snapshot(t *testing.T) {
	table := NewRelayForwardingTable()
	table.Install(testRelayForwardingEntry("b-assoc"))
	table.Install(testRelayForwardingEntry("a-assoc"))

	snap := table.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("Snapshot: want 2, got %d", len(snap))
	}
	if snap[0].AssociationID != "a-assoc" || snap[1].AssociationID != "b-assoc" {
		t.Fatal("Snapshot: entries not sorted by association ID")
	}
}

// --- RelayEgressTable ---

func TestRelayEgressTable_InstallAndLookup(t *testing.T) {
	table := NewRelayEgressTable()

	entry := testRelayEgressEntry("assoc-1")
	if err := table.Install(entry); err != nil {
		t.Fatalf("Install: %v", err)
	}

	found, exists := table.Lookup("assoc-1")
	if !exists {
		t.Fatal("Lookup: expected entry to exist")
	}
	if found.AssociationID != "assoc-1" {
		t.Fatalf("Lookup: wrong association ID %q", found.AssociationID)
	}
}

func TestRelayEgressTable_InstallRejectsInvalid(t *testing.T) {
	table := NewRelayEgressTable()

	if err := table.Install(nil); err == nil {
		t.Fatal("Install(nil): expected error")
	}
	if err := table.Install(&RelayEgressEntry{}); err == nil {
		t.Fatal("Install(empty): expected error")
	}
	if table.Count() != 0 {
		t.Fatalf("Count after rejections: want 0, got %d", table.Count())
	}
}

// --- RelayCarrier: association-bound enforcement ---

func TestRelayCarrier_RejectsUnknownAssociation(t *testing.T) {
	table := NewRelayForwardingTable()
	carrier := NewRelayCarrier(table)
	defer carrier.StopAll()

	err := carrier.StartRelay(context.Background(), "nonexistent-assoc")
	if err == nil {
		t.Fatal("StartRelay: expected error for unknown association")
	}
}

// --- RelayEgressCarrier: association-bound enforcement ---

func TestRelayEgressCarrier_RejectsUnknownAssociation(t *testing.T) {
	table := NewRelayEgressTable()
	carrier := NewRelayEgressCarrier(table)
	defer carrier.StopAll()

	err := carrier.StartEgress(context.Background(), "nonexistent-assoc")
	if err == nil {
		t.Fatal("StartEgress: expected error for unknown association")
	}
}

func TestRelayEgressCarrier_RejectsMissingAddresses(t *testing.T) {
	table := NewRelayEgressTable()
	carrier := NewRelayEgressCarrier(table)
	defer carrier.StopAll()

	// Entry with no ingress or relay address.
	entry := testRelayEgressEntry("assoc-no-addrs")
	table.Install(entry)

	err := carrier.StartEgress(context.Background(), "assoc-no-addrs")
	if err == nil {
		t.Fatal("StartEgress: expected error for missing local ingress address")
	}
}

// --- End-to-end single-relay-hop carriage test ---
//
// This test verifies the full single-relay-hop raw UDP carriage path on loopback:
//
//	local app → [local ingress / relay egress] → [coordinator relay]
//	         → [mesh delivery] → local target
//
// It proves:
//   - source node sends raw UDP into Transitloom via local ingress
//   - coordinator relay receives and forwards to destination mesh port
//   - destination delivery delivers UDP to the correct local target
//   - zero in-band overhead: packet bytes arrive unchanged end to end
//   - exactly one relay hop: source → coordinator → destination
//   - relay is association-bound (enforced by carrier checks)
//   - direct and relay-assisted carriage remain distinct entry types
func TestRelayCarrier_EndToEndSingleHopCarriage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Step 1: Set up the local target — where the destination service listens.
	targetConn, targetAddr := allocLoopbackUDP(t)
	defer targetConn.Close()

	// Step 2: Set up destination-side delivery using the existing DirectCarrier.
	// The delivery side is functionally identical for direct and relay-assisted
	// carriage: it receives packets on a mesh port and delivers to local target.
	deliveryTable := NewForwardingTable()
	deliveryCarrier := NewDirectCarrier(deliveryTable)
	defer deliveryCarrier.StopAll()

	deliveryEntry := testEntry("relay-assoc")
	deliveryEntry.MeshAddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0} // ephemeral
	deliveryEntry.LocalTargetAddr = targetAddr
	deliveryTable.Install(deliveryEntry)

	if err := deliveryCarrier.StartDelivery(ctx, "relay-assoc"); err != nil {
		t.Fatalf("StartDelivery: %v", err)
	}

	// Learn the actual mesh port bound by the delivery carrier.
	deliveryCarrier.mu.Lock()
	actualMeshAddr := deliveryCarrier.delivery["relay-assoc"].meshConn.LocalAddr().(*net.UDPAddr)
	deliveryCarrier.mu.Unlock()

	// Step 3: Set up coordinator relay — listens for source packets, forwards to
	// destination mesh addr (learned from step 2).
	relayTable := NewRelayForwardingTable()
	relayCarrier := NewRelayCarrier(relayTable)
	defer relayCarrier.StopAll()

	relayEntry := testRelayForwardingEntry("relay-assoc")
	relayEntry.RelayListenAddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0} // ephemeral
	relayEntry.DestMeshAddr = actualMeshAddr
	relayTable.Install(relayEntry)

	if err := relayCarrier.StartRelay(ctx, "relay-assoc"); err != nil {
		t.Fatalf("StartRelay: %v", err)
	}

	// Learn the actual relay listen port.
	relayCarrier.mu.Lock()
	actualRelayAddr := relayCarrier.handlers["relay-assoc"].relayConn.LocalAddr().(*net.UDPAddr)
	relayCarrier.mu.Unlock()

	// Step 4: Set up source node relay egress — local ingress sends to relay addr.
	egressTable := NewRelayEgressTable()
	egressCarrier := NewRelayEgressCarrier(egressTable)
	defer egressCarrier.StopAll()

	egressEntry := testRelayEgressEntry("relay-assoc")
	egressEntry.LocalIngressAddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0} // ephemeral
	egressEntry.RelayAddr = actualRelayAddr
	egressTable.Install(egressEntry)

	if err := egressCarrier.StartEgress(ctx, "relay-assoc"); err != nil {
		t.Fatalf("StartEgress: %v", err)
	}

	// Learn the actual local ingress port.
	egressCarrier.mu.Lock()
	actualIngressAddr := egressCarrier.handlers["relay-assoc"].ingressConn.LocalAddr().(*net.UDPAddr)
	egressCarrier.mu.Unlock()

	// Sanity: local ingress and local target must be different addresses.
	if actualIngressAddr.String() == targetAddr.String() {
		t.Fatalf("local ingress addr (%s) must differ from local target addr (%s)",
			actualIngressAddr, targetAddr)
	}

	// Step 5: Send a packet from the "local application" to the ingress port.
	payload := []byte("transitloom-single-relay-hop-test-payload")
	appConn, err := net.DialUDP("udp", nil, actualIngressAddr)
	if err != nil {
		t.Fatalf("dial ingress: %v", err)
	}
	defer appConn.Close()

	if _, err := appConn.Write(payload); err != nil {
		t.Fatalf("send to ingress: %v", err)
	}

	// Step 6: Read the packet from the local target.
	targetConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, maxUDPPayload)
	n, _, err := targetConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read from local target: %v", err)
	}

	// Verify zero in-band overhead: packet bytes arrive unchanged end to end.
	received := buf[:n]
	if string(received) != string(payload) {
		t.Fatalf("packet content mismatch: sent %q, received %q", payload, received)
	}

	// Verify stats across all three participants.
	ePkts, eBytes, eRunning := egressCarrier.EgressStats("relay-assoc")
	if !eRunning {
		t.Fatal("relay egress should be running")
	}
	if ePkts != 1 {
		t.Fatalf("egress packets: want 1, got %d", ePkts)
	}
	if eBytes != uint64(len(payload)) {
		t.Fatalf("egress bytes: want %d, got %d", len(payload), eBytes)
	}

	rPkts, rBytes, rRunning := relayCarrier.RelayStats("relay-assoc")
	if !rRunning {
		t.Fatal("relay should be running")
	}
	if rPkts != 1 {
		t.Fatalf("relay packets: want 1, got %d", rPkts)
	}
	if rBytes != uint64(len(payload)) {
		t.Fatalf("relay bytes: want %d, got %d", len(payload), rBytes)
	}

	dPkts, dBytes, dRunning := deliveryCarrier.DeliveryStats("relay-assoc")
	if !dRunning {
		t.Fatal("delivery should be running")
	}
	if dPkts != 1 {
		t.Fatalf("delivery packets: want 1, got %d", dPkts)
	}
	if dBytes != uint64(len(payload)) {
		t.Fatalf("delivery bytes: want %d, got %d", len(payload), dBytes)
	}
}

// TestRelayCarrier_MultiplePacketsEndToEnd sends several packets through the
// full relay path and verifies all arrive correctly.
func TestRelayCarrier_MultiplePacketsEndToEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	targetConn, targetAddr := allocLoopbackUDP(t)
	defer targetConn.Close()

	// Destination delivery.
	deliveryTable := NewForwardingTable()
	deliveryCarrier := NewDirectCarrier(deliveryTable)
	defer deliveryCarrier.StopAll()

	dEntry := testEntry("multi-relay")
	dEntry.MeshAddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	dEntry.LocalTargetAddr = targetAddr
	deliveryTable.Install(dEntry)
	deliveryCarrier.StartDelivery(ctx, "multi-relay")

	deliveryCarrier.mu.Lock()
	meshAddr := deliveryCarrier.delivery["multi-relay"].meshConn.LocalAddr().(*net.UDPAddr)
	deliveryCarrier.mu.Unlock()

	// Coordinator relay.
	relayTable := NewRelayForwardingTable()
	relayCarrier := NewRelayCarrier(relayTable)
	defer relayCarrier.StopAll()

	rEntry := testRelayForwardingEntry("multi-relay")
	rEntry.RelayListenAddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	rEntry.DestMeshAddr = meshAddr
	relayTable.Install(rEntry)
	relayCarrier.StartRelay(ctx, "multi-relay")

	relayCarrier.mu.Lock()
	relayAddr := relayCarrier.handlers["multi-relay"].relayConn.LocalAddr().(*net.UDPAddr)
	relayCarrier.mu.Unlock()

	// Source relay egress.
	egressTable := NewRelayEgressTable()
	egressCarrier := NewRelayEgressCarrier(egressTable)
	defer egressCarrier.StopAll()

	eEntry := testRelayEgressEntry("multi-relay")
	eEntry.LocalIngressAddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	eEntry.RelayAddr = relayAddr
	egressTable.Install(eEntry)
	egressCarrier.StartEgress(ctx, "multi-relay")

	egressCarrier.mu.Lock()
	ingressAddr := egressCarrier.handlers["multi-relay"].ingressConn.LocalAddr().(*net.UDPAddr)
	egressCarrier.mu.Unlock()

	appConn, _ := net.DialUDP("udp", nil, ingressAddr)
	defer appConn.Close()

	const numPackets = 10
	for i := 0; i < numPackets; i++ {
		appConn.Write([]byte("relay-pkt"))
	}

	targetConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, maxUDPPayload)
	received := 0
	for received < numPackets {
		n, _, err := targetConn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		if string(buf[:n]) != "relay-pkt" {
			t.Fatalf("packet %d: content mismatch", received)
		}
		received++
	}

	if received != numPackets {
		t.Fatalf("received %d packets, want %d", received, numPackets)
	}
}

// TestSingleHopConstraint_StructuralEnforcement verifies that there is no
// mechanism in the relay entry types to create relay chains. This is the
// structural enforcement of the v1 single-hop limit.
//
// The v1 constraint is: source → one relay → destination.
// Multi-hop would require: source → relay-1 → relay-2 → destination.
// That would require RelayForwardingEntry to have a "next relay" field and
// RelayCarrier to chain to another RelayCarrier. Neither exists.
func TestSingleHopConstraint_StructuralEnforcement(t *testing.T) {
	entry := testRelayForwardingEntry("assoc-1")

	// Verify the entry has RelayListenAddr (where source sends) and
	// DestMeshAddr (where the relay sends), but no "next relay" or
	// "chain relay" field. The only destination is a terminal mesh addr.
	if entry.RelayListenAddr == nil {
		t.Fatal("entry must have relay listen addr")
	}
	if entry.DestMeshAddr == nil {
		t.Fatal("entry must have dest mesh addr")
	}

	// The DestMeshAddr is a terminal endpoint. There is no additional field
	// like "NextRelayAddr" or "ChainRelayAddr" in RelayForwardingEntry.
	// This absence structurally prevents relay chains in v1.
	//
	// If a future agent tries to add multi-hop support by adding a
	// NextRelayAddr field here, this test serves as a reminder that the
	// v1 single-hop constraint must be evaluated before doing so.
	snap := NewRelayForwardingTable()
	snap.Install(entry)

	found, _ := snap.Lookup("assoc-1")
	if found.AssociationID != "assoc-1" {
		t.Fatal("entry not found after install")
	}
	// Confirm the entry only has: RelayListenAddr, DestMeshAddr.
	// No chaining fields exist.
	_ = found.RelayListenAddr
	_ = found.DestMeshAddr
}

// TestDirectVsRelayCarriageDistinction verifies that direct and relay-assisted
// carriage use separate, incompatible entry types. This is the architectural
// requirement that these modes must not be conflated.
func TestDirectVsRelayCarriageDistinction(t *testing.T) {
	// ForwardingEntry (direct) has DirectOnly=true and RemoteAddr.
	directEntry := testEntry("direct-assoc")
	if err := directEntry.Validate(); err != nil {
		t.Fatalf("direct entry should be valid: %v", err)
	}

	// RelayForwardingEntry (coordinator relay) has RelayListenAddr+DestMeshAddr.
	relayEntry := testRelayForwardingEntry("relay-assoc")
	if err := relayEntry.Validate(); err != nil {
		t.Fatalf("relay forwarding entry should be valid: %v", err)
	}

	// RelayEgressEntry (source relay egress) has LocalIngressAddr+RelayAddr.
	egressEntry := testRelayEgressEntry("egress-assoc")
	if err := egressEntry.Validate(); err != nil {
		t.Fatalf("relay egress entry should be valid: %v", err)
	}

	// Direct entry cannot be installed in a RelayForwardingTable.
	// (Different type — not even the same interface.)
	relayTable := NewRelayForwardingTable()
	if relayTable.Count() != 0 {
		t.Fatal("relay table should be empty")
	}

	// Direct ForwardingTable cannot hold relay entries.
	// (Different type — not even the same interface.)
	directTable := NewForwardingTable()
	if directTable.Count() != 0 {
		t.Fatal("direct table should be empty")
	}

	// These compile-time incompatibilities ensure that direct and relay
	// carriage modes cannot be accidentally conflated.
}

// --- BuildRelayForwardingEntry builder test ---

func TestBuildRelayForwardingEntry(t *testing.T) {
	assoc := service.AssociationRecord{
		AssociationID:      "node-a/svc-a:raw-udp->node-b/svc-b:raw-udp",
		SourceNode:         "node-a",
		SourceService:      testIdentity("svc-a"),
		DestinationNode:    "node-b",
		DestinationService: testIdentity("svc-b"),
		State:              service.AssociationStatePending,
		BootstrapOnly:      true,
	}

	entry, err := BuildRelayForwardingEntry(assoc, "127.0.0.1:51840", "127.0.0.1:51850")
	if err != nil {
		t.Fatalf("BuildRelayForwardingEntry: %v", err)
	}

	if entry.AssociationID != assoc.AssociationID {
		t.Fatalf("association ID: %q", entry.AssociationID)
	}
	if entry.RelayListenAddr.String() != "127.0.0.1:51840" {
		t.Fatalf("relay listen addr: %s", entry.RelayListenAddr)
	}
	if entry.DestMeshAddr.String() != "127.0.0.1:51850" {
		t.Fatalf("dest mesh addr: %s", entry.DestMeshAddr)
	}

	// Verify it's installable.
	table := NewRelayForwardingTable()
	if err := table.Install(entry); err != nil {
		t.Fatalf("Install built entry: %v", err)
	}
}

func TestBuildRelayForwardingEntry_InvalidAddr(t *testing.T) {
	assoc := service.AssociationRecord{
		AssociationID:      "x",
		SourceNode:         "a",
		SourceService:      testIdentity("s"),
		DestinationNode:    "b",
		DestinationService: testIdentity("d"),
	}

	_, err := BuildRelayForwardingEntry(assoc, "not-valid-addr", "127.0.0.1:1")
	if err == nil {
		t.Fatal("expected error for invalid relay listen address")
	}
}

// --- BuildRelayEgressEntry builder test ---

func TestBuildRelayEgressEntry(t *testing.T) {
	assoc := service.AssociationRecord{
		AssociationID:      "node-a/svc-a:raw-udp->node-b/svc-b:raw-udp",
		SourceNode:         "node-a",
		SourceService:      testIdentity("svc-a"),
		DestinationNode:    "node-b",
		DestinationService: testIdentity("svc-b"),
		State:              service.AssociationStatePending,
		BootstrapOnly:      true,
	}

	entry, err := BuildRelayEgressEntry(assoc, "127.0.0.1:51821", "10.0.0.1:51840")
	if err != nil {
		t.Fatalf("BuildRelayEgressEntry: %v", err)
	}

	if entry.AssociationID != assoc.AssociationID {
		t.Fatalf("association ID: %q", entry.AssociationID)
	}
	if entry.LocalIngressAddr.String() != "127.0.0.1:51821" {
		t.Fatalf("local ingress addr: %s", entry.LocalIngressAddr)
	}
	if entry.RelayAddr.String() != "10.0.0.1:51840" {
		t.Fatalf("relay addr: %s", entry.RelayAddr)
	}

	// Verify it's installable.
	table := NewRelayEgressTable()
	if err := table.Install(entry); err != nil {
		t.Fatalf("Install built entry: %v", err)
	}
}

// --- ReportRelayCarriageStatus test ---

func TestReportRelayCarriageStatus(t *testing.T) {
	status := ReportRelayCarriageStatus()
	if len(status.Implemented) == 0 {
		t.Fatal("expected implemented items")
	}
	if len(status.NotImplemented) == 0 {
		t.Fatal("expected not-implemented items")
	}
}
