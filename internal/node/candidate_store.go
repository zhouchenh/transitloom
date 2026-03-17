package node

import (
	"sort"
	"sync"

	"github.com/zhouchenh/transitloom/internal/controlplane"
)

// CandidateStore is the node-side store for coordinator-distributed path
// candidates.
//
// It stores only the candidates received from the coordinator. It does NOT store:
//   - scheduler decisions (which path was chosen by Scheduler.Decide())
//   - forwarding state (ForwardingEntry / RelayForwardingEntry from dataplane)
//   - locally derived candidates (e.g. from direct_endpoint config strings)
//   - runtime quality measurements or health state
//
// This separation is required by the architecture:
//   - Distributed candidate data (coordinator knowledge) must remain distinct
//     from local runtime scheduling and forwarding state.
//   - Candidate presence means only: the coordinator asserts this path may be
//     available. It does NOT mean traffic will succeed on that path.
//   - The chosen path is produced by Scheduler.Decide() from local + distributed
//     inputs; it is NOT stored in the CandidateStore.
//   - Installed forwarding state (ForwardingEntry) exists only when carriage is
//     actually active; it is NOT related to candidate existence.
//
// (spec/v1-object-model.md sections 15-17, agents/MEMORY.md durable decisions)
type CandidateStore struct {
	mu sync.RWMutex
	// candidates maps association ID to coordinator-distributed candidates.
	candidates map[string][]controlplane.DistributedPathCandidate
}

// NewCandidateStore creates an empty CandidateStore.
func NewCandidateStore() *CandidateStore {
	return &CandidateStore{
		candidates: make(map[string][]controlplane.DistributedPathCandidate),
	}
}

// Store saves the received candidates for an association, replacing any
// previously stored candidates for that association.
//
// Candidates stored here are the coordinator's view of possible paths; they are
// separate from any locally configured path endpoints and from scheduler decisions.
func (s *CandidateStore) Store(associationID string, candidates []controlplane.DistributedPathCandidate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Copy to avoid sharing the caller's underlying slice.
	stored := make([]controlplane.DistributedPathCandidate, len(candidates))
	copy(stored, candidates)
	s.candidates[associationID] = stored
}

// Lookup returns the stored candidates for an association, or nil if none.
// The returned slice is a copy; mutations do not affect the store.
func (s *CandidateStore) Lookup(associationID string) []controlplane.DistributedPathCandidate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	candidates, exists := s.candidates[associationID]
	if !exists {
		return nil
	}
	result := make([]controlplane.DistributedPathCandidate, len(candidates))
	copy(result, candidates)
	return result
}

// Snapshot returns a copy of all stored candidate sets, sorted by association ID.
func (s *CandidateStore) Snapshot() []controlplane.PathCandidateSet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sets := make([]controlplane.PathCandidateSet, 0, len(s.candidates))
	for assocID, candidates := range s.candidates {
		copied := make([]controlplane.DistributedPathCandidate, len(candidates))
		copy(copied, candidates)
		sets = append(sets, controlplane.PathCandidateSet{
			AssociationID: assocID,
			Candidates:    copied,
		})
	}
	sort.Slice(sets, func(i, j int) bool {
		return sets[i].AssociationID < sets[j].AssociationID
	})
	return sets
}

// AssociationCount returns the number of associations with stored candidates.
func (s *CandidateStore) AssociationCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.candidates)
}
