package config

import "fmt"

type NodeConfig struct {
	Identity              IdentityMetadata             `yaml:"identity"`
	Storage               StorageConfig                `yaml:"storage"`
	NodeIdentity          NodeIdentityConfig           `yaml:"node_identity"`
	Admission             NodeAdmissionConfig          `yaml:"admission"`
	Control               ControlPreferencesConfig     `yaml:"control"`
	BootstrapCoordinators []BootstrapCoordinatorConfig `yaml:"bootstrap_coordinators"`
	Services              []ServiceConfig              `yaml:"services"`
	Associations          []AssociationConfig          `yaml:"associations,omitempty"`
	LocalIngress          LocalIngressPolicyConfig     `yaml:"local_ingress"`
	// ExternalEndpoint carries explicitly configured external reachability
	// information for this node. This is distinct from local service bindings,
	// local ingress ports, and mesh listener ports. It represents what remote
	// nodes should use to reach this node from outside the local network,
	// including through DNAT rules on a router.
	ExternalEndpoint ExternalEndpointConfig       `yaml:"external_endpoint,omitempty"`
	Discovery        NodeDiscoveryConfig          `yaml:"discovery"`
	Relay            NodeRelayConfig              `yaml:"relay"`
	Observability    ObservabilityConfig          `yaml:"observability"`
}

type NodeIdentityConfig struct {
	CertificatePath string `yaml:"certificate_path"`
	PrivateKeyPath  string `yaml:"private_key_path"`
}

type NodeAdmissionConfig struct {
	CurrentTokenPath string `yaml:"current_token_path"`
}

func (c NodeConfig) Validate() error {
	var errs validationErrors

	validateIdentity("identity", c.Identity, &errs)
	validateStorage("storage", c.Storage, &errs)
	validateNodeIdentity("node_identity", c.NodeIdentity, &errs)
	validateNodeAdmission("admission", c.Admission, &errs)
	validateControlPreferences("control", c.Control, &errs)
	validateLocalIngressPolicy("local_ingress", c.LocalIngress, &errs)

	if len(c.BootstrapCoordinators) == 0 {
		errs.add("bootstrap_coordinators", "must contain at least one bootstrap coordinator")
	}
	for i, bootstrap := range c.BootstrapCoordinators {
		validateBootstrapCoordinator(fmt.Sprintf("bootstrap_coordinators[%d]", i), bootstrap, &errs)
	}

	if len(c.Services) == 0 {
		errs.add("services", "must contain at least one service")
	}

	serviceNames := make(map[string]struct{}, len(c.Services))
	for i, service := range c.Services {
		servicePath := fmt.Sprintf("services[%d]", i)
		validateService(servicePath, service, c.LocalIngress, &errs)
		if service.Name == "" {
			continue
		}
		if _, exists := serviceNames[service.Name]; exists {
			errs.add(servicePath+".name", "must be unique within the node config")
			continue
		}
		serviceNames[service.Name] = struct{}{}
	}

	for i, assoc := range c.Associations {
		validateAssociation(fmt.Sprintf("associations[%d]", i), assoc, serviceNames, &errs)
	}

	validateExternalEndpoint("external_endpoint", c.ExternalEndpoint, &errs)

	if c.Relay.MaxAssociations < 0 {
		errs.add("relay.max_associations", "must be zero or greater")
	}

	validateObservability("observability", c.Observability, &errs)

	return errs.err("node")
}
