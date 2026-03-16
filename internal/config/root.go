package config

import "strings"

type RootTrustConfig struct {
	RootCertPath string `yaml:"root_cert_path"`
	RootKeyPath  string `yaml:"root_key_path"`
	GenerateKey  bool   `yaml:"generate_key"`
}

type RootIssuanceConfig struct {
	DefaultIntermediateTTL string `yaml:"default_intermediate_ttl,omitempty"`
}

type RootAdminConfig struct {
	Enabled bool   `yaml:"enabled"`
	Listen  string `yaml:"listen,omitempty"`
}

type RootConfig struct {
	Identity      IdentityMetadata    `yaml:"identity"`
	Storage       StorageConfig       `yaml:"storage"`
	Trust         RootTrustConfig     `yaml:"trust"`
	Issuance      RootIssuanceConfig  `yaml:"issuance"`
	Admin         RootAdminConfig     `yaml:"admin"`
	Observability ObservabilityConfig `yaml:"observability"`
}

func (c RootConfig) Validate() error {
	var errs validationErrors

	validateIdentity("identity", c.Identity, &errs)
	validateStorage("storage", c.Storage, &errs)

	if strings.TrimSpace(c.Trust.RootCertPath) == "" {
		errs.add("trust.root_cert_path", "must be set")
	}
	if strings.TrimSpace(c.Trust.RootKeyPath) == "" {
		errs.add("trust.root_key_path", "must be set")
	}

	validateDuration("issuance.default_intermediate_ttl", c.Issuance.DefaultIntermediateTTL, &errs)

	if c.Admin.Enabled && strings.TrimSpace(c.Admin.Listen) == "" {
		errs.add("admin.listen", "must be set when admin.enabled is true")
	}
	if strings.TrimSpace(c.Admin.Listen) != "" {
		validateHostPort("admin.listen", c.Admin.Listen, true, &errs)
	}

	validateObservability("observability", c.Observability, &errs)

	return errs.err("root")
}
