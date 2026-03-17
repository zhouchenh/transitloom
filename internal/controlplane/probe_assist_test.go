package controlplane_test

import (
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/controlplane"
)

// TestValidateCoordinatorProbeRequest verifies that ValidateCoordinatorProbeRequest
// accepts valid requests and rejects malformed ones with useful errors.
func TestValidateCoordinatorProbeRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     controlplane.CoordinatorProbeRequest
		wantErr bool
	}{
		{
			"valid no-DNAT request",
			controlplane.CoordinatorProbeRequest{
				TargetHost:         "203.0.113.1",
				TargetPort:         51830,
				EffectiveLocalPort: 51830,
			},
			false,
		},
		{
			"valid DNAT request",
			controlplane.CoordinatorProbeRequest{
				TargetHost:         "203.0.113.1",
				TargetPort:         12000,
				EffectiveLocalPort: 51830,
			},
			false,
		},
		{
			"valid with timeout",
			controlplane.CoordinatorProbeRequest{
				TargetHost:         "203.0.113.1",
				TargetPort:         51830,
				EffectiveLocalPort: 51830,
				TimeoutMs:          3000,
			},
			false,
		},
		{
			"empty target host",
			controlplane.CoordinatorProbeRequest{
				TargetPort:         51830,
				EffectiveLocalPort: 51830,
			},
			true,
		},
		{
			"zero target port",
			controlplane.CoordinatorProbeRequest{
				TargetHost:         "203.0.113.1",
				TargetPort:         0,
				EffectiveLocalPort: 51830,
			},
			true,
		},
		{
			"zero effective local port",
			controlplane.CoordinatorProbeRequest{
				TargetHost: "203.0.113.1",
				TargetPort: 51830,
			},
			true,
		},
		{
			"negative timeout",
			controlplane.CoordinatorProbeRequest{
				TargetHost:         "203.0.113.1",
				TargetPort:         51830,
				EffectiveLocalPort: 51830,
				TimeoutMs:          -1,
			},
			true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := controlplane.ValidateCoordinatorProbeRequest(tc.req)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateCoordinatorProbeRequest() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// TestCoordinatorProbeRequestDNATDistinction verifies that the request type
// preserves the distinction between TargetPort (external) and
// EffectiveLocalPort (local mesh listener after DNAT).
//
// These must not be collapsed: in DNAT deployments they differ, and conflating
// them would mean the coordinator probes the wrong address or the node
// listens on the wrong port.
func TestCoordinatorProbeRequestDNATDistinction(t *testing.T) {
	t.Parallel()

	req := controlplane.CoordinatorProbeRequest{
		TargetHost:         "203.0.113.1",
		TargetPort:         12000,  // external port on router
		EffectiveLocalPort: 51830,  // local mesh listener port
	}

	if req.TargetPort == req.EffectiveLocalPort {
		t.Error("DNAT request: TargetPort and EffectiveLocalPort must differ")
	}
	if err := controlplane.ValidateCoordinatorProbeRequest(req); err != nil {
		t.Errorf("ValidateCoordinatorProbeRequest: %v", err)
	}
}

// TestCoordinatorProbeResponseFields verifies that CoordinatorProbeResponse
// fields are set and readable as expected for both reachable and unreachable cases.
func TestCoordinatorProbeResponseFields(t *testing.T) {
	t.Parallel()

	now := time.Now()

	t.Run("reachable response", func(t *testing.T) {
		t.Parallel()
		resp := controlplane.CoordinatorProbeResponse{
			TargetHost:  "203.0.113.1",
			TargetPort:  51830,
			Reachable:   true,
			RoundTripMs: 42,
			ProbedAt:    now,
		}
		if !resp.Reachable {
			t.Error("Reachable must be true")
		}
		if resp.RoundTripMs <= 0 {
			t.Error("RoundTripMs must be positive for a reachable response")
		}
		if resp.Error != "" {
			t.Errorf("Error must be empty for a reachable response, got %q", resp.Error)
		}
		if resp.ProbedAt.IsZero() {
			t.Error("ProbedAt must be set")
		}
	})

	t.Run("unreachable response (timeout)", func(t *testing.T) {
		t.Parallel()
		resp := controlplane.CoordinatorProbeResponse{
			TargetHost: "203.0.113.1",
			TargetPort: 51830,
			Reachable:  false,
			ProbedAt:   now,
		}
		if resp.Reachable {
			t.Error("Reachable must be false")
		}
		// Empty Error with Reachable=false means the probe ran but got no response.
		if resp.Error != "" {
			t.Errorf("Error must be empty for a timeout (probe attempted), got %q", resp.Error)
		}
	})

	t.Run("could not attempt response", func(t *testing.T) {
		t.Parallel()
		resp := controlplane.CoordinatorProbeResponse{
			TargetHost: "203.0.113.1",
			TargetPort: 51830,
			Reachable:  false,
			Error:      "address rejected by policy",
			ProbedAt:   now,
		}
		if resp.Error == "" {
			t.Error("Error must be set when probe could not be attempted")
		}
	})
}
