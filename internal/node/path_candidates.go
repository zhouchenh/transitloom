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

// PathCandidateAttemptResult holds the result of a path-candidate fetch.
type PathCandidateAttemptResult struct {
	CoordinatorLabel string
	Endpoint         string
	Response         controlplane.PathCandidateResponse
}

// BuildPathCandidateRequest constructs a path-candidate request from the
// node's accepted association IDs.
func BuildPathCandidateRequest(
	cfg config.NodeConfig,
	bootstrap BootstrapState,
	associationIDs []string,
) (controlplane.PathCandidateRequest, error) {
	ids := make([]string, 0, len(associationIDs))
	for _, id := range associationIDs {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return controlplane.PathCandidateRequest{}, fmt.Errorf("no association IDs provided for path candidate request")
	}
	return controlplane.PathCandidateRequest{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		NodeName:        cfg.Identity.Name,
		Readiness:       BuildBootstrapSessionRequest(cfg, bootstrap).Readiness,
		AssociationIDs:  ids,
	}, nil
}

// FetchPathCandidates sends a path-candidate request to the coordinator
// endpoint and returns the coordinator's response.
//
// The caller must use StoreCandidates to save the received candidates into a
// CandidateStore. Candidate storage is intentionally a separate step from
// fetching to keep the fetch/store boundary explicit and testable.
func FetchPathCandidates(
	ctx context.Context,
	cfg config.NodeConfig,
	bootstrap BootstrapState,
	session BootstrapSessionAttemptResult,
	associationIDs []string,
) (PathCandidateAttemptResult, error) {
	if !session.Response.Accepted() {
		return PathCandidateAttemptResult{}, fmt.Errorf("cannot fetch path candidates: bootstrap control session was not accepted")
	}
	if strings.TrimSpace(session.Endpoint) == "" {
		return PathCandidateAttemptResult{}, fmt.Errorf("cannot fetch path candidates: no coordinator endpoint available")
	}

	request, err := BuildPathCandidateRequest(cfg, bootstrap, associationIDs)
	if err != nil {
		return PathCandidateAttemptResult{}, fmt.Errorf("build path candidate request: %w", err)
	}

	client := controlplane.Client{
		HTTPClient: &http.Client{Timeout: 3 * time.Second},
	}

	response, err := client.RequestPathCandidates(ctx, session.Endpoint, request)
	if err != nil {
		return PathCandidateAttemptResult{}, fmt.Errorf("path candidate fetch from %q: %w", session.Endpoint, err)
	}

	return PathCandidateAttemptResult{
		CoordinatorLabel: session.CoordinatorLabel,
		Endpoint:         session.Endpoint,
		Response:         response,
	}, nil
}

// StoreCandidates saves received candidate sets into the node's CandidateStore.
//
// This function updates only the CandidateStore. It does NOT:
//   - modify scheduler state or SchedulerDecision
//   - install or modify forwarding entries (ForwardingEntry / RelayForwardingEntry)
//   - activate or deactivate any carrier
//
// Candidate storage is explicitly separate from carrier activation: candidates
// represent coordinator knowledge, not chosen runtime paths. The scheduler
// consumes candidates later (from the CandidateStore or from config) as inputs
// to Scheduler.Decide(), which is the actual path selection step.
//
// Returns the number of association candidate sets stored.
func StoreCandidates(store *CandidateStore, response controlplane.PathCandidateResponse) int {
	stored := 0
	for _, set := range response.CandidateSets {
		store.Store(set.AssociationID, set.Candidates)
		stored++
	}
	return stored
}

// ReportLines produces human-readable log lines for a path-candidate fetch result.
func (r PathCandidateAttemptResult) ReportLines() []string {
	lines := make([]string, 0, len(r.Response.CandidateSets)+4)

	if r.Endpoint != "" {
		lines = append(lines, fmt.Sprintf(
			"node path-candidate fetch: coordinator=%s endpoint=%s",
			r.CoordinatorLabel, r.Endpoint,
		))
	}

	if r.Response.CoordinatorName != "" {
		totalCandidates := 0
		usable := 0
		for _, set := range r.Response.CandidateSets {
			totalCandidates += len(set.Candidates)
			for _, c := range set.Candidates {
				if c.IsUsable() {
					usable++
				}
			}
		}
		lines = append(lines, fmt.Sprintf(
			"node path-candidate result: coordinator=%s sets=%d candidates=%d usable=%d "+
				"(bootstrap-only; not chosen-path or forwarding state)",
			r.Response.CoordinatorName,
			len(r.Response.CandidateSets),
			totalCandidates,
			usable,
		))
		for _, set := range r.Response.CandidateSets {
			setUsable := 0
			for _, c := range set.Candidates {
				if c.IsUsable() {
					setUsable++
				}
			}
			lines = append(lines, fmt.Sprintf(
				"  association %s -> %s/%s: candidates=%d usable=%d",
				set.AssociationID, set.SourceNode, set.DestinationNode,
				len(set.Candidates), setUsable,
			))
			for _, note := range set.Notes {
				lines = append(lines, "    note: "+note)
			}
		}
	}

	for _, detail := range r.Response.Details {
		lines = append(lines, "node path-candidate detail: "+detail)
	}

	return lines
}
