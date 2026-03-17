package coordinator_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/coordinator"
	"github.com/zhouchenh/transitloom/internal/pki"
	"github.com/zhouchenh/transitloom/internal/service"
)

func TestBootstrapListenerAssociationCreatesRecord(t *testing.T) {
	t.Parallel()

	listener := startTestListener(t)

	// Register services for both nodes first.
	client := controlplane.Client{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
	}

	registerNodeServices(t, client, listener.BoundEndpoints()[0], "node-a", "wg-home")
	registerNodeServices(t, client, listener.BoundEndpoints()[0], "node-b", "wg-home")

	// Request association.
	response, err := client.RequestAssociations(context.Background(), listener.BoundEndpoints()[0], validAssociationRequest("node-a", "wg-home", "node-b", "wg-home"))
	if err != nil {
		t.Fatalf("Client.RequestAssociations() error = %v", err)
	}

	if response.Outcome != controlplane.AssociationOutcomeAccepted {
		t.Fatalf("response.Outcome = %q, want %q", response.Outcome, controlplane.AssociationOutcomeAccepted)
	}
	if response.Reason != controlplane.AssociationReasonCreated {
		t.Fatalf("response.Reason = %q, want %q", response.Reason, controlplane.AssociationReasonCreated)
	}
	if response.AcceptedCount != 1 || response.RejectedCount != 0 {
		t.Fatalf("response counts = accepted:%d rejected:%d, want accepted:1 rejected:0", response.AcceptedCount, response.RejectedCount)
	}
	if !response.BootstrapOnly {
		t.Fatal("response.BootstrapOnly = false, want true")
	}

	// Verify stored association record.
	snapshot := listener.AssociationSnapshot()
	if got := len(snapshot); got != 1 {
		t.Fatalf("len(AssociationSnapshot()) = %d, want 1", got)
	}

	record := snapshot[0]
	if record.SourceNode != "node-a" {
		t.Fatalf("record.SourceNode = %q, want %q", record.SourceNode, "node-a")
	}
	if record.SourceService.Name != "wg-home" {
		t.Fatalf("record.SourceService.Name = %q, want %q", record.SourceService.Name, "wg-home")
	}
	if record.DestinationNode != "node-b" {
		t.Fatalf("record.DestinationNode = %q, want %q", record.DestinationNode, "node-b")
	}
	if record.DestinationService.Name != "wg-home" {
		t.Fatalf("record.DestinationService.Name = %q, want %q", record.DestinationService.Name, "wg-home")
	}
	if record.State != service.AssociationStatePending {
		t.Fatalf("record.State = %q, want %q", record.State, service.AssociationStatePending)
	}
	if !record.BootstrapOnly {
		t.Fatal("record.BootstrapOnly = false, want true")
	}
}

func TestBootstrapListenerAssociationRejectsUnregisteredSource(t *testing.T) {
	t.Parallel()

	listener := startTestListener(t)
	client := controlplane.Client{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
	}

	// Only register destination, not source.
	registerNodeServices(t, client, listener.BoundEndpoints()[0], "node-b", "wg-home")

	response, err := client.RequestAssociations(context.Background(), listener.BoundEndpoints()[0], validAssociationRequest("node-a", "wg-home", "node-b", "wg-home"))
	if err != nil {
		t.Fatalf("Client.RequestAssociations() error = %v", err)
	}

	if response.Outcome != controlplane.AssociationOutcomeRejected {
		t.Fatalf("response.Outcome = %q, want %q", response.Outcome, controlplane.AssociationOutcomeRejected)
	}
	if response.AcceptedCount != 0 {
		t.Fatalf("response.AcceptedCount = %d, want 0", response.AcceptedCount)
	}

	if len(response.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(response.Results))
	}
	if response.Results[0].Reason != controlplane.AssociationResultReasonSourceServiceNotRegistered {
		t.Fatalf("result.Reason = %q, want %q", response.Results[0].Reason, controlplane.AssociationResultReasonSourceServiceNotRegistered)
	}
}

func TestBootstrapListenerAssociationRejectsUnregisteredDestination(t *testing.T) {
	t.Parallel()

	listener := startTestListener(t)
	client := controlplane.Client{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
	}

	// Only register source, not destination.
	registerNodeServices(t, client, listener.BoundEndpoints()[0], "node-a", "wg-home")

	response, err := client.RequestAssociations(context.Background(), listener.BoundEndpoints()[0], validAssociationRequest("node-a", "wg-home", "node-b", "wg-home"))
	if err != nil {
		t.Fatalf("Client.RequestAssociations() error = %v", err)
	}

	if response.Outcome != controlplane.AssociationOutcomeRejected {
		t.Fatalf("response.Outcome = %q, want %q", response.Outcome, controlplane.AssociationOutcomeRejected)
	}
	if response.Results[0].Reason != controlplane.AssociationResultReasonDestServiceNotRegistered {
		t.Fatalf("result.Reason = %q, want %q", response.Results[0].Reason, controlplane.AssociationResultReasonDestServiceNotRegistered)
	}
}

func TestBootstrapListenerAssociationRejectsSelfAssociation(t *testing.T) {
	t.Parallel()

	listener := startTestListener(t)
	client := controlplane.Client{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
	}

	registerNodeServices(t, client, listener.BoundEndpoints()[0], "node-a", "wg-home")

	// Try to associate node-a/wg-home with itself.
	response, err := client.RequestAssociations(context.Background(), listener.BoundEndpoints()[0], validAssociationRequest("node-a", "wg-home", "node-a", "wg-home"))
	if err != nil {
		t.Fatalf("Client.RequestAssociations() error = %v", err)
	}

	if response.Outcome != controlplane.AssociationOutcomeRejected {
		t.Fatalf("response.Outcome = %q, want %q", response.Outcome, controlplane.AssociationOutcomeRejected)
	}
	if response.Results[0].Reason != controlplane.AssociationResultReasonSelfAssociation {
		t.Fatalf("result.Reason = %q, want %q", response.Results[0].Reason, controlplane.AssociationResultReasonSelfAssociation)
	}
}

func TestBootstrapListenerAssociationPartialRejectsMixed(t *testing.T) {
	t.Parallel()

	listener := startTestListener(t)
	client := controlplane.Client{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
	}

	// Register both node-a and node-b services.
	registerNodeServices(t, client, listener.BoundEndpoints()[0], "node-a", "wg-home")
	registerNodeServices(t, client, listener.BoundEndpoints()[0], "node-b", "wg-home")

	// Request two associations: one valid, one with unregistered destination.
	request := validAssociationRequest("node-a", "wg-home", "node-b", "wg-home")
	request.Associations = append(request.Associations, service.AssociationIntent{
		SourceService:      service.Identity{Name: "wg-home", Type: config.ServiceTypeRawUDP},
		DestinationNode:    "node-c",
		DestinationService: service.Identity{Name: "wg-home", Type: config.ServiceTypeRawUDP},
	})

	response, err := client.RequestAssociations(context.Background(), listener.BoundEndpoints()[0], request)
	if err != nil {
		t.Fatalf("Client.RequestAssociations() error = %v", err)
	}

	if response.Outcome != controlplane.AssociationOutcomePartial {
		t.Fatalf("response.Outcome = %q, want %q", response.Outcome, controlplane.AssociationOutcomePartial)
	}
	if response.AcceptedCount != 1 || response.RejectedCount != 1 {
		t.Fatalf("response counts = accepted:%d rejected:%d, want accepted:1 rejected:1", response.AcceptedCount, response.RejectedCount)
	}
}

func TestBootstrapListenerAssociationDistinctFromRegistration(t *testing.T) {
	t.Parallel()

	listener := startTestListener(t)
	client := controlplane.Client{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
	}

	// Register services.
	registerNodeServices(t, client, listener.BoundEndpoints()[0], "node-a", "wg-home")
	registerNodeServices(t, client, listener.BoundEndpoints()[0], "node-b", "wg-home")

	// Before association: registry has 2 records, association store is empty.
	if got := len(listener.RegistrySnapshot()); got != 2 {
		t.Fatalf("len(RegistrySnapshot()) = %d, want 2", got)
	}
	if got := len(listener.AssociationSnapshot()); got != 0 {
		t.Fatalf("len(AssociationSnapshot()) before association = %d, want 0", got)
	}

	// Create association.
	_, err := client.RequestAssociations(context.Background(), listener.BoundEndpoints()[0], validAssociationRequest("node-a", "wg-home", "node-b", "wg-home"))
	if err != nil {
		t.Fatalf("Client.RequestAssociations() error = %v", err)
	}

	// After association: registry still has 2 records, association store has 1.
	if got := len(listener.RegistrySnapshot()); got != 2 {
		t.Fatalf("len(RegistrySnapshot()) after association = %d, want 2", got)
	}
	if got := len(listener.AssociationSnapshot()); got != 1 {
		t.Fatalf("len(AssociationSnapshot()) after association = %d, want 1", got)
	}
}

// --- helpers ---

func startTestListener(t *testing.T) *coordinator.BootstrapListener {
	t.Helper()

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

	return listener
}

func registerNodeServices(t *testing.T, client controlplane.Client, endpoint, nodeName, serviceName string) {
	t.Helper()

	request := controlplane.ServiceRegistrationRequest{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		NodeName:        nodeName,
		Readiness:       validBootstrapRequest().Readiness,
		Services: []service.Registration{
			{
				Identity: service.Identity{
					Name: serviceName,
					Type: config.ServiceTypeRawUDP,
				},
				Metadata: service.Metadata{
					Discoverable: true,
				},
				Binding: service.Binding{
					LocalTarget: service.LocalTarget{
						Address: "127.0.0.1",
						Port:    51820,
					},
				},
			},
		},
	}

	response, err := client.RegisterServices(context.Background(), endpoint, request)
	if err != nil {
		t.Fatalf("RegisterServices(%s) error = %v", nodeName, err)
	}
	if response.Outcome != controlplane.ServiceRegistrationOutcomeAccepted {
		t.Fatalf("RegisterServices(%s) outcome = %q, want %q", nodeName, response.Outcome, controlplane.ServiceRegistrationOutcomeAccepted)
	}
}

func validAssociationRequest(sourceNode, sourceService, destNode, destService string) controlplane.AssociationRequest {
	return controlplane.AssociationRequest{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		NodeName:        sourceNode,
		Readiness:       validBootstrapRequest().Readiness,
		Associations: []service.AssociationIntent{
			{
				SourceService:      service.Identity{Name: sourceService, Type: config.ServiceTypeRawUDP},
				DestinationNode:    destNode,
				DestinationService: service.Identity{Name: destService, Type: config.ServiceTypeRawUDP},
			},
		},
	}
}
