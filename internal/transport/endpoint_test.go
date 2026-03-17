package transport_test

import (
	"strings"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/transport"
)

// TestExternalEndpointValidate verifies that Validate() accepts structurally
// valid endpoints and rejects invalid ones with useful errors.
func TestExternalEndpointValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ep      transport.ExternalEndpoint
		wantErr bool
	}{
		{
			name: "valid configured endpoint no dnat",
			ep: transport.ExternalEndpoint{
				Host:         "203.0.113.1",
				Port:         51830,
				Source:       transport.EndpointSourceConfigured,
				Verification: transport.VerificationStateUnverified,
			},
		},
		{
			name: "valid configured endpoint with dnat",
			ep: transport.ExternalEndpoint{
				Host:         "203.0.113.1",
				Port:         12000,
				LocalPort:    51830,
				Source:       transport.EndpointSourceConfigured,
				Verification: transport.VerificationStateUnverified,
			},
		},
		{
			name: "valid router-discovered endpoint",
			ep: transport.ExternalEndpoint{
				Host:         "198.51.100.5",
				Port:         51830,
				Source:       transport.EndpointSourceRouterDiscovered,
				Verification: transport.VerificationStateVerified,
			},
		},
		{
			name: "valid probe-discovered endpoint",
			ep: transport.ExternalEndpoint{
				Host:         "198.51.100.5",
				Port:         51830,
				Source:       transport.EndpointSourceProbeDiscovered,
				Verification: transport.VerificationStateVerified,
			},
		},
		{
			name: "valid coordinator-observed endpoint",
			ep: transport.ExternalEndpoint{
				Host:         "192.0.2.1",
				Port:         51830,
				Source:       transport.EndpointSourceCoordinatorObserved,
				Verification: transport.VerificationStateUnverified,
			},
		},
		{
			name: "stale state is valid",
			ep: transport.ExternalEndpoint{
				Host:         "203.0.113.1",
				Port:         51830,
				Source:       transport.EndpointSourceConfigured,
				Verification: transport.VerificationStateStale,
			},
		},
		{
			name: "failed state is valid",
			ep: transport.ExternalEndpoint{
				Host:         "203.0.113.1",
				Port:         51830,
				Source:       transport.EndpointSourceConfigured,
				Verification: transport.VerificationStateFailed,
			},
		},
		{
			name: "empty host",
			ep: transport.ExternalEndpoint{
				Port:         51830,
				Source:       transport.EndpointSourceConfigured,
				Verification: transport.VerificationStateUnverified,
			},
			wantErr: true,
		},
		{
			name: "zero port",
			ep: transport.ExternalEndpoint{
				Host:         "203.0.113.1",
				Port:         0,
				Source:       transport.EndpointSourceConfigured,
				Verification: transport.VerificationStateUnverified,
			},
			wantErr: true,
		},
		{
			name: "unknown source",
			ep: transport.ExternalEndpoint{
				Host:         "203.0.113.1",
				Port:         51830,
				Source:       "made-up-source",
				Verification: transport.VerificationStateUnverified,
			},
			wantErr: true,
		},
		{
			name: "unknown verification state",
			ep: transport.ExternalEndpoint{
				Host:         "203.0.113.1",
				Port:         51830,
				Source:       transport.EndpointSourceConfigured,
				Verification: "made-up-state",
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.ep.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// TestEndpointConceptualDistinctions verifies that external endpoint fields
// preserve the distinctions between local target, local ingress, mesh port,
// and external advertised endpoint.
//
// These must not be collapsed. The distinction is especially important in
// DNAT deployments where the external port and local mesh listener port differ.
func TestEndpointConceptualDistinctions(t *testing.T) {
	t.Parallel()

	// local target: where the local service receives carried traffic.
	// NOT an ExternalEndpoint. Example: 127.0.0.1:51820 for WireGuard.
	localTargetPort := uint16(51820)
	_ = localTargetPort

	// local ingress: Transitloom-provided loopback port for sending into the mesh.
	// NOT an ExternalEndpoint. Example: 127.0.0.1:52000 per association.
	localIngressPort := uint16(52000)
	_ = localIngressPort

	// mesh/runtime port: where Transitloom listens for inbound mesh traffic.
	// This is LocalPort in a DNAT ExternalEndpoint, not the ExternalEndpoint
	// itself.
	localMeshPort := uint16(51830)

	// external port: the public-facing port on the router that remote nodes
	// can reach. Different from localMeshPort in DNAT deployments.
	externalPort := uint16(12345)

	ext := transport.NewConfiguredEndpoint("203.0.113.1", externalPort, localMeshPort)

	if ext.Port != externalPort {
		t.Errorf("Port = %d, want externalPort %d", ext.Port, externalPort)
	}
	if ext.LocalPort != localMeshPort {
		t.Errorf("LocalPort = %d, want localMeshPort %d", ext.LocalPort, localMeshPort)
	}
	if ext.Port == ext.LocalPort {
		t.Error("DNAT endpoint: external port and local mesh port must differ")
	}
	if !ext.HasDNAT() {
		t.Error("HasDNAT() must return true when external port differs from local mesh port")
	}
	if ext.EffectiveLocalPort() != localMeshPort {
		t.Errorf("EffectiveLocalPort() = %d, want localMeshPort %d",
			ext.EffectiveLocalPort(), localMeshPort)
	}
}

// TestEffectiveLocalPortNoDNAT verifies that EffectiveLocalPort returns the
// external port itself when no DNAT is configured (LocalPort == 0).
func TestEffectiveLocalPortNoDNAT(t *testing.T) {
	t.Parallel()

	ext := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
	if ext.HasDNAT() {
		t.Error("HasDNAT() must return false when LocalPort is zero")
	}
	if ext.EffectiveLocalPort() != 51830 {
		t.Errorf("EffectiveLocalPort() = %d, want 51830", ext.EffectiveLocalPort())
	}
}

// TestExternalEndpointStateTransitions verifies that MarkStale, MarkVerified,
// and MarkFailed produce the correct state and timestamps, and that IsUsable
// reflects the expected usability contract.
func TestExternalEndpointStateTransitions(t *testing.T) {
	t.Parallel()

	now := time.Now()

	t.Run("configured starts unverified and is usable", func(t *testing.T) {
		t.Parallel()

		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		if ep.Verification != transport.VerificationStateUnverified {
			t.Errorf("new configured endpoint must start unverified, got %q",
				ep.Verification)
		}
		if ep.Source != transport.EndpointSourceConfigured {
			t.Errorf("source = %q, want configured", ep.Source)
		}
		// Configured unverified endpoints are usable: they represent operator intent.
		if !ep.IsUsable() {
			t.Error("configured unverified endpoint must be usable for direct-path attempts")
		}
	})

	t.Run("mark verified", func(t *testing.T) {
		t.Parallel()

		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		ep.MarkVerified(now)

		if ep.Verification != transport.VerificationStateVerified {
			t.Errorf("after MarkVerified, Verification = %q, want verified",
				ep.Verification)
		}
		if !ep.VerifiedAt.Equal(now) {
			t.Errorf("VerifiedAt = %v, want %v", ep.VerifiedAt, now)
		}
		if !ep.IsUsable() {
			t.Error("verified endpoint must be usable")
		}
	})

	t.Run("mark stale after unhealthy path event", func(t *testing.T) {
		t.Parallel()

		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		ep.MarkVerified(now)

		staleAt := now.Add(5 * time.Minute)
		ep.MarkStale(staleAt)

		if ep.Verification != transport.VerificationStateStale {
			t.Errorf("after MarkStale, Verification = %q, want stale",
				ep.Verification)
		}
		if !ep.StaleAt.Equal(staleAt) {
			t.Errorf("StaleAt = %v, want %v", ep.StaleAt, staleAt)
		}
		// Stale endpoint must not be used without revalidation.
		if ep.IsUsable() {
			t.Error("stale endpoint must not be usable without revalidation")
		}
	})

	t.Run("mark failed after unsuccessful probe", func(t *testing.T) {
		t.Parallel()

		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		ep.MarkFailed(now)

		if ep.Verification != transport.VerificationStateFailed {
			t.Errorf("after MarkFailed, Verification = %q, want failed",
				ep.Verification)
		}
		// Failed endpoint must not be used without revalidation.
		if ep.IsUsable() {
			t.Error("failed endpoint must not be usable without revalidation")
		}
	})

	t.Run("stale endpoint revalidated by mark verified", func(t *testing.T) {
		t.Parallel()

		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		ep.MarkStale(now)

		if ep.IsUsable() {
			t.Error("stale endpoint must not be usable before revalidation")
		}

		revalidatedAt := now.Add(10 * time.Minute)
		ep.MarkVerified(revalidatedAt)

		if ep.Verification != transport.VerificationStateVerified {
			t.Errorf("after revalidation, Verification = %q, want verified",
				ep.Verification)
		}
		if !ep.IsUsable() {
			t.Error("revalidated endpoint must be usable again")
		}
	})

	t.Run("failed endpoint revalidated by mark verified", func(t *testing.T) {
		t.Parallel()

		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		ep.MarkFailed(now)

		if ep.IsUsable() {
			t.Error("failed endpoint must not be usable before revalidation")
		}

		ep.MarkVerified(now.Add(5 * time.Minute))
		if !ep.IsUsable() {
			t.Error("revalidated failed endpoint must become usable again")
		}
	})
}

// TestVerificationStateIsUsableForDirectPath verifies the usability contract
// for all known verification states.
func TestVerificationStateIsUsableForDirectPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state      transport.VerificationState
		wantUsable bool
		reason     string
	}{
		{
			transport.VerificationStateUnverified, true,
			"configured unverified endpoints represent operator intent",
		},
		{
			transport.VerificationStateVerified, true,
			"verified endpoints are confirmed reachable",
		},
		{
			transport.VerificationStateStale, false,
			"stale endpoints may have a changed public IP or removed DNAT rule",
		},
		{
			transport.VerificationStateFailed, false,
			"failed endpoints were actively found unreachable",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.state), func(t *testing.T) {
			t.Parallel()

			got := tc.state.IsUsableForDirectPath()
			if got != tc.wantUsable {
				t.Errorf("VerificationState(%q).IsUsableForDirectPath() = %v, want %v (%s)",
					tc.state, got, tc.wantUsable, tc.reason)
			}
		})
	}
}

// TestSourceOfKnowledgeSemanticsPreserved verifies that all source-of-knowledge
// constants are distinct and that configured endpoints have the highest
// conceptual precedence by remaining usable even without verification.
func TestSourceOfKnowledgeSemanticsPreserved(t *testing.T) {
	t.Parallel()

	// All four source constants must be distinct.
	sources := []transport.EndpointSource{
		transport.EndpointSourceConfigured,
		transport.EndpointSourceRouterDiscovered,
		transport.EndpointSourceProbeDiscovered,
		transport.EndpointSourceCoordinatorObserved,
	}

	seen := make(map[transport.EndpointSource]struct{})
	for _, s := range sources {
		if _, exists := seen[s]; exists {
			t.Errorf("duplicate source constant: %q", s)
		}
		seen[s] = struct{}{}
		if string(s) == "" {
			t.Errorf("source constant must not be empty string")
		}
	}

	// A configured endpoint starting unverified must be usable.
	// This is the core precedence rule: explicit operator config beats
	// all discovered or observed sources and does not require pre-verification.
	configuredEp := transport.ExternalEndpoint{
		Host:         "203.0.113.1",
		Port:         51830,
		Source:       transport.EndpointSourceConfigured,
		Verification: transport.VerificationStateUnverified,
	}
	if !configuredEp.IsUsable() {
		t.Error("configured unverified endpoint must be usable (highest precedence source)")
	}
}

// TestRouterDiscoveryHintToExternalEndpoint verifies that ToExternalEndpoint
// produces a correctly populated ExternalEndpoint from a router discovery hint.
func TestRouterDiscoveryHintToExternalEndpoint(t *testing.T) {
	t.Parallel()

	now := time.Now()
	hint := transport.RouterDiscoveryHint{
		Protocol:     "upnp",
		ExternalHost: "203.0.113.1",
		ExternalPort: 12345,
		InternalPort: 51830,
		RecordedAt:   now,
	}

	ep := hint.ToExternalEndpoint()

	if ep.Host != hint.ExternalHost {
		t.Errorf("Host = %q, want %q", ep.Host, hint.ExternalHost)
	}
	if ep.Port != hint.ExternalPort {
		t.Errorf("Port = %d, want %d", ep.Port, hint.ExternalPort)
	}
	if ep.LocalPort != hint.InternalPort {
		t.Errorf("LocalPort = %d, want %d (router internal port)", ep.LocalPort, hint.InternalPort)
	}
	if ep.Source != transport.EndpointSourceRouterDiscovered {
		t.Errorf("Source = %q, want router-discovered", ep.Source)
	}
	// Router-protocol discovery provides first-hand confirmation of the
	// mapping, so the resulting endpoint starts as verified.
	if ep.Verification != transport.VerificationStateVerified {
		t.Errorf("Verification = %q after router discovery, want verified (router confirmed the mapping)",
			ep.Verification)
	}
	if !ep.HasDNAT() {
		t.Error("router hint with different external/internal ports must produce DNAT endpoint")
	}
	if err := ep.Validate(); err != nil {
		t.Errorf("ToExternalEndpoint() produced invalid endpoint: %v", err)
	}
}

// TestProbeResultApplyToEndpoint verifies that probe results correctly update
// endpoint verification state.
func TestProbeResultApplyToEndpoint(t *testing.T) {
	t.Parallel()

	now := time.Now()

	t.Run("successful probe marks verified", func(t *testing.T) {
		t.Parallel()

		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		result := transport.ProbeResult{
			TargetHost: "203.0.113.1",
			TargetPort: 51830,
			Reachable:  true,
			ProbedAt:   now,
		}
		result.ApplyToEndpoint(&ep)

		if ep.Verification != transport.VerificationStateVerified {
			t.Errorf("Verification = %q after successful probe, want verified",
				ep.Verification)
		}
		if !ep.IsUsable() {
			t.Error("endpoint after successful probe must be usable")
		}
	})

	t.Run("failed probe marks failed", func(t *testing.T) {
		t.Parallel()

		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		result := transport.ProbeResult{
			TargetHost: "203.0.113.1",
			TargetPort: 51830,
			Reachable:  false,
			ProbedAt:   now,
		}
		result.ApplyToEndpoint(&ep)

		if ep.Verification != transport.VerificationStateFailed {
			t.Errorf("Verification = %q after failed probe, want failed",
				ep.Verification)
		}
		if ep.IsUsable() {
			t.Error("endpoint after failed probe must not be usable")
		}
	})
}

// TestValidateAddrPort verifies that ValidateAddrPort accepts valid host:port
// combinations and rejects obviously invalid ones.
func TestValidateAddrPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		host    string
		port    uint16
		wantErr bool
	}{
		{"valid IPv4", "203.0.113.1", 51830, false},
		{"valid IPv6", "2001:db8::1", 51830, false},
		{"valid hostname", "example.com", 51830, false},
		{"empty host", "", 51830, true},
		{"zero port", "203.0.113.1", 0, true},
		{"host with space", "bad host", 51830, true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := transport.ValidateAddrPort(tc.host, tc.port)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateAddrPort(%q, %d) error = %v, wantErr = %v",
					tc.host, tc.port, err, tc.wantErr)
			}
		})
	}
}

// TestNewConfiguredEndpointConstructor verifies that NewConfiguredEndpoint
// produces an endpoint with the expected source and initial state.
func TestNewConfiguredEndpointConstructor(t *testing.T) {
	t.Parallel()

	ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)

	if ep.Source != transport.EndpointSourceConfigured {
		t.Errorf("Source = %q, want configured", ep.Source)
	}
	if ep.Verification != transport.VerificationStateUnverified {
		t.Errorf("Verification = %q, want unverified", ep.Verification)
	}
	if ep.RecordedAt.IsZero() {
		t.Error("RecordedAt must be set")
	}
	if !ep.VerifiedAt.IsZero() {
		t.Error("VerifiedAt must be zero for a new unverified endpoint")
	}
	if !ep.StaleAt.IsZero() {
		t.Error("StaleAt must be zero for a new endpoint")
	}
}

// TestEndpointStalenessAfterDownEvent illustrates the intended stale-after-down
// revalidation semantics: an endpoint that was verified becomes stale when the
// associated path goes down, and can only be used again after revalidation.
//
// This test documents the model; it does not test actual path health monitoring
// (which is not yet implemented).
func TestEndpointStalenessAfterDownEvent(t *testing.T) {
	t.Parallel()

	ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)

	// Initially usable (configured, unverified).
	if !ep.IsUsable() {
		t.Fatal("fresh configured endpoint must be usable")
	}

	// Path health monitoring verifies the endpoint via a probe.
	probeTime := time.Now()
	ep.MarkVerified(probeTime)
	if !ep.IsUsable() {
		t.Fatal("verified endpoint must be usable")
	}

	// Path goes down (e.g., WAN link failure, DNAT rule removed).
	// The endpoint must become stale so it is not used for direct-path
	// decisions without revalidation. Treating stale endpoints as timeless
	// truth would silently poison direct-path attempts after network changes.
	downAt := probeTime.Add(30 * time.Second)
	ep.MarkStale(downAt)

	if ep.IsUsable() {
		t.Error("endpoint must not be usable after path goes down (stale)")
	}
	if !strings.Contains(string(ep.Verification), "stale") {
		t.Errorf("Verification = %q, want stale after down event", ep.Verification)
	}

	// After the path recovers, a probe revalidates the endpoint.
	recoveryAt := downAt.Add(2 * time.Minute)
	ep.MarkVerified(recoveryAt)

	if !ep.IsUsable() {
		t.Error("endpoint must be usable again after revalidation")
	}
}
