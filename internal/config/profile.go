package config

// PolicyBundle defines a collection of related configuration settings that can be reused
// across multiple associations or services. It uses pointers so that unconfigured
// fields can be distinguished from zero values, allowing explicit override logic.
type PolicyBundle struct {
	Probing       *ProbingPolicyConfig       `yaml:"probing,omitempty"`
	Fallback      *FallbackPolicyConfig      `yaml:"fallback,omitempty"`
	MultiWAN      *MultiWANPolicyConfig      `yaml:"multi_wan,omitempty"`
	Observability *ObservabilityPolicyConfig `yaml:"observability,omitempty"`
}

// ProfileConfig represents a named reusable bundle of related Transitloom behavior.
type ProfileConfig struct {
	Name         string `yaml:"name"`
	PolicyBundle `yaml:",inline"`
}

type ProbingPolicyConfig struct {
	IntervalMs         *uint32 `yaml:"interval_ms,omitempty"`
	TimeoutMs          *uint32 `yaml:"timeout_ms,omitempty"`
	HealthyThreshold   *uint32 `yaml:"healthy_threshold,omitempty"`
	UnhealthyThreshold *uint32 `yaml:"unhealthy_threshold,omitempty"`
}

type FallbackPolicyConfig struct {
	DirectToRelayTimeoutMs  *uint32 `yaml:"direct_to_relay_timeout_ms,omitempty"`
	RelayToDirectRecoveryMs *uint32 `yaml:"relay_to_direct_recovery_ms,omitempty"`
}

type MultiWANPolicyConfig struct {
	HysteresisDelayMs *uint32 `yaml:"hysteresis_delay_ms,omitempty"`
}

type ObservabilityPolicyConfig struct {
	ExplainabilityLevel *string `yaml:"explainability_level,omitempty"` // e.g. "minimal", "standard", "debug"
}

// EffectivePolicy is the fully resolved configuration for an association after
// merging the base defaults, the selected profile, and any inline overrides.
// It contains only concrete values.
type EffectivePolicy struct {
	ProbingIntervalMs         uint32
	ProbingTimeoutMs          uint32
	ProbingHealthyThreshold   uint32
	ProbingUnhealthyThreshold uint32

	FallbackDirectToRelayTimeoutMs  uint32
	FallbackRelayToDirectRecoveryMs uint32

	MultiWANHysteresisDelayMs uint32

	ObservabilityExplainabilityLevel string
}

// DefaultPolicy returns the system-wide default policy values.
func DefaultPolicy() EffectivePolicy {
	return EffectivePolicy{
		ProbingIntervalMs:                5000,
		ProbingTimeoutMs:                 1000,
		ProbingHealthyThreshold:          3,
		ProbingUnhealthyThreshold:        3,
		FallbackDirectToRelayTimeoutMs:   10000,
		FallbackRelayToDirectRecoveryMs:  30000,
		MultiWANHysteresisDelayMs:        2000,
		ObservabilityExplainabilityLevel: "standard",
	}
}

// ResolvePolicy computes the EffectivePolicy by layering the profile (if any)
// and the overrides (if any) on top of the system defaults.
func ResolvePolicy(profile *ProfileConfig, overrides *PolicyBundle) EffectivePolicy {
	eff := DefaultPolicy()

	if profile != nil {
		applyBundle(&eff, &profile.PolicyBundle)
	}
	if overrides != nil {
		applyBundle(&eff, overrides)
	}

	return eff
}

func applyBundle(eff *EffectivePolicy, bundle *PolicyBundle) {
	if bundle.Probing != nil {
		if bundle.Probing.IntervalMs != nil {
			eff.ProbingIntervalMs = *bundle.Probing.IntervalMs
		}
		if bundle.Probing.TimeoutMs != nil {
			eff.ProbingTimeoutMs = *bundle.Probing.TimeoutMs
		}
		if bundle.Probing.HealthyThreshold != nil {
			eff.ProbingHealthyThreshold = *bundle.Probing.HealthyThreshold
		}
		if bundle.Probing.UnhealthyThreshold != nil {
			eff.ProbingUnhealthyThreshold = *bundle.Probing.UnhealthyThreshold
		}
	}

	if bundle.Fallback != nil {
		if bundle.Fallback.DirectToRelayTimeoutMs != nil {
			eff.FallbackDirectToRelayTimeoutMs = *bundle.Fallback.DirectToRelayTimeoutMs
		}
		if bundle.Fallback.RelayToDirectRecoveryMs != nil {
			eff.FallbackRelayToDirectRecoveryMs = *bundle.Fallback.RelayToDirectRecoveryMs
		}
	}

	if bundle.MultiWAN != nil {
		if bundle.MultiWAN.HysteresisDelayMs != nil {
			eff.MultiWANHysteresisDelayMs = *bundle.MultiWAN.HysteresisDelayMs
		}
	}

	if bundle.Observability != nil {
		if bundle.Observability.ExplainabilityLevel != nil {
			eff.ObservabilityExplainabilityLevel = *bundle.Observability.ExplainabilityLevel
		}
	}
}
