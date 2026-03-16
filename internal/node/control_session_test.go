package node_test

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/admission"
	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/coordinator"
	"github.com/zhouchenh/transitloom/internal/identity"
	"github.com/zhouchenh/transitloom/internal/node"
	"github.com/zhouchenh/transitloom/internal/pki"
)

func TestAttemptBootstrapSession(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		bootstrapState      node.BootstrapState
		wantAccepted        bool
		wantReasonSubstring string
		wantFailedAttempts  int
	}{
		{
			name:                "falls back to second endpoint and succeeds",
			bootstrapState:      readyBootstrapState(),
			wantAccepted:        true,
			wantReasonSubstring: "bootstrap-prerequisites-satisfied",
			wantFailedAttempts:  1,
		},
		{
			name:                "receives structured rejection for missing admission token",
			bootstrapState:      admissionMissingBootstrapState(),
			wantAccepted:        false,
			wantReasonSubstring: "node-admission-missing",
			wantFailedAttempts:  1,
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

			cfg := config.NodeConfig{
				Identity: config.IdentityMetadata{Name: "node-a"},
				BootstrapCoordinators: []config.BootstrapCoordinatorConfig{
					{
						Label:            "coord-a",
						ControlEndpoints: []string{closedLoopbackEndpoint(t), listener.BoundEndpoints()[0]},
					},
				},
			}

			attemptCtx, attemptCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer attemptCancel()

			result, err := node.AttemptBootstrapSession(attemptCtx, cfg, tt.bootstrapState)
			if err != nil {
				t.Fatalf("AttemptBootstrapSession() error = %v", err)
			}

			if got := result.Response.Accepted(); got != tt.wantAccepted {
				t.Fatalf("response.Accepted() = %t, want %t", got, tt.wantAccepted)
			}
			if got := len(result.FailedAttempts); got != tt.wantFailedAttempts {
				t.Fatalf("len(result.FailedAttempts) = %d, want %d", got, tt.wantFailedAttempts)
			}
			report := strings.Join(result.ReportLines(), "\n")
			if !strings.Contains(report, tt.wantReasonSubstring) {
				t.Fatalf("ReportLines() = %q, want substring %q", report, tt.wantReasonSubstring)
			}
			if !strings.Contains(report, "node bootstrap control attempt failed:") {
				t.Fatalf("ReportLines() = %q, want fallback failure line", report)
			}
		})
	}
}

func readyBootstrapState() node.BootstrapState {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)

	return node.BootstrapState{
		Identity: identity.NodeIdentityState{
			Phase: identity.NodeIdentityPhaseReady,
		},
		Admission: admission.TokenCacheState{
			Phase: admission.TokenCachePhaseUsable,
			Token: &admission.CachedTokenRecord{
				TokenID:             "tok-1",
				NodeID:              "node-a",
				IssuerCoordinatorID: "coord-a",
				IssuedAt:            now.Add(-5 * time.Minute),
				ExpiresAt:           now.Add(30 * time.Minute),
			},
		},
		Phase: node.BootstrapPhaseReady,
	}
}

func admissionMissingBootstrapState() node.BootstrapState {
	return node.BootstrapState{
		Identity: identity.NodeIdentityState{
			Phase: identity.NodeIdentityPhaseReady,
		},
		Admission: admission.TokenCacheState{
			Phase: admission.TokenCachePhaseMissing,
		},
		Phase: node.BootstrapPhaseAdmissionMissing,
	}
}

func closedLoopbackEndpoint(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return addr
}
