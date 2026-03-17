package transport_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/transport"
)

// TestIsProbeDatagram verifies that IsProbeDatagram correctly identifies
// probe datagrams by their magic header.
func TestIsProbeDatagram(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		buf  []byte
		want bool
	}{
		{
			"probe request magic",
			[]byte{0x54, 0x4C, 0x50, 0x52, 0x01, 0, 0, 0, 0, 0, 0, 0, 0},
			true,
		},
		{
			"probe response magic",
			[]byte{0x54, 0x4C, 0x50, 0x52, 0x02, 0, 0, 0, 0, 0, 0, 0, 0},
			true,
		},
		{
			"wrong magic",
			[]byte{0x00, 0x00, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0, 0, 0},
			false,
		},
		{
			"too short",
			[]byte{0x54, 0x4C, 0x50, 0x52},
			false,
		},
		{
			"empty",
			[]byte{},
			false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := transport.IsProbeDatagram(tc.buf)
			if got != tc.want {
				t.Errorf("IsProbeDatagram(%x) = %v, want %v", tc.buf, got, tc.want)
			}
		})
	}
}

// TestProbeCandidateValidate verifies that ProbeCandidate.Validate accepts
// structurally valid candidates and rejects invalid ones.
func TestProbeCandidateValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		c       transport.ProbeCandidate
		wantErr bool
	}{
		{
			"valid configured",
			transport.ProbeCandidate{Host: "203.0.113.1", Port: 51830, Reason: transport.CandidateReasonConfigured},
			false,
		},
		{
			"valid router-discovered",
			transport.ProbeCandidate{Host: "203.0.113.1", Port: 51830, Reason: transport.CandidateReasonRouterDiscovered},
			false,
		},
		{
			"valid coordinator-observed",
			transport.ProbeCandidate{Host: "203.0.113.1", Port: 51830, Reason: transport.CandidateReasonCoordinatorObserved},
			false,
		},
		{
			"valid previously-verified",
			transport.ProbeCandidate{Host: "203.0.113.1", Port: 51830, Reason: transport.CandidateReasonPreviouslyVerified},
			false,
		},
		{
			"empty host",
			transport.ProbeCandidate{Port: 51830, Reason: transport.CandidateReasonConfigured},
			true,
		},
		{
			"zero port",
			transport.ProbeCandidate{Host: "203.0.113.1", Port: 0, Reason: transport.CandidateReasonConfigured},
			true,
		},
		{
			"unknown reason",
			transport.ProbeCandidate{Host: "203.0.113.1", Port: 51830, Reason: "invented-reason"},
			true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.c.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// TestBuildCandidatesFromEndpoints verifies that BuildCandidatesFromEndpoints
// returns targeted candidates only for endpoints needing verification.
func TestBuildCandidatesFromEndpoints(t *testing.T) {
	t.Parallel()

	now := time.Now()

	t.Run("unverified endpoint included", func(t *testing.T) {
		t.Parallel()
		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		candidates := transport.BuildCandidatesFromEndpoints([]transport.ExternalEndpoint{ep}, false)
		if len(candidates) != 1 {
			t.Fatalf("got %d candidates, want 1", len(candidates))
		}
		if candidates[0].Host != "203.0.113.1" {
			t.Errorf("Host = %q, want 203.0.113.1", candidates[0].Host)
		}
		if candidates[0].Port != 51830 {
			t.Errorf("Port = %d, want 51830", candidates[0].Port)
		}
		if candidates[0].Reason != transport.CandidateReasonConfigured {
			t.Errorf("Reason = %q, want configured", candidates[0].Reason)
		}
	})

	t.Run("verified endpoint excluded by default", func(t *testing.T) {
		t.Parallel()
		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		ep.MarkVerified(now)
		candidates := transport.BuildCandidatesFromEndpoints([]transport.ExternalEndpoint{ep}, false)
		if len(candidates) != 0 {
			t.Errorf("got %d candidates for verified endpoint, want 0 (no re-probe needed)", len(candidates))
		}
	})

	t.Run("verified endpoint included when includeVerified=true", func(t *testing.T) {
		t.Parallel()
		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		ep.MarkVerified(now)
		candidates := transport.BuildCandidatesFromEndpoints([]transport.ExternalEndpoint{ep}, true)
		if len(candidates) != 1 {
			t.Fatalf("got %d candidates, want 1 when includeVerified=true", len(candidates))
		}
		if candidates[0].Reason != transport.CandidateReasonPreviouslyVerified {
			t.Errorf("Reason = %q, want previously-verified", candidates[0].Reason)
		}
	})

	t.Run("stale endpoint included with previously-verified reason", func(t *testing.T) {
		t.Parallel()
		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		ep.MarkVerified(now)
		ep.MarkStale(now.Add(time.Minute))
		candidates := transport.BuildCandidatesFromEndpoints([]transport.ExternalEndpoint{ep}, false)
		if len(candidates) != 1 {
			t.Fatalf("got %d candidates, want 1 for stale endpoint", len(candidates))
		}
		if candidates[0].Reason != transport.CandidateReasonPreviouslyVerified {
			t.Errorf("stale (previously verified) Reason = %q, want previously-verified",
				candidates[0].Reason)
		}
	})

	t.Run("stale never-verified endpoint uses source reason", func(t *testing.T) {
		t.Parallel()
		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		ep.MarkStale(now) // Stale without prior verification
		candidates := transport.BuildCandidatesFromEndpoints([]transport.ExternalEndpoint{ep}, false)
		if len(candidates) != 1 {
			t.Fatalf("got %d candidates, want 1", len(candidates))
		}
		if candidates[0].Reason != transport.CandidateReasonConfigured {
			t.Errorf("stale (never verified) Reason = %q, want configured", candidates[0].Reason)
		}
	})

	t.Run("failed endpoint included", func(t *testing.T) {
		t.Parallel()
		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		ep.MarkFailed(now)
		candidates := transport.BuildCandidatesFromEndpoints([]transport.ExternalEndpoint{ep}, false)
		if len(candidates) != 1 {
			t.Fatalf("got %d candidates, want 1 for failed endpoint", len(candidates))
		}
	})

	t.Run("candidates never exceed input count (no broad scan)", func(t *testing.T) {
		t.Parallel()
		// Key anti-scan property: BuildCandidatesFromEndpoints must never return
		// more candidates than the input endpoint count. It cannot invent new
		// host:port combinations that were not in the input.
		endpoints := []transport.ExternalEndpoint{
			transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0),
			transport.NewConfiguredEndpoint("198.51.100.5", 9000, 0),
		}
		candidates := transport.BuildCandidatesFromEndpoints(endpoints, true)
		if len(candidates) > len(endpoints) {
			t.Errorf("got %d candidates > %d input endpoints: BuildCandidatesFromEndpoints must not generate new host:port combinations",
				len(candidates), len(endpoints))
		}
		// Verify all candidates correspond to known addresses.
		knownAddrs := map[string]bool{
			"203.0.113.1:51830": true,
			"198.51.100.5:9000": true,
		}
		for _, c := range candidates {
			key := net.JoinHostPort(c.Host, "")
			_ = key
			addrKey := c.Host + ":" + portStr(c.Port)
			if !knownAddrs[addrKey] {
				t.Errorf("candidate %s:%d was not in input endpoints", c.Host, c.Port)
			}
		}
	})

	t.Run("router-discovered source maps to router-discovered reason", func(t *testing.T) {
		t.Parallel()
		hint := transport.RouterDiscoveryHint{
			Protocol:     "upnp",
			ExternalHost: "203.0.113.1",
			ExternalPort: 51830,
			InternalPort: 51820,
			RecordedAt:   now,
		}
		ep := hint.ToExternalEndpoint()
		// Router-discovered endpoints start verified. Make it stale to test
		// that it becomes a revalidation candidate.
		ep.MarkStale(now.Add(time.Minute))
		candidates := transport.BuildCandidatesFromEndpoints([]transport.ExternalEndpoint{ep}, false)
		if len(candidates) != 1 {
			t.Fatalf("got %d candidates, want 1", len(candidates))
		}
		// Was previously verified (VerifiedAt set by ToExternalEndpoint), so
		// the reason should be previously-verified.
		if candidates[0].Reason != transport.CandidateReasonPreviouslyVerified {
			t.Errorf("Reason = %q, want previously-verified (router endpoint had prior verification)",
				candidates[0].Reason)
		}
	})
}

// portStr converts a uint16 port to a decimal string for map key construction.
func portStr(port uint16) string {
	var buf [5]byte
	n := 0
	if port == 0 {
		return "0"
	}
	for p := port; p > 0; p /= 10 {
		buf[n] = byte('0' + p%10)
		n++
	}
	// Reverse
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf[:n])
}

// TestBuildCoordinatorObservedCandidates verifies that
// BuildCoordinatorObservedCandidates produces exactly the candidates matching
// the provided host + forwarded ports, with no additional port generation.
func TestBuildCoordinatorObservedCandidates(t *testing.T) {
	t.Parallel()

	t.Run("empty ports produces no candidates", func(t *testing.T) {
		t.Parallel()
		candidates := transport.BuildCoordinatorObservedCandidates("203.0.113.1", nil)
		if len(candidates) != 0 {
			t.Errorf("got %d candidates for empty ports, want 0", len(candidates))
		}
	})

	t.Run("one port produces one candidate", func(t *testing.T) {
		t.Parallel()
		candidates := transport.BuildCoordinatorObservedCandidates("203.0.113.1", []uint16{51830})
		if len(candidates) != 1 {
			t.Fatalf("got %d candidates, want 1", len(candidates))
		}
		if candidates[0].Host != "203.0.113.1" {
			t.Errorf("Host = %q, want 203.0.113.1", candidates[0].Host)
		}
		if candidates[0].Port != 51830 {
			t.Errorf("Port = %d, want 51830", candidates[0].Port)
		}
		if candidates[0].Reason != transport.CandidateReasonCoordinatorObserved {
			t.Errorf("Reason = %q, want coordinator-observed", candidates[0].Reason)
		}
	})

	t.Run("multiple ports produce matching candidates only", func(t *testing.T) {
		t.Parallel()
		ports := []uint16{51830, 51831, 52000}
		candidates := transport.BuildCoordinatorObservedCandidates("203.0.113.1", ports)
		if len(candidates) != len(ports) {
			t.Fatalf("got %d candidates, want %d", len(candidates), len(ports))
		}
		// Every candidate must have the observed host and coordinator-observed reason.
		for i, c := range candidates {
			if c.Host != "203.0.113.1" {
				t.Errorf("candidates[%d].Host = %q, want 203.0.113.1", i, c.Host)
			}
			if c.Reason != transport.CandidateReasonCoordinatorObserved {
				t.Errorf("candidates[%d].Reason = %q, want coordinator-observed", i, c.Reason)
			}
		}
	})
}

// TestEndpointRegistry verifies the core registry lifecycle: add, count,
// usable selection, revalidation selection, and snapshotting.
func TestEndpointRegistry(t *testing.T) {
	t.Parallel()

	now := time.Now()

	t.Run("new registry is empty", func(t *testing.T) {
		t.Parallel()
		r := transport.NewEndpointRegistry()
		if r.Count() != 0 {
			t.Errorf("Count() = %d, want 0", r.Count())
		}
		if len(r.UsableEndpoints()) != 0 {
			t.Errorf("UsableEndpoints() length = %d, want 0", len(r.UsableEndpoints()))
		}
		if len(r.Snapshot()) != 0 {
			t.Errorf("Snapshot() length = %d, want 0", len(r.Snapshot()))
		}
	})

	t.Run("add endpoint increases count", func(t *testing.T) {
		t.Parallel()
		r := transport.NewEndpointRegistry()
		r.Add(transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0))
		if r.Count() != 1 {
			t.Errorf("Count() = %d, want 1 after Add", r.Count())
		}
	})

	t.Run("unverified endpoint is usable", func(t *testing.T) {
		t.Parallel()
		r := transport.NewEndpointRegistry()
		r.Add(transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0))
		usable := r.UsableEndpoints()
		if len(usable) != 1 {
			t.Errorf("UsableEndpoints() = %d, want 1 (configured unverified is usable)", len(usable))
		}
	})

	t.Run("stale endpoint not in usable list", func(t *testing.T) {
		t.Parallel()
		r := transport.NewEndpointRegistry()
		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		ep.MarkVerified(now)
		ep.MarkStale(now.Add(time.Minute))
		r.Add(ep)
		usable := r.UsableEndpoints()
		if len(usable) != 0 {
			t.Errorf("UsableEndpoints() = %d, want 0 (stale endpoint must not be usable)", len(usable))
		}
	})

	t.Run("failed endpoint not in usable list", func(t *testing.T) {
		t.Parallel()
		r := transport.NewEndpointRegistry()
		ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
		ep.MarkFailed(now)
		r.Add(ep)
		usable := r.UsableEndpoints()
		if len(usable) != 0 {
			t.Errorf("UsableEndpoints() = %d, want 0 (failed endpoint must not be usable)", len(usable))
		}
	})

	t.Run("snapshot includes all endpoints regardless of state", func(t *testing.T) {
		t.Parallel()
		r := transport.NewEndpointRegistry()
		r.Add(transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0))
		ep2 := transport.NewConfiguredEndpoint("198.51.100.5", 9000, 0)
		ep2.MarkFailed(now)
		r.Add(ep2)
		snapshot := r.Snapshot()
		if len(snapshot) != 2 {
			t.Errorf("Snapshot() length = %d, want 2", len(snapshot))
		}
	})
}

// TestEndpointRegistryMarkAllStale verifies that MarkAllStale transitions all
// non-stale endpoints to stale, preserving the no-re-stale rule.
func TestEndpointRegistryMarkAllStale(t *testing.T) {
	t.Parallel()

	now := time.Now()

	r := transport.NewEndpointRegistry()

	// Add one verified endpoint and one unverified endpoint.
	verified := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
	verified.MarkVerified(now)
	r.Add(verified)

	r.Add(transport.NewConfiguredEndpoint("198.51.100.5", 9000, 0))

	// Also add one that is already stale — it should not be re-staled.
	alreadyStale := transport.NewConfiguredEndpoint("192.0.2.1", 7000, 0)
	alreadyStale.MarkStale(now.Add(-time.Minute))
	r.Add(alreadyStale)

	staleAt := now.Add(30 * time.Second)
	r.MarkAllStale(staleAt)

	snapshot := r.Snapshot()
	for _, ep := range snapshot {
		if ep.Verification != transport.VerificationStateStale {
			t.Errorf("endpoint %s:%d Verification = %q after MarkAllStale, want stale",
				ep.Host, ep.Port, ep.Verification)
		}
	}

	// After MarkAllStale, no endpoint should be usable.
	usable := r.UsableEndpoints()
	if len(usable) != 0 {
		t.Errorf("UsableEndpoints() = %d after MarkAllStale, want 0", len(usable))
	}

	// All three endpoints should now appear in revalidation candidates.
	needsRevalidation := r.SelectForRevalidation()
	if len(needsRevalidation) != 3 {
		t.Errorf("SelectForRevalidation() = %d, want 3", len(needsRevalidation))
	}
}

// TestEndpointRegistrySelectForInitialVerification verifies that
// SelectForInitialVerification returns only unverified endpoints.
func TestEndpointRegistrySelectForInitialVerification(t *testing.T) {
	t.Parallel()

	now := time.Now()
	r := transport.NewEndpointRegistry()

	// Unverified: should appear.
	r.Add(transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0))

	// Verified: should NOT appear.
	verified := transport.NewConfiguredEndpoint("198.51.100.5", 9000, 0)
	verified.MarkVerified(now)
	r.Add(verified)

	// Stale: should NOT appear (use SelectForRevalidation instead).
	stale := transport.NewConfiguredEndpoint("192.0.2.1", 7000, 0)
	stale.MarkStale(now)
	r.Add(stale)

	initial := r.SelectForInitialVerification()
	if len(initial) != 1 {
		t.Errorf("SelectForInitialVerification() = %d, want 1 (only unverified)", len(initial))
	}
	if len(initial) > 0 && initial[0].Host != "203.0.113.1" {
		t.Errorf("SelectForInitialVerification()[0].Host = %q, want 203.0.113.1", initial[0].Host)
	}
}

// TestEndpointRegistryApplyProbeResult verifies that ApplyProbeResult updates
// all matching endpoints and leaves non-matching endpoints unchanged.
func TestEndpointRegistryApplyProbeResult(t *testing.T) {
	t.Parallel()

	now := time.Now()
	r := transport.NewEndpointRegistry()

	// Add two endpoints: one matching the probe result, one not.
	r.Add(transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0))
	r.Add(transport.NewConfiguredEndpoint("198.51.100.5", 9000, 0))

	// Apply a successful probe result for the first endpoint.
	result := transport.ProbeResult{
		TargetHost: "203.0.113.1",
		TargetPort: 51830,
		Reachable:  true,
		ProbedAt:   now,
	}
	r.ApplyProbeResult(result)

	snapshot := r.Snapshot()
	for _, ep := range snapshot {
		if ep.Host == "203.0.113.1" && ep.Port == 51830 {
			if ep.Verification != transport.VerificationStateVerified {
				t.Errorf("probed endpoint Verification = %q, want verified", ep.Verification)
			}
		}
		if ep.Host == "198.51.100.5" && ep.Port == 9000 {
			if ep.Verification != transport.VerificationStateUnverified {
				t.Errorf("non-probed endpoint Verification = %q, want unverified (unchanged)",
					ep.Verification)
			}
		}
	}
}

// TestEndpointRegistryApplyFailedProbeResult verifies that a failed probe
// marks the matching endpoint as failed.
func TestEndpointRegistryApplyFailedProbeResult(t *testing.T) {
	t.Parallel()

	now := time.Now()
	r := transport.NewEndpointRegistry()
	r.Add(transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0))

	result := transport.ProbeResult{
		TargetHost: "203.0.113.1",
		TargetPort: 51830,
		Reachable:  false,
		ProbedAt:   now,
	}
	r.ApplyProbeResult(result)

	snapshot := r.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("Snapshot() length = %d, want 1", len(snapshot))
	}
	if snapshot[0].Verification != transport.VerificationStateFailed {
		t.Errorf("Verification = %q after failed probe, want failed", snapshot[0].Verification)
	}
	if snapshot[0].IsUsable() {
		t.Error("endpoint must not be usable after failed probe")
	}

	// Failed endpoint must appear in revalidation candidates.
	needsRevalidation := r.SelectForRevalidation()
	if len(needsRevalidation) != 1 {
		t.Errorf("SelectForRevalidation() = %d after failed probe, want 1", len(needsRevalidation))
	}
}

// TestEndpointRegistryRevalidationAfterStaleness verifies the full
// stale → revalidation → verified cycle that represents endpoint knowledge
// becoming suspect and then being confirmed again.
func TestEndpointRegistryRevalidationAfterStaleness(t *testing.T) {
	t.Parallel()

	now := time.Now()
	r := transport.NewEndpointRegistry()
	ep := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
	ep.MarkVerified(now)
	r.Add(ep)

	// Verify usable before staleness.
	if len(r.UsableEndpoints()) != 1 {
		t.Fatal("endpoint must be usable when verified")
	}

	// Path goes down: mark all stale.
	r.MarkAllStale(now.Add(time.Minute))

	if len(r.UsableEndpoints()) != 0 {
		t.Error("no endpoint must be usable after MarkAllStale")
	}
	if len(r.SelectForRevalidation()) != 1 {
		t.Error("stale endpoint must appear in revalidation candidates")
	}

	// Probe runs and succeeds: apply successful result.
	r.ApplyProbeResult(transport.ProbeResult{
		TargetHost: "203.0.113.1",
		TargetPort: 51830,
		Reachable:  true,
		ProbedAt:   now.Add(2 * time.Minute),
	})

	// After revalidation, the endpoint must be usable again.
	if len(r.UsableEndpoints()) != 1 {
		t.Error("endpoint must be usable again after successful revalidation probe")
	}
	if len(r.SelectForRevalidation()) != 0 {
		t.Error("revalidated endpoint must not remain in revalidation candidates")
	}
}

// TestProbeEndToEnd performs an actual UDP probe against a local ProbeResponder,
// verifying the full challenge/response cycle end-to-end.
func TestProbeEndToEnd(t *testing.T) {
	t.Parallel()

	// Start a ProbeResponder on a free loopback port.
	responder, err := transport.NewProbeResponder("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewProbeResponder: %v", err)
	}
	go responder.Serve() //nolint:errcheck
	defer responder.Close()

	// Extract the port assigned by the OS.
	respAddr := responder.Addr().(*net.UDPAddr)

	executor := transport.UDPProbeExecutor{Timeout: 2 * time.Second}
	candidate := transport.ProbeCandidate{
		Host:   "127.0.0.1",
		Port:   uint16(respAddr.Port),
		Reason: transport.CandidateReasonConfigured,
	}

	result, execErr := executor.Execute(context.Background(), candidate)
	if execErr != nil {
		t.Fatalf("Execute: unexpected error: %v", execErr)
	}

	if !result.Reachable {
		t.Error("probe to local responder must be reachable")
	}
	if result.RoundTripTime <= 0 {
		t.Error("RTT must be positive on a successful probe")
	}
	if result.TargetHost != "127.0.0.1" {
		t.Errorf("TargetHost = %q, want 127.0.0.1", result.TargetHost)
	}
	if result.TargetPort != uint16(respAddr.Port) {
		t.Errorf("TargetPort = %d, want %d", result.TargetPort, respAddr.Port)
	}
	if result.ProbedAt.IsZero() {
		t.Error("ProbedAt must be set on a successful probe")
	}
}

// TestProbeEndToEndApplyToEndpoint verifies the full pipeline from probe
// execution to endpoint state update.
func TestProbeEndToEndApplyToEndpoint(t *testing.T) {
	t.Parallel()

	responder, err := transport.NewProbeResponder("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewProbeResponder: %v", err)
	}
	go responder.Serve() //nolint:errcheck
	defer responder.Close()

	respAddr := responder.Addr().(*net.UDPAddr)

	ep := transport.NewConfiguredEndpoint("127.0.0.1", uint16(respAddr.Port), 0)
	if ep.Verification != transport.VerificationStateUnverified {
		t.Fatal("new endpoint must start unverified")
	}

	executor := transport.UDPProbeExecutor{Timeout: 2 * time.Second}
	result, execErr := executor.Execute(context.Background(), transport.ProbeCandidate{
		Host:   "127.0.0.1",
		Port:   uint16(respAddr.Port),
		Reason: transport.CandidateReasonConfigured,
	})
	if execErr != nil {
		t.Fatalf("Execute: %v", execErr)
	}

	result.ApplyToEndpoint(&ep)

	if ep.Verification != transport.VerificationStateVerified {
		t.Errorf("Verification = %q after successful probe + ApplyToEndpoint, want verified",
			ep.Verification)
	}
	if !ep.IsUsable() {
		t.Error("endpoint must be usable after successful probe")
	}
}

// TestProbeUnreachableEndpoint verifies that probing a non-listening port
// returns Reachable=false after the timeout (no panic, no internal error).
func TestProbeUnreachableEndpoint(t *testing.T) {
	t.Parallel()

	// Start a responder to get a free port, then close it before probing.
	// This ensures the port is not in use and the probe will time out or get
	// an ICMP unreachable response.
	responder, err := transport.NewProbeResponder("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewProbeResponder: %v", err)
	}
	port := uint16(responder.Addr().(*net.UDPAddr).Port)
	responder.Close() // Closed before probing: no one will answer.

	executor := transport.UDPProbeExecutor{Timeout: 300 * time.Millisecond}
	result, execErr := executor.Execute(context.Background(), transport.ProbeCandidate{
		Host:   "127.0.0.1",
		Port:   port,
		Reason: transport.CandidateReasonConfigured,
	})
	if execErr != nil {
		t.Fatalf("Execute: unexpected error: %v", execErr)
	}
	if result.Reachable {
		t.Error("probe to closed port must not be reachable")
	}
}

// TestProbeRegistryEndToEnd integrates the registry and probe executor,
// verifying that a registry endpoint transitions from unverified to verified
// via an actual probe, and stale → verified via revalidation.
func TestProbeRegistryEndToEnd(t *testing.T) {
	t.Parallel()

	responder, err := transport.NewProbeResponder("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewProbeResponder: %v", err)
	}
	go responder.Serve() //nolint:errcheck
	defer responder.Close()

	respAddr := responder.Addr().(*net.UDPAddr)
	port := uint16(respAddr.Port)

	r := transport.NewEndpointRegistry()
	r.Add(transport.NewConfiguredEndpoint("127.0.0.1", port, 0))

	// Phase 1: initial verification from unverified state.
	initial := r.SelectForInitialVerification()
	if len(initial) != 1 {
		t.Fatalf("SelectForInitialVerification() = %d, want 1", len(initial))
	}

	executor := transport.UDPProbeExecutor{Timeout: 2 * time.Second}
	for _, ep := range initial {
		candidates := transport.BuildCandidatesFromEndpoints([]transport.ExternalEndpoint{ep}, false)
		for _, c := range candidates {
			result, execErr := executor.Execute(context.Background(), c)
			if execErr != nil {
				t.Fatalf("Execute: %v", execErr)
			}
			r.ApplyProbeResult(result)
		}
	}

	if len(r.UsableEndpoints()) != 1 {
		t.Error("endpoint must be usable after initial verification probe")
	}

	// Phase 2: simulate path-down event → stale → revalidation probe.
	r.MarkAllStale(time.Now())
	if len(r.UsableEndpoints()) != 0 {
		t.Error("no endpoint must be usable after MarkAllStale")
	}

	revalidate := r.SelectForRevalidation()
	if len(revalidate) != 1 {
		t.Fatalf("SelectForRevalidation() = %d, want 1", len(revalidate))
	}

	candidates := transport.BuildCandidatesFromEndpoints(revalidate, false)
	for _, c := range candidates {
		result, execErr := executor.Execute(context.Background(), c)
		if execErr != nil {
			t.Fatalf("Execute (revalidation): %v", execErr)
		}
		r.ApplyProbeResult(result)
	}

	if len(r.UsableEndpoints()) != 1 {
		t.Error("endpoint must be usable again after revalidation probe")
	}
}
