package main

import (
	"strings"
	"testing"

	"github.com/zhouchenh/transitloom/internal/config"
)

// TestNodeConfigSummaryLines verifies that nodeConfigSummaryLines preserves
// key architectural distinctions in its output.
//
// These tests lock down semantic distinctions that must not be silently erased
// through code changes:
//   - configured state must not be mislabeled as runtime or coordinator state
//   - associations must not be labeled as active
//   - services must not be labeled as coordinator-registered
//   - external endpoints must not be labeled as verified
//   - DNAT and non-DNAT forwarded ports must be kept distinct
func TestNodeConfigSummaryLines(t *testing.T) {
	t.Run("identity labels configured-state-only", func(t *testing.T) {
		cfg := config.NodeConfig{
			Identity: config.IdentityMetadata{Name: "node-alpha"},
		}
		lines := nodeConfigSummaryLines(cfg)
		first := lines[0]
		// Must label output as configured-state-only to prevent misreading
		// as coordinator-verified or runtime state.
		if !strings.Contains(first, "configured-state-only") {
			t.Errorf("first line must contain 'configured-state-only'; got: %s", first)
		}
		if !strings.Contains(first, "node-alpha") {
			t.Errorf("first line must contain node name; got: %s", first)
		}
	})

	t.Run("services labeled as configured not coordinator-registered", func(t *testing.T) {
		cfg := config.NodeConfig{
			Identity: config.IdentityMetadata{Name: "node-beta"},
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
		lines := nodeConfigSummaryLines(cfg)
		joined := strings.Join(lines, "\n")

		// Services must be clearly labeled as configured, not registered.
		// "coordinator-registered" would overstate what configured means.
		if !strings.Contains(joined, "not coordinator-registered") {
			t.Errorf("service summary must include 'not coordinator-registered'; got:\n%s", joined)
		}
		if !strings.Contains(joined, "wg0") {
			t.Errorf("service name must appear in output; got:\n%s", joined)
		}
		if !strings.Contains(joined, "127.0.0.1:51820") {
			t.Errorf("service binding must appear in output; got:\n%s", joined)
		}
	})

	t.Run("associations labeled as configured not active", func(t *testing.T) {
		cfg := config.NodeConfig{
			Identity: config.IdentityMetadata{Name: "node-gamma"},
			Associations: []config.AssociationConfig{
				{
					SourceService:      "wg0",
					DestinationNode:    "node-delta",
					DestinationService: "wg0",
					DirectEndpoint:     "10.0.0.2:51830",
				},
			},
		}
		lines := nodeConfigSummaryLines(cfg)
		joined := strings.Join(lines, "\n")

		// Associations must be labeled as configured/not-active.
		// "active" would imply data is flowing, which cannot be known from config.
		if !strings.Contains(joined, "not active") {
			t.Errorf("association summary must include 'not active'; got:\n%s", joined)
		}
		if !strings.Contains(joined, "node-delta") {
			t.Errorf("destination node must appear; got:\n%s", joined)
		}
		if !strings.Contains(joined, "10.0.0.2:51830") {
			t.Errorf("direct endpoint must appear; got:\n%s", joined)
		}
	})

	t.Run("association with no direct endpoint shows none", func(t *testing.T) {
		cfg := config.NodeConfig{
			Identity: config.IdentityMetadata{Name: "node-epsilon"},
			Associations: []config.AssociationConfig{
				{
					SourceService:      "svc-a",
					DestinationNode:    "node-zeta",
					DestinationService: "svc-b",
					// No DirectEndpoint or RelayEndpoint configured
				},
			},
		}
		lines := nodeConfigSummaryLines(cfg)
		joined := strings.Join(lines, "\n")

		// Both direct and relay must show "(none)" when not configured,
		// so operators can see that no data-plane path is configured.
		if !strings.Contains(joined, "direct=(none)") {
			t.Errorf("missing direct endpoint must show '(none)'; got:\n%s", joined)
		}
		if !strings.Contains(joined, "relay=(none)") {
			t.Errorf("missing relay endpoint must show '(none)'; got:\n%s", joined)
		}
	})

	t.Run("external endpoint labeled as configured and unverified", func(t *testing.T) {
		cfg := config.NodeConfig{
			Identity: config.IdentityMetadata{Name: "node-eta"},
			ExternalEndpoint: config.ExternalEndpointConfig{
				PublicHost: "203.0.113.1",
				ForwardedPorts: []config.ForwardedPortConfig{
					{ExternalPort: 51820, LocalPort: 51821},
				},
			},
		}
		lines := nodeConfigSummaryLines(cfg)
		joined := strings.Join(lines, "\n")

		// External endpoints are configured (highest precedence source) but
		// start as unverified. This must be explicit in the output.
		if !strings.Contains(joined, "source=configured unverified") {
			t.Errorf("external endpoint must be labeled 'source=configured unverified'; got:\n%s", joined)
		}
		if !strings.Contains(joined, "203.0.113.1") {
			t.Errorf("public host must appear; got:\n%s", joined)
		}
	})

	t.Run("DNAT forwarded port keeps external and local port separate", func(t *testing.T) {
		cfg := config.NodeConfig{
			Identity: config.IdentityMetadata{Name: "node-theta"},
			ExternalEndpoint: config.ExternalEndpointConfig{
				PublicHost: "203.0.113.2",
				ForwardedPorts: []config.ForwardedPortConfig{
					{ExternalPort: 51820, LocalPort: 51821},
				},
			},
		}
		lines := nodeConfigSummaryLines(cfg)
		joined := strings.Join(lines, "\n")

		// DNAT must be explicitly labeled. Collapsing external port and local
		// port into a single value would silently break DNAT deployments.
		if !strings.Contains(joined, "[DNAT]") {
			t.Errorf("DNAT forwarded port must be labeled [DNAT]; got:\n%s", joined)
		}
		// Both ports must be visible so the operator can verify the mapping.
		if !strings.Contains(joined, "51820") {
			t.Errorf("external port must appear; got:\n%s", joined)
		}
		if !strings.Contains(joined, "51821") {
			t.Errorf("local port must appear; got:\n%s", joined)
		}
	})

	t.Run("non-DNAT forwarded port labeled no-DNAT", func(t *testing.T) {
		cfg := config.NodeConfig{
			Identity: config.IdentityMetadata{Name: "node-iota"},
			ExternalEndpoint: config.ExternalEndpointConfig{
				PublicHost: "203.0.113.3",
				ForwardedPorts: []config.ForwardedPortConfig{
					{ExternalPort: 51820, LocalPort: 51820},
				},
			},
		}
		lines := nodeConfigSummaryLines(cfg)
		joined := strings.Join(lines, "\n")

		// When external and local port are the same, label as no-DNAT so
		// operators can distinguish the two cases without inferring.
		if !strings.Contains(joined, "[no-DNAT]") {
			t.Errorf("same-port forwarding must be labeled [no-DNAT]; got:\n%s", joined)
		}
	})

	t.Run("no external endpoint shows not configured", func(t *testing.T) {
		cfg := config.NodeConfig{
			Identity: config.IdentityMetadata{Name: "node-kappa"},
		}
		lines := nodeConfigSummaryLines(cfg)
		joined := strings.Join(lines, "\n")

		if !strings.Contains(joined, "external-endpoint: (not configured)") {
			t.Errorf("absent external endpoint must show '(not configured)'; got:\n%s", joined)
		}
	})
}

// TestCoordinatorConfigSummaryLines verifies that coordinatorConfigSummaryLines
// preserves key distinctions in its output.
func TestCoordinatorConfigSummaryLines(t *testing.T) {
	t.Run("identity labels configured-state-only", func(t *testing.T) {
		cfg := config.CoordinatorConfig{
			Identity: config.IdentityMetadata{Name: "coord-alpha"},
			Trust: config.CoordinatorTrustConfig{
				RootAnchorPath:       "/data/root.crt",
				IntermediateCertPath: "/data/inter.crt",
				IntermediateKeyPath:  "/data/inter.key",
			},
		}
		lines := coordinatorConfigSummaryLines(cfg)
		first := lines[0]
		if !strings.Contains(first, "configured-state-only") {
			t.Errorf("first line must contain 'configured-state-only'; got: %s", first)
		}
		if !strings.Contains(first, "coord-alpha") {
			t.Errorf("first line must contain coordinator name; got: %s", first)
		}
	})

	t.Run("trust material paths appear without revealing content", func(t *testing.T) {
		cfg := config.CoordinatorConfig{
			Identity: config.IdentityMetadata{Name: "coord-beta"},
			Trust: config.CoordinatorTrustConfig{
				RootAnchorPath:       "/pki/root.crt",
				IntermediateCertPath: "/pki/inter.crt",
				IntermediateKeyPath:  "/pki/inter.key",
			},
		}
		lines := coordinatorConfigSummaryLines(cfg)
		joined := strings.Join(lines, "\n")

		if !strings.Contains(joined, "/pki/root.crt") {
			t.Errorf("root anchor path must appear; got:\n%s", joined)
		}
		if !strings.Contains(joined, "/pki/inter.crt") {
			t.Errorf("intermediate cert path must appear; got:\n%s", joined)
		}
		if !strings.Contains(joined, "/pki/inter.key") {
			t.Errorf("intermediate key path must appear; got:\n%s", joined)
		}
		// The output must direct operators to 'bootstrap' for readiness, not
		// expose certificate content here (wrong command for that).
		if !strings.Contains(joined, "tlctl coordinator bootstrap") {
			t.Errorf("output must reference 'tlctl coordinator bootstrap' for readiness; got:\n%s", joined)
		}
	})

	t.Run("QUIC and TCP transport shown separately", func(t *testing.T) {
		cfg := config.CoordinatorConfig{
			Identity: config.IdentityMetadata{Name: "coord-gamma"},
			Trust: config.CoordinatorTrustConfig{
				RootAnchorPath:       "/pki/root.crt",
				IntermediateCertPath: "/pki/inter.crt",
				IntermediateKeyPath:  "/pki/inter.key",
			},
			Control: config.ControlTransportConfig{
				TCP: config.TransportListenerConfig{
					Enabled:         true,
					ListenEndpoints: []string{"0.0.0.0:9000"},
				},
			},
		}
		lines := coordinatorConfigSummaryLines(cfg)
		joined := strings.Join(lines, "\n")

		// QUIC and TCP must remain distinct in the output.
		// Collapsing them would lose information about which transports are active.
		if !strings.Contains(joined, "quic=false") {
			t.Errorf("QUIC disabled must show quic=false; got:\n%s", joined)
		}
		if !strings.Contains(joined, "tcp=true") {
			t.Errorf("TCP enabled must show tcp=true; got:\n%s", joined)
		}
		if !strings.Contains(joined, "0.0.0.0:9000") {
			t.Errorf("TCP listen endpoint must appear; got:\n%s", joined)
		}
	})

	t.Run("relay not enabled shows not enabled", func(t *testing.T) {
		cfg := config.CoordinatorConfig{
			Identity: config.IdentityMetadata{Name: "coord-delta"},
			Trust: config.CoordinatorTrustConfig{
				RootAnchorPath:       "/pki/root.crt",
				IntermediateCertPath: "/pki/inter.crt",
				IntermediateKeyPath:  "/pki/inter.key",
			},
		}
		lines := coordinatorConfigSummaryLines(cfg)
		joined := strings.Join(lines, "\n")

		if !strings.Contains(joined, "relay: (not enabled)") {
			t.Errorf("disabled relay must show '(not enabled)'; got:\n%s", joined)
		}
	})
}

// TestNodeConfigSummaryLinesBootstrapCoordinators verifies bootstrap coordinator
// output is labeled as configured (not currently connected).
func TestNodeConfigSummaryLinesBootstrapCoordinators(t *testing.T) {
	cfg := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-lambda"},
		BootstrapCoordinators: []config.BootstrapCoordinatorConfig{
			{
				Label:            "primary",
				ControlEndpoints: []string{"10.0.0.1:9000"},
			},
		},
	}
	lines := nodeConfigSummaryLines(cfg)
	joined := strings.Join(lines, "\n")

	// Bootstrap coordinators must be labeled as configured, not connected.
	// "currently-connected" would imply live session state.
	if !strings.Contains(joined, "not currently-connected") {
		t.Errorf("bootstrap coordinator must be labeled 'not currently-connected'; got:\n%s", joined)
	}
	if !strings.Contains(joined, "primary") {
		t.Errorf("coordinator label must appear; got:\n%s", joined)
	}
	if !strings.Contains(joined, "10.0.0.1:9000") {
		t.Errorf("control endpoint must appear; got:\n%s", joined)
	}
}

// TestNodeConfigSummaryLinesRelayEndpoint verifies relay endpoints appear
// in association output alongside direct endpoints.
func TestNodeConfigSummaryLinesRelayEndpoint(t *testing.T) {
	cfg := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-mu"},
		Associations: []config.AssociationConfig{
			{
				SourceService:      "svc-a",
				DestinationNode:    "node-nu",
				DestinationService: "svc-b",
				RelayEndpoint:      "10.0.0.3:51840",
			},
		},
	}
	lines := nodeConfigSummaryLines(cfg)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "relay=10.0.0.3:51840") {
		t.Errorf("relay endpoint must appear in association output; got:\n%s", joined)
	}
	// Direct must still show (none) even when only relay is configured.
	if !strings.Contains(joined, "direct=(none)") {
		t.Errorf("absent direct endpoint must show '(none)' even when relay is set; got:\n%s", joined)
	}
}
