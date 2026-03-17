package node

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/service"
)

type AssociationAttemptResult struct {
	CoordinatorLabel string
	Endpoint         string
	Response         controlplane.AssociationResponse
}

// BuildAssociationRequest constructs an association request from the node's
// configured associations. It resolves service types from the local config
// and defaults the destination service type to raw-udp for v1.
//
// Association requests are intentionally distinct from service registration
// requests. A registered service does not automatically have associations.
func BuildAssociationRequest(cfg config.NodeConfig, bootstrap BootstrapState) (controlplane.AssociationRequest, error) {
	intents, err := service.BuildAssociationIntents(cfg)
	if err != nil {
		return controlplane.AssociationRequest{}, err
	}
	if len(intents) == 0 {
		return controlplane.AssociationRequest{}, fmt.Errorf("no associations configured")
	}

	return controlplane.AssociationRequest{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		NodeName:        cfg.Identity.Name,
		Readiness:       BuildBootstrapSessionRequest(cfg, bootstrap).Readiness,
		Associations:    intents,
	}, nil
}

// AttemptAssociation sends the association request to the coordinator endpoint
// that accepted the bootstrap control session. It requires both a successful
// bootstrap session and successful service registration as prerequisites.
func AttemptAssociation(ctx context.Context, cfg config.NodeConfig, bootstrap BootstrapState, session BootstrapSessionAttemptResult) (AssociationAttemptResult, error) {
	if !session.Response.Accepted() {
		return AssociationAttemptResult{}, fmt.Errorf("cannot attempt association because the bootstrap control session was not accepted")
	}
	if strings.TrimSpace(session.Endpoint) == "" {
		return AssociationAttemptResult{}, fmt.Errorf("cannot attempt association without a coordinator endpoint")
	}

	request, err := BuildAssociationRequest(cfg, bootstrap)
	if err != nil {
		return AssociationAttemptResult{}, fmt.Errorf("build association request: %w", err)
	}

	client := controlplane.Client{
		HTTPClient: &http.Client{Timeout: 3 * time.Second},
	}

	response, err := client.RequestAssociations(ctx, session.Endpoint, request)
	if err != nil {
		return AssociationAttemptResult{}, fmt.Errorf("association attempt to %q failed: %w", session.Endpoint, err)
	}

	return AssociationAttemptResult{
		CoordinatorLabel: session.CoordinatorLabel,
		Endpoint:         session.Endpoint,
		Response:         response,
	}, nil
}

func (r AssociationAttemptResult) ReportLines() []string {
	lines := make([]string, 0, len(r.Response.Results)+len(r.Response.Details)+4)

	if r.Endpoint != "" {
		lines = append(lines, fmt.Sprintf("node bootstrap association target: coordinator=%s endpoint=%s", r.CoordinatorLabel, r.Endpoint))
	}
	if r.Response.Outcome != "" {
		lines = append(lines, fmt.Sprintf("node bootstrap association outcome: %s (%s)", r.Response.Outcome, r.Response.Reason))
		lines = append(lines, fmt.Sprintf("node bootstrap association counts: accepted=%d rejected=%d", r.Response.AcceptedCount, r.Response.RejectedCount))
		for _, detail := range r.Response.Details {
			lines = append(lines, "node bootstrap association detail: "+detail)
		}
		for _, result := range r.Response.Results {
			lines = append(lines, fmt.Sprintf(
				"node bootstrap association result: source=%s destination_node=%s destination_service=%s outcome=%s reason=%s association_id=%s",
				describeAssociationService(result.SourceServiceName),
				describeAssociationNode(result.DestinationNode),
				describeAssociationService(result.DestinationService),
				result.Outcome,
				result.Reason,
				describeAssociationID(result.AssociationID),
			))
			for _, detail := range result.Details {
				lines = append(lines, "node bootstrap association detail: "+detail)
			}
		}
	}

	return lines
}

func describeAssociationService(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "<unnamed-service>"
	}
	return name
}

func describeAssociationNode(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "<unnamed-node>"
	}
	return name
}

func describeAssociationID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "<none>"
	}
	return id
}
