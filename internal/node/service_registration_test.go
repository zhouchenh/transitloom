package node_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/coordinator"
	"github.com/zhouchenh/transitloom/internal/node"
	"github.com/zhouchenh/transitloom/internal/pki"
)

func TestAttemptServiceRegistration(t *testing.T) {
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
		LocalIngress: config.LocalIngressPolicyConfig{
			DefaultMode:     config.IngressModeDeterministicRange,
			RangeStart:      61000,
			RangeEnd:        61999,
			LoopbackAddress: "127.0.0.1",
		},
		Services: []config.ServiceConfig{
			{
				Name:         "wg-home",
				Type:         config.ServiceTypeRawUDP,
				Discoverable: true,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    51820,
				},
				Ingress: &config.ServiceIngressConfig{
					Mode: config.IngressModeDeterministicRange,
				},
			},
		},
	}

	session := node.BootstrapSessionAttemptResult{
		CoordinatorLabel: "coord-a",
		Endpoint:         listener.BoundEndpoints()[0],
		Response: controlplane.BootstrapSessionResponse{
			ProtocolVersion: controlplane.BootstrapProtocolVersion,
			CoordinatorName: "coord-a",
			Outcome:         controlplane.BootstrapSessionOutcomeAccepted,
			Reason:          controlplane.BootstrapSessionReasonPrerequisitesSatisfied,
			BootstrapOnly:   true,
		},
	}

	clientCtx, clientCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer clientCancel()

	result, err := node.AttemptServiceRegistration(clientCtx, cfg, readyBootstrapState(), session)
	if err != nil {
		t.Fatalf("AttemptServiceRegistration() error = %v", err)
	}
	if !result.Response.AllRegistered() {
		t.Fatalf("result.Response.AllRegistered() = false, want true")
	}

	report := strings.Join(result.ReportLines(), "\n")
	if !strings.Contains(report, "node bootstrap service registration outcome: accepted") {
		t.Fatalf("ReportLines() = %q, want accepted outcome line", report)
	}
	if !strings.Contains(report, "requested local ingress intent was captured separately from the service binding") {
		t.Fatalf("ReportLines() = %q, want requested local ingress detail", report)
	}

	snapshot := listener.RegistrySnapshot()
	if got := len(snapshot); got != 1 {
		t.Fatalf("len(snapshot) = %d, want 1", got)
	}
	if snapshot[0].RequestedLocalIngress == nil {
		t.Fatal("snapshot[0].RequestedLocalIngress = nil, want non-nil")
	}
	if snapshot[0].Binding.LocalTarget.Port != 51820 {
		t.Fatalf("snapshot[0].Binding.LocalTarget.Port = %d, want 51820", snapshot[0].Binding.LocalTarget.Port)
	}
}
