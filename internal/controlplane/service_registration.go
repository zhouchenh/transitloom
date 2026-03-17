package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zhouchenh/transitloom/internal/service"
)

const ServiceRegistrationPath = "/v1/bootstrap/service-registration"

type ServiceRegistrationOutcome string

const (
	ServiceRegistrationOutcomeAccepted       ServiceRegistrationOutcome = "accepted"
	ServiceRegistrationOutcomePartial        ServiceRegistrationOutcome = "partial"
	ServiceRegistrationOutcomeRejected       ServiceRegistrationOutcome = "rejected"
	ServiceRegistrationOutcomeInvalidRequest ServiceRegistrationOutcome = "invalid-request"
)

type ServiceRegistrationReason string

const (
	ServiceRegistrationReasonRegistered                   ServiceRegistrationReason = "registered"
	ServiceRegistrationReasonPartiallyRegistered          ServiceRegistrationReason = "partially-registered"
	ServiceRegistrationReasonNoServicesRegistered         ServiceRegistrationReason = "no-services-registered"
	ServiceRegistrationReasonBootstrapPrerequisitesNotMet ServiceRegistrationReason = "bootstrap-prerequisites-not-satisfied"
	ServiceRegistrationReasonCoordinatorAwaitingMaterial  ServiceRegistrationReason = "coordinator-awaiting-intermediate"
	ServiceRegistrationReasonInvalidRequest               ServiceRegistrationReason = "invalid-request"
)

type ServiceRegistrationResultOutcome string

const (
	ServiceRegistrationResultOutcomeRegistered ServiceRegistrationResultOutcome = "registered"
	ServiceRegistrationResultOutcomeRejected   ServiceRegistrationResultOutcome = "rejected"
)

type ServiceRegistrationResultReason string

const (
	ServiceRegistrationResultReasonRegistered         ServiceRegistrationResultReason = "registered"
	ServiceRegistrationResultReasonDuplicateService   ServiceRegistrationResultReason = "duplicate-service"
	ServiceRegistrationResultReasonInvalidServiceDecl ServiceRegistrationResultReason = "invalid-service-declaration"
)

// ServiceRegistrationRequest keeps the bootstrap-only readiness summary
// explicit because this JSON exchange still does not create an authenticated
// long-lived control session. Services are validated individually by the
// coordinator so malformed declarations can be rejected without hiding which
// entry failed.
type ServiceRegistrationRequest struct {
	ProtocolVersion string                    `json:"protocol_version"`
	NodeName        string                    `json:"node_name"`
	Readiness       BootstrapReadinessSummary `json:"readiness"`
	Services        []service.Registration    `json:"services"`
}

type ServiceRegistrationResponse struct {
	ProtocolVersion string                      `json:"protocol_version"`
	CoordinatorName string                      `json:"coordinator_name"`
	Outcome         ServiceRegistrationOutcome  `json:"outcome"`
	Reason          ServiceRegistrationReason   `json:"reason"`
	BootstrapOnly   bool                        `json:"bootstrap_only"`
	AcceptedCount   int                         `json:"accepted_count"`
	RejectedCount   int                         `json:"rejected_count"`
	Results         []ServiceRegistrationResult `json:"results,omitempty"`
	Details         []string                    `json:"details,omitempty"`
}

type ServiceRegistrationResult struct {
	ServiceName string                           `json:"service_name,omitempty"`
	ServiceType string                           `json:"service_type,omitempty"`
	Outcome     ServiceRegistrationResultOutcome `json:"outcome"`
	Reason      ServiceRegistrationResultReason  `json:"reason"`
	RegistryKey string                           `json:"registry_key,omitempty"`
	Details     []string                         `json:"details,omitempty"`
}

func (r ServiceRegistrationRequest) Validate() error {
	if r.ProtocolVersion != BootstrapProtocolVersion {
		return fmt.Errorf("protocol_version must be %q", BootstrapProtocolVersion)
	}
	if strings.TrimSpace(r.NodeName) == "" {
		return fmt.Errorf("node_name must be set")
	}
	if err := r.Readiness.Validate(); err != nil {
		return fmt.Errorf("readiness: %w", err)
	}
	if len(r.Services) == 0 {
		return fmt.Errorf("services must contain at least one service declaration")
	}
	return nil
}

func (r ServiceRegistrationResponse) Validate() error {
	if r.ProtocolVersion != BootstrapProtocolVersion {
		return fmt.Errorf("protocol_version must be %q", BootstrapProtocolVersion)
	}
	if strings.TrimSpace(r.CoordinatorName) == "" {
		return fmt.Errorf("coordinator_name must be set")
	}
	switch r.Outcome {
	case ServiceRegistrationOutcomeAccepted, ServiceRegistrationOutcomePartial, ServiceRegistrationOutcomeRejected, ServiceRegistrationOutcomeInvalidRequest:
	default:
		return fmt.Errorf("outcome must be a recognized service-registration outcome")
	}
	switch r.Reason {
	case ServiceRegistrationReasonRegistered,
		ServiceRegistrationReasonPartiallyRegistered,
		ServiceRegistrationReasonNoServicesRegistered,
		ServiceRegistrationReasonBootstrapPrerequisitesNotMet,
		ServiceRegistrationReasonCoordinatorAwaitingMaterial,
		ServiceRegistrationReasonInvalidRequest:
	default:
		return fmt.Errorf("reason must be a recognized service-registration reason")
	}
	if !r.BootstrapOnly {
		return fmt.Errorf("bootstrap_only must remain true for the bootstrap service-registration path")
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

func (r ServiceRegistrationResult) Validate() error {
	switch r.Outcome {
	case ServiceRegistrationResultOutcomeRegistered, ServiceRegistrationResultOutcomeRejected:
	default:
		return fmt.Errorf("outcome must be a recognized service-registration result outcome")
	}
	switch r.Reason {
	case ServiceRegistrationResultReasonRegistered, ServiceRegistrationResultReasonDuplicateService, ServiceRegistrationResultReasonInvalidServiceDecl:
	default:
		return fmt.Errorf("reason must be a recognized service-registration result reason")
	}
	return nil
}

func (r ServiceRegistrationResponse) AllRegistered() bool {
	return r.Outcome == ServiceRegistrationOutcomeAccepted && r.RejectedCount == 0
}

func (r ServiceRegistrationResponse) AnyRegistered() bool {
	return r.AcceptedCount > 0
}

func (c Client) RegisterServices(ctx context.Context, endpoint string, request ServiceRegistrationRequest) (ServiceRegistrationResponse, error) {
	if err := request.Validate(); err != nil {
		return ServiceRegistrationResponse{}, fmt.Errorf("service registration request invalid: %w", err)
	}

	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ServiceRegistrationResponse{}, fmt.Errorf("service registration endpoint must be set")
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return ServiceRegistrationResponse{}, fmt.Errorf("marshal service registration request: %w", err)
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: BootstrapConnectTimeout}
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+endpoint+ServiceRegistrationPath, bytes.NewReader(payload))
	if err != nil {
		return ServiceRegistrationResponse{}, fmt.Errorf("create service registration request for %q: %w", endpoint, err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return ServiceRegistrationResponse{}, fmt.Errorf("perform service registration request to %q: %w", endpoint, err)
	}
	defer httpResponse.Body.Close()

	var response ServiceRegistrationResponse
	if err := decodeSingleJSONObject(httpResponse.Body, &response); err != nil {
		return ServiceRegistrationResponse{}, fmt.Errorf("decode service registration response from %q: %w", endpoint, err)
	}
	if err := response.Validate(); err != nil {
		return ServiceRegistrationResponse{}, fmt.Errorf("invalid service registration response from %q: %w", endpoint, err)
	}

	return response, nil
}

func DecodeServiceRegistrationRequest(body io.Reader) (ServiceRegistrationRequest, error) {
	var request ServiceRegistrationRequest
	if err := decodeSingleJSONObject(body, &request); err != nil {
		return ServiceRegistrationRequest{}, err
	}
	if err := request.Validate(); err != nil {
		return ServiceRegistrationRequest{}, err
	}
	return request, nil
}

func WriteServiceRegistrationResponse(w http.ResponseWriter, statusCode int, response ServiceRegistrationResponse) error {
	if err := response.Validate(); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(response)
}
