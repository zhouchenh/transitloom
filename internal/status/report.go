package status

import "fmt"

// ReportLines produces human-readable log lines for the bootstrap summary.
//
// The output explicitly labels the phase as local readiness only, preventing
// misreading of "ready" as "coordinator-authorized."
func (s BootstrapSummary) ReportLines() []string {
	lines := []string{
		fmt.Sprintf("bootstrap-summary: node=%q phase=%s (local-readiness-only, not coordinator-authorization)", s.NodeName, s.Phase),
		fmt.Sprintf("  identity-ready=%v admission-token-cached=%v admission-token-expired=%v",
			s.IdentityReady, s.AdmissionTokenCached, s.AdmissionTokenExpired),
	}
	return lines
}

// ReportLines produces human-readable log lines for the service registry summary.
//
// The output labels bootstrap-only records explicitly so operators and future
// agents do not mistake placeholder records for fully authenticated service state.
func (s ServiceRegistrySummary) ReportLines() []string {
	lines := make([]string, 0, 1+len(s.Entries))
	lines = append(lines, fmt.Sprintf("service-registry-summary: total=%d", s.TotalServices))
	for _, e := range s.Entries {
		bootstrapLabel := ""
		if e.BootstrapOnly {
			bootstrapLabel = " [bootstrap-placeholder]"
		}
		lines = append(lines, fmt.Sprintf("  service: key=%s node=%s name=%s type=%s%s",
			e.Key, e.NodeName, e.ServiceName, e.ServiceType, bootstrapLabel))
	}
	return lines
}

// ReportLines produces human-readable log lines for the association store summary.
//
// The output explicitly notes that association records are logical connectivity
// placeholders and do not imply forwarding-state installation or live traffic.
func (s AssociationStoreSummary) ReportLines() []string {
	lines := make([]string, 0, 1+len(s.Entries))
	lines = append(lines, fmt.Sprintf("association-store-summary: total=%d (logical-connectivity-placeholders; forwarding-state-installation is separate)", s.TotalAssociations))
	for _, e := range s.Entries {
		bootstrapLabel := ""
		if e.BootstrapOnly {
			bootstrapLabel = " [bootstrap-placeholder]"
		}
		lines = append(lines, fmt.Sprintf("  association: id=%s %s/%s -> %s/%s state=%s%s",
			e.AssociationID, e.SourceNode, e.SourceService, e.DestNode, e.DestService, e.State, bootstrapLabel))
	}
	return lines
}

// ReportLines produces human-readable log lines for the scheduled egress summary.
//
// The output distinguishes between scheduler decisions and applied carrier state,
// and includes live traffic counters so an operator can confirm data-plane
// behavior matches scheduler intent.
func (s ScheduledEgressSummary) ReportLines() []string {
	lines := make([]string, 0, 1+2*len(s.Entries))
	lines = append(lines, fmt.Sprintf(
		"scheduled-egress-summary: active=%d failed=%d no-eligible=%d",
		s.TotalActive, s.TotalFailed, s.TotalNoEligible,
	))

	for _, e := range s.Entries {
		// First line: applied carrier state and scheduler mode side by side.
		// This pairing lets an operator verify alignment at a glance.
		lines = append(lines, fmt.Sprintf(
			"  association %s: carrier-activated=%s scheduler-mode=%s",
			e.AssociationID, e.CarrierActivated, e.SchedulerMode,
		))

		// Second line: scheduler reason + any error.
		reasonLine := fmt.Sprintf("    reason: %s", e.SchedulerReason)
		if e.ActivationError != "" {
			reasonLine += fmt.Sprintf(" | activation-error: %s", e.ActivationError)
		}
		lines = append(lines, reasonLine)

		// Traffic counters: only emit when there is something to report.
		// Emitting zero counters for non-running carriers adds noise without value.
		if e.CarrierActivated == "direct" && (e.IngressPackets > 0 || e.IngressBytes > 0) {
			lines = append(lines, fmt.Sprintf(
				"    direct-egress-counters: packets=%d bytes=%d",
				e.IngressPackets, e.IngressBytes,
			))
		}
		if e.CarrierActivated == "relay" && (e.EgressPackets > 0 || e.EgressBytes > 0) {
			lines = append(lines, fmt.Sprintf(
				"    relay-egress-counters: packets=%d bytes=%d",
				e.EgressPackets, e.EgressBytes,
			))
		}
	}

	return lines
}
