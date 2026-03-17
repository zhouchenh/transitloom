package status

import (
	"fmt"

	"github.com/zhouchenh/transitloom/internal/transport"
)

// EndpointFreshnessSummary captures the verification and freshness state of
// a node's external endpoint collection.
//
// This summary distinguishes configured/unverified, verified, stale, and
// failed endpoints so operators can assess whether direct-path decisions are
// based on current or stale endpoint knowledge.
//
// An operator reading this summary can answer:
//   - how many endpoints are currently usable for direct-path attempts?
//   - which endpoints are stale and need revalidation?
//   - are any endpoints consistently failing probes?
//
// This summary is distinct from other status summaries: it reflects endpoint
// knowledge freshness, not bootstrap authorization or carrier activation state.
type EndpointFreshnessSummary struct {
	TotalEndpoints  int
	UsableCount     int
	UnverifiedCount int
	VerifiedCount   int
	StaleCount      int
	FailedCount     int
	Entries         []EndpointFreshnessEntry
}

// EndpointFreshnessEntry describes one endpoint's verification and freshness state.
type EndpointFreshnessEntry struct {
	Host string
	Port uint16

	// LocalPort is the local mesh listener port. Nonzero when DNAT is
	// configured (external port differs from local mesh port).
	LocalPort    uint16
	Source       string
	Verification string
	HasDNAT      bool

	// IsUsable reports whether this endpoint can be used for direct-path
	// reachability decisions. False for stale and failed endpoints, which
	// must be revalidated before use.
	IsUsable bool
}

// MakeEndpointFreshnessSummary builds an EndpointFreshnessSummary from a
// slice of ExternalEndpoint values (e.g., from EndpointRegistry.Snapshot()).
func MakeEndpointFreshnessSummary(endpoints []transport.ExternalEndpoint) EndpointFreshnessSummary {
	s := EndpointFreshnessSummary{
		TotalEndpoints: len(endpoints),
		Entries:        make([]EndpointFreshnessEntry, 0, len(endpoints)),
	}
	for _, ep := range endpoints {
		switch ep.Verification {
		case transport.VerificationStateUnverified:
			s.UnverifiedCount++
		case transport.VerificationStateVerified:
			s.VerifiedCount++
		case transport.VerificationStateStale:
			s.StaleCount++
		case transport.VerificationStateFailed:
			s.FailedCount++
		}
		if ep.IsUsable() {
			s.UsableCount++
		}
		s.Entries = append(s.Entries, EndpointFreshnessEntry{
			Host:         ep.Host,
			Port:         ep.Port,
			LocalPort:    ep.LocalPort,
			Source:       string(ep.Source),
			Verification: string(ep.Verification),
			HasDNAT:      ep.HasDNAT(),
			IsUsable:     ep.IsUsable(),
		})
	}
	return s
}

// ReportLines produces human-readable log lines for the endpoint freshness summary.
//
// The output distinguishes configured/unverified endpoints (still usable,
// representing operator intent) from stale and failed endpoints (must be
// revalidated) so operators can quickly assess endpoint health.
func (s EndpointFreshnessSummary) ReportLines() []string {
	lines := make([]string, 0, 1+len(s.Entries))
	lines = append(lines, fmt.Sprintf(
		"endpoint-freshness-summary: total=%d usable=%d (unverified=%d verified=%d stale=%d failed=%d)",
		s.TotalEndpoints, s.UsableCount, s.UnverifiedCount, s.VerifiedCount, s.StaleCount, s.FailedCount,
	))
	for _, e := range s.Entries {
		dnLabel := ""
		if e.HasDNAT {
			dnLabel = fmt.Sprintf(" dnat=local:%d", e.LocalPort)
		}
		usableLabel := ""
		if !e.IsUsable {
			// Stale or failed: operator needs to know this endpoint cannot be
			// used for direct-path decisions without revalidation.
			usableLabel = " [needs-revalidation]"
		}
		lines = append(lines, fmt.Sprintf(
			"  endpoint: %s:%d source=%s verification=%s%s%s",
			e.Host, e.Port, e.Source, e.Verification, dnLabel, usableLabel,
		))
	}
	return lines
}
