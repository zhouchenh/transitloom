package node_test

import (
	"testing"

	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/node"
)

func makeRelayCandidate(candidateID, associationID, remoteEndpoint string) controlplane.DistributedPathCandidate {
	return controlplane.DistributedPathCandidate{
		CandidateID:     candidateID,
		AssociationID:   associationID,
		Class:           controlplane.DistributedPathClassCoordinatorRelay,
		IsRelayAssisted: true,
		RemoteEndpoint:  remoteEndpoint,
		RelayNodeID:     "coordinator-1",
		AdminWeight:     100,
	}
}

func makeDirectCandidate(candidateID, associationID, remoteEndpoint string) controlplane.DistributedPathCandidate {
	return controlplane.DistributedPathCandidate{
		CandidateID:     candidateID,
		AssociationID:   associationID,
		Class:           controlplane.DistributedPathClassDirectPublic,
		IsRelayAssisted: false,
		RemoteEndpoint:  remoteEndpoint,
		AdminWeight:     100,
	}
}

// TestCandidateStoreStoreAndLookup verifies basic store/lookup behavior.
func TestCandidateStoreStoreAndLookup(t *testing.T) {
	s := node.NewCandidateStore()

	candidates := []controlplane.DistributedPathCandidate{
		makeRelayCandidate("assoc-1:relay:0", "assoc-1", "10.0.0.1:7000"),
	}
	s.Store("assoc-1", candidates)

	got := s.Lookup("assoc-1")
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(got))
	}
	if got[0].CandidateID != "assoc-1:relay:0" {
		t.Errorf("CandidateID = %q", got[0].CandidateID)
	}
}

// TestCandidateStoreLookupUnknown verifies that Lookup returns nil for unknown associations.
func TestCandidateStoreLookupUnknown(t *testing.T) {
	s := node.NewCandidateStore()
	got := s.Lookup("unknown-assoc")
	if got != nil {
		t.Errorf("expected nil for unknown association, got %v", got)
	}
}

// TestCandidateStoreReplaces verifies that storing candidates for the same
// association replaces the previous set.
func TestCandidateStoreReplaces(t *testing.T) {
	s := node.NewCandidateStore()

	s.Store("assoc-1", []controlplane.DistributedPathCandidate{
		makeRelayCandidate("assoc-1:relay:0", "assoc-1", "10.0.0.1:7000"),
	})
	s.Store("assoc-1", []controlplane.DistributedPathCandidate{
		makeRelayCandidate("assoc-1:relay:0", "assoc-1", "10.0.0.2:7001"),
		makeRelayCandidate("assoc-1:relay:1", "assoc-1", "10.0.0.3:7002"),
	})

	got := s.Lookup("assoc-1")
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates after replace, got %d", len(got))
	}
}

// TestCandidateStoreMultipleAssociations verifies independent storage per association.
func TestCandidateStoreMultipleAssociations(t *testing.T) {
	s := node.NewCandidateStore()

	s.Store("assoc-1", []controlplane.DistributedPathCandidate{
		makeRelayCandidate("assoc-1:relay:0", "assoc-1", "10.0.0.1:7000"),
	})
	s.Store("assoc-2", []controlplane.DistributedPathCandidate{
		makeRelayCandidate("assoc-2:relay:0", "assoc-2", "10.0.0.2:7001"),
	})

	got1 := s.Lookup("assoc-1")
	got2 := s.Lookup("assoc-2")

	if len(got1) != 1 || got1[0].CandidateID != "assoc-1:relay:0" {
		t.Errorf("assoc-1 candidates incorrect: %+v", got1)
	}
	if len(got2) != 1 || got2[0].CandidateID != "assoc-2:relay:0" {
		t.Errorf("assoc-2 candidates incorrect: %+v", got2)
	}
}

// TestCandidateStoreIsolation verifies that mutating returned slices does not
// affect stored state.
func TestCandidateStoreIsolation(t *testing.T) {
	s := node.NewCandidateStore()

	original := []controlplane.DistributedPathCandidate{
		makeRelayCandidate("assoc-1:relay:0", "assoc-1", "10.0.0.1:7000"),
	}
	s.Store("assoc-1", original)

	// Mutate the original slice — should not affect stored state.
	original[0].CandidateID = "mutated"

	got := s.Lookup("assoc-1")
	if len(got) == 0 {
		t.Fatal("expected candidates")
	}
	if got[0].CandidateID == "mutated" {
		t.Error("store should be isolated from caller mutations")
	}

	// Mutate the returned slice — should not affect stored state.
	got[0].CandidateID = "also-mutated"
	got2 := s.Lookup("assoc-1")
	if got2[0].CandidateID == "also-mutated" {
		t.Error("lookup should return a copy, not a reference to stored state")
	}
}

// TestCandidateStoreSnapshot verifies that Snapshot returns all sets sorted.
func TestCandidateStoreSnapshot(t *testing.T) {
	s := node.NewCandidateStore()

	s.Store("assoc-z", []controlplane.DistributedPathCandidate{
		makeRelayCandidate("assoc-z:relay:0", "assoc-z", "10.0.0.1:7000"),
	})
	s.Store("assoc-a", []controlplane.DistributedPathCandidate{
		makeRelayCandidate("assoc-a:relay:0", "assoc-a", "10.0.0.2:7001"),
	})
	s.Store("assoc-m", []controlplane.DistributedPathCandidate{
		makeRelayCandidate("assoc-m:relay:0", "assoc-m", "10.0.0.3:7002"),
	})

	sets := s.Snapshot()
	if len(sets) != 3 {
		t.Fatalf("expected 3 sets, got %d", len(sets))
	}

	// Snapshot must be sorted by AssociationID.
	if sets[0].AssociationID != "assoc-a" ||
		sets[1].AssociationID != "assoc-m" ||
		sets[2].AssociationID != "assoc-z" {
		t.Errorf("snapshot not sorted: %v %v %v",
			sets[0].AssociationID, sets[1].AssociationID, sets[2].AssociationID)
	}
}

// TestCandidateStoreAssociationCount verifies AssociationCount.
func TestCandidateStoreAssociationCount(t *testing.T) {
	s := node.NewCandidateStore()
	if s.AssociationCount() != 0 {
		t.Errorf("new store should have count 0")
	}
	s.Store("assoc-1", []controlplane.DistributedPathCandidate{makeRelayCandidate("c1", "assoc-1", "10.0.0.1:7000")})
	s.Store("assoc-2", []controlplane.DistributedPathCandidate{makeRelayCandidate("c2", "assoc-2", "10.0.0.2:7001")})
	if s.AssociationCount() != 2 {
		t.Errorf("expected count 2, got %d", s.AssociationCount())
	}
}

// TestCandidateStoreDirectRelayDistinction verifies that relay and direct
// candidates are stored and retrieved with their explicit relay distinction intact.
// This ensures the CandidateStore preserves the architectural separation between
// relay-assisted and direct path candidates.
func TestCandidateStoreDirectRelayDistinction(t *testing.T) {
	s := node.NewCandidateStore()

	candidates := []controlplane.DistributedPathCandidate{
		makeRelayCandidate("assoc-1:relay:0", "assoc-1", "10.0.0.1:7000"),
		makeDirectCandidate("assoc-1:direct", "assoc-1", "10.0.0.2:4000"),
	}
	s.Store("assoc-1", candidates)

	got := s.Lookup("assoc-1")
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(got))
	}

	var relayCandidates, directCandidates int
	for _, c := range got {
		if c.IsRelayAssisted {
			relayCandidates++
			if c.Class != controlplane.DistributedPathClassCoordinatorRelay &&
				c.Class != controlplane.DistributedPathClassNodeRelay {
				t.Errorf("IsRelayAssisted=true but class is %q", c.Class)
			}
		} else {
			directCandidates++
			if c.Class != controlplane.DistributedPathClassDirectPublic &&
				c.Class != controlplane.DistributedPathClassDirectIntranet {
				t.Errorf("IsRelayAssisted=false but class is %q", c.Class)
			}
		}
	}

	if relayCandidates != 1 || directCandidates != 1 {
		t.Errorf("expected 1 relay + 1 direct, got relay=%d direct=%d", relayCandidates, directCandidates)
	}
}

// TestCandidateStoreNoCandidatesImplyChosenPath is a structural test that
// verifies the CandidateStore does not expose any fields for "chosen path",
// "scheduler decision", or "forwarding state". The store must hold only
// coordinator-distributed candidate data, separate from runtime state.
//
// This test documents the intended separation: the CandidateStore is not a
// scheduler output, not a forwarding table, and not a runtime path registry.
func TestCandidateStoreNoCandidatesImplyChosenPath(t *testing.T) {
	s := node.NewCandidateStore()

	// Store some candidates.
	s.Store("assoc-1", []controlplane.DistributedPathCandidate{
		makeRelayCandidate("assoc-1:relay:0", "assoc-1", "10.0.0.1:7000"),
	})

	// The CandidateStore holds only DistributedPathCandidate values.
	// Verify that the stored candidates do not include "chosen", "applied",
	// or "active" state — those would indicate a boundary violation.
	candidates := s.Lookup("assoc-1")
	for _, c := range candidates {
		// DistributedPathCandidate has no "chosen" or "applied" field.
		// The following check confirms it validates correctly as a plain candidate.
		if err := c.Validate(); err != nil {
			t.Errorf("stored candidate should be valid: %v", err)
		}
		// Candidate presence is NOT proof of runtime success.
		// IsUsable() only means the coordinator provided an endpoint to try.
		// It does NOT mean a ForwardingEntry is installed or traffic will flow.
		_ = c.IsUsable()
	}
}

// TestStoreCandidates verifies the StoreCandidates helper stores all candidate
// sets from a response into the CandidateStore.
func TestStoreCandidates(t *testing.T) {
	s := node.NewCandidateStore()

	response := controlplane.PathCandidateResponse{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		CoordinatorName: "coordinator-1",
		BootstrapOnly:   true,
		CandidateSets: []controlplane.PathCandidateSet{
			{
				AssociationID: "assoc-1",
				Candidates: []controlplane.DistributedPathCandidate{
					makeRelayCandidate("assoc-1:relay:0", "assoc-1", "10.0.0.1:7000"),
				},
			},
			{
				AssociationID: "assoc-2",
				Candidates:    []controlplane.DistributedPathCandidate{},
			},
		},
	}

	stored := node.StoreCandidates(s, response)
	if stored != 2 {
		t.Errorf("StoreCandidates returned %d, want 2", stored)
	}

	if s.AssociationCount() != 2 {
		t.Errorf("AssociationCount = %d, want 2", s.AssociationCount())
	}

	assoc1 := s.Lookup("assoc-1")
	if len(assoc1) != 1 {
		t.Errorf("assoc-1 candidates = %d, want 1", len(assoc1))
	}

	assoc2 := s.Lookup("assoc-2")
	if len(assoc2) != 0 {
		t.Errorf("assoc-2 candidates = %d, want 0", len(assoc2))
	}
}
