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

// BootstrapEndpointAttempt records a single failed transport attempt toward
// one coordinator endpoint. ErrorKind is the normalized category from the
// controlplane error classifier so callers and logs can understand why the
// attempt failed without parsing raw error strings.
type BootstrapEndpointAttempt struct {
	Coordinator string
	Endpoint    string
	ErrorKind   controlplane.TransportErrorKind
	Error       string
}

// BootstrapSessionAttemptResult keeps transport failures separate from a
// structured coordinator response. A structured rejection is still useful
// bootstrap progress, while endpoint failures mean the node never reached a
// coordinator session endpoint at all.
//
// FailedAttempts may contain multiple records for the same endpoint if bounded
// retries were attempted for transient (timeout) failures.
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

// AttemptBootstrapSession tries each configured coordinator endpoint in order
// until one returns a structured response (accepted or rejected). It records
// all transport-level failures with their error kinds so the caller can
// understand exactly why the bootstrap session did not reach a coordinator.
//
// Transient transport failures (timeouts) are retried up to
// controlplane.BootstrapRetryMaxAttempts times per endpoint with exponential
// backoff capped at controlplane.BootstrapRetryMaxBackoff. Non-retryable
// failures (connection refused, context canceled) skip immediately to the
// next endpoint without burning retry budget.
//
// The retry/backoff behavior is deliberately bounded and explicit: a node
// should not spin indefinitely on a single endpoint when other coordinators
// are available. The bounded budget also makes startup latency predictable.
func AttemptBootstrapSession(ctx context.Context, cfg config.NodeConfig, bootstrap BootstrapState) (BootstrapSessionAttemptResult, error) {
	result := BootstrapSessionAttemptResult{}
	request := BuildBootstrapSessionRequest(cfg, bootstrap)
	client := controlplane.Client{
		HTTPClient: &http.Client{Timeout: controlplane.BootstrapConnectTimeout},
	}

	var lastErr error
	for _, coordinator := range cfg.BootstrapCoordinators {
		label := describeBootstrapCoordinator(coordinator)
		for _, endpoint := range coordinator.ControlEndpoints {
			response, err := attemptEndpointWithRetry(ctx, client, endpoint, request, &result, label)
			if err == nil {
				result.CoordinatorLabel = label
				result.Endpoint = endpoint
				result.Response = response
				return result, nil
			}
			lastErr = err
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no bootstrap coordinator endpoints were configured")
	}

	return result, fmt.Errorf("bootstrap control session attempt failed after %d endpoint attempt(s): %w", len(result.FailedAttempts), lastErr)
}

// attemptEndpointWithRetry tries a single coordinator endpoint up to
// BootstrapRetryMaxAttempts times for retryable (timeout) transport failures.
// Non-retryable failures return immediately. Context cancellation also returns
// immediately regardless of the retry budget.
func attemptEndpointWithRetry(
	ctx context.Context,
	client controlplane.Client,
	endpoint string,
	request controlplane.BootstrapSessionRequest,
	result *BootstrapSessionAttemptResult,
	coordinatorLabel string,
) (controlplane.BootstrapSessionResponse, error) {
	backoff := controlplane.BootstrapRetryInitialBackoff

	for attempt := 1; attempt <= controlplane.BootstrapRetryMaxAttempts; attempt++ {
		// Abort immediately if the caller's context is already done. Checking
		// before the attempt avoids a useless connection attempt and makes
		// cancellation latency predictable.
		select {
		case <-ctx.Done():
			te := controlplane.ClassifyTransportError(ctx.Err(), endpoint)
			result.FailedAttempts = append(result.FailedAttempts, BootstrapEndpointAttempt{
				Coordinator: coordinatorLabel,
				Endpoint:    endpoint,
				ErrorKind:   te.Kind,
				Error:       te.Error(),
			})
			return controlplane.BootstrapSessionResponse{}, ctx.Err()
		default:
		}

		response, err := client.Attempt(ctx, endpoint, request)
		if err == nil {
			return response, nil
		}

		te := controlplane.ClassifyTransportError(err, endpoint)
		result.FailedAttempts = append(result.FailedAttempts, BootstrapEndpointAttempt{
			Coordinator: coordinatorLabel,
			Endpoint:    endpoint,
			ErrorKind:   te.Kind,
			Error:       te.Error(),
		})

		// Context canceled means the operation was deliberately abandoned.
		// Do not retry; propagate immediately.
		if te.Kind == controlplane.TransportErrorKindContextCanceled {
			return controlplane.BootstrapSessionResponse{}, err
		}

		// Non-retryable failures (connection refused, unknown) do not benefit
		// from retry — skip to the next endpoint immediately.
		if !te.Retryable() {
			return controlplane.BootstrapSessionResponse{}, err
		}

		// Retryable failure (timeout): wait with bounded exponential backoff
		// before the next attempt. This absorbs brief transient failures
		// without hammering an overloaded coordinator endpoint.
		if attempt < controlplane.BootstrapRetryMaxAttempts {
			select {
			case <-ctx.Done():
				// Context was canceled during the backoff wait. Do not start
				// another attempt.
				return controlplane.BootstrapSessionResponse{}, ctx.Err()
			case <-time.After(backoff):
			}
			backoff = minDuration(backoff*2, controlplane.BootstrapRetryMaxBackoff)
		}
	}

	// Exhausted retry budget for this endpoint.
	return controlplane.BootstrapSessionResponse{}, fmt.Errorf("endpoint %q: exhausted %d attempt(s)", endpoint, controlplane.BootstrapRetryMaxAttempts)
}

func (r BootstrapSessionAttemptResult) ReportLines() []string {
	lines := make([]string, 0, len(r.FailedAttempts)+len(r.Response.Details)+4)

	for _, attempt := range r.FailedAttempts {
		lines = append(lines, fmt.Sprintf("node bootstrap control attempt failed: coordinator=%s endpoint=%s kind=%s error=%s", attempt.Coordinator, attempt.Endpoint, attempt.ErrorKind, attempt.Error))
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

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
