package coordinator

import (
	"fmt"
	"strings"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/service"
)

// GenerateCandidatesForAssociation generates the coordinator's known path
// candidates for a single association.
//
// This function asserts only what the coordinator can currently know:
//   - relay-assisted candidates: generated when the coordinator has data relay
//     enabled and relay listen endpoints configured
//   - direct candidates: require explicit node external endpoint advertisement,
//     which is not yet available in the bootstrap-only control path
//
// What this function does NOT claim:
//   - verified reachability for any generated candidate
//   - measured or scored path quality
//   - chosen or active runtime path state
//   - forwarding-state installation or readiness
//
// The distinction between relay candidate and path candidate is preserved:
// a relay-assisted DistributedPathCandidate represents the resolved path that
// uses a relay, not the relay itself (spec/v1-object-model.md section 16-17).
// The relay participant identity is carried in RelayNodeID as a reference, not
// collapsed into the path candidate.
//
// Candidates are association-bound: every generated candidate carries the
// AssociationID from the association record.
func GenerateCandidatesForAssociation(
	assoc service.AssociationRecord,
	relayCfg config.CoordinatorRelayConfig,
	coordinatorNodeID string,
) controlplane.PathCandidateSet {
	set := controlplane.PathCandidateSet{
		AssociationID:   assoc.AssociationID,
		SourceNode:      assoc.SourceNode,
		DestinationNode: assoc.DestinationNode,
	}

	var candidates []controlplane.DistributedPathCandidate
	var notes []string

	// Generate relay-assisted candidates if data relay is active.
	// Each configured relay listen endpoint becomes one candidate.
	// The single-hop v1 constraint (spec/v1-data-plane.md section 7) is
	// preserved: each relay candidate points to one coordinator relay endpoint;
	// there is no "next relay" field.
	if relayCfg.DataEnabled && !relayCfg.DrainMode {
		for i, endpoint := range relayCfg.ListenEndpoints {
			endpoint = strings.TrimSpace(endpoint)
			if endpoint == "" {
				continue
			}
			candidateID := fmt.Sprintf("%s:coordinator-relay:%d", assoc.AssociationID, i)
			candidates = append(candidates, controlplane.DistributedPathCandidate{
				CandidateID:   candidateID,
				AssociationID: assoc.AssociationID,
				// Relay path: source → this coordinator endpoint → destination.
				// The v1 single-hop constraint is implicit: this candidate has no
				// next-relay reference, so it cannot form a relay chain.
				Class:           controlplane.DistributedPathClassCoordinatorRelay,
				IsRelayAssisted: true,
				RemoteEndpoint:  endpoint,
				RelayNodeID:     coordinatorNodeID,
				AdminWeight:     100,
				Note:            "coordinator data relay endpoint; v1 single-hop relay only",
			})
		}
		if len(candidates) == 0 {
			notes = append(notes, "coordinator relay is enabled but no relay listen endpoints are configured")
		}
	} else if relayCfg.DrainMode {
		notes = append(notes, "coordinator relay is in drain mode; no relay-assisted candidates generated")
	} else {
		notes = append(notes, "coordinator data relay is not enabled; no relay-assisted candidates available")
	}

	// Direct candidates require node external endpoint advertisement.
	// Until nodes advertise their external endpoints to the coordinator in a
	// subsequent control-plane exchange, the coordinator cannot generate direct
	// path candidates. This is intentional: endpoint advertisement must not be
	// collapsed into path-candidate truth (task T-0018 non-negotiable constraint).
	notes = append(notes,
		"direct path candidates require node external endpoint advertisement; not yet available in bootstrap-only mode",
	)

	set.Candidates = candidates
	set.Notes = notes
	return set
}

// GenerateCandidateSets generates path-candidate sets for a batch of
// association IDs. Requested IDs not found in the store are silently skipped.
//
// The function snapshots the association store before generating to avoid
// holding the store lock during candidate construction.
func GenerateCandidateSets(
	store *AssociationStore,
	associationIDs []string,
	relayCfg config.CoordinatorRelayConfig,
	coordinatorNodeID string,
) []controlplane.PathCandidateSet {
	records := store.Snapshot()
	recordMap := make(map[string]service.AssociationRecord, len(records))
	for _, r := range records {
		recordMap[r.AssociationID] = r
	}

	sets := make([]controlplane.PathCandidateSet, 0, len(associationIDs))
	for _, id := range associationIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		assoc, exists := recordMap[id]
		if !exists {
			continue
		}
		sets = append(sets, GenerateCandidatesForAssociation(assoc, relayCfg, coordinatorNodeID))
	}
	return sets
}
