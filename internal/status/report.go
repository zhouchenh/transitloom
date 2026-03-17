package status

import (
	"fmt"
	"time"
)

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
	lines = append(lines, fmt.Sprintf(
		"probe-loop: state=%s interval=%s max-targets=%d",
		describeField(s.ProbeLoop.State, "disabled"),
		s.ProbeLoop.ProbeInterval,
		s.ProbeLoop.MaxTargetsPerRound,
	))
	if s.ProbeLoop.Reason != "" {
		lines = append(lines, fmt.Sprintf("  probe-loop-reason: %s", s.ProbeLoop.Reason))
	}
	if !s.ProbeLoop.LastRoundAt.IsZero() {
		lines = append(lines, fmt.Sprintf(
			"  probe-loop-last-round: at=%s selected=%d reachable=%d unreachable=%d errors=%d",
			s.ProbeLoop.LastRoundAt.Format(time.RFC3339),
			s.ProbeLoop.LastRound.TargetsSelected,
			s.ProbeLoop.LastRound.Reachable,
			s.ProbeLoop.LastRound.Unreachable,
			s.ProbeLoop.LastRound.Errors,
		))
	}

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

		// Stickiness policy state: show when present to give operators
		// switch-stability visibility alongside the fallback state.
		if e.StickinessReason != "" {
			switchLabel := ""
			if e.SwitchOccurred {
				switchLabel = " [SWITCH]"
			}
			holdLabel := ""
			if e.HoldDownActive {
				holdLabel = " [hold-down]"
			}
			lines = append(lines, fmt.Sprintf(
				"    stickiness%s%s: %s",
				switchLabel, holdLabel, e.StickinessReason,
			))
		}

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

		// Candidate diagnostics: detailed "why" for each considered path.
		for _, c := range e.Candidates {
			statusLabel := "usable"
			if !c.Usable {
				statusLabel = fmt.Sprintf("EXCLUDED: %s", c.ExcludeReason)
			} else if c.DegradedReason != "" {
				statusLabel = fmt.Sprintf("degraded: %s", c.DegradedReason)
			}

			qualityLabel := "unmeasured"
			if c.Confidence > 0 {
				qualityLabel = fmt.Sprintf("rtt=%v loss=%.2f%% conf=%.2f", c.RTT, c.LossFraction*100, c.Confidence)
			}

			lines = append(lines, fmt.Sprintf(
				"    candidate %s: %s | class=%s health=%s endpoint=%s quality=%s",
				c.ID, statusLabel, c.Class, c.Health, c.EndpointState, qualityLabel,
			))
		}
	}

	if len(s.RecentEvents) > 0 {
		lines = append(lines, "")
		lines = append(lines, "recent-path-events:")
		for _, e := range s.RecentEvents {
			assocStr := ""
			if e.AssociationID != "" {
				assocStr = fmt.Sprintf(" assoc=%s", e.AssociationID)
			}
			candStr := ""
			if e.CandidateID != "" {
				candStr = fmt.Sprintf(" cand=%s", e.CandidateID)
			}
			lines = append(lines, fmt.Sprintf(
				"  [%s] type=%s%s%s msg=%q",
				e.Timestamp.Format(time.RFC3339), e.Type, assocStr, candStr, e.Message,
			))
		}
	}

	return lines
}

func (s ControlReconciliationSummary) ReportLines() []string {
	lines := []string{
		fmt.Sprintf(
			"control-reconciliation: phase=%s transport-connected=%v session-established=%v session-authenticated=%v logical-state-reconciled=%v transport-mode=%s coordinator=%s",
			s.Phase,
			s.TransportConnected,
			s.SessionEstablished,
			s.SessionAuthenticated,
			s.LogicalStateReconciled,
			describeField(s.TransportMode, "<unknown>"),
			describeField(s.CurrentCoordinator, "<unknown>"),
		),
		fmt.Sprintf(
			"  steps: service=%s association=%s path-candidates=%s",
			s.ServiceRefresh,
			s.AssociationRefresh,
			s.PathCandidateRefresh,
		),
	}
	if !s.LastTransitionAt.IsZero() {
		lines = append(lines, fmt.Sprintf("  last-transition-at=%s", s.LastTransitionAt.Format("2006-01-02T15:04:05Z07:00")))
	}
	if !s.LastTransportReconnectAt.IsZero() {
		lines = append(lines, fmt.Sprintf("  last-transport-reconnect-at=%s", s.LastTransportReconnectAt.Format("2006-01-02T15:04:05Z07:00")))
	}
	if !s.LastSessionEstablishedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("  last-session-established-at=%s", s.LastSessionEstablishedAt.Format("2006-01-02T15:04:05Z07:00")))
	}
	if !s.LastReconciledAt.IsZero() {
		lines = append(lines, fmt.Sprintf("  last-reconciled-at=%s", s.LastReconciledAt.Format("2006-01-02T15:04:05Z07:00")))
	}
	if s.LastFailure != "" {
		lines = append(lines, "  last-failure="+s.LastFailure)
	}
	return lines
}

func describeField(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
