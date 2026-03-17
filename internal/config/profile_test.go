package config

import (
	"testing"
)

func ptr[T any](v T) *T {
	return &v
}

func TestResolvePolicy(t *testing.T) {
	// Baseline system defaults
	defaults := DefaultPolicy()

	profile := &ProfileConfig{
		Name: "aggressive",
		PolicyBundle: PolicyBundle{
			Probing: &ProbingPolicyConfig{
				IntervalMs: ptr(uint32(2000)),
			},
			Fallback: &FallbackPolicyConfig{
				DirectToRelayTimeoutMs: ptr(uint32(5000)),
			},
			Observability: &ObservabilityPolicyConfig{
				ExplainabilityLevel: ptr("debug"),
			},
		},
	}

	overrides := &PolicyBundle{
		Fallback: &FallbackPolicyConfig{
			DirectToRelayTimeoutMs: ptr(uint32(3000)), // Overrides profile
		},
		MultiWAN: &MultiWANPolicyConfig{
			HysteresisDelayMs: ptr(uint32(1000)), // Overrides default
		},
	}

	resolved := ResolvePolicy(profile, overrides)

	// Check that profile applied
	if resolved.ProbingIntervalMs != 2000 {
		t.Errorf("expected ProbingIntervalMs 2000, got %d", resolved.ProbingIntervalMs)
	}

	// Check that override applied over profile
	if resolved.FallbackDirectToRelayTimeoutMs != 3000 {
		t.Errorf("expected FallbackDirectToRelayTimeoutMs 3000, got %d", resolved.FallbackDirectToRelayTimeoutMs)
	}

	// Check that override applied over default
	if resolved.MultiWANHysteresisDelayMs != 1000 {
		t.Errorf("expected MultiWANHysteresisDelayMs 1000, got %d", resolved.MultiWANHysteresisDelayMs)
	}

	// Check that profile applied
	if resolved.ObservabilityExplainabilityLevel != "debug" {
		t.Errorf("expected ObservabilityExplainabilityLevel 'debug', got %q", resolved.ObservabilityExplainabilityLevel)
	}

	// Check that untouched default remained
	if resolved.ProbingTimeoutMs != defaults.ProbingTimeoutMs {
		t.Errorf("expected ProbingTimeoutMs %d, got %d", defaults.ProbingTimeoutMs, resolved.ProbingTimeoutMs)
	}
}
