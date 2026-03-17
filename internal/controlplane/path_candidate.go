package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const PathCandidatePath = "/v1/bootstrap/path-candidates"

// DistributedPathCandidateClass identifies the path class of a distributed candidate.
// Values match the v1 path classes from spec/v1-data-plane.md section 7 and
// align with scheduler.PathClass, but this is the wire/distribution type, kept
// separate from the scheduler's runtime input type.
type DistributedPathCandidateClass = string

const (
	DistributedPathClassDirectPublic     DistributedPathCandidateClass = "direct-public"
	DistributedPathClassDirectIntranet   DistributedPathCandidateClass = "direct-intranet"
	DistributedPathClassCoordinatorRelay DistributedPathCandidateClass = "coordinator-relay"
	DistributedPathClassNodeRelay        DistributedPathCandidateClass = "node-relay"
)

// DistributedPathCandidate is a coordinator-distributed path candidate for an
// association. It represents what the coordinator knows about a possible path.
//
// This is the wire/distribution format. It is explicitly distinct from:
//   - scheduler.PathCandidate: the local scheduler runtime input (may include
//     quality measurements, health state, used directly by Scheduler.Decide())
//   - dataplane.ForwardingEntry / RelayForwardingEntry: installed forwarding
//     state (only exists when carriage is actually active)
//   - scheduler.SchedulerDecision: the endpoint's runtime path selection output
//
// A DistributedPathCandidate does NOT represent:
//   - a chosen or active runtime path
//   - installed forwarding state
//   - a confirmed reachable or measured path
//   - the result of endpoint scheduling
//
// Candidate presence means only: the coordinator asserts this path may be
// available for the association. Nodes must not treat receiving a candidate
// as proof that traffic will succeed on that path.
//
// This separation preserves the object-model boundary defined in
// spec/v1-object-model.md section 16 and the architectural rule in
// agents/MEMORY.md: "PathCandidate is the scheduler's input type...explicitly
// distinct from ForwardingEntry, RelayForwardingEntry, and SchedulerDecision."
type DistributedPathCandidate struct {
	// CandidateID uniquely identifies this candidate within its association.
	CandidateID string `json:"candidate_id"`

	// AssociationID is the association this candidate belongs to.
	// Candidates without a matching association must not be used for scheduling.
	AssociationID string `json:"association_id"`

	// Class is the path class (see DistributedPathClass* constants).
	Class DistributedPathCandidateClass `json:"class"`

	// IsRelayAssisted explicitly marks whether this candidate uses a relay hop.
	// This flag is in addition to Class to prevent relay/direct ambiguity:
	// the relay vs direct distinction must remain architecturally explicit
	// (AGENTS.md package-boundary expectations, spec/v1-object-model.md section 17).
	// A relay-class candidate must have IsRelayAssisted=true; a direct-class
	// candidate must have IsRelayAssisted=false.
	IsRelayAssisted bool `json:"is_relay_assisted"`

	// RemoteEndpoint is the network address to use for this path.
	// For direct candidates: the remote peer's reachable mesh address.
	// For relay candidates: the relay's per-association listen address.
	// Empty means no usable endpoint is known for this candidate; such
	// candidates are informational only and must not be used for scheduling.
	RemoteEndpoint string `json:"remote_endpoint,omitempty"`

	// RelayNodeID identifies the relay participant for relay-assisted candidates.
	// Empty for direct candidates. Required when IsRelayAssisted is true.
	RelayNodeID string `json:"relay_node_id,omitempty"`

	// AdminWeight is the coordinator-assigned administrative preference weight
	// in [1, 100]. Zero is treated as 100. Higher weight means more preference.
	AdminWeight uint8 `json:"admin_weight,omitempty"`

	// IsMetered marks this path as metered (cost-sensitive or limited bandwidth).
	IsMetered bool `json:"is_metered,omitempty"`

	// Note is a coordinator-provided human-readable comment explaining why
	// this candidate was generated and any caveats about its usability.
	// Candidates with a non-empty Note but empty RemoteEndpoint are
	// informational only and must not be used for scheduling.
	Note string `json:"note,omitempty"`
}

// IsUsable returns true when this candidate has a non-empty RemoteEndpoint
// and therefore represents a potentially actionable path for scheduling.
// Candidates without a RemoteEndpoint are informational placeholders only.
func (c DistributedPathCandidate) IsUsable() bool {
	return strings.TrimSpace(c.RemoteEndpoint) != ""
}

// Validate checks that the candidate's fields are internally consistent.
func (c DistributedPathCandidate) Validate() error {
	if strings.TrimSpace(c.CandidateID) == "" {
		return fmt.Errorf("candidate_id must not be empty")
	}
	if strings.TrimSpace(c.AssociationID) == "" {
		return fmt.Errorf("association_id must not be empty")
	}
	switch c.Class {
	case DistributedPathClassDirectPublic, DistributedPathClassDirectIntranet,
		DistributedPathClassCoordinatorRelay, DistributedPathClassNodeRelay:
	default:
		return fmt.Errorf("class %q is not a recognized path candidate class", c.Class)
	}
	// Relay class must have IsRelayAssisted=true; direct class must have IsRelayAssisted=false.
	// Both conditions together prevent any future code from treating a relay
	// candidate as a direct path or vice versa.
	isRelayClass := c.Class == DistributedPathClassCoordinatorRelay || c.Class == DistributedPathClassNodeRelay
	if isRelayClass != c.IsRelayAssisted {
		return fmt.Errorf("class %q and is_relay_assisted=%v are inconsistent: relay classes require is_relay_assisted=true, direct classes require is_relay_assisted=false", c.Class, c.IsRelayAssisted)
	}
	// Relay-assisted candidates require a relay node ID to identify the
	// relay participant (spec/v1-object-model.md section 17: a RelayCandidate
	// is identified by its coordinator/node ID).
	if c.IsRelayAssisted && strings.TrimSpace(c.RelayNodeID) == "" {
		return fmt.Errorf("relay-assisted candidate must have relay_node_id set")
	}
	return nil
}

// PathCandidateSet is the coordinator's candidate data for one association.
// It groups all candidates known for the association alongside context notes.
//
// An empty Candidates slice means the coordinator has no path candidates
// for this association at this time. This is distinct from the association
// not existing: the association may be recorded but have no known paths yet.
type PathCandidateSet struct {
	// AssociationID is the association these candidates belong to.
	AssociationID string `json:"association_id"`

	// SourceNode is the source node of the association.
	SourceNode string `json:"source_node,omitempty"`

	// DestinationNode is the destination node of the association.
	DestinationNode string `json:"destination_node,omitempty"`

	// Candidates is the set of path candidates known for this association.
	// May be empty if the coordinator has no candidate data yet.
	Candidates []DistributedPathCandidate `json:"candidates,omitempty"`

	// Notes provides coordinator-level commentary on why certain candidate
	// types are absent or what constraints apply to the current candidate set.
	Notes []string `json:"notes,omitempty"`
}

// Validate checks that the set's fields are internally consistent.
func (s PathCandidateSet) Validate() error {
	if strings.TrimSpace(s.AssociationID) == "" {
		return fmt.Errorf("association_id must not be empty")
	}
	for i, c := range s.Candidates {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("candidates[%d]: %w", i, err)
		}
		if c.AssociationID != s.AssociationID {
			return fmt.Errorf("candidates[%d]: candidate association_id %q does not match set association_id %q",
				i, c.AssociationID, s.AssociationID)
		}
	}
	return nil
}

// PathCandidateRequest is sent by a node to request path candidates for its
// accepted associations. Association IDs are taken from the association response.
type PathCandidateRequest struct {
	ProtocolVersion string                    `json:"protocol_version"`
	NodeName        string                    `json:"node_name"`
	Readiness       BootstrapReadinessSummary `json:"readiness"`

	// AssociationIDs lists the association IDs for which candidates are requested.
	AssociationIDs []string `json:"association_ids"`
}

// Validate checks that the request has the required fields.
func (r PathCandidateRequest) Validate() error {
	if r.ProtocolVersion != BootstrapProtocolVersion {
		return fmt.Errorf("protocol_version must be %q", BootstrapProtocolVersion)
	}
	if strings.TrimSpace(r.NodeName) == "" {
		return fmt.Errorf("node_name must be set")
	}
	if err := r.Readiness.Validate(); err != nil {
		return fmt.Errorf("readiness: %w", err)
	}
	if len(r.AssociationIDs) == 0 {
		return fmt.Errorf("association_ids must contain at least one association ID")
	}
	for i, id := range r.AssociationIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("association_ids[%d]: must not be empty", i)
		}
	}
	return nil
}

// PathCandidateResponse carries the coordinator's path-candidate data for the
// requested associations.
//
// BootstrapOnly is always true: these candidates are coordinator knowledge at
// bootstrap-session level. They are NOT proven runtime paths and do NOT
// represent chosen-path or forwarding-state. Nodes must store these candidates
// separately from their runtime scheduling and forwarding state.
type PathCandidateResponse struct {
	ProtocolVersion string `json:"protocol_version"`
	CoordinatorName string `json:"coordinator_name"`

	// BootstrapOnly must always be true for this endpoint. It explicitly labels
	// these candidates as coordinator-distributed bootstrap knowledge, not as
	// chosen runtime paths or installed forwarding state.
	BootstrapOnly bool `json:"bootstrap_only"`

	// CandidateSets is one set per found association. Requested association IDs
	// not found in the coordinator's store are omitted from this slice.
	CandidateSets []PathCandidateSet `json:"candidate_sets,omitempty"`

	Details []string `json:"details,omitempty"`
}

// Validate checks that the response has the required fields.
func (r PathCandidateResponse) Validate() error {
	if r.ProtocolVersion != BootstrapProtocolVersion {
		return fmt.Errorf("protocol_version must be %q", BootstrapProtocolVersion)
	}
	if strings.TrimSpace(r.CoordinatorName) == "" {
		return fmt.Errorf("coordinator_name must be set")
	}
	if !r.BootstrapOnly {
		return fmt.Errorf("bootstrap_only must remain true for the bootstrap path-candidates endpoint")
	}
	for i, s := range r.CandidateSets {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("candidate_sets[%d]: %w", i, err)
		}
	}
	return nil
}

// HTTP helpers

// DecodePathCandidateRequest decodes and validates a path-candidate request
// from an HTTP request body.
func DecodePathCandidateRequest(body io.Reader) (PathCandidateRequest, error) {
	var request PathCandidateRequest
	if err := decodeSingleJSONObject(body, &request); err != nil {
		return PathCandidateRequest{}, err
	}
	if err := request.Validate(); err != nil {
		return PathCandidateRequest{}, err
	}
	return request, nil
}

// WritePathCandidateResponse encodes and writes a path-candidate response.
func WritePathCandidateResponse(w http.ResponseWriter, statusCode int, response PathCandidateResponse) error {
	if err := response.Validate(); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(response)
}

// RequestPathCandidates sends a path-candidate request to the coordinator
// endpoint and returns the validated response.
func (c Client) RequestPathCandidates(ctx context.Context, endpoint string, request PathCandidateRequest) (PathCandidateResponse, error) {
	if err := request.Validate(); err != nil {
		return PathCandidateResponse{}, fmt.Errorf("path candidate request invalid: %w", err)
	}

	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return PathCandidateResponse{}, fmt.Errorf("path candidate endpoint must be set")
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return PathCandidateResponse{}, fmt.Errorf("marshal path candidate request: %w", err)
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: BootstrapConnectTimeout}
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+endpoint+PathCandidatePath, bytes.NewReader(payload))
	if err != nil {
		return PathCandidateResponse{}, fmt.Errorf("create path candidate request for %q: %w", endpoint, err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return PathCandidateResponse{}, fmt.Errorf("perform path candidate request to %q: %w", endpoint, err)
	}
	defer httpResponse.Body.Close()

	var response PathCandidateResponse
	if err := decodeSingleJSONObject(httpResponse.Body, &response); err != nil {
		return PathCandidateResponse{}, fmt.Errorf("decode path candidate response from %q: %w", endpoint, err)
	}
	if err := response.Validate(); err != nil {
		return PathCandidateResponse{}, fmt.Errorf("invalid path candidate response from %q: %w", endpoint, err)
	}

	return response, nil
}
