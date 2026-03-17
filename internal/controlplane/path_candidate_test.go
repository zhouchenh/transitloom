package controlplane_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/controlplane"
)

func TestDistributedPathCandidateValidate(t *testing.T) {
	tests := []struct {
		name    string
		c       controlplane.DistributedPathCandidate
		wantErr bool
	}{
		{
			name: "valid direct-public candidate",
			c: controlplane.DistributedPathCandidate{
				CandidateID:     "assoc-1:direct",
				AssociationID:   "assoc-1",
				Class:           controlplane.DistributedPathClassDirectPublic,
				IsRelayAssisted: false,
				RemoteEndpoint:  "10.0.0.1:4000",
				AdminWeight:     100,
			},
			wantErr: false,
		},
		{
			name: "valid direct-intranet candidate",
			c: controlplane.DistributedPathCandidate{
				CandidateID:     "assoc-1:intranet",
				AssociationID:   "assoc-1",
				Class:           controlplane.DistributedPathClassDirectIntranet,
				IsRelayAssisted: false,
				RemoteEndpoint:  "192.168.1.2:4000",
			},
			wantErr: false,
		},
		{
			name: "valid coordinator-relay candidate",
			c: controlplane.DistributedPathCandidate{
				CandidateID:     "assoc-1:relay:0",
				AssociationID:   "assoc-1",
				Class:           controlplane.DistributedPathClassCoordinatorRelay,
				IsRelayAssisted: true,
				RemoteEndpoint:  "relay.example.com:7000",
				RelayNodeID:     "coordinator-1",
				AdminWeight:     80,
			},
			wantErr: false,
		},
		{
			name: "valid node-relay candidate",
			c: controlplane.DistributedPathCandidate{
				CandidateID:     "assoc-1:node-relay",
				AssociationID:   "assoc-1",
				Class:           controlplane.DistributedPathClassNodeRelay,
				IsRelayAssisted: true,
				RemoteEndpoint:  "10.0.0.3:7001",
				RelayNodeID:     "relay-node-1",
			},
			wantErr: false,
		},
		{
			name:    "empty candidate_id rejected",
			c:       controlplane.DistributedPathCandidate{AssociationID: "assoc-1", Class: controlplane.DistributedPathClassDirectPublic},
			wantErr: true,
		},
		{
			name:    "empty association_id rejected",
			c:       controlplane.DistributedPathCandidate{CandidateID: "c1", Class: controlplane.DistributedPathClassDirectPublic},
			wantErr: true,
		},
		{
			name:    "unknown class rejected",
			c:       controlplane.DistributedPathCandidate{CandidateID: "c1", AssociationID: "a1", Class: "fancy-new-path"},
			wantErr: true,
		},
		{
			// relay class with IsRelayAssisted=false is inconsistent
			name: "relay class with IsRelayAssisted=false rejected",
			c: controlplane.DistributedPathCandidate{
				CandidateID:     "c1",
				AssociationID:   "a1",
				Class:           controlplane.DistributedPathClassCoordinatorRelay,
				IsRelayAssisted: false,
				RelayNodeID:     "coord-1",
			},
			wantErr: true,
		},
		{
			// direct class with IsRelayAssisted=true is inconsistent
			name: "direct class with IsRelayAssisted=true rejected",
			c: controlplane.DistributedPathCandidate{
				CandidateID:     "c1",
				AssociationID:   "a1",
				Class:           controlplane.DistributedPathClassDirectPublic,
				IsRelayAssisted: true,
			},
			wantErr: true,
		},
		{
			// relay candidate without relay_node_id is incomplete
			name: "relay candidate without relay_node_id rejected",
			c: controlplane.DistributedPathCandidate{
				CandidateID:     "c1",
				AssociationID:   "a1",
				Class:           controlplane.DistributedPathClassCoordinatorRelay,
				IsRelayAssisted: true,
				RemoteEndpoint:  "10.0.0.1:7000",
			},
			wantErr: true,
		},
		{
			// usable candidate without remote_endpoint is valid structurally
			// (informational placeholder)
			name: "direct candidate without remote_endpoint is valid",
			c: controlplane.DistributedPathCandidate{
				CandidateID:     "c1",
				AssociationID:   "a1",
				Class:           controlplane.DistributedPathClassDirectPublic,
				IsRelayAssisted: false,
				Note:            "no endpoint data available yet",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.c.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDistributedPathCandidateIsUsable(t *testing.T) {
	tests := []struct {
		name           string
		remoteEndpoint string
		want           bool
	}{
		{"empty endpoint not usable", "", false},
		{"whitespace endpoint not usable", "   ", false},
		{"non-empty endpoint usable", "10.0.0.1:4000", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := controlplane.DistributedPathCandidate{RemoteEndpoint: tt.remoteEndpoint}
			if got := c.IsUsable(); got != tt.want {
				t.Errorf("IsUsable() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDirectRelayDistinction verifies that direct and relay-assisted
// candidates are explicitly distinct at the structural level. The
// IsRelayAssisted flag must match the Class field: this is the primary guard
// against accidentally treating a relay path as a direct path or vice versa.
func TestDirectRelayDistinction(t *testing.T) {
	directClasses := []controlplane.DistributedPathCandidateClass{
		controlplane.DistributedPathClassDirectPublic,
		controlplane.DistributedPathClassDirectIntranet,
	}
	relayClasses := []controlplane.DistributedPathCandidateClass{
		controlplane.DistributedPathClassCoordinatorRelay,
		controlplane.DistributedPathClassNodeRelay,
	}

	for _, class := range directClasses {
		// Direct class must have IsRelayAssisted=false.
		c := controlplane.DistributedPathCandidate{
			CandidateID: "c1", AssociationID: "a1",
			Class: class, IsRelayAssisted: false,
		}
		if err := c.Validate(); err != nil {
			t.Errorf("direct class %q with IsRelayAssisted=false should be valid, got: %v", class, err)
		}
		// Direct class with IsRelayAssisted=true must be rejected.
		c.IsRelayAssisted = true
		if err := c.Validate(); err == nil {
			t.Errorf("direct class %q with IsRelayAssisted=true should be invalid", class)
		}
	}

	for _, class := range relayClasses {
		// Relay class must have IsRelayAssisted=true and a relay node ID.
		c := controlplane.DistributedPathCandidate{
			CandidateID: "c1", AssociationID: "a1",
			Class: class, IsRelayAssisted: true, RelayNodeID: "relay-1",
		}
		if err := c.Validate(); err != nil {
			t.Errorf("relay class %q with IsRelayAssisted=true should be valid, got: %v", class, err)
		}
		// Relay class with IsRelayAssisted=false must be rejected.
		c.IsRelayAssisted = false
		if err := c.Validate(); err == nil {
			t.Errorf("relay class %q with IsRelayAssisted=false should be invalid", class)
		}
	}
}

func TestPathCandidateSetValidate(t *testing.T) {
	validCandidate := controlplane.DistributedPathCandidate{
		CandidateID:     "assoc-1:relay:0",
		AssociationID:   "assoc-1",
		Class:           controlplane.DistributedPathClassCoordinatorRelay,
		IsRelayAssisted: true,
		RemoteEndpoint:  "10.0.0.1:7000",
		RelayNodeID:     "coord-1",
	}

	tests := []struct {
		name    string
		s       controlplane.PathCandidateSet
		wantErr bool
	}{
		{
			name: "valid set with one relay candidate",
			s:    controlplane.PathCandidateSet{AssociationID: "assoc-1", Candidates: []controlplane.DistributedPathCandidate{validCandidate}},
		},
		{
			name: "valid empty set",
			s:    controlplane.PathCandidateSet{AssociationID: "assoc-1"},
		},
		{
			name:    "empty association_id rejected",
			s:       controlplane.PathCandidateSet{},
			wantErr: true,
		},
		{
			// A candidate in the set must have the same AssociationID as the set.
			// This prevents candidates from being accidentally distributed to the
			// wrong association.
			name: "candidate association_id mismatch rejected",
			s: controlplane.PathCandidateSet{
				AssociationID: "assoc-1",
				Candidates: []controlplane.DistributedPathCandidate{
					{
						CandidateID:     "assoc-2:relay:0",
						AssociationID:   "assoc-2", // wrong
						Class:           controlplane.DistributedPathClassCoordinatorRelay,
						IsRelayAssisted: true,
						RemoteEndpoint:  "10.0.0.1:7000",
						RelayNodeID:     "coord-1",
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.s.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPathCandidateRequestValidate(t *testing.T) {
	now := time.Now().UTC()
	validReadiness := controlplane.BootstrapReadinessSummary{
		OverallPhase:   controlplane.ReadinessPhaseReady,
		IdentityPhase:  "ready",
		AdmissionPhase: "usable",
		CachedToken: &controlplane.BootstrapTokenSummary{
			TokenID:             "tok-1",
			NodeID:              "node-1",
			IssuerCoordinatorID: "coord-1",
			IssuedAt:            now,
			ExpiresAt:           now.Add(24 * time.Hour),
		},
	}
	tests := []struct {
		name    string
		r       controlplane.PathCandidateRequest
		wantErr bool
	}{
		{
			name: "valid request",
			r: controlplane.PathCandidateRequest{
				ProtocolVersion: controlplane.BootstrapProtocolVersion,
				NodeName:        "node-1",
				Readiness:       validReadiness,
				AssociationIDs:  []string{"assoc-1", "assoc-2"},
			},
		},
		{
			name: "wrong protocol version",
			r: controlplane.PathCandidateRequest{
				ProtocolVersion: "wrong-version",
				NodeName:        "node-1",
				Readiness:       validReadiness,
				AssociationIDs:  []string{"assoc-1"},
			},
			wantErr: true,
		},
		{
			name: "empty node_name",
			r: controlplane.PathCandidateRequest{
				ProtocolVersion: controlplane.BootstrapProtocolVersion,
				Readiness:       validReadiness,
				AssociationIDs:  []string{"assoc-1"},
			},
			wantErr: true,
		},
		{
			name: "empty association_ids",
			r: controlplane.PathCandidateRequest{
				ProtocolVersion: controlplane.BootstrapProtocolVersion,
				NodeName:        "node-1",
				Readiness:       validReadiness,
			},
			wantErr: true,
		},
		{
			name: "blank association ID in list rejected",
			r: controlplane.PathCandidateRequest{
				ProtocolVersion: controlplane.BootstrapProtocolVersion,
				NodeName:        "node-1",
				Readiness:       validReadiness,
				AssociationIDs:  []string{"assoc-1", ""},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.r.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPathCandidateResponseValidate(t *testing.T) {
	tests := []struct {
		name    string
		r       controlplane.PathCandidateResponse
		wantErr bool
	}{
		{
			name: "valid empty response",
			r: controlplane.PathCandidateResponse{
				ProtocolVersion: controlplane.BootstrapProtocolVersion,
				CoordinatorName: "coord-1",
				BootstrapOnly:   true,
			},
		},
		{
			name: "wrong protocol version",
			r: controlplane.PathCandidateResponse{
				ProtocolVersion: "wrong",
				CoordinatorName: "coord-1",
				BootstrapOnly:   true,
			},
			wantErr: true,
		},
		{
			name: "empty coordinator name",
			r: controlplane.PathCandidateResponse{
				ProtocolVersion: controlplane.BootstrapProtocolVersion,
				BootstrapOnly:   true,
			},
			wantErr: true,
		},
		{
			// BootstrapOnly must always be true. It is the explicit label that
			// these candidates are coordinator knowledge, not chosen-path state.
			name: "bootstrap_only=false rejected",
			r: controlplane.PathCandidateResponse{
				ProtocolVersion: controlplane.BootstrapProtocolVersion,
				CoordinatorName: "coord-1",
				BootstrapOnly:   false,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.r.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPathCandidateJSONRoundtrip verifies that DistributedPathCandidate
// survives a JSON encode/decode roundtrip with all fields preserved.
func TestPathCandidateJSONRoundtrip(t *testing.T) {
	original := controlplane.DistributedPathCandidate{
		CandidateID:     "assoc-x:coordinator-relay:0",
		AssociationID:   "assoc-x",
		Class:           controlplane.DistributedPathClassCoordinatorRelay,
		IsRelayAssisted: true,
		RemoteEndpoint:  "relay.example.com:7777",
		RelayNodeID:     "coordinator-alpha",
		AdminWeight:     75,
		IsMetered:       true,
		Note:            "coordinator data relay endpoint; v1 single-hop relay only",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded controlplane.DistributedPathCandidate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.CandidateID != original.CandidateID {
		t.Errorf("CandidateID mismatch: got %q, want %q", decoded.CandidateID, original.CandidateID)
	}
	if decoded.AssociationID != original.AssociationID {
		t.Errorf("AssociationID mismatch")
	}
	if decoded.Class != original.Class {
		t.Errorf("Class mismatch: got %q, want %q", decoded.Class, original.Class)
	}
	if decoded.IsRelayAssisted != original.IsRelayAssisted {
		t.Errorf("IsRelayAssisted mismatch")
	}
	if decoded.RemoteEndpoint != original.RemoteEndpoint {
		t.Errorf("RemoteEndpoint mismatch")
	}
	if decoded.RelayNodeID != original.RelayNodeID {
		t.Errorf("RelayNodeID mismatch")
	}
	if decoded.AdminWeight != original.AdminWeight {
		t.Errorf("AdminWeight mismatch")
	}
	if decoded.IsMetered != original.IsMetered {
		t.Errorf("IsMetered mismatch")
	}
	if decoded.Note != original.Note {
		t.Errorf("Note mismatch")
	}
}

// TestPathCandidateHTTPEndpoint verifies the full HTTP encode/decode path for
// path-candidate responses served by the coordinator.
func TestPathCandidateHTTPEndpoint(t *testing.T) {
	response := controlplane.PathCandidateResponse{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		CoordinatorName: "coordinator-test",
		BootstrapOnly:   true,
		CandidateSets: []controlplane.PathCandidateSet{
			{
				AssociationID:   "assoc-1",
				SourceNode:      "node-a",
				DestinationNode: "node-b",
				Candidates: []controlplane.DistributedPathCandidate{
					{
						CandidateID:     "assoc-1:coordinator-relay:0",
						AssociationID:   "assoc-1",
						Class:           controlplane.DistributedPathClassCoordinatorRelay,
						IsRelayAssisted: true,
						RemoteEndpoint:  "10.0.0.1:7000",
						RelayNodeID:     "coordinator-test",
						AdminWeight:     100,
					},
				},
				Notes: []string{"direct path candidates not available yet"},
			},
		},
		Details: []string{"bootstrap-only candidate data"},
	}

	rw := httptest.NewRecorder()
	if err := controlplane.WritePathCandidateResponse(rw, http.StatusOK, response); err != nil {
		t.Fatalf("WritePathCandidateResponse: %v", err)
	}

	if rw.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rw.Code, http.StatusOK)
	}
	if !strings.Contains(rw.Header().Get("Content-Type"), "application/json") {
		t.Errorf("expected JSON content type, got %q", rw.Header().Get("Content-Type"))
	}

	var decoded controlplane.PathCandidateResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	if err := decoded.Validate(); err != nil {
		t.Errorf("decoded response invalid: %v", err)
	}
	if len(decoded.CandidateSets) != 1 {
		t.Fatalf("expected 1 candidate set, got %d", len(decoded.CandidateSets))
	}
	if decoded.CandidateSets[0].AssociationID != "assoc-1" {
		t.Errorf("association ID mismatch")
	}
	if len(decoded.CandidateSets[0].Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(decoded.CandidateSets[0].Candidates))
	}
	c := decoded.CandidateSets[0].Candidates[0]
	if !c.IsRelayAssisted {
		t.Errorf("expected IsRelayAssisted=true for relay candidate")
	}
	if c.Class != controlplane.DistributedPathClassCoordinatorRelay {
		t.Errorf("class mismatch: got %q", c.Class)
	}
}
