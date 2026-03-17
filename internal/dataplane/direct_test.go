package dataplane

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/service"
)

// testIdentity returns a valid service identity for testing.
func testIdentity(name string) service.Identity {
	return service.Identity{Name: name, Type: config.ServiceTypeRawUDP}
}

// testEntry returns a minimal valid ForwardingEntry for testing.
func testEntry(id string) *ForwardingEntry {
	return &ForwardingEntry{
		AssociationID: id,
		SourceNode:    "node-a",
		SourceService: testIdentity("svc-a"),
		DestNode:      "node-b",
		DestService:   testIdentity("svc-b"),
		DirectOnly:    true,
	}
}

// allocLoopbackUDP binds a UDP socket on loopback with an ephemeral port and
// returns the connection and its resolved address.
func allocLoopbackUDP(t *testing.T) (*net.UDPConn, *net.UDPAddr) {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	return conn, conn.LocalAddr().(*net.UDPAddr)
}

// --- ForwardingEntry validation tests ---

func TestForwardingEntry_Validate(t *testing.T) {
	tests := []struct {
		name    string
		entry   *ForwardingEntry
		wantErr bool
	}{
		{
			name:    "valid entry",
			entry:   testEntry("assoc-1"),
			wantErr: false,
		},
		{
			name: "missing association ID",
			entry: &ForwardingEntry{
				SourceNode:    "a",
				SourceService: testIdentity("s"),
				DestNode:      "b",
				DestService:   testIdentity("d"),
				DirectOnly:    true,
			},
			wantErr: true,
		},
		{
			name: "missing source node",
			entry: &ForwardingEntry{
				AssociationID: "x",
				SourceService: testIdentity("s"),
				DestNode:      "b",
				DestService:   testIdentity("d"),
				DirectOnly:    true,
			},
			wantErr: true,
		},
		{
			name: "missing dest node",
			entry: &ForwardingEntry{
				AssociationID: "x",
				SourceNode:    "a",
				SourceService: testIdentity("s"),
				DestService:   testIdentity("d"),
				DirectOnly:    true,
			},
			wantErr: true,
		},
		{
			name: "invalid source service",
			entry: &ForwardingEntry{
				AssociationID: "x",
				SourceNode:    "a",
				SourceService: service.Identity{Name: "", Type: config.ServiceTypeRawUDP},
				DestNode:      "b",
				DestService:   testIdentity("d"),
				DirectOnly:    true,
			},
			wantErr: true,
		},
		{
			name: "invalid dest service",
			entry: &ForwardingEntry{
				AssociationID: "x",
				SourceNode:    "a",
				SourceService: testIdentity("s"),
				DestNode:      "b",
				DestService:   service.Identity{Name: "d", Type: "tcp"},
				DirectOnly:    true,
			},
			wantErr: true,
		},
		{
			name: "direct_only false rejected",
			entry: &ForwardingEntry{
				AssociationID: "x",
				SourceNode:    "a",
				SourceService: testIdentity("s"),
				DestNode:      "b",
				DestService:   testIdentity("d"),
				DirectOnly:    false,
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

// --- ForwardingTable tests ---

func TestForwardingTable_InstallAndLookup(t *testing.T) {
	table := NewForwardingTable()

	entry := testEntry("assoc-1")
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
	if found.SourceNode != "node-a" || found.DestNode != "node-b" {
		t.Fatal("Lookup: wrong node names")
	}
}

func TestForwardingTable_LookupMissing(t *testing.T) {
	table := NewForwardingTable()

	_, exists := table.Lookup("nonexistent")
	if exists {
		t.Fatal("Lookup: expected miss for nonexistent association")
	}
}

func TestForwardingTable_InstallRejectsInvalid(t *testing.T) {
	table := NewForwardingTable()

	err := table.Install(nil)
	if err == nil {
		t.Fatal("Install(nil): expected error")
	}

	err = table.Install(&ForwardingEntry{DirectOnly: true})
	if err == nil {
		t.Fatal("Install(invalid): expected error")
	}

	if table.Count() != 0 {
		t.Fatalf("table should be empty after rejected installs, got %d", table.Count())
	}
}

func TestForwardingTable_Remove(t *testing.T) {
	table := NewForwardingTable()

	entry := testEntry("assoc-1")
	table.Install(entry)

	if !table.Remove("assoc-1") {
		t.Fatal("Remove: expected true for existing entry")
	}
	if table.Remove("assoc-1") {
		t.Fatal("Remove: expected false for already-removed entry")
	}
	if table.Count() != 0 {
		t.Fatalf("Count after remove: want 0, got %d", table.Count())
	}
}

func TestForwardingTable_LookupByIngress(t *testing.T) {
	table := NewForwardingTable()

	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 51821}
	entry := testEntry("assoc-1")
	entry.LocalIngressAddr = addr
	table.Install(entry)

	found, exists := table.LookupByIngress(addr.String())
	if !exists {
		t.Fatal("LookupByIngress: expected entry")
	}
	if found.AssociationID != "assoc-1" {
		t.Fatalf("LookupByIngress: wrong ID %q", found.AssociationID)
	}

	_, exists = table.LookupByIngress("127.0.0.1:99999")
	if exists {
		t.Fatal("LookupByIngress: expected miss for wrong address")
	}
}

func TestForwardingTable_LookupByMeshAddr(t *testing.T) {
	table := NewForwardingTable()

	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 51830}
	entry := testEntry("assoc-1")
	entry.MeshAddr = addr
	table.Install(entry)

	found, exists := table.LookupByMeshAddr(addr.String())
	if !exists {
		t.Fatal("LookupByMeshAddr: expected entry")
	}
	if found.AssociationID != "assoc-1" {
		t.Fatalf("LookupByMeshAddr: wrong ID %q", found.AssociationID)
	}
}

func TestForwardingTable_Snapshot(t *testing.T) {
	table := NewForwardingTable()

	table.Install(testEntry("b-assoc"))
	table.Install(testEntry("a-assoc"))

	snap := table.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("Snapshot: want 2 entries, got %d", len(snap))
	}
	if snap[0].AssociationID != "a-assoc" || snap[1].AssociationID != "b-assoc" {
		t.Fatal("Snapshot: entries not sorted by association ID")
	}
}

// --- DirectCarrier: association-bound enforcement ---

func TestDirectCarrier_IngressRejectsUnknownAssociation(t *testing.T) {
	table := NewForwardingTable()
	carrier := NewDirectCarrier(table)
	defer carrier.StopAll()

	ctx := context.Background()
	err := carrier.StartIngress(ctx, "nonexistent-assoc")
	if err == nil {
		t.Fatal("StartIngress: expected error for unknown association")
	}
}

func TestDirectCarrier_DeliveryRejectsUnknownAssociation(t *testing.T) {
	table := NewForwardingTable()
	carrier := NewDirectCarrier(table)
	defer carrier.StopAll()

	ctx := context.Background()
	err := carrier.StartDelivery(ctx, "nonexistent-assoc")
	if err == nil {
		t.Fatal("StartDelivery: expected error for unknown association")
	}
}

func TestDirectCarrier_IngressRejectsMissingAddresses(t *testing.T) {
	table := NewForwardingTable()
	carrier := NewDirectCarrier(table)
	defer carrier.StopAll()

	// Entry with no ingress or remote address
	entry := testEntry("assoc-no-addrs")
	table.Install(entry)

	ctx := context.Background()
	err := carrier.StartIngress(ctx, "assoc-no-addrs")
	if err == nil {
		t.Fatal("StartIngress: expected error for missing local ingress address")
	}
}

func TestDirectCarrier_DeliveryRejectsMissingAddresses(t *testing.T) {
	table := NewForwardingTable()
	carrier := NewDirectCarrier(table)
	defer carrier.StopAll()

	entry := testEntry("assoc-no-addrs")
	table.Install(entry)

	ctx := context.Background()
	err := carrier.StartDelivery(ctx, "assoc-no-addrs")
	if err == nil {
		t.Fatal("StartDelivery: expected error for missing mesh address")
	}
}

// --- DirectCarrier: end-to-end direct carriage test ---
//
// This test verifies the full direct raw UDP carriage path on loopback:
//
//   local app → [local ingress] → [mesh] → [delivery] → local target
//
// It proves:
//   - source node can send raw UDP into Transitloom via local ingress
//   - destination node delivers UDP to the correct local target
//   - zero in-band overhead: packet bytes arrive unchanged
//   - carriage is association-bound (checked by other tests)
//   - local ingress addr ≠ local target addr (enforced by setup)
func TestDirectCarrier_EndToEndDirectCarriage(t *testing.T) {
	// Set up the "local target" — where the destination service listens.
	// This simulates the service's own listen port (e.g., WireGuard ListenPort).
	targetConn, targetAddr := allocLoopbackUDP(t)
	defer targetConn.Close()

	// Set up the forwarding table and carrier.
	table := NewForwardingTable()
	carrier := NewDirectCarrier(table)
	defer carrier.StopAll()

	// Allocate addresses for ingress and mesh (port 0 = ephemeral).
	ingressAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	meshAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}

	// We need to know the mesh address before starting ingress, so we
	// pre-bind the mesh listener to learn its port, then close it and let
	// the carrier re-bind. Instead, let's start delivery first.
	//
	// Approach: start delivery first to learn the mesh port, then configure
	// the ingress entry to send to that mesh port.

	// Step 1: Create and install the delivery-side entry.
	deliveryEntry := testEntry("test-assoc")
	deliveryEntry.MeshAddr = meshAddr
	deliveryEntry.LocalTargetAddr = targetAddr
	table.Install(deliveryEntry)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := carrier.StartDelivery(ctx, "test-assoc")
	if err != nil {
		t.Fatalf("StartDelivery: %v", err)
	}

	// Learn the actual mesh port that was bound.
	carrier.mu.Lock()
	deliveryHandle := carrier.delivery["test-assoc"]
	actualMeshAddr := deliveryHandle.meshConn.LocalAddr().(*net.UDPAddr)
	carrier.mu.Unlock()

	// Step 2: Remove the old entry and install the ingress-side entry with
	// the actual mesh address as the remote target.
	table.Remove("test-assoc")
	ingressEntry := testEntry("test-assoc")
	ingressEntry.LocalIngressAddr = ingressAddr
	ingressEntry.RemoteAddr = actualMeshAddr
	table.Install(ingressEntry)

	err = carrier.StartIngress(ctx, "test-assoc")
	if err != nil {
		t.Fatalf("StartIngress: %v", err)
	}

	// Learn the actual ingress port that was bound.
	carrier.mu.Lock()
	ingressHandle := carrier.ingress["test-assoc"]
	actualIngressAddr := ingressHandle.ingressConn.LocalAddr().(*net.UDPAddr)
	carrier.mu.Unlock()

	// Verify local ingress and local target are different addresses.
	if actualIngressAddr.String() == targetAddr.String() {
		t.Fatalf("local ingress addr (%s) must differ from local target addr (%s)",
			actualIngressAddr, targetAddr)
	}

	// Step 3: Send a packet from the "local application" to the ingress port.
	payload := []byte("transitloom-direct-carriage-test-payload")
	appConn, err := net.DialUDP("udp", nil, actualIngressAddr)
	if err != nil {
		t.Fatalf("dial ingress: %v", err)
	}
	defer appConn.Close()

	_, err = appConn.Write(payload)
	if err != nil {
		t.Fatalf("send to ingress: %v", err)
	}

	// Step 4: Read the packet from the local target.
	targetConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, maxUDPPayload)
	n, _, err := targetConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read from local target: %v", err)
	}

	// Verify zero in-band overhead: packet bytes arrive unchanged.
	received := buf[:n]
	if string(received) != string(payload) {
		t.Fatalf("packet content mismatch: sent %q, received %q", payload, received)
	}

	// Verify stats.
	packets, bytes, running := carrier.IngressStats("test-assoc")
	if !running {
		t.Fatal("ingress should be running")
	}
	if packets != 1 {
		t.Fatalf("ingress packets: want 1, got %d", packets)
	}
	if bytes != uint64(len(payload)) {
		t.Fatalf("ingress bytes: want %d, got %d", len(payload), bytes)
	}

	dPackets, dBytes, dRunning := carrier.DeliveryStats("test-assoc")
	if !dRunning {
		t.Fatal("delivery should be running")
	}
	if dPackets != 1 {
		t.Fatalf("delivery packets: want 1, got %d", dPackets)
	}
	if dBytes != uint64(len(payload)) {
		t.Fatalf("delivery bytes: want %d, got %d", len(payload), dBytes)
	}
}

// TestDirectCarrier_MultiplePackets sends several packets and verifies all
// arrive at the local target with correct content and stats.
func TestDirectCarrier_MultiplePackets(t *testing.T) {
	targetConn, targetAddr := allocLoopbackUDP(t)
	defer targetConn.Close()

	table := NewForwardingTable()
	carrier := NewDirectCarrier(table)
	defer carrier.StopAll()

	// Start delivery first.
	deliveryEntry := testEntry("multi-assoc")
	deliveryEntry.MeshAddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}
	deliveryEntry.LocalTargetAddr = targetAddr
	table.Install(deliveryEntry)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	carrier.StartDelivery(ctx, "multi-assoc")

	carrier.mu.Lock()
	meshPort := carrier.delivery["multi-assoc"].meshConn.LocalAddr().(*net.UDPAddr)
	carrier.mu.Unlock()

	// Start ingress.
	table.Remove("multi-assoc")
	ingressEntry := testEntry("multi-assoc")
	ingressEntry.LocalIngressAddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}
	ingressEntry.RemoteAddr = meshPort
	table.Install(ingressEntry)

	carrier.StartIngress(ctx, "multi-assoc")

	carrier.mu.Lock()
	actualIngress := carrier.ingress["multi-assoc"].ingressConn.LocalAddr().(*net.UDPAddr)
	carrier.mu.Unlock()

	appConn, _ := net.DialUDP("udp", nil, actualIngress)
	defer appConn.Close()

	const numPackets = 10
	for i := 0; i < numPackets; i++ {
		appConn.Write([]byte("pkt"))
	}

	targetConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, maxUDPPayload)
	received := 0
	for received < numPackets {
		n, _, err := targetConn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		if string(buf[:n]) != "pkt" {
			t.Fatalf("packet %d: content mismatch", received)
		}
		received++
	}

	if received != numPackets {
		t.Fatalf("received %d packets, want %d", received, numPackets)
	}
}

// TestDirectCarrier_StopIngress verifies that stopping ingress stops forwarding.
func TestDirectCarrier_StopIngress(t *testing.T) {
	table := NewForwardingTable()
	carrier := NewDirectCarrier(table)
	defer carrier.StopAll()

	entry := testEntry("stop-assoc")
	entry.LocalIngressAddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}
	entry.RemoteAddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 19999}
	table.Install(entry)

	ctx := context.Background()
	carrier.StartIngress(ctx, "stop-assoc")

	_, _, running := carrier.IngressStats("stop-assoc")
	if !running {
		t.Fatal("ingress should be running")
	}

	carrier.StopIngress("stop-assoc")

	_, _, running = carrier.IngressStats("stop-assoc")
	if running {
		t.Fatal("ingress should be stopped")
	}
}

// --- BuildDirectForwardingEntry tests ---

func TestBuildDirectForwardingEntry(t *testing.T) {
	assoc := service.AssociationRecord{
		AssociationID:      "node-a/svc-a:raw-udp->node-b/svc-b:raw-udp",
		SourceNode:         "node-a",
		SourceService:      testIdentity("svc-a"),
		DestinationNode:    "node-b",
		DestinationService: testIdentity("svc-b"),
		State:              service.AssociationStatePending,
		BootstrapOnly:      true,
	}

	sourceRecord := service.Record{
		NodeName: "node-a",
		Identity: testIdentity("svc-a"),
		Binding: service.Binding{
			LocalTarget: service.LocalTarget{Address: "127.0.0.1", Port: 51820},
		},
	}

	destRecord := service.Record{
		NodeName: "node-b",
		Identity: testIdentity("svc-b"),
		Binding: service.Binding{
			LocalTarget: service.LocalTarget{Address: "127.0.0.1", Port: 51820},
		},
	}

	entry, err := BuildDirectForwardingEntry(
		assoc, sourceRecord, destRecord,
		"192.0.2.1:51830",
		"127.0.0.1:51821",
		"0.0.0.0:51830",
	)
	if err != nil {
		t.Fatalf("BuildDirectForwardingEntry: %v", err)
	}

	if entry.AssociationID != assoc.AssociationID {
		t.Fatalf("association ID: %q", entry.AssociationID)
	}
	if !entry.DirectOnly {
		t.Fatal("expected DirectOnly=true")
	}
	if entry.LocalIngressAddr.String() != "127.0.0.1:51821" {
		t.Fatalf("ingress addr: %s", entry.LocalIngressAddr)
	}
	if entry.RemoteAddr.String() != "192.0.2.1:51830" {
		t.Fatalf("remote addr: %s", entry.RemoteAddr)
	}
	if entry.LocalTargetAddr.Port != 51820 {
		t.Fatalf("target port: %d", entry.LocalTargetAddr.Port)
	}
	if entry.MeshAddr.String() != "0.0.0.0:51830" {
		t.Fatalf("mesh addr: %s", entry.MeshAddr)
	}

	// Verify it's installable.
	table := NewForwardingTable()
	if err := table.Install(entry); err != nil {
		t.Fatalf("Install built entry: %v", err)
	}
}

func TestBuildDirectForwardingEntry_InvalidEndpoint(t *testing.T) {
	assoc := service.AssociationRecord{
		AssociationID:      "x",
		SourceNode:         "a",
		SourceService:      testIdentity("s"),
		DestinationNode:    "b",
		DestinationService: testIdentity("d"),
	}
	src := service.Record{
		NodeName: "a",
		Identity: testIdentity("s"),
		Binding:  service.Binding{LocalTarget: service.LocalTarget{Address: "127.0.0.1", Port: 1}},
	}
	dst := service.Record{
		NodeName: "b",
		Identity: testIdentity("d"),
		Binding:  service.Binding{LocalTarget: service.LocalTarget{Address: "127.0.0.1", Port: 2}},
	}

	_, err := BuildDirectForwardingEntry(assoc, src, dst, "not-a-valid-endpoint", "127.0.0.1:1", "0.0.0.0:1")
	if err == nil {
		t.Fatal("expected error for invalid direct endpoint")
	}
}

// --- ReportDirectCarriageStatus test ---

func TestReportDirectCarriageStatus(t *testing.T) {
	status := ReportDirectCarriageStatus()
	if len(status.Implemented) == 0 {
		t.Fatal("expected implemented items")
	}
	if len(status.NotImplemented) == 0 {
		t.Fatal("expected not-implemented items")
	}
}

// --- Verify local ingress vs local target distinction ---
//
// This test explicitly verifies that the runtime path keeps local ingress
// and local target addresses separate, which is a core architectural
// requirement.
func TestLocalIngressVsLocalTarget_Distinction(t *testing.T) {
	entry := testEntry("distinction-test")
	ingressAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 51821}
	targetAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 51820}

	entry.LocalIngressAddr = ingressAddr
	entry.LocalTargetAddr = targetAddr

	if entry.LocalIngressAddr.String() == entry.LocalTargetAddr.String() {
		t.Fatal("local ingress and local target must be different addresses")
	}

	// Verify that ForwardingTable correctly stores both separately.
	entry.RemoteAddr = &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 51830}
	entry.MeshAddr = &net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 51830}

	table := NewForwardingTable()
	table.Install(entry)

	found, _ := table.Lookup("distinction-test")
	if found.LocalIngressAddr.String() == found.LocalTargetAddr.String() {
		t.Fatal("after install+lookup, ingress and target must still differ")
	}
	if found.LocalIngressAddr.Port != 51821 {
		t.Fatalf("ingress port: want 51821, got %d", found.LocalIngressAddr.Port)
	}
	if found.LocalTargetAddr.Port != 51820 {
		t.Fatalf("target port: want 51820, got %d", found.LocalTargetAddr.Port)
	}
}
