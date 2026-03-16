package config

import (
	"fmt"
	"strings"
)

type CoordinatorTrustConfig struct {
	RootAnchorPath       string `yaml:"root_anchor_path"`
	IntermediateCertPath string `yaml:"intermediate_cert_path"`
	IntermediateKeyPath  string `yaml:"intermediate_key_path"`
}

type CoordinatorConfig struct {
	Identity      IdentityMetadata             `yaml:"identity"`
	Storage       StorageConfig                `yaml:"storage"`
	Control       ControlTransportConfig       `yaml:"control"`
	Trust         CoordinatorTrustConfig       `yaml:"trust"`
	Peers         []BootstrapCoordinatorConfig `yaml:"peers,omitempty"`
	Discovery     CoordinatorDiscoveryConfig   `yaml:"discovery"`
	Relay         CoordinatorRelayConfig       `yaml:"relay"`
	Observability ObservabilityConfig          `yaml:"observability"`
}

func (c CoordinatorConfig) Validate() error {
	var errs validationErrors

	validateIdentity("identity", c.Identity, &errs)
	validateStorage("storage", c.Storage, &errs)

	validateTransportListener("control.quic", c.Control.QUIC, &errs)
	validateTransportListener("control.tcp", c.Control.TCP, &errs)
	if !c.Control.QUIC.Enabled && !c.Control.TCP.Enabled {
		errs.add("control", "must enable at least one control transport")
	}

	if strings.TrimSpace(c.Trust.RootAnchorPath) == "" {
		errs.add("trust.root_anchor_path", "must be set")
	}
	if strings.TrimSpace(c.Trust.IntermediateCertPath) == "" {
		errs.add("trust.intermediate_cert_path", "must be set")
	}
	if strings.TrimSpace(c.Trust.IntermediateKeyPath) == "" {
		errs.add("trust.intermediate_key_path", "must be set")
	}

	for i, peer := range c.Peers {
		validateBootstrapCoordinator(fmt.Sprintf("peers[%d]", i), peer, &errs)
	}

	if c.Relay.ControlEnabled || c.Relay.DataEnabled {
		if len(c.Relay.ListenEndpoints) == 0 {
			errs.add("relay.listen_endpoints", "must contain at least one endpoint when relay is enabled")
		}
	}
	for i, endpoint := range c.Relay.ListenEndpoints {
		validateHostPort(fmt.Sprintf("relay.listen_endpoints[%d]", i), endpoint, true, &errs)
	}

	validateObservability("observability", c.Observability, &errs)

	return errs.err("coordinator")
}
