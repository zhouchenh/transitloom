package node

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
)

type BootstrapEndpointAttempt struct {
	Coordinator string
	Endpoint    string
	Error       string
}

// BootstrapSessionAttemptResult keeps transport failures separate from a
// structured coordinator response. A structured rejection is still useful
// bootstrap progress, while endpoint failures mean the node never reached a
// coordinator session endpoint at all.
type BootstrapSessionAttemptResult struct {
	CoordinatorLabel string
	Endpoint         string
	Response         controlplane.BootstrapSessionResponse
	FailedAttempts   []BootstrapEndpointAttempt
}

func BuildBootstrapSessionRequest(cfg config.NodeConfig, bootstrap BootstrapState) controlplane.BootstrapSessionRequest {
	request := controlplane.BootstrapSessionRequest{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		NodeName:        cfg.Identity.Name,
		Readiness: controlplane.BootstrapReadinessSummary{
			OverallPhase:   string(bootstrap.Phase),
			IdentityPhase:  string(bootstrap.Identity.Phase),
			AdmissionPhase: string(bootstrap.Admission.Phase),
		},
	}

	if bootstrap.Admission.Token != nil {
		request.Readiness.CachedToken = &controlplane.BootstrapTokenSummary{
			TokenID:             bootstrap.Admission.Token.TokenID,
			NodeID:              bootstrap.Admission.Token.NodeID,
			IssuerCoordinatorID: bootstrap.Admission.Token.IssuerCoordinatorID,
			IssuedAt:            bootstrap.Admission.Token.IssuedAt,
			ExpiresAt:           bootstrap.Admission.Token.ExpiresAt,
		}
	}

	return request
}

func AttemptBootstrapSession(ctx context.Context, cfg config.NodeConfig, bootstrap BootstrapState) (BootstrapSessionAttemptResult, error) {
	result := BootstrapSessionAttemptResult{}
	request := BuildBootstrapSessionRequest(cfg, bootstrap)
	client := controlplane.Client{
		HTTPClient: &http.Client{Timeout: 3 * time.Second},
	}

	var lastErr error
	for _, coordinator := range cfg.BootstrapCoordinators {
		label := describeBootstrapCoordinator(coordinator)
		for _, endpoint := range coordinator.ControlEndpoints {
			response, err := client.Attempt(ctx, endpoint, request)
			if err != nil {
				lastErr = err
				result.FailedAttempts = append(result.FailedAttempts, BootstrapEndpointAttempt{
					Coordinator: label,
					Endpoint:    endpoint,
					Error:       err.Error(),
				})
				continue
			}

			result.CoordinatorLabel = label
			result.Endpoint = endpoint
			result.Response = response
			return result, nil
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no bootstrap coordinator endpoints were configured")
	}

	return result, fmt.Errorf("bootstrap control session attempt failed after %d endpoint attempt(s): %w", len(result.FailedAttempts), lastErr)
}

func (r BootstrapSessionAttemptResult) ReportLines() []string {
	lines := make([]string, 0, len(r.FailedAttempts)+len(r.Response.Details)+4)

	for _, attempt := range r.FailedAttempts {
		lines = append(lines, fmt.Sprintf("node bootstrap control attempt failed: coordinator=%s endpoint=%s error=%s", attempt.Coordinator, attempt.Endpoint, attempt.Error))
	}

	if r.Endpoint != "" {
		lines = append(lines, fmt.Sprintf("node bootstrap control session target: coordinator=%s endpoint=%s", r.CoordinatorLabel, r.Endpoint))
	}
	if r.Response.Outcome != "" {
		lines = append(lines, fmt.Sprintf("node bootstrap control session outcome: %s (%s)", r.Response.Outcome, r.Response.Reason))
		for _, detail := range r.Response.Details {
			lines = append(lines, "node bootstrap control detail: "+detail)
		}
	}

	return lines
}

func describeBootstrapCoordinator(coordinator config.BootstrapCoordinatorConfig) string {
	if label := strings.TrimSpace(coordinator.Label); label != "" {
		return label
	}
	if hint := strings.TrimSpace(coordinator.CoordinatorIDHint); hint != "" {
		return hint
	}
	if len(coordinator.ControlEndpoints) > 0 {
		return coordinator.ControlEndpoints[0]
	}
	return "bootstrap-coordinator"
}
