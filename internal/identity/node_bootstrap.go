package identity

import (
	"fmt"

	"github.com/zhouchenh/transitloom/internal/config"
)

type NodeIdentityPhase string

const (
	NodeIdentityPhaseBootstrapRequired   NodeIdentityPhase = "bootstrap-required"
	NodeIdentityPhaseAwaitingCertificate NodeIdentityPhase = "awaiting-certificate"
	NodeIdentityPhaseReady               NodeIdentityPhase = "ready"
)

// NodeIdentityState inspects only local persisted identity material. It does
// not perform certificate parsing or chain validation yet because this task is
// intentionally limited to bootstrap scaffolding.
type NodeIdentityState struct {
	Certificate MaterialStatus
	PrivateKey  MaterialStatus
	Phase       NodeIdentityPhase
}

func InspectNodeIdentity(cfg config.NodeConfig) (NodeIdentityState, error) {
	var state NodeIdentityState

	var err error
	state.Certificate, err = inspectMaterial(cfg.Storage.DataDir, cfg.NodeIdentity.CertificatePath, "node identity certificate")
	if err != nil {
		return state, fmt.Errorf("node identity bootstrap validation failed: %w", err)
	}
	state.PrivateKey, err = inspectMaterial(cfg.Storage.DataDir, cfg.NodeIdentity.PrivateKeyPath, "node identity private key")
	if err != nil {
		return state, fmt.Errorf("node identity bootstrap validation failed: %w", err)
	}

	switch {
	case state.Certificate.Exists && state.PrivateKey.Exists:
		state.Phase = NodeIdentityPhaseReady
	case !state.Certificate.Exists && !state.PrivateKey.Exists:
		state.Phase = NodeIdentityPhaseBootstrapRequired
	case !state.Certificate.Exists && state.PrivateKey.Exists:
		// Keeping the private key without a certificate is a coherent bootstrap
		// state for later enrollment work, but it is not identity readiness.
		state.Phase = NodeIdentityPhaseAwaitingCertificate
	default:
		return state, fmt.Errorf("node identity bootstrap validation failed: node_identity.certificate_path %q exists but node_identity.private_key_path %q is missing", state.Certificate.ResolvedPath, state.PrivateKey.ResolvedPath)
	}

	return state, nil
}

func (s NodeIdentityState) Ready() bool {
	return s.Phase == NodeIdentityPhaseReady
}

func (s NodeIdentityState) ReportLines() []string {
	return []string{
		fmt.Sprintf("node identity bootstrap state: %s", describeNodeIdentityPhase(s.Phase)),
		fmt.Sprintf("node identity certificate: %s (%s)", s.Certificate.DisplayPath(), s.Certificate.Presence()),
		fmt.Sprintf("node identity private key: %s (%s)", s.PrivateKey.DisplayPath(), s.PrivateKey.Presence()),
		"node identity note: certificate-backed identity answers who the node is; it does not imply current participation permission",
	}
}

func describeNodeIdentityPhase(phase NodeIdentityPhase) string {
	switch phase {
	case NodeIdentityPhaseBootstrapRequired:
		return "bootstrap required (no persisted node identity material is present)"
	case NodeIdentityPhaseAwaitingCertificate:
		return "awaiting certificate issuance (private key is present, certificate is absent)"
	case NodeIdentityPhaseReady:
		return "ready"
	default:
		return string(phase)
	}
}
