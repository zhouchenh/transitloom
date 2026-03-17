package node_test

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/coordinator"
	"github.com/zhouchenh/transitloom/internal/node"
	"github.com/zhouchenh/transitloom/internal/pki"
)

// TestAttemptBootstrapSessionConnectionRefusedSkipsImmediately verifies that a
// connection-refused transport error is classified correctly and does not
// trigger retries. Because connection-refused means the endpoint is not
// listening at all, retrying wastes time and does not help.
func TestAttemptBootstrapSessionConnectionRefusedSkipsImmediately(t *testing.T) {
	t.Parallel()

	// Start a real coordinator so we have a valid endpoint to succeed on.
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
	go func() { runErr <- listener.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-runErr; err != nil {
			t.Fatalf("BootstrapListener.Run() error = %v", err)
		}
	})

	// First endpoint: a closed port that will return connection-refused.
	// Second endpoint: the live coordinator.
	cfg := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-a"},
		BootstrapCoordinators: []config.BootstrapCoordinatorConfig{
			{
				Label: "coord-a",
				ControlEndpoints: []string{
					closedLoopbackEndpoint(t),
					listener.BoundEndpoints()[0],
				},
			},
		},
	}

	start := time.Now()
	result, err := node.AttemptBootstrapSession(context.Background(), cfg, readyBootstrapState())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("AttemptBootstrapSession() error = %v", err)
	}
	if !result.Response.Accepted() {
		t.Fatalf("response.Accepted() = false, want true; outcome=%s reason=%s", result.Response.Outcome, result.Response.Reason)
	}

	// Exactly one failed attempt (connection-refused, no retries).
	if got := len(result.FailedAttempts); got != 1 {
		t.Fatalf("len(FailedAttempts) = %d, want 1 (no retries for connection-refused)", got)
	}

	fa := result.FailedAttempts[0]
	if fa.ErrorKind != controlplane.TransportErrorKindConnectionRefused {
		t.Errorf("FailedAttempts[0].ErrorKind = %q, want %q", fa.ErrorKind, controlplane.TransportErrorKindConnectionRefused)
	}

	// Connection-refused should not add noticeable retry latency.
	// 2 seconds is generous; actual latency should be well under 100 ms.
	if elapsed > 2*time.Second {
		t.Errorf("elapsed = %v, want < 2s (connection-refused should not retry)", elapsed)
	}
}

// TestAttemptBootstrapSessionErrorKindInReportLines verifies that the error
// kind is included in report lines so operators can understand why transport
// attempts failed without parsing raw error strings.
func TestAttemptBootstrapSessionErrorKindInReportLines(t *testing.T) {
	t.Parallel()

	cfg := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-a"},
		BootstrapCoordinators: []config.BootstrapCoordinatorConfig{
			{
				Label:            "coord-a",
				ControlEndpoints: []string{closedLoopbackEndpoint(t)},
			},
		},
	}

	// This will fail — the closed endpoint returns connection-refused.
	result, _ := node.AttemptBootstrapSession(context.Background(), cfg, readyBootstrapState())

	if len(result.FailedAttempts) == 0 {
		t.Fatal("expected at least one failed attempt")
	}
	report := strings.Join(result.ReportLines(), "\n")

	// Report lines must include the error kind so operators can classify
	// failures without parsing error strings.
	if !strings.Contains(report, string(controlplane.TransportErrorKindConnectionRefused)) {
		t.Errorf("report lines do not include error kind %q:\n%s", controlplane.TransportErrorKindConnectionRefused, report)
	}
	// The kind field must be explicitly labeled in the report.
	if !strings.Contains(report, "kind=") {
		t.Errorf("report lines do not include 'kind=' label:\n%s", report)
	}
}

// TestBootstrapListenerShutdownClean verifies that BootstrapListener.Run
// returns cleanly (nil error) when its context is canceled.
func TestBootstrapListenerShutdownClean(t *testing.T) {
	t.Parallel()

	listener, err := coordinator.NewBootstrapListener(config.CoordinatorConfig{
		Identity: config.IdentityMetadata{Name: "coord-shutdown"},
		Control: config.ControlTransportConfig{
			TCP: config.TransportListenerConfig{
				Enabled:         true,
				ListenEndpoints: []string{"127.0.0.1:0"},
			},
		},
	}, pki.CoordinatorBootstrapState{
		CoordinatorName: "coord-shutdown",
		Phase:           pki.CoordinatorBootstrapPhaseReady,
	})
	if err != nil {
		t.Fatalf("NewBootstrapListener() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	runErr := make(chan error, 1)
	go func() { runErr <- listener.Run(ctx) }()

	// Let the listener start accepting connections.
	time.Sleep(20 * time.Millisecond)

	// Cancel the context and wait for Run to return within the shutdown window.
	cancel()

	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("BootstrapListener.Run() after context cancel = %v, want nil", err)
		}
	case <-time.After(controlplane.BootstrapServerShutdownTimeout + time.Second):
		t.Fatal("BootstrapListener.Run() did not return within shutdown timeout")
	}
}

// TestBootstrapListenerRejectsOversizedBody verifies that the coordinator
// rejects requests whose bodies exceed BootstrapMaxRequestBodyBytes.
func TestBootstrapListenerRejectsOversizedBody(t *testing.T) {
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
	go func() { runErr <- listener.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-runErr; err != nil {
			t.Fatalf("BootstrapListener.Run() error = %v", err)
		}
	})

	// Build a body that exceeds BootstrapMaxRequestBodyBytes.
	// The body is not valid JSON so the server cannot return 200 OK under any
	// code path that would indicate it accepted the payload.
	oversizedBody := bytes.Repeat([]byte("X"), controlplane.BootstrapMaxRequestBodyBytes+1)

	endpoint := listener.BoundEndpoints()[0]
	httpClient := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://"+endpoint+controlplane.BootstrapSessionPath,
		bytes.NewReader(oversizedBody),
	)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		// Some platforms may close the connection before returning a status.
		// That is also a valid rejection behavior.
		t.Logf("POST oversized body returned error (connection closed by server): %v", err)
		return
	}
	defer resp.Body.Close()

	// Any non-200 status means the server correctly rejected the oversized body.
	if resp.StatusCode == http.StatusOK {
		t.Errorf("status = 200, want non-200 for oversized body exceeding %d bytes", controlplane.BootstrapMaxRequestBodyBytes)
	}
}

// TestAttemptBootstrapSessionContextCanceledAborts verifies that canceling
// the context during an attempt causes AttemptBootstrapSession to abort
// immediately rather than continuing to try other endpoints.
func TestAttemptBootstrapSessionContextCanceledAborts(t *testing.T) {
	t.Parallel()

	// Use a listener that accepts TCP connections but never responds to verify
	// that context cancellation aborts the attempt cleanly.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	// Accept connections silently so the client actually connects but blocks.
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Hold the connection open without responding.
			go func() { defer conn.Close(); time.Sleep(10 * time.Second) }()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())

	cfg := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-a"},
		BootstrapCoordinators: []config.BootstrapCoordinatorConfig{
			{
				Label:            "coord-silent",
				ControlEndpoints: []string{ln.Addr().String()},
			},
		},
	}

	// Cancel the context shortly after starting the attempt.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = node.AttemptBootstrapSession(ctx, cfg, readyBootstrapState())
	elapsed := time.Since(start)

	// The call must return an error (context cancellation or timeout).
	if err == nil {
		t.Fatal("AttemptBootstrapSession() = nil, want error after context cancel")
	}

	// The call must not take longer than the connect timeout (3 s) plus a
	// small tolerance. In practice it should return almost immediately after
	// the cancel signal.
	if elapsed > controlplane.BootstrapConnectTimeout+500*time.Millisecond {
		t.Errorf("elapsed = %v, want < %v after context cancel", elapsed, controlplane.BootstrapConnectTimeout+500*time.Millisecond)
	}
}
