package coordinator_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/coordinator"
	"github.com/zhouchenh/transitloom/internal/pki"
	"github.com/zhouchenh/transitloom/internal/service"
)

func TestBootstrapListenerResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		bootstrap   pki.CoordinatorBootstrapState
		wantOutcome controlplane.BootstrapSessionOutcome
		wantReason  controlplane.BootstrapSessionReason
	}{
		{
			name: "accepted when coordinator bootstrap is ready",
			bootstrap: pki.CoordinatorBootstrapState{
				CoordinatorName: "coord-a",
				Phase:           pki.CoordinatorBootstrapPhaseReady,
			},
			wantOutcome: controlplane.BootstrapSessionOutcomeAccepted,
			wantReason:  controlplane.BootstrapSessionReasonPrerequisitesSatisfied,
		},
		{
			name: "rejected when coordinator is still awaiting intermediate material",
			bootstrap: pki.CoordinatorBootstrapState{
				CoordinatorName: "coord-a",
				Phase:           pki.CoordinatorBootstrapPhaseAwaitingIntermediate,
			},
			wantOutcome: controlplane.BootstrapSessionOutcomeRejected,
			wantReason:  controlplane.BootstrapSessionReasonCoordinatorAwaitingMaterial,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			listener, err := coordinator.NewBootstrapListener(config.CoordinatorConfig{
				Identity: config.IdentityMetadata{Name: "coord-a"},
				Control: config.ControlTransportConfig{
					TCP: config.TransportListenerConfig{
						Enabled:         true,
						ListenEndpoints: []string{"127.0.0.1:0"},
					},
				},
			}, tt.bootstrap)
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

			client := controlplane.Client{
				HTTPClient: &http.Client{Timeout: 2 * time.Second},
			}

			response, err := client.Attempt(context.Background(), listener.BoundEndpoints()[0], validBootstrapRequest())
			if err != nil {
				t.Fatalf("Client.Attempt() error = %v", err)
			}

			if response.Outcome != tt.wantOutcome {
				t.Fatalf("response.Outcome = %q, want %q", response.Outcome, tt.wantOutcome)
			}
			if response.Reason != tt.wantReason {
				t.Fatalf("response.Reason = %q, want %q", response.Reason, tt.wantReason)
			}
			if !response.BootstrapOnly {
				t.Fatal("response.BootstrapOnly = false, want true")
			}
		})
	}
}

func validBootstrapRequest() controlplane.BootstrapSessionRequest {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)

	return controlplane.BootstrapSessionRequest{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		NodeName:        "node-a",
		Readiness: controlplane.BootstrapReadinessSummary{
			OverallPhase:   controlplane.ReadinessPhaseReady,
			IdentityPhase:  "ready",
			AdmissionPhase: "usable",
			CachedToken: &controlplane.BootstrapTokenSummary{
				TokenID:             "tok-1",
				NodeID:              "node-a",
				IssuerCoordinatorID: "coord-a",
				IssuedAt:            now.Add(-5 * time.Minute),
				ExpiresAt:           now.Add(30 * time.Minute),
			},
		},
	}
}

func TestBootstrapListenerServiceRegistrationStoresRecords(t *testing.T) {
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

	client := controlplane.Client{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
	}

	response, err := client.RegisterServices(context.Background(), listener.BoundEndpoints()[0], validServiceRegistrationRequest())
	if err != nil {
		t.Fatalf("Client.RegisterServices() error = %v", err)
	}

	if response.Outcome != controlplane.ServiceRegistrationOutcomeAccepted {
		t.Fatalf("response.Outcome = %q, want %q", response.Outcome, controlplane.ServiceRegistrationOutcomeAccepted)
	}
	if response.Reason != controlplane.ServiceRegistrationReasonRegistered {
		t.Fatalf("response.Reason = %q, want %q", response.Reason, controlplane.ServiceRegistrationReasonRegistered)
	}
	if response.AcceptedCount != 1 || response.RejectedCount != 0 {
		t.Fatalf("response counts = accepted:%d rejected:%d, want accepted:1 rejected:0", response.AcceptedCount, response.RejectedCount)
	}

	snapshot := listener.RegistrySnapshot()
	if got := len(snapshot); got != 1 {
		t.Fatalf("len(snapshot) = %d, want 1", got)
	}

	record := snapshot[0]
	if record.Identity.Name != "wg-home" {
		t.Fatalf("record.Identity.Name = %q, want %q", record.Identity.Name, "wg-home")
	}
	if record.Binding.LocalTarget.Port != 51820 {
		t.Fatalf("record.Binding.LocalTarget.Port = %d, want 51820", record.Binding.LocalTarget.Port)
	}
	if record.RequestedLocalIngress == nil {
		t.Fatal("record.RequestedLocalIngress = nil, want non-nil")
	}
	if record.RequestedLocalIngress.RangeStart != 61000 || record.RequestedLocalIngress.RangeEnd != 61999 {
		t.Fatalf("record.RequestedLocalIngress range = %d-%d, want 61000-61999", record.RequestedLocalIngress.RangeStart, record.RequestedLocalIngress.RangeEnd)
	}
	if !record.BootstrapOnly {
		t.Fatal("record.BootstrapOnly = false, want true")
	}
}

func TestBootstrapListenerServiceRegistrationPartiallyRejectsInvalidDeclarations(t *testing.T) {
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

	request := validServiceRegistrationRequest()
	request.Services = append(request.Services, service.Registration{
		Identity: service.Identity{
			Name: "broken",
			Type: config.ServiceTypeRawUDP,
		},
		Binding: service.Binding{
			LocalTarget: service.LocalTarget{
				Address: "bad-ip",
				Port:    5300,
			},
		},
	})

	client := controlplane.Client{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
	}

	response, err := client.RegisterServices(context.Background(), listener.BoundEndpoints()[0], request)
	if err != nil {
		t.Fatalf("Client.RegisterServices() error = %v", err)
	}

	if response.Outcome != controlplane.ServiceRegistrationOutcomePartial {
		t.Fatalf("response.Outcome = %q, want %q", response.Outcome, controlplane.ServiceRegistrationOutcomePartial)
	}
	if response.Reason != controlplane.ServiceRegistrationReasonPartiallyRegistered {
		t.Fatalf("response.Reason = %q, want %q", response.Reason, controlplane.ServiceRegistrationReasonPartiallyRegistered)
	}
	if response.AcceptedCount != 1 || response.RejectedCount != 1 {
		t.Fatalf("response counts = accepted:%d rejected:%d, want accepted:1 rejected:1", response.AcceptedCount, response.RejectedCount)
	}

	if got := len(listener.RegistrySnapshot()); got != 1 {
		t.Fatalf("len(listener.RegistrySnapshot()) = %d, want 1", got)
	}

	foundInvalid := false
	for _, result := range response.Results {
		if result.ServiceName != "broken" {
			continue
		}
		foundInvalid = true
		if result.Reason != controlplane.ServiceRegistrationResultReasonInvalidServiceDecl {
			t.Fatalf("broken service reason = %q, want %q", result.Reason, controlplane.ServiceRegistrationResultReasonInvalidServiceDecl)
		}
		if !strings.Contains(strings.Join(result.Details, "\n"), "invalid") {
			t.Fatalf("broken service details = %q, want invalid detail", strings.Join(result.Details, "\n"))
		}
	}
	if !foundInvalid {
		t.Fatal("response.Results did not include the invalid service result")
	}
}

func validServiceRegistrationRequest() controlplane.ServiceRegistrationRequest {
	return controlplane.ServiceRegistrationRequest{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		NodeName:        "node-a",
		Readiness:       validBootstrapRequest().Readiness,
		Services: []service.Registration{
			{
				Identity: service.Identity{
					Name: "wg-home",
					Type: config.ServiceTypeRawUDP,
				},
				Metadata: service.Metadata{
					Labels:       []string{"wireguard"},
					PolicyLabels: []string{"lab"},
					Discoverable: true,
				},
				Binding: service.Binding{
					LocalTarget: service.LocalTarget{
						Address: "127.0.0.1",
						Port:    51820,
					},
				},
				RequestedLocalIngress: &service.LocalIngressIntent{
					Mode:            config.IngressModeDeterministicRange,
					LoopbackAddress: "127.0.0.1",
					RangeStart:      61000,
					RangeEnd:        61999,
				},
			},
		},
	}
}
