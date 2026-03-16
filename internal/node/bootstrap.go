package node

import (
	"fmt"
	"time"

	"github.com/zhouchenh/transitloom/internal/admission"
	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/identity"
)

type BootstrapPhase string

const (
	BootstrapPhaseIdentityBootstrapRequired BootstrapPhase = "identity-bootstrap-required"
	BootstrapPhaseAwaitingCertificate       BootstrapPhase = "awaiting-certificate"
	BootstrapPhaseAdmissionMissing          BootstrapPhase = "admission-token-missing"
	BootstrapPhaseAdmissionExpired          BootstrapPhase = "admission-token-expired"
	BootstrapPhaseReady                     BootstrapPhase = "ready"
)

// BootstrapState keeps node identity and admission inspection distinct while
// still giving transitloom-node one startup report surface.
type BootstrapState struct {
	Identity  identity.NodeIdentityState
	Admission admission.TokenCacheState
	Phase     BootstrapPhase
}

func InspectBootstrap(cfg config.NodeConfig, now time.Time) (BootstrapState, error) {
	var state BootstrapState

	var err error
	state.Identity, err = identity.InspectNodeIdentity(cfg)
	if err != nil {
		return state, err
	}
	state.Admission, err = admission.InspectTokenCache(cfg, now)
	if err != nil {
		return state, err
	}

	if state.Admission.HasTokenFile() && !state.Identity.Ready() {
		return state, fmt.Errorf("node bootstrap validation failed: admission.current_token_path %q exists but node identity bootstrap state is %q", state.Admission.Cache.ResolvedPath, state.Identity.Phase)
	}

	switch state.Identity.Phase {
	case identity.NodeIdentityPhaseBootstrapRequired:
		state.Phase = BootstrapPhaseIdentityBootstrapRequired
	case identity.NodeIdentityPhaseAwaitingCertificate:
		state.Phase = BootstrapPhaseAwaitingCertificate
	default:
		switch state.Admission.Phase {
		case admission.TokenCachePhaseUsable:
			state.Phase = BootstrapPhaseReady
		case admission.TokenCachePhaseExpired:
			state.Phase = BootstrapPhaseAdmissionExpired
		default:
			state.Phase = BootstrapPhaseAdmissionMissing
		}
	}

	return state, nil
}

func (s BootstrapState) ReportLines() []string {
	lines := []string{
		fmt.Sprintf("node bootstrap readiness: %s", describeBootstrapPhase(s.Phase)),
	}
	lines = append(lines, s.Identity.ReportLines()...)
	lines = append(lines, s.Admission.ReportLines()...)
	return lines
}

func describeBootstrapPhase(phase BootstrapPhase) string {
	switch phase {
	case BootstrapPhaseIdentityBootstrapRequired:
		return "identity bootstrap required before the node can become eligible for admission"
	case BootstrapPhaseAwaitingCertificate:
		return "node has local key material but still needs certificate issuance before admission can matter"
	case BootstrapPhaseAdmissionMissing:
		return "node identity is ready, but no cached current admission token is available"
	case BootstrapPhaseAdmissionExpired:
		return "node identity is ready, but the cached current admission token is expired"
	case BootstrapPhaseReady:
		return "node identity material is ready and a cached current admission token is present"
	default:
		return string(phase)
	}
}
