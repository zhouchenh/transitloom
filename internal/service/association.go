package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
)

// AssociationState represents the current lifecycle state of an association.
// At this stage only "pending" exists because association creation does not
// yet imply path selection, relay eligibility, forwarding-state installation,
// or that traffic can actually flow. Later work will add additional states.
type AssociationState string

const (
	AssociationStatePending AssociationState = "pending"
)

// AssociationIntent is a node-submitted request for a single association
// between one of its local services and a remote service on another node.
//
// An association intent is intentionally narrow: it names the source and
// destination service identities without implying path selection, relay
// eligibility, or data-plane readiness. Association creation is distinct
// from service registration and from authorization completion.
type AssociationIntent struct {
	SourceService      Identity `json:"source_service"`
	DestinationNode    string   `json:"destination_node"`
	DestinationService Identity `json:"destination_service"`
}

// AssociationRecord is the coordinator-side placeholder state for an
// association between two registered services. At this stage it represents
// only that the coordinator accepted the association intent and recorded it;
// it does not imply:
//   - path selection or path candidate discovery
//   - relay selection or relay eligibility
//   - forwarding-state installation
//   - data-plane readiness or active traffic flow
//   - complete authorization for all future behavior
//
// The record is intentionally narrow so later policy, path, and forwarding
// work can extend it safely without finding that association already claimed
// too much.
type AssociationRecord struct {
	AssociationID      string
	SourceNode         string
	SourceService      Identity
	DestinationNode    string
	DestinationService Identity
	State              AssociationState
	CreatedAt          time.Time
	BootstrapOnly      bool
}

// AssociationKey produces a deterministic key that uniquely identifies an
// association by its source and destination service endpoints. This key is
// used for deduplication and lookup in the coordinator-side association store.
func AssociationKey(sourceNode string, sourceService Identity, destNode string, destService Identity) string {
	return fmt.Sprintf("%s/%s:%s->%s/%s:%s",
		strings.TrimSpace(sourceNode),
		strings.TrimSpace(sourceService.Name), sourceService.Type,
		strings.TrimSpace(destNode),
		strings.TrimSpace(destService.Name), destService.Type,
	)
}

func (r AssociationRecord) Key() string {
	return AssociationKey(r.SourceNode, r.SourceService, r.DestinationNode, r.DestinationService)
}

func (r AssociationRecord) Clone() AssociationRecord {
	return AssociationRecord{
		AssociationID:      r.AssociationID,
		SourceNode:         r.SourceNode,
		SourceService:      r.SourceService,
		DestinationNode:    r.DestinationNode,
		DestinationService: r.DestinationService,
		State:              r.State,
		CreatedAt:          r.CreatedAt,
		BootstrapOnly:      r.BootstrapOnly,
	}
}

func (i AssociationIntent) Validate() error {
	if err := i.SourceService.Validate(); err != nil {
		return fmt.Errorf("source_service: %w", err)
	}
	if strings.TrimSpace(i.DestinationNode) == "" {
		return fmt.Errorf("destination_node must be set")
	}
	if err := i.DestinationService.Validate(); err != nil {
		return fmt.Errorf("destination_service: %w", err)
	}
	return nil
}

// BuildAssociationIntents constructs association intents from the node config.
// It resolves the source service type from the local services config and
// defaults the destination service type to raw-udp for v1.
func BuildAssociationIntents(cfg config.NodeConfig) ([]AssociationIntent, error) {
	if len(cfg.Associations) == 0 {
		return nil, nil
	}

	serviceTypes := make(map[string]config.ServiceType, len(cfg.Services))
	for _, svc := range cfg.Services {
		serviceTypes[svc.Name] = svc.Type
	}

	intents := make([]AssociationIntent, 0, len(cfg.Associations))
	for i, assoc := range cfg.Associations {
		sourceType, exists := serviceTypes[assoc.SourceService]
		if !exists {
			return nil, fmt.Errorf("associations[%d]: source_service %q does not match any configured service", i, assoc.SourceService)
		}

		intent := AssociationIntent{
			SourceService: Identity{
				Name: strings.TrimSpace(assoc.SourceService),
				Type: sourceType,
			},
			DestinationNode: strings.TrimSpace(assoc.DestinationNode),
			DestinationService: Identity{
				// Destination service type defaults to raw-udp for v1 because
				// that is the only supported service type. The node does not
				// have the remote service config, so this is a safe default.
				Name: strings.TrimSpace(assoc.DestinationService),
				Type: config.ServiceTypeRawUDP,
			},
		}

		if err := intent.Validate(); err != nil {
			return nil, fmt.Errorf("associations[%d]: %w", i, err)
		}

		intents = append(intents, intent)
	}

	return intents, nil
}
