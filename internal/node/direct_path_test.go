package node

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/service"
)

// --- Helper for allocating a loopback UDP socket ---

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

// --- Test: resolve local ingress address from config ---

func TestResolveLocalIngressAddr(t *testing.T) {
	tests := []struct {
		name        string
		cfg         config.NodeConfig
		serviceName string
		wantAddr    string
		wantErr     bool
	}{
		{
			name: "static ingress port",
			cfg: config.NodeConfig{
				Services: []config.ServiceConfig{
					{
						Name: "wg0",
						Type: config.ServiceTypeRawUDP,
						Binding: config.ServiceBindingConfig{
							Address: "127.0.0.1",
							Port:    51820,
						},
						Ingress: &config.ServiceIngressConfig{
							Mode:       config.IngressModeStatic,
							StaticPort: 41002,
						},
					},
				},
			},
			serviceName: "wg0",
			wantAddr:    "127.0.0.1:41002",
		},
		{
			name: "static ingress with custom loopback",
			cfg: config.NodeConfig{
				Services: []config.ServiceConfig{
					{
						Name: "wg0",
						Type: config.ServiceTypeRawUDP,
						Binding: config.ServiceBindingConfig{
							Address: "127.0.0.1",
							Port:    51820,
						},
						Ingress: &config.ServiceIngressConfig{
							Mode:            config.IngressModeStatic,
							StaticPort:      41002,
							LoopbackAddress: "127.0.0.2",
						},
					},
				},
			},
			serviceName: "wg0",
			wantAddr:    "127.0.0.2:41002",
		},
		{
			name: "ephemeral when no ingress config",
			cfg: config.NodeConfig{
				Services: []config.ServiceConfig{
					{
						Name: "wg0",
						Type: config.ServiceTypeRawUDP,
						Binding: config.ServiceBindingConfig{
							Address: "127.0.0.1",
							Port:    51820,
						},
					},
				},
			},
			serviceName: "wg0",
			wantAddr:    "127.0.0.1:0",
		},
		{
			name: "service not found",
			cfg: config.NodeConfig{
				Services: []config.ServiceConfig{},
			},
			serviceName: "nonexistent",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := resolveLocalIngressAddr(tt.cfg, tt.serviceName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveLocalIngressAddr() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && addr != tt.wantAddr {
				t.Fatalf("resolveLocalIngressAddr() = %q, want %q", addr, tt.wantAddr)
			}
		})
	}
}

// --- Test: resolve local target address from config ---

func TestResolveLocalTargetAddr(t *testing.T) {
	cfg := config.NodeConfig{
		Services: []config.ServiceConfig{
			{
				Name: "wg0",
				Type: config.ServiceTypeRawUDP,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    51820,
				},
			},
		},
	}

	addr, err := resolveLocalTargetAddr(cfg, "wg0")
	if err != nil {
		t.Fatalf("resolveLocalTargetAddr: %v", err)
	}
	if addr != "127.0.0.1:51820" {
		t.Fatalf("resolveLocalTargetAddr = %q, want %q", addr, "127.0.0.1:51820")
	}

	_, err = resolveLocalTargetAddr(cfg, "nonexistent")
	if err == nil {
		t.Fatal("resolveLocalTargetAddr: expected error for nonexistent service")
	}
}

// --- Test: build activation inputs from config and association results ---

func TestBuildAssociationActivationInputs(t *testing.T) {
	cfg := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-a"},
		Services: []config.ServiceConfig{
			{
				Name: "wg0",
				Type: config.ServiceTypeRawUDP,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    51820,
				},
			},
		},
		Associations: []config.AssociationConfig{
			{
				SourceService:      "wg0",
				DestinationNode:    "node-b",
				DestinationService: "wg0",
				DirectEndpoint:     "192.0.2.1:51830",
				MeshListenPort:     51830,
			},
			{
				SourceService:      "wg0",
				DestinationNode:    "node-c",
				DestinationService: "wg0",
				// No direct_endpoint — should be skipped
			},
		},
	}

	results := []AssociationResultEntry{
		{
			AssociationID:      "node-a/wg0:raw-udp->node-b/wg0:raw-udp",
			SourceServiceName:  "wg0",
			DestinationNode:    "node-b",
			DestinationService: "wg0",
			Accepted:           true,
		},
		{
			AssociationID:      "node-a/wg0:raw-udp->node-c/wg0:raw-udp",
			SourceServiceName:  "wg0",
			DestinationNode:    "node-c",
			DestinationService: "wg0",
			Accepted:           true,
		},
	}

	inputs := BuildAssociationActivationInputs(cfg, results)
	if len(inputs) != 1 {
		t.Fatalf("expected 1 activation input (only the one with direct_endpoint), got %d", len(inputs))
	}

	if inputs[0].AssociationID != "node-a/wg0:raw-udp->node-b/wg0:raw-udp" {
		t.Fatalf("wrong association ID: %s", inputs[0].AssociationID)
	}
	if inputs[0].DirectEndpoint != "192.0.2.1:51830" {
		t.Fatalf("wrong direct endpoint: %s", inputs[0].DirectEndpoint)
	}
	if inputs[0].MeshListenPort != 51830 {
		t.Fatalf("wrong mesh listen port: %d", inputs[0].MeshListenPort)
	}
}

// --- Test: ingress vs target distinction is preserved ---

func TestDirectPathActivation_IngressTargetDistinction(t *testing.T) {
	// Allocate a real UDP target (simulates WireGuard ListenPort).
	targetConn, targetAddr := allocLoopbackUDP(t)
	defer targetConn.Close()

	// Allocate a mesh endpoint for delivery.
	meshConn, meshAddr := allocLoopbackUDP(t)
	meshConn.Close() // close so carrier can rebind

	cfg := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-a"},
		Services: []config.ServiceConfig{
			{
				Name: "wg0",
				Type: config.ServiceTypeRawUDP,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    uint16(targetAddr.Port),
				},
				Ingress: &config.ServiceIngressConfig{
					Mode:       config.IngressModeStatic,
					StaticPort: 0, // will use ephemeral
				},
			},
		},
	}

	runtime := NewDirectPathRuntime()
	defer runtime.Carrier.StopAll()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inputs := []AssociationActivationInput{
		{
			AssociationID:  "test-assoc",
			SourceNode:     "node-a",
			SourceService:  service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DestNode:       "node-b",
			DestService:    service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DirectEndpoint: meshAddr.String(), // for test, point at local mesh
			MeshListenPort: uint16(meshAddr.Port),
		},
	}

	result := ActivateDirectPaths(ctx, cfg, runtime, inputs)
	if result.TotalFailed > 0 {
		for _, a := range result.Activations {
			if a.Error != "" {
				t.Fatalf("activation failed: %s", a.Error)
			}
		}
	}

	if len(result.Activations) != 1 {
		t.Fatalf("expected 1 activation, got %d", len(result.Activations))
	}

	act := result.Activations[0]

	// Verify the local ingress and local target are different.
	if act.LocalIngress == act.LocalTarget {
		t.Fatalf("local ingress (%s) must differ from local target (%s) — these are architecturally separate concepts",
			act.LocalIngress, act.LocalTarget)
	}

	// Verify the report mentions the right addresses.
	if act.LocalTarget != fmt.Sprintf("127.0.0.1:%d", uint16(targetAddr.Port)) {
		t.Fatalf("local target should be WireGuard listen port, got %s", act.LocalTarget)
	}
}

// --- Test: missing direct_endpoint means no activation ---

func TestDirectPathActivation_NoDirectEndpoint(t *testing.T) {
	cfg := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-a"},
		Services: []config.ServiceConfig{
			{
				Name: "wg0",
				Type: config.ServiceTypeRawUDP,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    51820,
				},
			},
		},
	}

	runtime := NewDirectPathRuntime()
	defer runtime.Carrier.StopAll()

	ctx := context.Background()

	// No activation inputs at all.
	result := ActivateDirectPaths(ctx, cfg, runtime, nil)
	if result.TotalActive != 0 || result.TotalFailed != 0 {
		t.Fatalf("expected no activations, got active=%d failed=%d", result.TotalActive, result.TotalFailed)
	}
}

// --- Test: invalid association input fails clearly ---

func TestDirectPathActivation_InvalidInput(t *testing.T) {
	cfg := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-a"},
		Services: []config.ServiceConfig{
			{
				Name: "wg0",
				Type: config.ServiceTypeRawUDP,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    51820,
				},
			},
		},
	}

	runtime := NewDirectPathRuntime()
	defer runtime.Carrier.StopAll()

	ctx := context.Background()

	inputs := []AssociationActivationInput{
		{
			AssociationID:  "bad-assoc",
			SourceNode:     "node-a",
			SourceService:  service.Identity{Name: "nonexistent-service", Type: config.ServiceTypeRawUDP},
			DestNode:       "node-b",
			DestService:    service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DirectEndpoint: "192.0.2.1:51830",
		},
	}

	result := ActivateDirectPaths(ctx, cfg, runtime, inputs)
	if result.TotalFailed != 1 {
		t.Fatalf("expected 1 failure, got %d", result.TotalFailed)
	}
	if result.Activations[0].Error == "" {
		t.Fatal("expected error message for invalid input")
	}
}

// --- Test: WireGuard-over-mesh end-to-end direct-path validation ---
//
// This is the flagship validation test for T-0009. It proves:
//
//  1. A node can expose a Transitloom local ingress endpoint for a
//     direct-path associated service
//  2. A peer can send WireGuard UDP traffic to that local ingress endpoint
//  3. Transitloom delivers that traffic to the correct WireGuard local target
//  4. The implementation preserves the distinction between local target,
//     local ingress, service binding, and association
//  5. Standard WireGuard remains unchanged — Transitloom carries raw UDP
//     packets with zero in-band overhead
//  6. This is direct-path only — no relay, scheduler, or multi-WAN support
//     is claimed
//
// The test simulates a WireGuard-like UDP flow through Transitloom:
//
//   WireGuard app → [Transitloom local ingress] → [mesh] → [delivery] → WireGuard local target
//
// On loopback, the "mesh" is just a local UDP hop, but the architectural
// flow is identical to a real deployment.
func TestWireGuardOverMesh_DirectPath_EndToEnd(t *testing.T) {
	// Step 1: Set up the "WireGuard ListenPort" — the local target where
	// Transitloom delivers inbound carried traffic. In a real deployment,
	// this is the actual WireGuard UDP listen port (e.g., 51820).
	wgListenConn, wgListenAddr := allocLoopbackUDP(t)
	defer wgListenConn.Close()

	// Step 2: Allocate a mesh listen port for inbound delivery.
	// Pre-allocate and close so the carrier can rebind.
	meshPreConn, meshPreAddr := allocLoopbackUDP(t)
	meshPreConn.Close()

	// Step 2b: Pre-allocate a local ingress port so we get a known address.
	ingressPreConn, ingressPreAddr := allocLoopbackUDP(t)
	ingressPreConn.Close()

	// Step 3: Set up the node config with a WireGuard-style service.
	cfg := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-a"},
		Services: []config.ServiceConfig{
			{
				Name: "wg0",
				Type: config.ServiceTypeRawUDP,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    uint16(wgListenAddr.Port),
				},
				Ingress: &config.ServiceIngressConfig{
					Mode:       config.IngressModeStatic,
					StaticPort: uint16(ingressPreAddr.Port),
				},
			},
		},
	}

	// Step 4: Create and activate the direct-path runtime.
	runtime := NewDirectPathRuntime()
	defer runtime.Carrier.StopAll()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inputs := []AssociationActivationInput{
		{
			AssociationID: "node-a/wg0:raw-udp->node-b/wg0:raw-udp",
			SourceNode:    "node-a",
			SourceService: service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DestNode:      "node-b",
			DestService:   service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			// For this test, the "remote peer" is also on loopback — we send
			// to the mesh listen port which then delivers to the WireGuard target.
			DirectEndpoint: meshPreAddr.String(),
			MeshListenPort: uint16(meshPreAddr.Port),
		},
	}

	result := ActivateDirectPaths(ctx, cfg, runtime, inputs)
	if result.TotalFailed > 0 {
		for _, a := range result.Activations {
			if a.Error != "" {
				t.Fatalf("direct-path activation failed: %s", a.Error)
			}
		}
	}
	if result.TotalActive != 1 {
		t.Fatalf("expected 1 active association, got %d", result.TotalActive)
	}

	act := result.Activations[0]
	if !act.IngressActive {
		t.Fatal("ingress should be active")
	}
	if !act.DeliveryActive {
		t.Fatal("delivery should be active")
	}

	// Verify architectural distinction: local ingress ≠ local target.
	if act.LocalIngress == act.LocalTarget {
		t.Fatalf("ARCHITECTURAL VIOLATION: local ingress (%s) must differ from local target (%s)",
			act.LocalIngress, act.LocalTarget)
	}

	// Step 5: Simulate WireGuard sending a packet to the Transitloom local
	// ingress endpoint. In a real deployment, WireGuard would have:
	//
	//   [Peer]
	//   Endpoint = 127.0.0.1:<ingress_port>
	//
	// and would send encrypted WireGuard packets to that address.
	ingressAddr, err := net.ResolveUDPAddr("udp", act.LocalIngress)
	if err != nil {
		t.Fatalf("resolve ingress addr: %v", err)
	}

	// Simulate WireGuard sending an encrypted packet (opaque bytes).
	wgPacket := []byte("WireGuard-encrypted-packet-simulation-payload-64-bytes-of-data-here!")
	wgSendConn, err := net.DialUDP("udp", nil, ingressAddr)
	if err != nil {
		t.Fatalf("dial ingress: %v", err)
	}
	defer wgSendConn.Close()

	_, err = wgSendConn.Write(wgPacket)
	if err != nil {
		t.Fatalf("WireGuard send to Transitloom ingress: %v", err)
	}

	// Step 6: Read the packet from the WireGuard local target (ListenPort).
	// This proves Transitloom delivered the packet to the correct service.
	wgListenConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 65535)
	n, _, err := wgListenConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("WireGuard target did not receive packet: %v", err)
	}

	// Step 7: Verify zero in-band overhead — packet bytes arrive unchanged.
	received := buf[:n]
	if string(received) != string(wgPacket) {
		t.Fatalf("zero-overhead violation: sent %q, received %q", wgPacket, received)
	}

	// Step 8: Verify forwarding table state.
	entry, exists := runtime.Table.Lookup("node-a/wg0:raw-udp->node-b/wg0:raw-udp")
	if !exists {
		t.Fatal("forwarding entry should exist in table")
	}
	if !entry.DirectOnly {
		t.Fatal("entry must be direct-only")
	}
	if entry.LocalIngressAddr.String() == entry.LocalTargetAddr.String() {
		t.Fatal("forwarding entry: local ingress and local target must be different")
	}

	// Step 9: Verify carrier stats.
	packets, bytes, running := runtime.Carrier.IngressStats("node-a/wg0:raw-udp->node-b/wg0:raw-udp")
	if !running {
		t.Fatal("ingress should be running")
	}
	if packets != 1 {
		t.Fatalf("ingress packets: want 1, got %d", packets)
	}
	if bytes != uint64(len(wgPacket)) {
		t.Fatalf("ingress bytes: want %d, got %d", len(wgPacket), bytes)
	}

	dPackets, dBytes, dRunning := runtime.Carrier.DeliveryStats("node-a/wg0:raw-udp->node-b/wg0:raw-udp")
	if !dRunning {
		t.Fatal("delivery should be running")
	}
	if dPackets != 1 {
		t.Fatalf("delivery packets: want 1, got %d", dPackets)
	}
	if dBytes != uint64(len(wgPacket)) {
		t.Fatalf("delivery bytes: want %d, got %d", len(wgPacket), dBytes)
	}

	// Step 10: Log the report for human review.
	for _, line := range result.ReportLines() {
		t.Log(line)
	}
}

// TestWireGuardOverMesh_DirectPath_MultiplePackets sends multiple WireGuard
// packets through the Transitloom direct path and verifies all arrive intact.
func TestWireGuardOverMesh_DirectPath_MultiplePackets(t *testing.T) {
	wgListenConn, wgListenAddr := allocLoopbackUDP(t)
	defer wgListenConn.Close()

	meshPreConn, meshPreAddr := allocLoopbackUDP(t)
	meshPreConn.Close()

	ingressPreConn, ingressPreAddr := allocLoopbackUDP(t)
	ingressPreConn.Close()

	cfg := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-a"},
		Services: []config.ServiceConfig{
			{
				Name: "wg0",
				Type: config.ServiceTypeRawUDP,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    uint16(wgListenAddr.Port),
				},
				Ingress: &config.ServiceIngressConfig{
					Mode:       config.IngressModeStatic,
					StaticPort: uint16(ingressPreAddr.Port),
				},
			},
		},
	}

	runtime := NewDirectPathRuntime()
	defer runtime.Carrier.StopAll()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inputs := []AssociationActivationInput{
		{
			AssociationID:  "node-a/wg0:raw-udp->node-b/wg0:raw-udp",
			SourceNode:     "node-a",
			SourceService:  service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DestNode:       "node-b",
			DestService:    service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DirectEndpoint: meshPreAddr.String(),
			MeshListenPort: uint16(meshPreAddr.Port),
		},
	}

	result := ActivateDirectPaths(ctx, cfg, runtime, inputs)
	if result.TotalFailed > 0 {
		t.Fatalf("activation failed: %+v", result.Activations)
	}

	act := result.Activations[0]
	ingressAddr, _ := net.ResolveUDPAddr("udp", act.LocalIngress)

	wgSendConn, err := net.DialUDP("udp", nil, ingressAddr)
	if err != nil {
		t.Fatalf("dial ingress: %v", err)
	}
	defer wgSendConn.Close()

	const numPackets = 20
	for i := 0; i < numPackets; i++ {
		payload := fmt.Sprintf("wg-pkt-%03d", i)
		wgSendConn.Write([]byte(payload))
	}

	wgListenConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 65535)
	received := 0
	for received < numPackets {
		n, _, err := wgListenConn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		expected := fmt.Sprintf("wg-pkt-%03d", received)
		if string(buf[:n]) != expected {
			t.Fatalf("packet %d: got %q, want %q", received, buf[:n], expected)
		}
		received++
	}

	if received != numPackets {
		t.Fatalf("received %d/%d packets", received, numPackets)
	}
}

// TestWireGuardOverMesh_DirectPath_Bidirectional proves that traffic can flow
// in both directions through Transitloom local ingress ports. This simulates
// the real WireGuard-over-mesh scenario where both peers send to each other's
// Transitloom ingress endpoints.
func TestWireGuardOverMesh_DirectPath_Bidirectional(t *testing.T) {
	// Set up two "WireGuard services" — one per simulated node.
	wgA_Conn, wgA_Addr := allocLoopbackUDP(t)
	defer wgA_Conn.Close()
	wgB_Conn, wgB_Addr := allocLoopbackUDP(t)
	defer wgB_Conn.Close()

	// Pre-allocate mesh listen ports.
	meshA_PreConn, meshA_Addr := allocLoopbackUDP(t)
	meshA_PreConn.Close()
	meshB_PreConn, meshB_Addr := allocLoopbackUDP(t)
	meshB_PreConn.Close()

	// Pre-allocate ingress ports.
	ingressA_PreConn, ingressA_Addr := allocLoopbackUDP(t)
	ingressA_PreConn.Close()
	ingressB_PreConn, ingressB_Addr := allocLoopbackUDP(t)
	ingressB_PreConn.Close()

	// Node A config.
	cfgA := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-a"},
		Services: []config.ServiceConfig{
			{
				Name: "wg0",
				Type: config.ServiceTypeRawUDP,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    uint16(wgA_Addr.Port),
				},
				Ingress: &config.ServiceIngressConfig{
					Mode:       config.IngressModeStatic,
					StaticPort: uint16(ingressA_Addr.Port),
				},
			},
		},
	}

	// Node B config.
	cfgB := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-b"},
		Services: []config.ServiceConfig{
			{
				Name: "wg0",
				Type: config.ServiceTypeRawUDP,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    uint16(wgB_Addr.Port),
				},
				Ingress: &config.ServiceIngressConfig{
					Mode:       config.IngressModeStatic,
					StaticPort: uint16(ingressB_Addr.Port),
				},
			},
		},
	}

	runtimeA := NewDirectPathRuntime()
	defer runtimeA.Carrier.StopAll()
	runtimeB := NewDirectPathRuntime()
	defer runtimeB.Carrier.StopAll()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Node A association: outbound to node B, inbound delivery from node B.
	inputsA := []AssociationActivationInput{
		{
			AssociationID:  "node-a/wg0:raw-udp->node-b/wg0:raw-udp",
			SourceNode:     "node-a",
			SourceService:  service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DestNode:       "node-b",
			DestService:    service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DirectEndpoint: meshB_Addr.String(), // send to node B's mesh port
			MeshListenPort: uint16(meshA_Addr.Port),     // receive from node B on this port
		},
	}

	// Node B association: outbound to node A, inbound delivery from node A.
	inputsB := []AssociationActivationInput{
		{
			AssociationID:  "node-b/wg0:raw-udp->node-a/wg0:raw-udp",
			SourceNode:     "node-b",
			SourceService:  service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DestNode:       "node-a",
			DestService:    service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DirectEndpoint: meshA_Addr.String(), // send to node A's mesh port
			MeshListenPort: uint16(meshB_Addr.Port),     // receive from node A on this port
		},
	}

	resultA := ActivateDirectPaths(ctx, cfgA, runtimeA, inputsA)
	resultB := ActivateDirectPaths(ctx, cfgB, runtimeB, inputsB)

	if resultA.TotalFailed > 0 || resultB.TotalFailed > 0 {
		t.Fatalf("activation failed: A=%+v B=%+v", resultA.Activations, resultB.Activations)
	}

	// A sends to B.
	ingressA, _ := net.ResolveUDPAddr("udp", resultA.Activations[0].LocalIngress)
	sendA, _ := net.DialUDP("udp", nil, ingressA)
	defer sendA.Close()
	sendA.Write([]byte("hello-from-A"))

	wgB_Conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 65535)
	n, _, err := wgB_Conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("node B WireGuard did not receive: %v", err)
	}
	if string(buf[:n]) != "hello-from-A" {
		t.Fatalf("node B received %q, want %q", buf[:n], "hello-from-A")
	}

	// B sends to A.
	ingressB, _ := net.ResolveUDPAddr("udp", resultB.Activations[0].LocalIngress)
	sendB, _ := net.DialUDP("udp", nil, ingressB)
	defer sendB.Close()
	sendB.Write([]byte("hello-from-B"))

	wgA_Conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, _, err = wgA_Conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("node A WireGuard did not receive: %v", err)
	}
	if string(buf[:n]) != "hello-from-B" {
		t.Fatalf("node A received %q, want %q", buf[:n], "hello-from-B")
	}
}

// TestDirectPathResult_ReportLines verifies that reporting output is structured
// and preserves architectural intent.
func TestDirectPathResult_ReportLines(t *testing.T) {
	result := DirectPathResult{
		Activations: []DirectPathActivation{
			{
				AssociationID:  "test-assoc",
				SourceService:  "wg0",
				DestNode:       "node-b",
				DestService:    "wg0",
				LocalIngress:   "127.0.0.1:41002",
				LocalTarget:    "127.0.0.1:51820",
				RemoteEndpoint: "192.0.2.1:51830",
				MeshListen:     "0.0.0.0:51830",
				IngressActive:  true,
				DeliveryActive: true,
			},
		},
		TotalActive: 1,
	}

	lines := result.ReportLines()
	if len(lines) == 0 {
		t.Fatal("expected report lines")
	}

	// Check that the report mentions the direct-path-only limitation.
	found := false
	for _, line := range lines {
		if contains(line, "direct-path only") || contains(line, "standard WireGuard remains unchanged") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("report should mention direct-path-only limitation")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
