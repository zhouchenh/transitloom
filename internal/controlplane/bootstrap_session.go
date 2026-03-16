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
)

const (
	// BootstrapProtocolVersion keeps the bootstrap-only session shape explicit so
	// later QUIC+mTLS and TCP+TLS work can replace the transport without
	// pretending this narrow JSON exchange is already the final control protocol.
	BootstrapProtocolVersion = "tl-bootstrap-v1alpha1"
	BootstrapSessionPath     = "/v1/bootstrap/control-session"
)

type BootstrapSessionOutcome string

const (
	BootstrapSessionOutcomeAccepted       BootstrapSessionOutcome = "accepted"
	BootstrapSessionOutcomeRejected       BootstrapSessionOutcome = "rejected"
	BootstrapSessionOutcomeInvalidRequest BootstrapSessionOutcome = "invalid-request"
)

type BootstrapSessionReason string

const (
	BootstrapSessionReasonPrerequisitesSatisfied      BootstrapSessionReason = "bootstrap-prerequisites-satisfied"
	BootstrapSessionReasonCoordinatorAwaitingMaterial BootstrapSessionReason = "coordinator-awaiting-intermediate"
	BootstrapSessionReasonNodeIdentityBootstrap       BootstrapSessionReason = "node-identity-bootstrap-required"
	BootstrapSessionReasonNodeAwaitingCertificate     BootstrapSessionReason = "node-awaiting-certificate"
	BootstrapSessionReasonNodeAdmissionMissing        BootstrapSessionReason = "node-admission-missing"
	BootstrapSessionReasonNodeAdmissionExpired        BootstrapSessionReason = "node-admission-expired"
	BootstrapSessionReasonUnsupportedProtocol         BootstrapSessionReason = "unsupported-protocol-version"
	BootstrapSessionReasonInvalidReadiness            BootstrapSessionReason = "invalid-readiness-input"
)

const (
	ReadinessPhaseIdentityBootstrapRequired = "identity-bootstrap-required"
	ReadinessPhaseAwaitingCertificate       = "awaiting-certificate"
	ReadinessPhaseAdmissionMissing          = "admission-token-missing"
	ReadinessPhaseAdmissionExpired          = "admission-token-expired"
	ReadinessPhaseReady                     = "ready"
)

// BootstrapSessionRequest is intentionally narrow: it only carries the node's
// local bootstrap-readiness snapshot, not live certificate proofs or token
// secrets. This keeps the first control path honest about what is and is not
// implemented yet.
type BootstrapSessionRequest struct {
	ProtocolVersion string                    `json:"protocol_version"`
	NodeName        string                    `json:"node_name"`
	Readiness       BootstrapReadinessSummary `json:"readiness"`
}

type BootstrapReadinessSummary struct {
	OverallPhase   string                 `json:"overall_phase"`
	IdentityPhase  string                 `json:"identity_phase"`
	AdmissionPhase string                 `json:"admission_phase"`
	CachedToken    *BootstrapTokenSummary `json:"cached_token,omitempty"`
}

type BootstrapTokenSummary struct {
	TokenID             string    `json:"token_id"`
	NodeID              string    `json:"node_id"`
	IssuerCoordinatorID string    `json:"issuer_coordinator_id"`
	IssuedAt            time.Time `json:"issued_at"`
	ExpiresAt           time.Time `json:"expires_at"`
}

// BootstrapSessionResponse always describes a bootstrap-only result. Even the
// accepted outcome means only that both sides reached the minimal bootstrap
// prerequisites for this task, not that a normal authenticated session exists.
type BootstrapSessionResponse struct {
	ProtocolVersion string                  `json:"protocol_version"`
	CoordinatorName string                  `json:"coordinator_name"`
	Outcome         BootstrapSessionOutcome `json:"outcome"`
	Reason          BootstrapSessionReason  `json:"reason"`
	BootstrapOnly   bool                    `json:"bootstrap_only"`
	Details         []string                `json:"details,omitempty"`
}

type Client struct {
	HTTPClient *http.Client
}

func (r BootstrapSessionRequest) Validate() error {
	if r.ProtocolVersion != BootstrapProtocolVersion {
		return fmt.Errorf("protocol_version must be %q", BootstrapProtocolVersion)
	}
	if strings.TrimSpace(r.NodeName) == "" {
		return fmt.Errorf("node_name must be set")
	}
	if err := r.Readiness.Validate(); err != nil {
		return fmt.Errorf("readiness: %w", err)
	}
	return nil
}

func (r BootstrapReadinessSummary) Validate() error {
	switch r.OverallPhase {
	case ReadinessPhaseIdentityBootstrapRequired, ReadinessPhaseAwaitingCertificate, ReadinessPhaseAdmissionMissing, ReadinessPhaseAdmissionExpired, ReadinessPhaseReady:
	default:
		return fmt.Errorf("overall_phase must be a recognized bootstrap phase")
	}
	if strings.TrimSpace(r.IdentityPhase) == "" {
		return fmt.Errorf("identity_phase must be set")
	}
	if strings.TrimSpace(r.AdmissionPhase) == "" {
		return fmt.Errorf("admission_phase must be set")
	}
	if r.OverallPhase == ReadinessPhaseReady && r.CachedToken == nil {
		return fmt.Errorf("cached_token must be present when overall_phase is %q", ReadinessPhaseReady)
	}
	if r.CachedToken != nil {
		if err := r.CachedToken.Validate(); err != nil {
			return fmt.Errorf("cached_token: %w", err)
		}
	}
	return nil
}

func (t BootstrapTokenSummary) Validate() error {
	if strings.TrimSpace(t.TokenID) == "" {
		return fmt.Errorf("token_id must be set")
	}
	if strings.TrimSpace(t.NodeID) == "" {
		return fmt.Errorf("node_id must be set")
	}
	if strings.TrimSpace(t.IssuerCoordinatorID) == "" {
		return fmt.Errorf("issuer_coordinator_id must be set")
	}
	if t.IssuedAt.IsZero() {
		return fmt.Errorf("issued_at must be set")
	}
	if t.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at must be set")
	}
	if !t.IssuedAt.Before(t.ExpiresAt) {
		return fmt.Errorf("issued_at must be before expires_at")
	}
	return nil
}

func (r BootstrapSessionResponse) Validate() error {
	if r.ProtocolVersion != BootstrapProtocolVersion {
		return fmt.Errorf("protocol_version must be %q", BootstrapProtocolVersion)
	}
	if strings.TrimSpace(r.CoordinatorName) == "" {
		return fmt.Errorf("coordinator_name must be set")
	}
	switch r.Outcome {
	case BootstrapSessionOutcomeAccepted, BootstrapSessionOutcomeRejected, BootstrapSessionOutcomeInvalidRequest:
	default:
		return fmt.Errorf("outcome must be a recognized bootstrap session outcome")
	}
	if strings.TrimSpace(string(r.Reason)) == "" {
		return fmt.Errorf("reason must be set")
	}
	if !r.BootstrapOnly {
		return fmt.Errorf("bootstrap_only must remain true for the bootstrap session path")
	}
	return nil
}

func (r BootstrapSessionResponse) Accepted() bool {
	return r.Outcome == BootstrapSessionOutcomeAccepted
}

func (c Client) Attempt(ctx context.Context, endpoint string, request BootstrapSessionRequest) (BootstrapSessionResponse, error) {
	if err := request.Validate(); err != nil {
		return BootstrapSessionResponse{}, fmt.Errorf("bootstrap session request invalid: %w", err)
	}

	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return BootstrapSessionResponse{}, fmt.Errorf("bootstrap session endpoint must be set")
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return BootstrapSessionResponse{}, fmt.Errorf("marshal bootstrap session request: %w", err)
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 3 * time.Second}
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+endpoint+BootstrapSessionPath, bytes.NewReader(payload))
	if err != nil {
		return BootstrapSessionResponse{}, fmt.Errorf("create bootstrap session request for %q: %w", endpoint, err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return BootstrapSessionResponse{}, fmt.Errorf("perform bootstrap session request to %q: %w", endpoint, err)
	}
	defer httpResponse.Body.Close()

	var response BootstrapSessionResponse
	if err := decodeSingleJSONObject(httpResponse.Body, &response); err != nil {
		return BootstrapSessionResponse{}, fmt.Errorf("decode bootstrap session response from %q: %w", endpoint, err)
	}
	if err := response.Validate(); err != nil {
		return BootstrapSessionResponse{}, fmt.Errorf("invalid bootstrap session response from %q: %w", endpoint, err)
	}

	return response, nil
}

func DecodeBootstrapSessionRequest(body io.Reader) (BootstrapSessionRequest, error) {
	var request BootstrapSessionRequest
	if err := decodeSingleJSONObject(body, &request); err != nil {
		return BootstrapSessionRequest{}, err
	}
	if err := request.Validate(); err != nil {
		return BootstrapSessionRequest{}, err
	}
	return request, nil
}

func WriteBootstrapSessionResponse(w http.ResponseWriter, statusCode int, response BootstrapSessionResponse) error {
	if err := response.Validate(); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(response)
}

func decodeSingleJSONObject(reader io.Reader, out any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(out); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("expected exactly one JSON object")
		}
		return err
	}

	return nil
}
