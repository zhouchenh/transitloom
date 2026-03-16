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
