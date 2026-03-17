package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zhouchenh/transitloom/internal/service"
)

const AssociationPath = "/v1/bootstrap/association"

// AssociationOutcome is the top-level result of an association request.
type AssociationOutcome string

const (
	AssociationOutcomeAccepted       AssociationOutcome = "accepted"
	AssociationOutcomePartial        AssociationOutcome = "partial"
	AssociationOutcomeRejected       AssociationOutcome = "rejected"
	AssociationOutcomeInvalidRequest AssociationOutcome = "invalid-request"
)

// AssociationReason explains the top-level outcome.
type AssociationReason string

const (
	AssociationReasonCreated                      AssociationReason = "associations-created"
	AssociationReasonPartiallyCreated             AssociationReason = "associations-partially-created"
	AssociationReasonNoAssociationsCreated        AssociationReason = "no-associations-created"
	AssociationReasonBootstrapPrerequisitesNotMet AssociationReason = "bootstrap-prerequisites-not-satisfied"
	AssociationReasonCoordinatorAwaitingMaterial   AssociationReason = "coordinator-awaiting-intermediate"
	AssociationReasonInvalidRequest               AssociationReason = "invalid-request"
)

// AssociationResultOutcome is the per-association result outcome.
type AssociationResultOutcome string

const (
	AssociationResultOutcomeCreated  AssociationResultOutcome = "created"
	AssociationResultOutcomeRejected AssociationResultOutcome = "rejected"
)

// AssociationResultReason explains why an individual association was
// created or rejected.
type AssociationResultReason string

const (
	AssociationResultReasonCreated                    AssociationResultReason = "created"
	AssociationResultReasonSourceServiceNotRegistered AssociationResultReason = "source-service-not-registered"
	AssociationResultReasonDestServiceNotRegistered   AssociationResultReason = "destination-service-not-registered"
	AssociationResultReasonDuplicateAssociation       AssociationResultReason = "duplicate-association"
	AssociationResultReasonInvalidIntent              AssociationResultReason = "invalid-association-intent"
	AssociationResultReasonSelfAssociation            AssociationResultReason = "self-association-not-allowed"
)

// AssociationRequest carries the node's association intents to the coordinator
// using the bootstrap control path. Like service registration, this is a
// bootstrap-only exchange that does not claim final authentication or
// authorization semantics.
//
// Association creation is intentionally distinct from service registration.
// A registered service does not automatically have associations, and an
// association does not imply path selection, relay eligibility, or
// forwarding-state readiness.
type AssociationRequest struct {
	ProtocolVersion string                    `json:"protocol_version"`
	NodeName        string                    `json:"node_name"`
	Readiness       BootstrapReadinessSummary `json:"readiness"`
	Associations    []service.AssociationIntent `json:"associations"`
}

type AssociationResponse struct {
	ProtocolVersion string                `json:"protocol_version"`
	CoordinatorName string                `json:"coordinator_name"`
	Outcome         AssociationOutcome    `json:"outcome"`
	Reason          AssociationReason     `json:"reason"`
	BootstrapOnly   bool                  `json:"bootstrap_only"`
	AcceptedCount   int                   `json:"accepted_count"`
	RejectedCount   int                   `json:"rejected_count"`
	Results         []AssociationResult   `json:"results,omitempty"`
	Details         []string              `json:"details,omitempty"`
}

type AssociationResult struct {
	AssociationID      string                   `json:"association_id,omitempty"`
	SourceServiceName  string                   `json:"source_service_name,omitempty"`
	DestinationNode    string                   `json:"destination_node,omitempty"`
	DestinationService string                   `json:"destination_service_name,omitempty"`
	Outcome            AssociationResultOutcome `json:"outcome"`
	Reason             AssociationResultReason  `json:"reason"`
	Details            []string                 `json:"details,omitempty"`
}

func (r AssociationRequest) Validate() error {
	if r.ProtocolVersion != BootstrapProtocolVersion {
		return fmt.Errorf("protocol_version must be %q", BootstrapProtocolVersion)
	}
	if strings.TrimSpace(r.NodeName) == "" {
		return fmt.Errorf("node_name must be set")
	}
	if err := r.Readiness.Validate(); err != nil {
		return fmt.Errorf("readiness: %w", err)
	}
	if len(r.Associations) == 0 {
		return fmt.Errorf("associations must contain at least one association intent")
	}
	return nil
}

func (r AssociationResponse) Validate() error {
	if r.ProtocolVersion != BootstrapProtocolVersion {
		return fmt.Errorf("protocol_version must be %q", BootstrapProtocolVersion)
	}
	if strings.TrimSpace(r.CoordinatorName) == "" {
		return fmt.Errorf("coordinator_name must be set")
	}
	switch r.Outcome {
	case AssociationOutcomeAccepted, AssociationOutcomePartial, AssociationOutcomeRejected, AssociationOutcomeInvalidRequest:
	default:
		return fmt.Errorf("outcome must be a recognized association outcome")
	}
	switch r.Reason {
	case AssociationReasonCreated,
		AssociationReasonPartiallyCreated,
		AssociationReasonNoAssociationsCreated,
		AssociationReasonBootstrapPrerequisitesNotMet,
		AssociationReasonCoordinatorAwaitingMaterial,
		AssociationReasonInvalidRequest:
	default:
		return fmt.Errorf("reason must be a recognized association reason")
	}
	if !r.BootstrapOnly {
		return fmt.Errorf("bootstrap_only must remain true for the bootstrap association path")
	}
	for i, result := range r.Results {
		if err := result.Validate(); err != nil {
			return fmt.Errorf("results[%d]: %w", i, err)
		}
	}
	if len(r.Results) > 0 && r.AcceptedCount+r.RejectedCount != len(r.Results) {
		return fmt.Errorf("accepted_count + rejected_count must equal len(results)")
	}
	return nil
}

func (r AssociationResult) Validate() error {
	switch r.Outcome {
	case AssociationResultOutcomeCreated, AssociationResultOutcomeRejected:
	default:
		return fmt.Errorf("outcome must be a recognized association result outcome")
	}
	switch r.Reason {
	case AssociationResultReasonCreated,
		AssociationResultReasonSourceServiceNotRegistered,
		AssociationResultReasonDestServiceNotRegistered,
		AssociationResultReasonDuplicateAssociation,
		AssociationResultReasonInvalidIntent,
		AssociationResultReasonSelfAssociation:
	default:
		return fmt.Errorf("reason must be a recognized association result reason")
	}
	return nil
}

func (r AssociationResponse) AllCreated() bool {
	return r.Outcome == AssociationOutcomeAccepted && r.RejectedCount == 0
}

func (r AssociationResponse) AnyCreated() bool {
	return r.AcceptedCount > 0
}

func (c Client) RequestAssociations(ctx context.Context, endpoint string, request AssociationRequest) (AssociationResponse, error) {
	if err := request.Validate(); err != nil {
		return AssociationResponse{}, fmt.Errorf("association request invalid: %w", err)
	}

	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return AssociationResponse{}, fmt.Errorf("association endpoint must be set")
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return AssociationResponse{}, fmt.Errorf("marshal association request: %w", err)
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 3 * time.Second}
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+endpoint+AssociationPath, bytes.NewReader(payload))
	if err != nil {
		return AssociationResponse{}, fmt.Errorf("create association request for %q: %w", endpoint, err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return AssociationResponse{}, fmt.Errorf("perform association request to %q: %w", endpoint, err)
	}
	defer httpResponse.Body.Close()

	var response AssociationResponse
	if err := decodeSingleJSONObject(httpResponse.Body, &response); err != nil {
		return AssociationResponse{}, fmt.Errorf("decode association response from %q: %w", endpoint, err)
	}
	if err := response.Validate(); err != nil {
		return AssociationResponse{}, fmt.Errorf("invalid association response from %q: %w", endpoint, err)
	}

	return response, nil
}

func DecodeAssociationRequest(body io.Reader) (AssociationRequest, error) {
	var request AssociationRequest
	if err := decodeSingleJSONObject(body, &request); err != nil {
		return AssociationRequest{}, err
	}
	if err := request.Validate(); err != nil {
		return AssociationRequest{}, err
	}
	return request, nil
}

func WriteAssociationResponse(w http.ResponseWriter, statusCode int, response AssociationResponse) error {
	if err := response.Validate(); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(response)
}
