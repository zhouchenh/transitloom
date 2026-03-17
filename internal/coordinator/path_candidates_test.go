package coordinator_test

import (
	"strings"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/coordinator"
	"github.com/zhouchenh/transitloom/internal/service"
)

func makeTestAssocRecord(associationID, srcNode, dstNode string) service.AssociationRecord {
	return service.AssociationRecord{
		AssociationID:   associationID,
		SourceNode:      srcNode,
		SourceService:   service.Identity{Name: "svc-a", Type: "raw-udp"},
		DestinationNode: dstNode,
		DestinationService: service.Identity{Name: "svc-b", Type: "raw-udp"},
		State:           service.AssociationStatePending,
		CreatedAt:       time.Now(),
		BootstrapOnly:   true,
	}
}

func relayEnabledCfg(endpoints ...string) config.CoordinatorRelayConfig {
	return config.CoordinatorRelayConfig{
		DataEnabled:     true,
		ControlEnabled:  false,
		DrainMode:       false,
		ListenEndpoints: endpoints,
	}
}

func relayDisabledCfg() config.CoordinatorRelayConfig {
	return config.CoordinatorRelayConfig{DataEnabled: false}
}

func relayDrainCfg(endpoints ...string) config.CoordinatorRelayConfig {
	return config.CoordinatorRelayConfig{
		DataEnabled:     true,
		DrainMode:       true,
		ListenEndpoints: endpoints,
	}
}

// TestGenerateCandidatesForAssociation_RelayEnabled verifies that the coordinator
// generates relay-assisted candidates when data relay is enabled.
func TestGenerateCandidatesForAssociation_RelayEnabled(t *testing.T) {
	assoc := makeTestAssocRecord("assoc-1", "node-a", "node-b")
	relayCfg := relayEnabledCfg("10.0.0.1:7000", "10.0.0.2:7001")

	set := coordinator.GenerateCandidatesForAssociation(assoc, relayCfg, "coordinator-1")

	if set.AssociationID != "assoc-1" {
		t.Errorf("AssociationID = %q, want %q", set.AssociationID, "assoc-1")
	}
	if set.SourceNode != "node-a" {
		t.Errorf("SourceNode = %q, want %q", set.SourceNode, "node-a")
	}
	if set.DestinationNode != "node-b" {
		t.Errorf("DestinationNode = %q, want %q", set.DestinationNode, "node-b")
	}

	if len(set.Candidates) != 2 {
		t.Fatalf("expected 2 relay candidates, got %d", len(set.Candidates))
	}

	for i, c := range set.Candidates {
		t.Run(c.CandidateID, func(t *testing.T) {
			if err := c.Validate(); err != nil {
				t.Errorf("candidate[%d] Validate(): %v", i, err)
			}
			if c.AssociationID != "assoc-1" {
				t.Errorf("candidate AssociationID = %q", c.AssociationID)
			}
			// Relay candidates must be explicitly marked as relay-assisted.
			if !c.IsRelayAssisted {
				t.Errorf("relay candidate IsRelayAssisted should be true")
			}
			if c.Class != controlplane.DistributedPathClassCoordinatorRelay {
				t.Errorf("class = %q, want coordinator-relay", c.Class)
			}
			// The relay node ID identifies the coordinator as the relay participant.
			if c.RelayNodeID != "coordinator-1" {
				t.Errorf("RelayNodeID = %q, want coordinator-1", c.RelayNodeID)
			}
			// Relay candidates must have a usable remote endpoint.
			if !c.IsUsable() {
				t.Errorf("relay candidate should be usable (has RemoteEndpoint)")
			}
		})
	}

	// Verify endpoints are set correctly.
	endpoints := map[string]bool{
		set.Candidates[0].RemoteEndpoint: true,
		set.Candidates[1].RemoteEndpoint: true,
	}
	if !endpoints["10.0.0.1:7000"] || !endpoints["10.0.0.2:7001"] {
		t.Errorf("relay endpoints not set correctly: %v", endpoints)
	}
}

// TestGenerateCandidatesForAssociation_RelayDisabled verifies that no relay
// candidates are generated when data relay is disabled, and that a note
// explains the absence.
func TestGenerateCandidatesForAssociation_RelayDisabled(t *testing.T) {
	assoc := makeTestAssocRecord("assoc-1", "node-a", "node-b")
	set := coordinator.GenerateCandidatesForAssociation(assoc, relayDisabledCfg(), "coordinator-1")

	// No relay candidates when relay is disabled.
	relayCandidates := 0
	for _, c := range set.Candidates {
		if c.IsRelayAssisted {
			relayCandidates++
		}
	}
	if relayCandidates != 0 {
		t.Errorf("expected 0 relay candidates when relay disabled, got %d", relayCandidates)
	}

	// A note must explain why no relay candidates were generated.
	if len(set.Notes) == 0 {
		t.Error("expected at least one note explaining absent relay candidates")
	}
	found := false
	for _, n := range set.Notes {
		if strings.Contains(n, "not enabled") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("notes should mention relay not being enabled; notes: %v", set.Notes)
	}
}

// TestGenerateCandidatesForAssociation_DrainMode verifies that no relay
// candidates are generated when the coordinator relay is in drain mode.
func TestGenerateCandidatesForAssociation_DrainMode(t *testing.T) {
	assoc := makeTestAssocRecord("assoc-1", "node-a", "node-b")
	set := coordinator.GenerateCandidatesForAssociation(assoc, relayDrainCfg("10.0.0.1:7000"), "coordinator-1")

	for _, c := range set.Candidates {
		if c.IsRelayAssisted {
			t.Errorf("relay candidate generated in drain mode: %+v", c)
		}
	}

	drainNoted := false
	for _, n := range set.Notes {
		if strings.Contains(n, "drain") {
			drainNoted = true
			break
		}
	}
	if !drainNoted {
		t.Errorf("notes should mention drain mode; notes: %v", set.Notes)
	}
}

// TestGenerateCandidatesForAssociation_NoCandidatesNotedAsDirectMissing verifies
// that the coordinator always includes a note explaining that direct candidates
// require node endpoint advertisement, so operators know why they are absent.
func TestGenerateCandidatesForAssociation_NoCandidatesNotedAsDirectMissing(t *testing.T) {
	assoc := makeTestAssocRecord("assoc-1", "node-a", "node-b")

	// Test both relay-enabled and relay-disabled configurations.
	for _, relayCfg := range []config.CoordinatorRelayConfig{
		relayEnabledCfg("10.0.0.1:7000"),
		relayDisabledCfg(),
	} {
		set := coordinator.GenerateCandidatesForAssociation(assoc, relayCfg, "coordinator-1")
		found := false
		for _, n := range set.Notes {
			if strings.Contains(n, "direct") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("notes should mention direct candidate absence; notes: %v", set.Notes)
		}
	}
}

// TestGenerateCandidatesForAssociation_CandidatesAreAssociationBound verifies
// that every generated candidate carries the correct AssociationID. This enforces
// the association-bound scheduling requirement: candidates must not be used for
// associations other than the one they were generated for.
func TestGenerateCandidatesForAssociation_CandidatesAreAssociationBound(t *testing.T) {
	assoc := makeTestAssocRecord("the-real-assoc", "node-x", "node-y")
	relayCfg := relayEnabledCfg("10.0.0.1:7000")
	set := coordinator.GenerateCandidatesForAssociation(assoc, relayCfg, "coordinator-1")

	for i, c := range set.Candidates {
		if c.AssociationID != "the-real-assoc" {
			t.Errorf("candidate[%d] has wrong AssociationID %q", i, c.AssociationID)
		}
	}
}

// TestGenerateCandidatesForAssociation_NoCandidatesImplyChosenPath verifies
// that the generated candidates have no "chosen" or "active" or "applied" fields.
// The DistributedPathCandidate type must represent only coordinator knowledge,
// not runtime selection state.
func TestGenerateCandidatesForAssociation_NoCandidatesImplyChosenPath(t *testing.T) {
	assoc := makeTestAssocRecord("assoc-1", "node-a", "node-b")
	relayCfg := relayEnabledCfg("10.0.0.1:7000")
	set := coordinator.GenerateCandidatesForAssociation(assoc, relayCfg, "coordinator-1")

	// Structural check: the PathCandidateSet and DistributedPathCandidate types
	// do not have "chosen", "applied", "active", or "forwarding" fields.
	// We verify this by checking that Validate passes for the generated set —
	// a set with a "chosen" candidate would require a different type entirely.
	if err := set.Validate(); err != nil {
		t.Errorf("generated set should be valid: %v", err)
	}

	for _, c := range set.Candidates {
		// A candidate with a usable endpoint is a potential path, not a confirmed path.
		// The scheduler decision (Scheduler.Decide) is a separate step.
		if c.IsUsable() {
			// IsUsable() means RemoteEndpoint is set — this is a potential path,
			// but there is no "chosen" or "applied" annotation on it.
			if err := c.Validate(); err != nil {
				t.Errorf("usable candidate invalid: %v", err)
			}
		}
	}
}

// TestGenerateCandidateSets verifies that GenerateCandidateSets generates
// candidate sets only for known association IDs.
func TestGenerateCandidateSets(t *testing.T) {
	registry := coordinator.NewServiceRegistry()
	store := coordinator.NewAssociationStore(registry)

	// Create two test associations via the store directly.
	// We seed the registry first so the association store accepts the records.
	registry.Apply("node-a", []service.Registration{
		{Identity: service.Identity{Name: "svc-a", Type: "raw-udp"},
			Binding: service.Binding{LocalTarget: service.LocalTarget{Address: "127.0.0.1", Port: 51820}}},
	}, time.Now())
	registry.Apply("node-b", []service.Registration{
		{Identity: service.Identity{Name: "svc-b", Type: "raw-udp"},
			Binding: service.Binding{LocalTarget: service.LocalTarget{Address: "127.0.0.1", Port: 51821}}},
	}, time.Now())

	intents := []service.AssociationIntent{
		{
			SourceService:      service.Identity{Name: "svc-a", Type: "raw-udp"},
			DestinationNode:    "node-b",
			DestinationService: service.Identity{Name: "svc-b", Type: "raw-udp"},
		},
	}
	results := store.Apply("node-a", intents, time.Now())
	if len(results) != 1 || results[0].Outcome != controlplane.AssociationResultOutcomeCreated {
		t.Fatalf("setup: expected one created association, got: %+v", results)
	}
	associationID := results[0].AssociationID

	relayCfg := relayEnabledCfg("10.0.0.1:7000")

	t.Run("known association ID returns set", func(t *testing.T) {
		sets := coordinator.GenerateCandidateSets(store, []string{associationID}, relayCfg, "coordinator-1")
		if len(sets) != 1 {
			t.Fatalf("expected 1 set, got %d", len(sets))
		}
		if sets[0].AssociationID != associationID {
			t.Errorf("set AssociationID = %q", sets[0].AssociationID)
		}
	})

	t.Run("unknown association ID silently skipped", func(t *testing.T) {
		sets := coordinator.GenerateCandidateSets(store, []string{"does-not-exist"}, relayCfg, "coordinator-1")
		if len(sets) != 0 {
			t.Errorf("expected 0 sets for unknown ID, got %d", len(sets))
		}
	})

	t.Run("mixed known and unknown IDs", func(t *testing.T) {
		sets := coordinator.GenerateCandidateSets(store, []string{associationID, "unknown-id"}, relayCfg, "coordinator-1")
		if len(sets) != 1 {
			t.Errorf("expected 1 set for mixed IDs, got %d", len(sets))
		}
	})

	t.Run("empty ID strings skipped", func(t *testing.T) {
		sets := coordinator.GenerateCandidateSets(store, []string{"", "  "}, relayCfg, "coordinator-1")
		if len(sets) != 0 {
			t.Errorf("expected 0 sets for blank IDs, got %d", len(sets))
		}
	})
}

// TestGenerateCandidates_RelayDirectDistinction verifies that relay and direct
// candidates are strictly distinct in the generated output. This is a belt-and-
// suspenders check for the core architectural invariant.
func TestGenerateCandidates_RelayDirectDistinction(t *testing.T) {
	assoc := makeTestAssocRecord("assoc-relay-test", "node-a", "node-b")
	relayCfg := relayEnabledCfg("10.0.0.1:7000")

	set := coordinator.GenerateCandidatesForAssociation(assoc, relayCfg, "coordinator-1")

	for _, c := range set.Candidates {
		isRelayClass := c.Class == controlplane.DistributedPathClassCoordinatorRelay ||
			c.Class == controlplane.DistributedPathClassNodeRelay
		if isRelayClass != c.IsRelayAssisted {
			t.Errorf("candidate %q: class %q and IsRelayAssisted=%v are inconsistent",
				c.CandidateID, c.Class, c.IsRelayAssisted)
		}
	}
}
