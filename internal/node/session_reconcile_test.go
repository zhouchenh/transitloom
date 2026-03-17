package node

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/scheduler"
	"github.com/zhouchenh/transitloom/internal/status"
	"github.com/zhouchenh/transitloom/internal/transport"
)

func TestControlSessionRuntime_DisconnectMarksStateStale(t *testing.T) {
	runtime := newTestControlSessionRuntime()
	runtime.associationIDs = []string{"assoc-1"}
	runtime.freshnessStore.MarkRefreshed("assoc-1", time.Now().UTC())
	runtime.attemptSession = func(context.Context, config.NodeConfig, BootstrapState) (BootstrapSessionAttemptResult, error) {
		return BootstrapSessionAttemptResult{}, fmt.Errorf("dial timeout")
	}

	runtime.runCycle(context.Background())

	summary := runtime.Summary()
	if summary.Phase != status.ControlReconciliationPhaseDisconnected {
		t.Fatalf("phase = %q, want %q", summary.Phase, status.ControlReconciliationPhaseDisconnected)
	}
	if summary.TransportConnected {
		t.Fatalf("transport_connected = true, want false")
	}
	if summary.LogicalStateReconciled {
		t.Fatalf("logical_state_reconciled = true, want false")
	}
	if runtime.freshnessStore.FreshnessState("assoc-1") != CandidateFreshnessStateStale {
		t.Fatalf("freshness state = %q, want %q", runtime.freshnessStore.FreshnessState("assoc-1"), CandidateFreshnessStateStale)
	}
}

func TestControlSessionRuntime_SuccessfulCycleReconciles(t *testing.T) {
	runtime := newTestControlSessionRuntime()
	runtime.associationIDs = []string{"assoc-1"}
	runtime.attemptSession = func(context.Context, config.NodeConfig, BootstrapState) (BootstrapSessionAttemptResult, error) {
		return BootstrapSessionAttemptResult{
			CoordinatorLabel: "coord-a",
			Response: controlplane.BootstrapSessionResponse{
				ProtocolVersion: controlplane.BootstrapProtocolVersion,
				CoordinatorName: "coord-a",
				Outcome:         controlplane.BootstrapSessionOutcomeAccepted,
				Reason:          controlplane.BootstrapSessionReasonPrerequisitesSatisfied,
				BootstrapOnly:   true,
			},
		}, nil
	}
	runtime.registerService = func(context.Context, config.NodeConfig, BootstrapState, BootstrapSessionAttemptResult) (ServiceRegistrationAttemptResult, error) {
		return ServiceRegistrationAttemptResult{
			Response: controlplane.ServiceRegistrationResponse{
				ProtocolVersion: controlplane.BootstrapProtocolVersion,
				CoordinatorName: "coord-a",
				Outcome:         controlplane.ServiceRegistrationOutcomeAccepted,
				Reason:          controlplane.ServiceRegistrationReasonRegistered,
				BootstrapOnly:   true,
				AcceptedCount:   1,
			},
		}, nil
	}
	runtime.associate = func(context.Context, config.NodeConfig, BootstrapState, BootstrapSessionAttemptResult) (AssociationAttemptResult, error) {
		return AssociationAttemptResult{
			Response: controlplane.AssociationResponse{
				ProtocolVersion: controlplane.BootstrapProtocolVersion,
				CoordinatorName: "coord-a",
				Outcome:         controlplane.AssociationOutcomeAccepted,
				Reason:          controlplane.AssociationReasonCreated,
				BootstrapOnly:   true,
				AcceptedCount:   1,
				Results: []controlplane.AssociationResult{
					{
						AssociationID: "assoc-1",
						Outcome:       controlplane.AssociationResultOutcomeCreated,
						Reason:        controlplane.AssociationResultReasonCreated,
					},
				},
			},
		}, nil
	}
	runtime.fetchCandidates = func(context.Context, config.NodeConfig, BootstrapState, BootstrapSessionAttemptResult, []string) (PathCandidateAttemptResult, error) {
		return PathCandidateAttemptResult{
			Response: controlplane.PathCandidateResponse{
				ProtocolVersion: controlplane.BootstrapProtocolVersion,
				CoordinatorName: "coord-a",
				BootstrapOnly:   true,
				CandidateSets: []controlplane.PathCandidateSet{
					{
						AssociationID: "assoc-1",
						Candidates: []controlplane.DistributedPathCandidate{
							{
								CandidateID:     "assoc-1:direct",
								AssociationID:   "assoc-1",
								Class:           controlplane.DistributedPathClassDirectPublic,
								IsRelayAssisted: false,
								RemoteEndpoint:  "127.0.0.1:9000",
								AdminWeight:     100,
								RelayNodeID:     "",
								IsMetered:       false,
								Note:            "",
							},
						},
					},
				},
			},
		}, nil
	}
	runtime.executeRefresh = func(context.Context, config.NodeConfig, BootstrapState, BootstrapSessionAttemptResult, *CandidateStore, *CandidateFreshnessStore, []CandidateRefreshTarget) CandidateRefreshResult {
		return CandidateRefreshResult{}
	}

	runtime.runCycle(context.Background())

	summary := runtime.Summary()
	if summary.Phase != status.ControlReconciliationPhaseReconciled {
		t.Fatalf("phase = %q, want %q", summary.Phase, status.ControlReconciliationPhaseReconciled)
	}
	if !summary.TransportConnected {
		t.Fatalf("transport_connected = false, want true")
	}
	if !summary.SessionEstablished {
		t.Fatalf("session_established = false, want true")
	}
	if summary.SessionAuthenticated {
		t.Fatalf("session_authenticated = true, want false for bootstrap transport")
	}
	if !summary.LogicalStateReconciled {
		t.Fatalf("logical_state_reconciled = false, want true")
	}
	if summary.ServiceRefresh != status.ControlReconciliationStepSucceeded {
		t.Fatalf("service refresh = %q, want %q", summary.ServiceRefresh, status.ControlReconciliationStepSucceeded)
	}
	if summary.AssociationRefresh != status.ControlReconciliationStepSucceeded {
		t.Fatalf("association refresh = %q, want %q", summary.AssociationRefresh, status.ControlReconciliationStepSucceeded)
	}
	if summary.PathCandidateRefresh != status.ControlReconciliationStepSucceeded {
		t.Fatalf("path refresh = %q, want %q", summary.PathCandidateRefresh, status.ControlReconciliationStepSucceeded)
	}
	if runtime.freshnessStore.FreshnessState("assoc-1") != CandidateFreshnessStateFresh {
		t.Fatalf("freshness state = %q, want %q", runtime.freshnessStore.FreshnessState("assoc-1"), CandidateFreshnessStateFresh)
	}
}

func TestControlSessionRuntime_FailedReconciliationIsExplicit(t *testing.T) {
	runtime := newTestControlSessionRuntime()
	runtime.attemptSession = func(context.Context, config.NodeConfig, BootstrapState) (BootstrapSessionAttemptResult, error) {
		return BootstrapSessionAttemptResult{
			Response: controlplane.BootstrapSessionResponse{
				ProtocolVersion: controlplane.BootstrapProtocolVersion,
				CoordinatorName: "coord-a",
				Outcome:         controlplane.BootstrapSessionOutcomeAccepted,
				Reason:          controlplane.BootstrapSessionReasonPrerequisitesSatisfied,
				BootstrapOnly:   true,
			},
		}, nil
	}
	runtime.registerService = func(context.Context, config.NodeConfig, BootstrapState, BootstrapSessionAttemptResult) (ServiceRegistrationAttemptResult, error) {
		return ServiceRegistrationAttemptResult{
			Response: controlplane.ServiceRegistrationResponse{
				ProtocolVersion: controlplane.BootstrapProtocolVersion,
				CoordinatorName: "coord-a",
				Outcome:         controlplane.ServiceRegistrationOutcomePartial,
				Reason:          controlplane.ServiceRegistrationReasonPartiallyRegistered,
				BootstrapOnly:   true,
				AcceptedCount:   0,
				RejectedCount:   1,
			},
		}, nil
	}

	runtime.runCycle(context.Background())

	summary := runtime.Summary()
	if summary.Phase != status.ControlReconciliationPhaseReconciliationFailed {
		t.Fatalf("phase = %q, want %q", summary.Phase, status.ControlReconciliationPhaseReconciliationFailed)
	}
	if summary.ServiceRefresh != status.ControlReconciliationStepFailed {
		t.Fatalf("service refresh = %q, want %q", summary.ServiceRefresh, status.ControlReconciliationStepFailed)
	}
	if summary.LastFailure == "" {
		t.Fatalf("last_failure is empty, want explicit failure detail")
	}
	if summary.LogicalStateReconciled {
		t.Fatalf("logical_state_reconciled = true, want false")
	}
}

func newTestControlSessionRuntime() *ControlSessionRuntime {
	return &ControlSessionRuntime{
		cfg: config.NodeConfig{
			Identity: config.IdentityMetadata{Name: "node-a"},
			Services: []config.ServiceConfig{
				{
					Name: "svc-a",
					Type: config.ServiceTypeRawUDP,
					Binding: config.ServiceBindingConfig{
						Address: "127.0.0.1",
						Port:    51820,
					},
				},
			},
			Associations: []config.AssociationConfig{
				{
					SourceService:      "svc-a",
					DestinationNode:    "node-b",
					DestinationService: "svc-b",
					DirectEndpoint:     "127.0.0.1:9000",
					MeshListenPort:     6000,
				},
			},
		},
		bootstrap:       BootstrapState{},
		candidateStore:  NewCandidateStore(),
		freshnessStore:  NewCandidateFreshnessStore(DefaultCandidateMaxAge),
		endpointStore:   transport.NewEndpointRegistry(),
		qualityStore:    scheduler.NewPathQualityStore(scheduler.DefaultQualityMaxAge),
		storeCandidates: StoreCandidates,
		selectTargets:   SelectCandidateRefreshTargets,
		executeRefresh:  ExecuteCandidateRefresh,
		summary: status.ControlReconciliationSummary{
			Phase:                status.ControlReconciliationPhaseDisconnected,
			ServiceRefresh:       status.ControlReconciliationStepPending,
			AssociationRefresh:   status.ControlReconciliationStepPending,
			PathCandidateRefresh: status.ControlReconciliationStepPending,
		},
	}
}
