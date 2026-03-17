package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/scheduler"
	"github.com/zhouchenh/transitloom/internal/transport"
)

// DefaultCandidateMaxAge is the default freshness window for distributed path
// candidates received from the coordinator.
//
// After this duration without a refresh, stored candidates are treated as stale
// and the association is added to the next refresh target list. This age limit
// is intentionally longer than DefaultQualityMaxAge (60s) because candidate
// sets are coordinator-distributed objects that change less frequently than
// per-path quality measurements.
//
// Distinct from the other freshness windows:
//   - Endpoint-level freshness (EndpointRegistry / VerificationState): tracks
//     whether a remote IP:port address is reachable.
//   - Quality measurement freshness (PathQualityStore / DefaultQualityMaxAge):
//     tracks whether RTT/loss/jitter measurements are recent enough for scoring.
//   - Candidate set freshness (this constant): tracks whether the coordinator's
//     view of available paths has been recently re-fetched.
const DefaultCandidateMaxAge = 5 * time.Minute

// CandidateRefreshTrigger identifies why a candidate set was marked stale or
// why a coordinator refresh was requested.
//
// Recording the trigger when candidates are marked stale lets operators and
// tests verify that the right associations were selected for refresh and that
// the refresh automation is responding to the correct signals — not running
// blindly or for opaque internal reasons.
type CandidateRefreshTrigger string

const (
	// CandidateRefreshTriggerEndpointStale means the candidates' supporting
	// endpoint(s) are stale in the EndpointRegistry. The remote address may
	// have changed (public IP rotation, DNAT rule change, etc.). A coordinator
	// refresh may return updated endpoint information for these candidates.
	CandidateRefreshTriggerEndpointStale CandidateRefreshTrigger = "endpoint-stale"

	// CandidateRefreshTriggerEndpointFailed means a targeted probe confirmed
	// that the candidates' supporting endpoint(s) are unreachable. The
	// refinement layer already excludes these candidates from scheduler inputs.
	// A coordinator refresh is needed to obtain alternative or updated candidates.
	CandidateRefreshTriggerEndpointFailed CandidateRefreshTrigger = "endpoint-failed"

	// CandidateRefreshTriggerQualityStale means at least one candidate for
	// this association had quality measurements that have since expired
	// (older than PathQualityStore's MaxAge). The coordinator may know about
	// different or better-performing candidates than those currently stored.
	//
	// Note: quality staleness does NOT directly imply that candidate endpoints
	// are unreachable — it means measured path performance data is outdated.
	// This trigger is advisory: the coordinator refresh may yield alternative
	// candidates; whether to re-probe is a separate probing decision.
	CandidateRefreshTriggerQualityStale CandidateRefreshTrigger = "quality-stale"

	// CandidateRefreshTriggerCandidateExpired means the stored candidate set
	// has not been refreshed from the coordinator for longer than
	// DefaultCandidateMaxAge. The coordinator may have updated its view of
	// available relay endpoints or discovered new direct-path candidates.
	CandidateRefreshTriggerCandidateExpired CandidateRefreshTrigger = "candidate-expired"

	// CandidateRefreshTriggerPathUnhealthy means a path health signal
	// (unhealthy/down) was reported for this association. The existing stored
	// candidates may not accurately reflect current reachability conditions.
	// This trigger is set explicitly by the path health monitoring layer.
	CandidateRefreshTriggerPathUnhealthy CandidateRefreshTrigger = "path-unhealthy"

	// CandidateRefreshTriggerExplicit means an explicit refresh was requested
	// by an operator command, test harness, or policy-driven trigger.
	CandidateRefreshTriggerExplicit CandidateRefreshTrigger = "explicit"
)

// CandidateFreshnessState describes whether stored candidates for an association
// are considered fresh, stale, or in an unknown initial state.
//
// This is explicitly distinct from the other freshness concepts in the system:
//   - transport.VerificationState: endpoint-level address reachability.
//   - scheduler.PathQualityStore staleness: RTT/jitter/loss measurement freshness.
//   - CandidateFreshnessState (this type): coordinator-distribution freshness
//     — how recently were the candidates re-fetched from the coordinator?
//
// The three freshness layers must not be collapsed. Each reflects a different
// signal with a different remediation path:
//   - Stale endpoint → trigger endpoint re-probing.
//   - Stale quality → trigger path re-probing or coordinator refresh.
//   - Stale candidates → trigger coordinator candidate re-fetch.
type CandidateFreshnessState string

const (
	// CandidateFreshnessStateFresh means candidates for this association were
	// recently received from the coordinator and have not been explicitly marked
	// stale or expired by age. The candidate set may still be used by the
	// scheduler without a coordinator refresh.
	CandidateFreshnessStateFresh CandidateFreshnessState = "fresh"

	// CandidateFreshnessStateStale means candidates for this association should
	// be refreshed from the coordinator. Stale state is set when:
	//   - a refresh trigger fires (endpoint staleness, quality expiry, etc.)
	//   - the candidate set has not been refreshed for longer than DefaultCandidateMaxAge
	//
	// Stale candidates are still available in the CandidateStore and can be used
	// as a fallback by the refinement layer — staleness is an advisory state, not
	// a hard exclusion. The scheduler prefers fresher candidates when available.
	CandidateFreshnessStateStale CandidateFreshnessState = "stale"

	// CandidateFreshnessStateUnknown means the freshness of candidates for this
	// association is not yet tracked. This is the initial state for associations
	// registered via TrackAssociation before any coordinator fetch has occurred.
	CandidateFreshnessStateUnknown CandidateFreshnessState = "unknown"
)

// CandidateFreshnessRecord is the per-association candidate freshness record.
//
// All fields are exported so tests and operator tooling can inspect the full
// state without opaque internals. The StaleReason and StaleDetail fields ensure
// that operator observability explains *why* an association was marked stale,
// not just that it is.
type CandidateFreshnessRecord struct {
	AssociationID   string
	State           CandidateFreshnessState
	LastRefreshedAt time.Time               // zero when never refreshed
	MarkedStaleAt   time.Time               // zero when not currently stale
	StaleReason     CandidateRefreshTrigger // the trigger that caused staleness
	StaleDetail     string                  // human-readable detail for the trigger
}

// CandidateRefreshTarget identifies one association that needs a coordinator
// candidate refresh, with an explicit reason why.
//
// The Trigger and Detail fields make the automation inspectable: operators and
// tests can verify that the correct associations are selected and for the right
// reasons without reading through opaque selection logic.
type CandidateRefreshTarget struct {
	AssociationID string
	Trigger       CandidateRefreshTrigger
	Detail        string
}

// CandidateRefreshDetail describes the outcome of one association's refresh attempt.
type CandidateRefreshDetail struct {
	AssociationID  string
	Trigger        CandidateRefreshTrigger
	Refreshed      bool
	CandidateCount int // number of candidate sets stored after a successful refresh
	Error          string
}

// CandidateRefreshResult summarizes the outcome of a bounded candidate refresh run.
//
// All fields are explicit so the operator can understand what happened without
// reading internal log state. The Details slice provides per-association outcomes.
type CandidateRefreshResult struct {
	Attempted        int
	Refreshed        int
	Failed           int
	SkippedNoSession int
	Details          []CandidateRefreshDetail
}

// ReportLines produces human-readable log lines for a candidate refresh result.
// This is the primary operator-facing observability surface for refresh outcomes.
func (r CandidateRefreshResult) ReportLines() []string {
	lines := make([]string, 0, len(r.Details)+2)
	lines = append(lines, fmt.Sprintf(
		"candidate-refresh: attempted=%d refreshed=%d failed=%d skipped-no-session=%d",
		r.Attempted, r.Refreshed, r.Failed, r.SkippedNoSession,
	))
	for _, d := range r.Details {
		switch {
		case d.Error != "":
			lines = append(lines, fmt.Sprintf(
				"  association %s [trigger=%s]: FAILED: %s",
				d.AssociationID, d.Trigger, d.Error,
			))
		case !d.Refreshed:
			lines = append(lines, fmt.Sprintf(
				"  association %s [trigger=%s]: skipped (no session)",
				d.AssociationID, d.Trigger,
			))
		default:
			lines = append(lines, fmt.Sprintf(
				"  association %s [trigger=%s]: refreshed sets=%d",
				d.AssociationID, d.Trigger, d.CandidateCount,
			))
		}
	}
	return lines
}

// CandidateFreshnessStore tracks per-association candidate freshness state.
//
// This is the per-association tracking layer for coordinator-distributed
// candidates. It answers:
//   - Has this association's candidate set been recently refreshed from the coordinator?
//   - Was it explicitly marked stale by a trigger event?
//   - Which associations are overdue for a coordinator re-fetch?
//
// What this store does NOT track:
//   - Endpoint-level reachability (EndpointRegistry).
//   - Measured path quality freshness (PathQualityStore).
//   - Which path was chosen by the scheduler (ScheduledEgressActivation).
//
// The three freshness layers remain explicitly separate throughout. Collapsing
// them into one "stale" concept would make it impossible to tell whether an
// association's data plane degradation was caused by endpoint unreachability,
// quality degradation, or simply outdated coordinator knowledge.
//
// Thread-safe.
type CandidateFreshnessStore struct {
	mu      sync.RWMutex
	records map[string]*CandidateFreshnessRecord
	maxAge  time.Duration
}

// NewCandidateFreshnessStore creates a new CandidateFreshnessStore with the
// given maximum candidate age. Use DefaultCandidateMaxAge for the v1 default.
func NewCandidateFreshnessStore(maxAge time.Duration) *CandidateFreshnessStore {
	return &CandidateFreshnessStore{
		records: make(map[string]*CandidateFreshnessRecord),
		maxAge:  maxAge,
	}
}

// TrackAssociation ensures an association has a freshness record in Unknown state.
// Calling this for an association that already has a record is a no-op.
//
// Use this when a new association is created so that freshness tracking is
// initialized before the first coordinator fetch. Without this, SelectForRefresh
// cannot know about associations that have never had candidates fetched.
func (s *CandidateFreshnessStore) TrackAssociation(assocID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[assocID]; !exists {
		s.records[assocID] = &CandidateFreshnessRecord{
			AssociationID: assocID,
			State:         CandidateFreshnessStateUnknown,
		}
	}
}

// MarkRefreshed records that an association's candidates were successfully
// refreshed from the coordinator at the given time. Clears any prior stale state.
//
// Call this after successfully storing new candidates for an association.
// Without this call, the association would remain stale and trigger another
// refresh on the next automation run even though candidates are now current.
func (s *CandidateFreshnessStore) MarkRefreshed(assocID string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, exists := s.records[assocID]
	if !exists {
		r = &CandidateFreshnessRecord{AssociationID: assocID}
		s.records[assocID] = r
	}
	r.State = CandidateFreshnessStateFresh
	r.LastRefreshedAt = at
	r.MarkedStaleAt = time.Time{}
	r.StaleReason = ""
	r.StaleDetail = ""
}

// MarkStale explicitly marks an association's candidates as stale due to a
// known trigger event.
//
// If the association is already stale, the existing trigger and detail are
// preserved. This keeps the root-cause trigger visible: the first trigger to
// fire is the most diagnostically useful, and overwriting it would lose
// information about why the staleness started.
//
// After MarkStale, SelectForRefresh will include this association in its output
// until MarkRefreshed is called.
func (s *CandidateFreshnessStore) MarkStale(assocID string, trigger CandidateRefreshTrigger, detail string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, exists := s.records[assocID]
	if !exists {
		r = &CandidateFreshnessRecord{AssociationID: assocID}
		s.records[assocID] = r
	}
	// Preserve the original root-cause trigger if already stale.
	if r.State != CandidateFreshnessStateStale {
		r.State = CandidateFreshnessStateStale
		r.MarkedStaleAt = at
		r.StaleReason = trigger
		r.StaleDetail = detail
	}
}

// FreshnessState returns the current freshness state for an association,
// including age-based expiry check.
//
// A fresh association becomes stale if it has not been refreshed for longer
// than maxAge, even without an explicit MarkStale call. This age-based check
// is applied lazily here rather than on a timer, keeping the store simple and
// avoiding background goroutines.
//
// Returns CandidateFreshnessStateUnknown if the association is not tracked.
func (s *CandidateFreshnessStore) FreshnessState(assocID string) CandidateFreshnessState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, exists := s.records[assocID]
	if !exists {
		return CandidateFreshnessStateUnknown
	}
	if r.State == CandidateFreshnessStateFresh && !r.LastRefreshedAt.IsZero() {
		if time.Since(r.LastRefreshedAt) > s.maxAge {
			// Age-based expiry: treat as stale even without an explicit trigger.
			return CandidateFreshnessStateStale
		}
	}
	return r.State
}

// SelectForRefresh returns associations that need candidate refresh:
//   - explicitly marked stale via MarkStale
//   - fresh but older than maxAge since last refresh (age-based expiry)
//
// Unknown-state associations (never refreshed) are NOT included here; they
// appear only in the SelectCandidateRefreshTargets output as special initial-fetch
// candidates when they exist in the CandidateStore.
//
// Returns a copy of matching records. Modifications to the returned records do
// not affect the store.
func (s *CandidateFreshnessStore) SelectForRefresh() []CandidateFreshnessRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	var result []CandidateFreshnessRecord
	for _, r := range s.records {
		switch r.State {
		case CandidateFreshnessStateStale:
			result = append(result, *r)
		case CandidateFreshnessStateFresh:
			if !r.LastRefreshedAt.IsZero() && now.Sub(r.LastRefreshedAt) > s.maxAge {
				result = append(result, *r)
			}
		}
	}
	return result
}

// Snapshot returns a copy of all freshness records for observability.
// All tracked associations are included regardless of their state.
func (s *CandidateFreshnessStore) Snapshot() []CandidateFreshnessRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]CandidateFreshnessRecord, 0, len(s.records))
	for _, r := range s.records {
		result = append(result, *r)
	}
	return result
}

// Count returns the number of tracked associations.
func (s *CandidateFreshnessStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// ReportLines produces human-readable lines for operator logging of freshness state.
func (s *CandidateFreshnessStore) ReportLines() []string {
	records := s.Snapshot()
	lines := make([]string, 0, len(records)+1)
	lines = append(lines, fmt.Sprintf(
		"candidate-freshness: tracking %d associations (candidate-freshness distinct from endpoint-freshness and quality-freshness)",
		len(records),
	))
	for _, r := range records {
		state := string(s.FreshnessState(r.AssociationID))
		if r.StaleReason != "" {
			lines = append(lines, fmt.Sprintf(
				"  association %s: state=%s stale-reason=%s detail=%q",
				r.AssociationID, state, r.StaleReason, r.StaleDetail,
			))
		} else if !r.LastRefreshedAt.IsZero() {
			lines = append(lines, fmt.Sprintf(
				"  association %s: state=%s last-refreshed=%s ago",
				r.AssociationID, state, time.Since(r.LastRefreshedAt).Truncate(time.Second),
			))
		} else {
			lines = append(lines, fmt.Sprintf(
				"  association %s: state=%s (never refreshed)",
				r.AssociationID, state,
			))
		}
	}
	return lines
}

// SelectCandidateRefreshTargets identifies associations that need a coordinator
// candidate refresh by scanning three explicit freshness inputs.
//
// Three trigger sources are checked in priority order:
//
//  1. CandidateFreshnessStore (explicitly stale or age-expired): highest priority.
//     Associations that were explicitly marked stale or have timed out.
//
//  2. EndpointRegistry (stale or failed endpoints): checks whether any
//     distributed candidates reference endpoint addresses that are stale or
//     failed in the registry. A coordinator refresh may provide updated paths.
//
//  3. PathQualityStore (quality measurements expired): checks whether any
//     candidate's quality measurement has gone stale. The coordinator may know
//     about different candidates with better path characteristics.
//
// This function is bounded:
//   - Only associations already in candidateStore / freshnessStore are scanned.
//   - No new host:port combinations are generated.
//   - No network activity is triggered.
//   - Each association appears in the output at most once (first trigger wins).
//   - Refresh targets are inputs for coordinator fetch, NOT scheduler decisions.
//
// nil inputs are handled gracefully (treated as empty).
func SelectCandidateRefreshTargets(
	candidateStore *CandidateStore,
	registry *transport.EndpointRegistry,
	qualityStore *scheduler.PathQualityStore,
	freshnessStore *CandidateFreshnessStore,
) []CandidateRefreshTarget {
	if candidateStore == nil {
		return nil
	}

	candidateSets := candidateStore.Snapshot()

	// Build stale/failed endpoint address set from the registry.
	// true = failed (probe-confirmed unreachable), false = stale (needs revalidation).
	staleAddrSet := make(map[string]bool)
	if registry != nil {
		for _, ep := range registry.SelectForRevalidation() {
			addr := fmt.Sprintf("%s:%d", ep.Host, ep.Port)
			if ep.Verification == transport.VerificationStateFailed {
				staleAddrSet[addr] = true // failed takes precedence over stale
			} else if _, alreadyFailed := staleAddrSet[addr]; !alreadyFailed {
				staleAddrSet[addr] = false // stale
			}
		}
	}

	// Build set of quality-stale candidate IDs from the quality store snapshot.
	// Only IDs with stale=true (measured but aged out) count; absent IDs are not stale.
	staleQualityIDs := make(map[string]bool)
	if qualityStore != nil {
		for _, m := range qualityStore.Snapshot() {
			if m.Stale {
				staleQualityIDs[m.PathID] = true
			}
		}
	}

	// Deduplication: each association gets at most one refresh target.
	seen := make(map[string]bool)
	var targets []CandidateRefreshTarget

	// Priority 1: freshness store — explicitly stale or age-expired associations.
	if freshnessStore != nil {
		for _, record := range freshnessStore.SelectForRefresh() {
			if seen[record.AssociationID] {
				continue
			}
			trigger := record.StaleReason
			if trigger == "" {
				// Age-based expiry: no explicit trigger was set.
				trigger = CandidateRefreshTriggerCandidateExpired
			}
			targets = append(targets, CandidateRefreshTarget{
				AssociationID: record.AssociationID,
				Trigger:       trigger,
				Detail:        record.StaleDetail,
			})
			seen[record.AssociationID] = true
		}
	}

	// Priority 2: endpoint-stale/failed trigger — scan candidate remote endpoints.
	for _, set := range candidateSets {
		if seen[set.AssociationID] {
			continue
		}
		for _, c := range set.Candidates {
			if c.RemoteEndpoint == "" {
				continue
			}
			isFailed, exists := staleAddrSet[c.RemoteEndpoint]
			if !exists {
				continue
			}
			trigger := CandidateRefreshTriggerEndpointStale
			detail := fmt.Sprintf("endpoint %s is stale in registry", c.RemoteEndpoint)
			if isFailed {
				trigger = CandidateRefreshTriggerEndpointFailed
				detail = fmt.Sprintf("endpoint %s is failed (probe-confirmed unreachable)", c.RemoteEndpoint)
			}
			targets = append(targets, CandidateRefreshTarget{
				AssociationID: set.AssociationID,
				Trigger:       trigger,
				Detail:        detail,
			})
			seen[set.AssociationID] = true
			break // one trigger per association is sufficient
		}
	}

	// Priority 3: quality-stale trigger — at least one candidate has expired measurements.
	// Only fires when quality measurements have aged out (Stale=true in the snapshot),
	// not merely absent (never measured). Absent measurements mean probing hasn't run,
	// which is handled separately; stale measurements mean probing ran and then stopped.
	if len(staleQualityIDs) > 0 {
		for _, set := range candidateSets {
			if seen[set.AssociationID] {
				continue
			}
			for _, c := range set.Candidates {
				if c.CandidateID != "" && staleQualityIDs[c.CandidateID] {
					targets = append(targets, CandidateRefreshTarget{
						AssociationID: set.AssociationID,
						Trigger:       CandidateRefreshTriggerQualityStale,
						Detail:        fmt.Sprintf("candidate %s has stale quality measurements", c.CandidateID),
					})
					seen[set.AssociationID] = true
					break
				}
			}
		}
	}

	return targets
}

// ExecuteCandidateRefresh performs a bounded coordinator candidate refresh for
// the given targets. Each target is one association that needs fresh candidate
// data.
//
// For each target, this function:
//  1. Requests updated path candidates from the coordinator for that association.
//  2. Stores the received candidates into the CandidateStore (via StoreCandidates).
//  3. Updates the CandidateFreshnessStore to reflect the refresh outcome:
//     - Success: association marked fresh with current timestamp.
//     - Failure: association remains stale; the error is recorded in the result.
//
// Boundaries this function explicitly respects:
//   - It only refreshes the associations in the targets list (no broad resync).
//   - It does NOT call Scheduler.Decide() or activate/deactivate carriers.
//   - It does NOT install or remove forwarding entries.
//   - Refreshed candidates update only the CandidateStore. They are consumed
//     by the normal RefineCandidates → UsableSchedulerCandidates path on the
//     next scheduler run. Refresh and scheduling remain separate steps.
//
// Session constraint: this function uses the bootstrap control session. In a
// future task the secure control session (TCP+TLS 1.3 mTLS) should be used
// instead. The separation is intentional: the bootstrap session is available
// now; the session handoff is a future improvement, not a current blocker.
func ExecuteCandidateRefresh(
	ctx context.Context,
	cfg config.NodeConfig,
	bootstrap BootstrapState,
	session BootstrapSessionAttemptResult,
	store *CandidateStore,
	freshnessStore *CandidateFreshnessStore,
	targets []CandidateRefreshTarget,
) CandidateRefreshResult {
	result := CandidateRefreshResult{
		Attempted: len(targets),
	}

	// Without an accepted session we cannot reach the coordinator.
	// Skip all targets and record the reason so the caller can observe this.
	if !session.Response.Accepted() {
		result.SkippedNoSession = len(targets)
		for _, t := range targets {
			result.Details = append(result.Details, CandidateRefreshDetail{
				AssociationID: t.AssociationID,
				Trigger:       t.Trigger,
				Refreshed:     false,
				Error:         "no accepted coordinator session available for refresh",
			})
		}
		return result
	}

	if len(targets) == 0 {
		return result
	}

	now := time.Now()

	for _, t := range targets {
		detail := CandidateRefreshDetail{
			AssociationID: t.AssociationID,
			Trigger:       t.Trigger,
		}

		// Fetch fresh candidates for this specific association only.
		// We pass only this association's ID so the coordinator returns a
		// targeted response rather than a full-state dump. This keeps refresh
		// bounded to the scope identified by SelectCandidateRefreshTargets.
		fetchResult, err := FetchPathCandidates(ctx, cfg, bootstrap, session, []string{t.AssociationID})
		if err != nil {
			detail.Error = fmt.Sprintf("fetch failed: %v", err)
			result.Failed++
			result.Details = append(result.Details, detail)
			continue
		}

		// Store the received candidates. This updates only the CandidateStore.
		// No scheduler decisions are made, no carrier state is changed, and no
		// forwarding entries are installed or removed. The scheduler will consume
		// the updated candidates on the next ActivateScheduledEgress call via
		// the normal RefineCandidates path.
		stored := StoreCandidates(store, fetchResult.Response)
		detail.Refreshed = true
		detail.CandidateCount = stored

		// Mark the association as freshly refreshed. Without this update the
		// association would remain in the stale set and trigger another refresh
		// on every subsequent automation run, creating an uncontrolled loop.
		if freshnessStore != nil {
			freshnessStore.MarkRefreshed(t.AssociationID, now)
		}

		result.Refreshed++
		result.Details = append(result.Details, detail)
	}

	return result
}
