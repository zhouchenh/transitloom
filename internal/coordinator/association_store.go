package coordinator

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/service"
)

// AssociationStore stores the minimal coordinator-side placeholder association
// state. Records remain bootstrap-only placeholders until later authenticated
// control sessions, policy evaluation, and path/relay selection exist.
//
// Association records are intentionally narrow: they represent only that the
// coordinator accepted the intent and recorded the logical connectivity
// object. They do not imply path selection, relay eligibility, forwarding-state
// installation, or that traffic can already flow.
type AssociationStore struct {
	mu       sync.Mutex
	records  map[string]service.AssociationRecord
	registry *ServiceRegistry
}

func NewAssociationStore(registry *ServiceRegistry) *AssociationStore {
	return &AssociationStore{
		records:  make(map[string]service.AssociationRecord),
		registry: registry,
	}
}

// Apply processes a batch of association intents from a node. It validates
// each intent against the service registry and stores accepted associations.
// The registry is consulted to verify that both the source service (owned by
// the requesting node) and the destination service (owned by another node)
// are registered. This validation is intentionally minimal: it checks service
// existence, not full policy authorization.
func (s *AssociationStore) Apply(nodeName string, intents []service.AssociationIntent, now time.Time) []controlplane.AssociationResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	registrySnapshot := s.registry.snapshotMap()
	seen := make(map[string]struct{}, len(intents))
	results := make([]controlplane.AssociationResult, 0, len(intents))

	for _, intent := range intents {
		result := controlplane.AssociationResult{
			SourceServiceName:  intent.SourceService.Name,
			DestinationNode:    intent.DestinationNode,
			DestinationService: intent.DestinationService.Name,
		}

		key := service.AssociationKey(nodeName, intent.SourceService, intent.DestinationNode, intent.DestinationService)

		// Reject invalid intent shape.
		if err := intent.Validate(); err != nil {
			result.Outcome = controlplane.AssociationResultOutcomeRejected
			result.Reason = controlplane.AssociationResultReasonInvalidIntent
			result.Details = []string{
				fmt.Sprintf("association intent is invalid: %v", err),
			}
			results = append(results, result)
			continue
		}

		// Reject self-association (source and destination on same node
		// pointing to the same service).
		if strings.TrimSpace(nodeName) == strings.TrimSpace(intent.DestinationNode) &&
			intent.SourceService.Name == intent.DestinationService.Name &&
			intent.SourceService.Type == intent.DestinationService.Type {
			result.Outcome = controlplane.AssociationResultOutcomeRejected
			result.Reason = controlplane.AssociationResultReasonSelfAssociation
			result.Details = []string{
				"an association must connect two distinct service endpoints; source and destination resolve to the same service on the same node",
			}
			results = append(results, result)
			continue
		}

		// Reject duplicates within the same batch.
		if _, exists := seen[key]; exists {
			result.Outcome = controlplane.AssociationResultOutcomeRejected
			result.Reason = controlplane.AssociationResultReasonDuplicateAssociation
			result.Details = []string{
				"this association intent appeared more than once in the same batch",
			}
			results = append(results, result)
			continue
		}
		seen[key] = struct{}{}

		// Check that the source service is registered by the requesting node.
		sourceKey := service.RecordKey(nodeName, intent.SourceService)
		if _, exists := registrySnapshot[sourceKey]; !exists {
			result.Outcome = controlplane.AssociationResultOutcomeRejected
			result.Reason = controlplane.AssociationResultReasonSourceServiceNotRegistered
			result.Details = []string{
				fmt.Sprintf("source service %q is not registered by node %q", intent.SourceService.Name, nodeName),
				"service registration must precede association creation",
			}
			results = append(results, result)
			continue
		}

		// Check that the destination service is registered by the destination node.
		destKey := service.RecordKey(intent.DestinationNode, intent.DestinationService)
		if _, exists := registrySnapshot[destKey]; !exists {
			result.Outcome = controlplane.AssociationResultOutcomeRejected
			result.Reason = controlplane.AssociationResultReasonDestServiceNotRegistered
			result.Details = []string{
				fmt.Sprintf("destination service %q is not registered by node %q", intent.DestinationService.Name, intent.DestinationNode),
				"both services must be registered before an association can be created",
			}
			results = append(results, result)
			continue
		}

		// Create and store the association record.
		record := service.AssociationRecord{
			AssociationID:      key,
			SourceNode:         strings.TrimSpace(nodeName),
			SourceService:      intent.SourceService,
			DestinationNode:    strings.TrimSpace(intent.DestinationNode),
			DestinationService: intent.DestinationService,
			State:              service.AssociationStatePending,
			CreatedAt:          now,
			BootstrapOnly:      true,
		}

		_, replaced := s.records[key]
		s.records[key] = record

		result.Outcome = controlplane.AssociationResultOutcomeCreated
		result.Reason = controlplane.AssociationResultReasonCreated
		result.AssociationID = key
		if replaced {
			result.Details = append(result.Details, "replaced an existing bootstrap-only association record")
		} else {
			result.Details = append(result.Details, "stored a bootstrap-only association record")
		}
		result.Details = append(result.Details,
			"this association is a logical connectivity placeholder only; it does not imply path selection, relay eligibility, forwarding-state installation, or that traffic can flow",
		)

		results = append(results, result)
	}

	return results
}

func (s *AssociationStore) Snapshot() []service.AssociationRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	records := make([]service.AssociationRecord, 0, len(s.records))
	for _, record := range s.records {
		records = append(records, record.Clone())
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Key() < records[j].Key()
	})

	return records
}
