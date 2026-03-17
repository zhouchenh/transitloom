package node_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/coordinator"
	"github.com/zhouchenh/transitloom/internal/node"
	"github.com/zhouchenh/transitloom/internal/pki"
	"github.com/zhouchenh/transitloom/internal/service"
)

func TestBuildAssociationRequest(t *testing.T) {
	t.Parallel()

	cfg := validNodeConfigWithAssociation()
	bootstrap := readyBootstrapState()

	request, err := node.BuildAssociationRequest(cfg, bootstrap)
	if err != nil {
		t.Fatalf("BuildAssociationRequest() error = %v", err)
	}
	if request.ProtocolVersion != controlplane.BootstrapProtocolVersion {
		t.Fatalf("ProtocolVersion = %q, want %q", request.ProtocolVersion, controlplane.BootstrapProtocolVersion)
	}
	if request.NodeName != "node-a" {
		t.Fatalf("NodeName = %q, want %q", request.NodeName, "node-a")
	}
	if len(request.Associations) != 1 {
		t.Fatalf("len(Associations) = %d, want 1", len(request.Associations))
	}
	if request.Associations[0].SourceService.Name != "wg-home" {
		t.Fatalf("SourceService.Name = %q, want %q", request.Associations[0].SourceService.Name, "wg-home")
	}
	if request.Associations[0].DestinationNode != "node-b" {
		t.Fatalf("DestinationNode = %q, want %q", request.Associations[0].DestinationNode, "node-b")
	}
}

func TestBuildAssociationRequestFailsWithoutAssociations(t *testing.T) {
	t.Parallel()

	cfg := validNodeConfigWithAssociation()
	cfg.Associations = nil
	bootstrap := readyBootstrapState()

	_, err := node.BuildAssociationRequest(cfg, bootstrap)
	if err == nil {
		t.Fatal("BuildAssociationRequest() should fail when no associations configured")
	}
}

func TestAttemptAssociationEndToEnd(t *testing.T) {
	t.Parallel()

	listener, err := coordinator.NewBootstrapListener(config.CoordinatorConfig{
		Identity: config.IdentityMetadata{Name: "coord-a"},
		Control: config.ControlTransportConfig{
			TCP: config.TransportListenerConfig{
				Enabled:         true,
				ListenEndpoints: []string{"127.0.0.1:0"},
			},
		},
	}, pki.CoordinatorBootstrapState{
		CoordinatorName: "coord-a",
		Phase:           pki.CoordinatorBootstrapPhaseReady,
	})
	if err != nil {
		t.Fatalf("NewBootstrapListener() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErr := make(chan error, 1)
	go func() {
		runErr <- listener.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-runErr; err != nil {
			t.Fatalf("BootstrapListener.Run() error = %v", err)
		}
	})

	endpoint := listener.BoundEndpoints()[0]
	client := controlplane.Client{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
	}

	// Register services for both nodes.
	registerService(t, client, endpoint, "node-a", "wg-home")
	registerService(t, client, endpoint, "node-b", "wg-home")

	// Simulate a successful bootstrap session result.
	session := node.BootstrapSessionAttemptResult{
		CoordinatorLabel: "coord-a",
		Endpoint:         endpoint,
		Response: controlplane.BootstrapSessionResponse{
			ProtocolVersion: controlplane.BootstrapProtocolVersion,
			CoordinatorName: "coord-a",
			Outcome:         controlplane.BootstrapSessionOutcomeAccepted,
			Reason:          controlplane.BootstrapSessionReasonPrerequisitesSatisfied,
			BootstrapOnly:   true,
		},
	}

	cfg := validNodeConfigWithAssociation()
	bootstrap := readyBootstrapState()

	result, err := node.AttemptAssociation(context.Background(), cfg, bootstrap, session)
	if err != nil {
		t.Fatalf("AttemptAssociation() error = %v", err)
	}
	if result.Response.Outcome != controlplane.AssociationOutcomeAccepted {
		t.Fatalf("response.Outcome = %q, want %q", result.Response.Outcome, controlplane.AssociationOutcomeAccepted)
	}
	if !result.Response.AllCreated() {
		t.Fatal("AllCreated() = false, want true")
	}

	// Verify report lines contain useful output.
	lines := result.ReportLines()
	if len(lines) == 0 {
		t.Fatal("ReportLines() returned no lines")
	}
}

func TestAttemptAssociationRejectsWithoutAcceptedSession(t *testing.T) {
	t.Parallel()

	session := node.BootstrapSessionAttemptResult{
		Response: controlplane.BootstrapSessionResponse{
			Outcome: controlplane.BootstrapSessionOutcomeRejected,
		},
	}

	cfg := validNodeConfigWithAssociation()
	bootstrap := readyBootstrapState()

	_, err := node.AttemptAssociation(context.Background(), cfg, bootstrap, session)
	if err == nil {
		t.Fatal("AttemptAssociation() should fail without accepted session")
	}
}

// --- helpers ---

func registerService(t *testing.T, client controlplane.Client, endpoint, nodeName, serviceName string) {
	t.Helper()

	request := controlplane.ServiceRegistrationRequest{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		NodeName:        nodeName,
		Readiness: controlplane.BootstrapReadinessSummary{
			OverallPhase:   controlplane.ReadinessPhaseReady,
			IdentityPhase:  "ready",
			AdmissionPhase: "usable",
			CachedToken: &controlplane.BootstrapTokenSummary{
				TokenID:             "tok-1",
				NodeID:              nodeName,
				IssuerCoordinatorID: "coord-a",
				IssuedAt:            time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
				ExpiresAt:           time.Date(2026, 3, 16, 12, 30, 0, 0, time.UTC),
			},
		},
		Services: []service.Registration{
			{
				Identity: service.Identity{Name: serviceName, Type: config.ServiceTypeRawUDP},
				Metadata: service.Metadata{Discoverable: true},
				Binding: service.Binding{
					LocalTarget: service.LocalTarget{Address: "127.0.0.1", Port: 51820},
				},
			},
		},
	}

	response, err := client.RegisterServices(context.Background(), endpoint, request)
	if err != nil {
		t.Fatalf("RegisterServices(%s) error = %v", nodeName, err)
	}
	if response.Outcome != controlplane.ServiceRegistrationOutcomeAccepted {
		t.Fatalf("RegisterServices(%s) outcome = %q", nodeName, response.Outcome)
	}
}

func validNodeConfigWithAssociation() config.NodeConfig {
	return config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-a"},
		Services: []config.ServiceConfig{
			{
				Name: "wg-home",
				Type: config.ServiceTypeRawUDP,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    51820,
				},
			},
		},
		Associations: []config.AssociationConfig{
			{
				SourceService:      "wg-home",
				DestinationNode:    "node-b",
				DestinationService: "wg-home",
			},
		},
	}
}
