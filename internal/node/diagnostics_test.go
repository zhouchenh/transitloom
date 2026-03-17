package node

import (
	"strings"
	"testing"

	"github.com/zhouchenh/transitloom/internal/scheduler"
	"github.com/zhouchenh/transitloom/internal/status"
)

func TestBuildCandidateStatus(t *testing.T) {
	configCandidates := []scheduler.PathCandidate{
		{
			ID:            "assoc1:direct",
			AssociationID: "assoc1",
			Class:         scheduler.PathClassDirectPublic,
			Health:        scheduler.HealthStateActive,
			Quality: scheduler.PathQuality{
				RTT:        10,
				Confidence: 0.9,
			},
		},
	}

	refined := []RefinedCandidate{
		{
			DistributedID: "dist1",
			Usable:        false,
			ExcludeReason: "endpoint failed",
			Candidate: scheduler.PathCandidate{
				Class: scheduler.PathClassDirectPublic,
			},
			EndpointState: CandidateEndpointFailed,
		},
		{
			DistributedID: "dist2",
			Usable:        true,
			Candidate: scheduler.PathCandidate{
				ID:     "dist2",
				Class:  scheduler.PathClassCoordinatorRelay,
				Health: scheduler.HealthStateDegraded,
				Quality: scheduler.PathQuality{
					RTT:        100,
					Confidence: 0.5,
				},
			},
			DegradedReason: "stale endpoint",
			EndpointState:  CandidateEndpointStale,
		},
	}

	stats := buildCandidateStatus(configCandidates, refined)

	if len(stats) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(stats))
	}

	// Verify config candidate
	if stats[0].ID != "assoc1:direct" || !stats[0].Usable || stats[0].RTT != 10 {
		t.Errorf("config candidate mismatch: %+v", stats[0])
	}

	// Verify excluded candidate
	if stats[1].ID != "dist1" || stats[1].Usable || stats[1].ExcludeReason != "endpoint failed" {
		t.Errorf("excluded candidate mismatch: %+v", stats[1])
	}

	// Verify degraded candidate
	if stats[2].ID != "dist2" || !stats[2].Usable || stats[2].DegradedReason != "stale endpoint" || stats[2].RTT != 100 {
		t.Errorf("degraded candidate mismatch: %+v", stats[2])
	}
}

func TestScheduledEgressSummary_ReportLines_Diagnostics(t *testing.T) {
	s := status.ScheduledEgressSummary{
		TotalActive: 1,
		Entries: []status.ScheduledEgressEntry{
			{
				AssociationID:    "assoc1",
				CarrierActivated: "direct",
				SchedulerMode:    "weighted-burst-flowlet",
				SchedulerReason:  "best path chosen",
				Candidates: []status.PathCandidateStatus{
					{
						ID:            "c1",
						Class:         "direct-public",
						Usable:        true,
						Health:        "active",
						EndpointState: "usable",
						Confidence:    0.9,
						RTT:           10,
					},
					{
						ID:            "c2",
						Class:         "coordinator-relay",
						Usable:        false,
						ExcludeReason: "endpoint failed",
						Health:        "failed",
						EndpointState: "failed",
					},
				},
			},
		},
	}

	lines := s.ReportLines()
	foundUsable := false
	foundExcluded := false

	for _, line := range lines {
		if strings.Contains(line, "candidate c1: usable") && strings.Contains(line, "rtt=10") {
			foundUsable = true
		}
		if strings.Contains(line, "candidate c2: EXCLUDED: endpoint failed") {
			foundExcluded = true
		}
	}

	if !foundUsable {
		t.Error("ReportLines missing usable candidate diagnostic")
	}
	if !foundExcluded {
		t.Error("ReportLines missing excluded candidate diagnostic")
	}
}
