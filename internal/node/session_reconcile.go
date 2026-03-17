package node

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/scheduler"
	"github.com/zhouchenh/transitloom/internal/status"
	"github.com/zhouchenh/transitloom/internal/transport"
)

const (
	ControlSessionResumeInterval = 10 * time.Second
	ControlSessionResumeTimeout  = 5 * time.Second
)

type ControlSessionRuntime struct {
	cfg             config.NodeConfig
	bootstrap       BootstrapState
	candidateStore  *CandidateStore
	freshnessStore  *CandidateFreshnessStore
	endpointStore   *transport.EndpointRegistry
	qualityStore    *scheduler.PathQualityStore
	associationIDs  []string
	attemptSession  func(context.Context, config.NodeConfig, BootstrapState) (BootstrapSessionAttemptResult, error)
	registerService func(context.Context, config.NodeConfig, BootstrapState, BootstrapSessionAttemptResult) (ServiceRegistrationAttemptResult, error)
	associate       func(context.Context, config.NodeConfig, BootstrapState, BootstrapSessionAttemptResult) (AssociationAttemptResult, error)
	fetchCandidates func(context.Context, config.NodeConfig, BootstrapState, BootstrapSessionAttemptResult, []string) (PathCandidateAttemptResult, error)
	storeCandidates func(*CandidateStore, controlplane.PathCandidateResponse) int
	selectTargets   func(*CandidateStore, *transport.EndpointRegistry, *scheduler.PathQualityStore, *CandidateFreshnessStore) []CandidateRefreshTarget
	executeRefresh  func(context.Context, config.NodeConfig, BootstrapState, BootstrapSessionAttemptResult, *CandidateStore, *CandidateFreshnessStore, []CandidateRefreshTarget) CandidateRefreshResult
	mu              sync.RWMutex
	summary         status.ControlReconciliationSummary
}

func NewControlSessionRuntime(
	cfg config.NodeConfig,
	bootstrap BootstrapState,
	associationResults []AssociationResultEntry,
	candidateStore *CandidateStore,
	endpointStore *transport.EndpointRegistry,
	qualityStore *scheduler.PathQualityStore,
) *ControlSessionRuntime {
	now := time.Now().UTC()
	associationIDs := make([]string, 0, len(associationResults))
	for _, r := range associationResults {
		if !r.Accepted {
			continue
		}
		id := strings.TrimSpace(r.AssociationID)
		if id != "" {
			associationIDs = append(associationIDs, id)
		}
	}
	freshnessStore := NewCandidateFreshnessStore(DefaultCandidateMaxAge)
	for _, id := range associationIDs {
		freshnessStore.TrackAssociation(id)
	}
	return &ControlSessionRuntime{
		cfg:            cfg,
		bootstrap:      bootstrap,
		candidateStore: candidateStore,
		freshnessStore: freshnessStore,
		endpointStore:  endpointStore,
		qualityStore:   qualityStore,
		associationIDs: associationIDs,
		attemptSession: AttemptBootstrapSession,
		registerService: func(ctx context.Context, cfg config.NodeConfig, bootstrap BootstrapState, session BootstrapSessionAttemptResult) (ServiceRegistrationAttemptResult, error) {
			return AttemptServiceRegistration(ctx, cfg, bootstrap, session)
		},
		associate:       AttemptAssociation,
		fetchCandidates: FetchPathCandidates,
		storeCandidates: StoreCandidates,
		selectTargets:   SelectCandidateRefreshTargets,
		executeRefresh:  ExecuteCandidateRefresh,
		summary: status.ControlReconciliationSummary{
			Phase:                status.ControlReconciliationPhaseDisconnected,
			ServiceRefresh:       status.ControlReconciliationStepPending,
			AssociationRefresh:   status.ControlReconciliationStepPending,
			PathCandidateRefresh: status.ControlReconciliationStepPending,
			LastTransitionAt:     now,
		},
	}
}

func (r *ControlSessionRuntime) Run(ctx context.Context) error {
	r.runCycle(ctx)
	ticker := time.NewTicker(ControlSessionResumeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.runCycle(ctx)
		}
	}
}

func (r *ControlSessionRuntime) Summary() status.ControlReconciliationSummary {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.summary
}

func (r *ControlSessionRuntime) runCycle(ctx context.Context) {
	attemptCtx, cancel := context.WithTimeout(ctx, ControlSessionResumeTimeout)
	defer cancel()

	session, err := r.attemptSession(attemptCtx, r.cfg, r.bootstrap)
	if err != nil {
		r.handleDisconnect(err.Error())
		return
	}
	if !session.Response.Accepted() {
		r.handleDisconnect(fmt.Sprintf("control session rejected: %s", session.Response.Reason))
		return
	}

	transportStatus := controlplane.BootstrapOnlyTransportStatus()
	now := time.Now().UTC()

	r.mu.Lock()
	wasConnected := r.summary.TransportConnected
	r.summary.TransportConnected = true
	r.summary.TransportMode = string(transportStatus.Mode)
	r.summary.SessionEstablished = true
	r.summary.SessionAuthenticated = transportStatus.Authenticated
	r.summary.CurrentCoordinator = session.CoordinatorLabel
	if r.summary.CurrentCoordinator == "" {
		r.summary.CurrentCoordinator = session.Response.CoordinatorName
	}
	if !wasConnected {
		r.summary.Phase = status.ControlReconciliationPhaseTransportReconnected
		r.summary.LastTransportReconnectAt = now
		r.summary.LastTransitionAt = now
	}
	r.summary.Phase = status.ControlReconciliationPhaseSessionEstablished
	r.summary.LastSessionEstablishedAt = now
	r.summary.LastTransitionAt = now
	r.summary.LastFailure = ""
	r.summary.ServiceRefresh = status.ControlReconciliationStepPending
	r.summary.AssociationRefresh = status.ControlReconciliationStepPending
	r.summary.PathCandidateRefresh = status.ControlReconciliationStepPending
	r.mu.Unlock()

	if err := r.reconcile(attemptCtx, session); err != nil {
		r.mu.Lock()
		r.summary.Phase = status.ControlReconciliationPhaseReconciliationFailed
		r.summary.LastFailure = err.Error()
		r.summary.LastTransitionAt = time.Now().UTC()
		r.mu.Unlock()
		return
	}

	r.mu.Lock()
	r.summary.Phase = status.ControlReconciliationPhaseReconciled
	r.summary.LogicalStateReconciled = true
	r.summary.LastReconciledAt = time.Now().UTC()
	r.summary.LastTransitionAt = r.summary.LastReconciledAt
	r.summary.LastFailure = ""
	r.mu.Unlock()
}

func (r *ControlSessionRuntime) reconcile(ctx context.Context, session BootstrapSessionAttemptResult) error {
	r.mu.Lock()
	r.summary.Phase = status.ControlReconciliationPhaseReconciling
	r.summary.LogicalStateReconciled = false
	r.summary.LastTransitionAt = time.Now().UTC()
	r.mu.Unlock()

	if len(r.cfg.Services) == 0 {
		r.mu.Lock()
		r.summary.ServiceRefresh = status.ControlReconciliationStepSkipped
		r.mu.Unlock()
	} else {
		res, err := r.registerService(ctx, r.cfg, r.bootstrap, session)
		if err != nil {
			r.mu.Lock()
			r.summary.ServiceRefresh = status.ControlReconciliationStepFailed
			r.mu.Unlock()
			return fmt.Errorf("service refresh failed: %w", err)
		}
		if !res.Response.AllRegistered() {
			r.mu.Lock()
			r.summary.ServiceRefresh = status.ControlReconciliationStepFailed
			r.mu.Unlock()
			return fmt.Errorf("service refresh incomplete: accepted=%d rejected=%d", res.Response.AcceptedCount, res.Response.RejectedCount)
		}
		r.mu.Lock()
		r.summary.ServiceRefresh = status.ControlReconciliationStepSucceeded
		r.mu.Unlock()
	}

	if len(r.cfg.Associations) == 0 {
		r.mu.Lock()
		r.summary.AssociationRefresh = status.ControlReconciliationStepSkipped
		r.mu.Unlock()
	} else {
		res, err := r.associate(ctx, r.cfg, r.bootstrap, session)
		if err != nil {
			r.mu.Lock()
			r.summary.AssociationRefresh = status.ControlReconciliationStepFailed
			r.mu.Unlock()
			return fmt.Errorf("association refresh failed: %w", err)
		}
		if !res.Response.AllCreated() {
			r.mu.Lock()
			r.summary.AssociationRefresh = status.ControlReconciliationStepFailed
			r.mu.Unlock()
			return fmt.Errorf("association refresh incomplete: accepted=%d rejected=%d", res.Response.AcceptedCount, res.Response.RejectedCount)
		}
		ids := make([]string, 0, len(res.Response.Results))
		for _, result := range res.Response.Results {
			if result.Outcome != controlplane.AssociationResultOutcomeCreated {
				continue
			}
			id := strings.TrimSpace(result.AssociationID)
			if id != "" {
				ids = append(ids, id)
			}
		}
		r.mu.Lock()
		r.associationIDs = ids
		r.summary.AssociationRefresh = status.ControlReconciliationStepSucceeded
		r.mu.Unlock()
		for _, id := range ids {
			r.freshnessStore.TrackAssociation(id)
		}
	}

	if r.candidateStore == nil || len(r.associationIDsSnapshot()) == 0 {
		r.mu.Lock()
		r.summary.PathCandidateRefresh = status.ControlReconciliationStepSkipped
		r.mu.Unlock()
		return nil
	}

	ids := r.associationIDsSnapshot()
	res, err := r.fetchCandidates(ctx, r.cfg, r.bootstrap, session, ids)
	if err != nil {
		r.mu.Lock()
		r.summary.PathCandidateRefresh = status.ControlReconciliationStepFailed
		r.mu.Unlock()
		return fmt.Errorf("path candidate refresh failed: %w", err)
	}
	if r.storeCandidates != nil {
		r.storeCandidates(r.candidateStore, res.Response)
	}
	now := time.Now().UTC()
	for _, id := range ids {
		r.freshnessStore.MarkRefreshed(id, now)
	}
	targets := r.selectTargets(r.candidateStore, r.endpointStore, r.qualityStore, r.freshnessStore)
	refreshResult := r.executeRefresh(ctx, r.cfg, r.bootstrap, session, r.candidateStore, r.freshnessStore, targets)
	if refreshResult.Failed > 0 {
		r.mu.Lock()
		r.summary.PathCandidateRefresh = status.ControlReconciliationStepFailed
		r.mu.Unlock()
		return fmt.Errorf("candidate refresh follow-up failed: failed=%d", refreshResult.Failed)
	}
	r.mu.Lock()
	r.summary.PathCandidateRefresh = status.ControlReconciliationStepSucceeded
	r.mu.Unlock()
	return nil
}

func (r *ControlSessionRuntime) handleDisconnect(reason string) {
	now := time.Now().UTC()
	for _, id := range r.associationIDsSnapshot() {
		r.freshnessStore.MarkStale(id, CandidateRefreshTriggerPathUnhealthy, "control session disconnected", now)
	}
	if r.endpointStore != nil {
		r.endpointStore.MarkAllStale(now)
	}

	r.mu.Lock()
	r.summary.TransportConnected = false
	r.summary.SessionEstablished = false
	r.summary.SessionAuthenticated = false
	r.summary.LogicalStateReconciled = false
	r.summary.Phase = status.ControlReconciliationPhaseDisconnected
	r.summary.ServiceRefresh = status.ControlReconciliationStepPending
	r.summary.AssociationRefresh = status.ControlReconciliationStepPending
	r.summary.PathCandidateRefresh = status.ControlReconciliationStepPending
	r.summary.LastFailure = reason
	r.summary.LastTransitionAt = now
	r.mu.Unlock()
}

func (r *ControlSessionRuntime) associationIDsSnapshot() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, len(r.associationIDs))
	copy(ids, r.associationIDs)
	return ids
}
