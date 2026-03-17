package status_test

import (
	"strings"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/service"
	"github.com/zhouchenh/transitloom/internal/status"
)

// TestBootstrapSummary_Phases verifies that BootstrapSummary preserves the
// exact phase string passed by the caller and does not conflate phases.
func TestBootstrapSummary_Phases(t *testing.T) {
	cases := []struct {
		name          string
		phase         string
		identityReady bool
		tokenCached   bool
		tokenExpired  bool
	}{
		{
			name:  "identity-bootstrap-required",
			phase: "identity-bootstrap-required",
		},
		{
			name:  "awaiting-certificate",
			phase: "awaiting-certificate",
		},
		{
			name:          "admission-token-missing",
			phase:         "admission-token-missing",
			identityReady: true,
		},
		{
			name:          "admission-token-expired",
			phase:         "admission-token-expired",
			identityReady: true,
			tokenCached:   true,
			tokenExpired:  true,
		},
		{
			name:          "ready",
			phase:         "ready",
			identityReady: true,
			tokenCached:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := status.MakeBootstrapSummary("node-a", tc.phase, tc.identityReady, tc.tokenCached, tc.tokenExpired)
			if s.Phase != tc.phase {
				t.Errorf("Phase: got %q, want %q", s.Phase, tc.phase)
			}
			if s.IdentityReady != tc.identityReady {
				t.Errorf("IdentityReady: got %v, want %v", s.IdentityReady, tc.identityReady)
			}
			if s.AdmissionTokenCached != tc.tokenCached {
				t.Errorf("AdmissionTokenCached: got %v, want %v", s.AdmissionTokenCached, tc.tokenCached)
			}
			if s.AdmissionTokenExpired != tc.tokenExpired {
				t.Errorf("AdmissionTokenExpired: got %v, want %v", s.AdmissionTokenExpired, tc.tokenExpired)
			}
		})
	}
}

// TestBootstrapSummary_ReadyIsNotAuthorized verifies that the "ready" phase
// ReportLines output explicitly marks the summary as local-readiness-only and
// does not claim coordinator authorization.
func TestBootstrapSummary_ReadyIsNotAuthorized(t *testing.T) {
	s := status.MakeBootstrapSummary("node-a", "ready", true, true, false)
	lines := s.ReportLines()

	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "local-readiness-only") {
		t.Errorf("ReportLines must label 'ready' as local-readiness-only, not coordinator-authorization; got:\n%s", joined)
	}
	if strings.Contains(joined, "authorized") {
		t.Errorf("ReportLines must not claim coordinator authorization for a bootstrap summary; got:\n%s", joined)
	}
}

// TestServiceRegistrySummary_Basic verifies correct count and entry population.
func TestServiceRegistrySummary_Basic(t *testing.T) {
	now := time.Now()
	records := []service.Record{
		{
			NodeName:      "node-a",
			Identity:      service.Identity{Name: "wg0", Type: "raw-udp"},
			BootstrapOnly: true,
			RegisteredAt:  now,
		},
		{
			NodeName:      "node-b",
			Identity:      service.Identity{Name: "wg0", Type: "raw-udp"},
			BootstrapOnly: true,
			RegisteredAt:  now,
		},
	}

	s := status.MakeServiceRegistrySummary(records)

	if s.TotalServices != 2 {
		t.Errorf("TotalServices: got %d, want 2", s.TotalServices)
	}
	if len(s.Entries) != 2 {
		t.Errorf("Entries len: got %d, want 2", len(s.Entries))
	}

	// Verify that BootstrapOnly propagates correctly.
	for _, e := range s.Entries {
		if !e.BootstrapOnly {
			t.Errorf("entry %s: BootstrapOnly should be true for bootstrap records", e.Key)
		}
	}
}

// TestServiceRegistrySummary_BootstrapOnlyLabelInReport verifies that
// bootstrap-only records are labeled in the report output.
// "Registered" must not be presented as "authorized."
func TestServiceRegistrySummary_BootstrapOnlyLabelInReport(t *testing.T) {
	now := time.Now()
	records := []service.Record{
		{
			NodeName:      "node-a",
			Identity:      service.Identity{Name: "wg0", Type: "raw-udp"},
			BootstrapOnly: true,
			RegisteredAt:  now,
		},
	}

	s := status.MakeServiceRegistrySummary(records)
	lines := s.ReportLines()
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "bootstrap-placeholder") {
		t.Errorf("ReportLines must label bootstrap-only records; got:\n%s", joined)
	}
}

// TestServiceRegistrySummary_Empty verifies zero-count summary is valid.
func TestServiceRegistrySummary_Empty(t *testing.T) {
	s := status.MakeServiceRegistrySummary(nil)
	if s.TotalServices != 0 {
		t.Errorf("expected TotalServices=0 for empty input")
	}
	lines := s.ReportLines()
	if len(lines) == 0 {
		t.Errorf("ReportLines must return at least a header line even for empty registry")
	}
}

// TestAssociationStoreSummary_Basic verifies correct count and entry population.
func TestAssociationStoreSummary_Basic(t *testing.T) {
	now := time.Now()
	records := []service.AssociationRecord{
		{
			AssociationID:      "assoc-1",
			SourceNode:         "node-a",
			SourceService:      service.Identity{Name: "wg0", Type: "raw-udp"},
			DestinationNode:    "node-b",
			DestinationService: service.Identity{Name: "wg0", Type: "raw-udp"},
			State:              service.AssociationStatePending,
			CreatedAt:          now,
			BootstrapOnly:      true,
		},
	}

	s := status.MakeAssociationStoreSummary(records)

	if s.TotalAssociations != 1 {
		t.Errorf("TotalAssociations: got %d, want 1", s.TotalAssociations)
	}
	e := s.Entries[0]
	if e.AssociationID != "assoc-1" {
		t.Errorf("AssociationID: got %q, want %q", e.AssociationID, "assoc-1")
	}
	if e.State != "pending" {
		t.Errorf("State: got %q, want %q", e.State, "pending")
	}
	if !e.BootstrapOnly {
		t.Errorf("BootstrapOnly: should be true for bootstrap records")
	}
}

// TestAssociationStoreSummary_PendingIsNotActive verifies that the "pending"
// state is surfaced clearly and is not labeled as "active" or "forwarding."
// Pending associations do not imply forwarding-state installation.
func TestAssociationStoreSummary_PendingIsNotActive(t *testing.T) {
	now := time.Now()
	records := []service.AssociationRecord{
		{
			AssociationID:      "assoc-1",
			SourceNode:         "node-a",
			SourceService:      service.Identity{Name: "wg0", Type: "raw-udp"},
			DestinationNode:    "node-b",
			DestinationService: service.Identity{Name: "wg0", Type: "raw-udp"},
			State:              service.AssociationStatePending,
			CreatedAt:          now,
			BootstrapOnly:      true,
		},
	}

	s := status.MakeAssociationStoreSummary(records)
	lines := s.ReportLines()
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "state=pending") {
		t.Errorf("ReportLines must show state=pending for pending associations; got:\n%s", joined)
	}
	// The header must note that these are placeholders, not active forwarding state.
	if !strings.Contains(joined, "placeholder") {
		t.Errorf("ReportLines must label association records as placeholders; got:\n%s", joined)
	}
}

// TestScheduledEgressSummary_AppliedVsComputed verifies that the summary
// distinguishes between the scheduler's computed mode and the carrier that
// was actually activated. This is the key semantic distinction this task
// requires: "applied" is stronger than "computed."
func TestScheduledEgressSummary_AppliedVsComputed(t *testing.T) {
	cases := []struct {
		name             string
		carrierActivated string
		schedulerMode    string
		wantActiveCount  int
		wantFailedCount  int
		wantNoEligCount  int
	}{
		{
			name:             "direct carrier active",
			carrierActivated: "direct",
			schedulerMode:    "weighted-burst-flowlet",
			wantActiveCount:  1,
		},
		{
			name:             "relay carrier active",
			carrierActivated: "relay",
			schedulerMode:    "single-path",
			wantActiveCount:  1,
		},
		{
			name:             "no eligible path",
			carrierActivated: "none",
			schedulerMode:    "no-eligible-path",
			wantNoEligCount:  1,
		},
		{
			name:             "stripe mode decided but single path activated",
			carrierActivated: "direct",
			schedulerMode:    "per-packet-stripe",
			wantActiveCount:  1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := status.ScheduledEgressSummary{
				TotalActive:     tc.wantActiveCount,
				TotalFailed:     tc.wantFailedCount,
				TotalNoEligible: tc.wantNoEligCount,
				Entries: []status.ScheduledEgressEntry{
					{
						AssociationID:    "assoc-1",
						SourceService:    "wg0",
						DestNode:         "node-b",
						DestService:      "wg0",
						CarrierActivated: tc.carrierActivated,
						SchedulerMode:    tc.schedulerMode,
						SchedulerReason:  "test reason",
					},
				},
			}

			lines := s.ReportLines()
			joined := strings.Join(lines, "\n")

			if !strings.Contains(joined, "carrier-activated="+tc.carrierActivated) {
				t.Errorf("ReportLines must contain carrier-activated=%s; got:\n%s", tc.carrierActivated, joined)
			}
			if !strings.Contains(joined, "scheduler-mode="+tc.schedulerMode) {
				t.Errorf("ReportLines must contain scheduler-mode=%s; got:\n%s", tc.schedulerMode, joined)
			}
			// Verify both fields appear on the SAME line so alignment is visible.
			for _, line := range lines {
				if strings.Contains(line, "carrier-activated=") && strings.Contains(line, "scheduler-mode=") {
					goto foundAlignment
				}
			}
			t.Errorf("carrier-activated and scheduler-mode should appear on the same line for alignment check; got:\n%s", joined)
		foundAlignment:
		})
	}
}

// TestScheduledEgressSummary_StripeGapIsVisible verifies that when the
// scheduler decides per-packet-stripe but only a single direct carrier is
// activated (because multi-carrier striping is not yet implemented), both
// values appear distinctly in the report so the gap is observable.
func TestScheduledEgressSummary_StripeGapIsVisible(t *testing.T) {
	s := status.ScheduledEgressSummary{
		TotalActive: 1,
		Entries: []status.ScheduledEgressEntry{
			{
				AssociationID:    "assoc-1",
				CarrierActivated: "direct",   // single path activated
				SchedulerMode:    "per-packet-stripe", // scheduler wanted stripe
				SchedulerReason:  "paths closely matched",
			},
		},
	}

	lines := s.ReportLines()
	joined := strings.Join(lines, "\n")

	// Both the scheduler decision and the carrier state must be visible.
	if !strings.Contains(joined, "carrier-activated=direct") {
		t.Errorf("must show carrier-activated=direct; got:\n%s", joined)
	}
	if !strings.Contains(joined, "scheduler-mode=per-packet-stripe") {
		t.Errorf("must show scheduler-mode=per-packet-stripe; got:\n%s", joined)
	}
}

// TestScheduledEgressSummary_ActivationError verifies that activation errors
// are surfaced in the report output.
func TestScheduledEgressSummary_ActivationError(t *testing.T) {
	s := status.ScheduledEgressSummary{
		TotalFailed: 1,
		Entries: []status.ScheduledEgressEntry{
			{
				AssociationID:    "assoc-1",
				CarrierActivated: "none",
				SchedulerMode:    "single-path",
				SchedulerReason:  "single eligible path: id=assoc-1:direct",
				ActivationError:  "activate direct carrier: bind local ingress :51820: address already in use",
			},
		},
	}

	lines := s.ReportLines()
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "activation-error") {
		t.Errorf("ReportLines must surface activation errors; got:\n%s", joined)
	}
	if !strings.Contains(joined, "address already in use") {
		t.Errorf("ReportLines must include the error message; got:\n%s", joined)
	}
}

// TestScheduledEgressSummary_TrafficCounters verifies that live traffic counters
// appear in the report when they are non-zero.
func TestScheduledEgressSummary_TrafficCounters(t *testing.T) {
	t.Run("direct counters appear when non-zero", func(t *testing.T) {
		s := status.ScheduledEgressSummary{
			TotalActive: 1,
			Entries: []status.ScheduledEgressEntry{
				{
					AssociationID:    "assoc-1",
					CarrierActivated: "direct",
					SchedulerMode:    "weighted-burst-flowlet",
					SchedulerReason:  "best direct path",
					IngressPackets:   42,
					IngressBytes:     5040,
				},
			},
		}
		lines := s.ReportLines()
		joined := strings.Join(lines, "\n")
		if !strings.Contains(joined, "packets=42") {
			t.Errorf("must show packet count; got:\n%s", joined)
		}
		if !strings.Contains(joined, "bytes=5040") {
			t.Errorf("must show byte count; got:\n%s", joined)
		}
	})

	t.Run("relay counters appear when non-zero", func(t *testing.T) {
		s := status.ScheduledEgressSummary{
			TotalActive: 1,
			Entries: []status.ScheduledEgressEntry{
				{
					AssociationID:    "assoc-1",
					CarrierActivated: "relay",
					SchedulerMode:    "single-path",
					SchedulerReason:  "relay only",
					EgressPackets:    10,
					EgressBytes:      1200,
				},
			},
		}
		lines := s.ReportLines()
		joined := strings.Join(lines, "\n")
		if !strings.Contains(joined, "packets=10") {
			t.Errorf("must show relay packet count; got:\n%s", joined)
		}
	})

	t.Run("zero counters not emitted for non-running carriers", func(t *testing.T) {
		s := status.ScheduledEgressSummary{
			TotalNoEligible: 1,
			Entries: []status.ScheduledEgressEntry{
				{
					AssociationID:    "assoc-1",
					CarrierActivated: "none",
					SchedulerMode:    "no-eligible-path",
					SchedulerReason:  "no candidates",
					IngressPackets:   0,
					IngressBytes:     0,
				},
			},
		}
		lines := s.ReportLines()
		joined := strings.Join(lines, "\n")
		// Zero counters for a non-running carrier should not be reported.
		if strings.Contains(joined, "packets=0") {
			t.Errorf("must not emit zero counters for non-running carriers; got:\n%s", joined)
		}
	})
}
