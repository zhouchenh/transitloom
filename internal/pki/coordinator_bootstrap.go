package pki

import (
	"fmt"

	"github.com/zhouchenh/transitloom/internal/config"
)

type CoordinatorBootstrapPhase string

const (
	CoordinatorBootstrapPhaseReady                CoordinatorBootstrapPhase = "ready"
	CoordinatorBootstrapPhaseAwaitingIntermediate CoordinatorBootstrapPhase = "awaiting-intermediate"
)

// CoordinatorBootstrapState keeps coordinator trust bootstrap explicit so the
// coordinator's root trust anchor, local intermediate material, and later node
// issuance logic do not get collapsed into generic startup code.
type CoordinatorBootstrapState struct {
	CoordinatorName  string
	RootAnchor       MaterialStatus
	IntermediateCert MaterialStatus
	IntermediateKey  MaterialStatus
	Phase            CoordinatorBootstrapPhase
}

func InspectCoordinatorBootstrap(cfg config.CoordinatorConfig) (CoordinatorBootstrapState, error) {
	state := CoordinatorBootstrapState{
		CoordinatorName: cfg.Identity.Name,
	}

	var err error
	state.RootAnchor, err = inspectMaterial(cfg.Storage.DataDir, cfg.Trust.RootAnchorPath, "coordinator root trust anchor")
	if err != nil {
		return state, fmt.Errorf("coordinator trust bootstrap validation failed: %w", err)
	}
	if !state.RootAnchor.Exists {
		return state, fmt.Errorf("coordinator trust bootstrap validation failed: trust.root_anchor_path %q is missing", state.RootAnchor.ResolvedPath)
	}

	state.IntermediateCert, err = inspectMaterial(cfg.Storage.DataDir, cfg.Trust.IntermediateCertPath, "coordinator intermediate certificate")
	if err != nil {
		return state, fmt.Errorf("coordinator trust bootstrap validation failed: %w", err)
	}
	state.IntermediateKey, err = inspectMaterial(cfg.Storage.DataDir, cfg.Trust.IntermediateKeyPath, "coordinator intermediate key")
	if err != nil {
		return state, fmt.Errorf("coordinator trust bootstrap validation failed: %w", err)
	}

	switch {
	case state.IntermediateCert.Exists && state.IntermediateKey.Exists:
		state.Phase = CoordinatorBootstrapPhaseReady
	case !state.IntermediateCert.Exists && !state.IntermediateKey.Exists:
		// A coordinator with a valid root anchor but no intermediate yet is a
		// coherent bootstrap state for this task. It keeps issuance separate
		// from trust bootstrap instead of pretending issuance is already wired.
		state.Phase = CoordinatorBootstrapPhaseAwaitingIntermediate
	default:
		if state.IntermediateCert.Exists {
			return state, fmt.Errorf("coordinator trust bootstrap validation failed: trust.intermediate_cert_path %q exists but trust.intermediate_key_path %q is missing", state.IntermediateCert.ResolvedPath, state.IntermediateKey.ResolvedPath)
		}
		return state, fmt.Errorf("coordinator trust bootstrap validation failed: trust.intermediate_key_path %q exists but trust.intermediate_cert_path %q is missing", state.IntermediateKey.ResolvedPath, state.IntermediateCert.ResolvedPath)
	}

	return state, nil
}

func (s CoordinatorBootstrapState) ReportLines() []string {
	return []string{
		fmt.Sprintf("coordinator trust bootstrap state: %s", describeCoordinatorBootstrapPhase(s.Phase)),
		fmt.Sprintf("coordinator root trust anchor: %s (%s)", s.RootAnchor.DisplayPath(), s.RootAnchor.Presence()),
		fmt.Sprintf("coordinator intermediate certificate: %s (%s)", s.IntermediateCert.DisplayPath(), s.IntermediateCert.Presence()),
		fmt.Sprintf("coordinator intermediate key: %s (%s)", s.IntermediateKey.DisplayPath(), s.IntermediateKey.Presence()),
		"coordinator bootstrap note: root trust bootstrap stays separate from future node issuance and admission-token logic",
	}
}

func describeCoordinatorBootstrapPhase(phase CoordinatorBootstrapPhase) string {
	switch phase {
	case CoordinatorBootstrapPhaseReady:
		return "ready"
	case CoordinatorBootstrapPhaseAwaitingIntermediate:
		return "awaiting intermediate material from the root authority"
	default:
		return string(phase)
	}
}
