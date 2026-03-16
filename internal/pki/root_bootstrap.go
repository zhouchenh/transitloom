package pki

import (
	"fmt"

	"github.com/zhouchenh/transitloom/internal/config"
)

type RootBootstrapPhase string

const (
	RootBootstrapPhaseReady               RootBootstrapPhase = "ready"
	RootBootstrapPhaseInitializationReady RootBootstrapPhase = "initialization-required"
)

// RootBootstrapState keeps root bootstrap distinct from coordinator trust
// bootstrap because the root is the deployment trust anchor rather than a
// normal coordinator endpoint.
type RootBootstrapState struct {
	RootName    string
	GenerateKey bool
	RootCert    MaterialStatus
	RootKey     MaterialStatus
	Phase       RootBootstrapPhase
}

func InspectRootBootstrap(cfg config.RootConfig) (RootBootstrapState, error) {
	state := RootBootstrapState{
		RootName:    cfg.Identity.Name,
		GenerateKey: cfg.Trust.GenerateKey,
	}

	var err error
	state.RootCert, err = inspectMaterial(cfg.Storage.DataDir, cfg.Trust.RootCertPath, "root certificate")
	if err != nil {
		return state, fmt.Errorf("root bootstrap validation failed: %w", err)
	}
	state.RootKey, err = inspectMaterial(cfg.Storage.DataDir, cfg.Trust.RootKeyPath, "root key")
	if err != nil {
		return state, fmt.Errorf("root bootstrap validation failed: %w", err)
	}

	switch {
	case state.RootCert.Exists && state.RootKey.Exists:
		state.Phase = RootBootstrapPhaseReady
	case !state.RootCert.Exists && !state.RootKey.Exists:
		if !cfg.Trust.GenerateKey {
			return state, fmt.Errorf("root bootstrap validation failed: trust.root_cert_path %q and trust.root_key_path %q are missing and trust.generate_key is false", state.RootCert.ResolvedPath, state.RootKey.ResolvedPath)
		}
		state.Phase = RootBootstrapPhaseInitializationReady
	default:
		if state.RootCert.Exists {
			return state, fmt.Errorf("root bootstrap validation failed: trust.root_cert_path %q exists but trust.root_key_path %q is missing", state.RootCert.ResolvedPath, state.RootKey.ResolvedPath)
		}
		return state, fmt.Errorf("root bootstrap validation failed: trust.root_key_path %q exists but trust.root_cert_path %q is missing", state.RootKey.ResolvedPath, state.RootCert.ResolvedPath)
	}

	return state, nil
}

func (s RootBootstrapState) ReportLines() []string {
	return []string{
		fmt.Sprintf("root bootstrap state: %s", describeRootBootstrapPhase(s.Phase)),
		fmt.Sprintf("root trust anchor certificate: %s (%s)", s.RootCert.DisplayPath(), s.RootCert.Presence()),
		fmt.Sprintf("root trust anchor key: %s (%s)", s.RootKey.DisplayPath(), s.RootKey.Presence()),
		"root bootstrap note: the root authority remains a separate trust role; coordinator intermediate issuance is intentionally not implemented in this task",
	}
}

func describeRootBootstrapPhase(phase RootBootstrapPhase) string {
	switch phase {
	case RootBootstrapPhaseReady:
		return "ready"
	case RootBootstrapPhaseInitializationReady:
		return "initialization required (trust material is absent and trust.generate_key=true)"
	default:
		return string(phase)
	}
}
