package status_test

import (
	"strings"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/status"
	"github.com/zhouchenh/transitloom/internal/transport"
)

// TestMakeEndpointFreshnessSummaryEmpty verifies that an empty endpoint slice
// produces a zero-count summary with no entries.
func TestMakeEndpointFreshnessSummaryEmpty(t *testing.T) {
	t.Parallel()

	s := status.MakeEndpointFreshnessSummary(nil)
	if s.TotalEndpoints != 0 {
		t.Errorf("TotalEndpoints = %d, want 0", s.TotalEndpoints)
	}
	if s.UsableCount != 0 {
		t.Errorf("UsableCount = %d, want 0", s.UsableCount)
	}
	if len(s.Entries) != 0 {
		t.Errorf("Entries length = %d, want 0", len(s.Entries))
	}
}

// TestMakeEndpointFreshnessSummaryCounters verifies that the summary correctly
// counts endpoints by verification state and usability.
func TestMakeEndpointFreshnessSummaryCounters(t *testing.T) {
	t.Parallel()

	now := time.Now()

	unverified := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)

	verified := transport.NewConfiguredEndpoint("198.51.100.5", 9000, 0)
	verified.MarkVerified(now)

	stale := transport.NewConfiguredEndpoint("192.0.2.1", 7000, 0)
	stale.MarkVerified(now)
	stale.MarkStale(now.Add(time.Minute))

	failed := transport.NewConfiguredEndpoint("10.0.0.1", 5000, 0)
	failed.MarkFailed(now)

	endpoints := []transport.ExternalEndpoint{unverified, verified, stale, failed}
	s := status.MakeEndpointFreshnessSummary(endpoints)

	if s.TotalEndpoints != 4 {
		t.Errorf("TotalEndpoints = %d, want 4", s.TotalEndpoints)
	}
	// Usable: unverified (configured = usable) + verified = 2.
	if s.UsableCount != 2 {
		t.Errorf("UsableCount = %d, want 2 (unverified + verified)", s.UsableCount)
	}
	if s.UnverifiedCount != 1 {
		t.Errorf("UnverifiedCount = %d, want 1", s.UnverifiedCount)
	}
	if s.VerifiedCount != 1 {
		t.Errorf("VerifiedCount = %d, want 1", s.VerifiedCount)
	}
	if s.StaleCount != 1 {
		t.Errorf("StaleCount = %d, want 1", s.StaleCount)
	}
	if s.FailedCount != 1 {
		t.Errorf("FailedCount = %d, want 1", s.FailedCount)
	}
	if len(s.Entries) != 4 {
		t.Errorf("Entries length = %d, want 4", len(s.Entries))
	}
}

// TestEndpointFreshnessSummaryDNATEntry verifies that DNAT information is
// preserved in the summary entries.
func TestEndpointFreshnessSummaryDNATEntry(t *testing.T) {
	t.Parallel()

	// external port 12000, local port 51830 = DNAT case.
	ep := transport.NewConfiguredEndpoint("203.0.113.1", 12000, 51830)
	s := status.MakeEndpointFreshnessSummary([]transport.ExternalEndpoint{ep})

	if len(s.Entries) != 1 {
		t.Fatalf("Entries length = %d, want 1", len(s.Entries))
	}
	entry := s.Entries[0]
	if !entry.HasDNAT {
		t.Error("HasDNAT must be true for endpoint with different external and local ports")
	}
	if entry.LocalPort != 51830 {
		t.Errorf("LocalPort = %d, want 51830", entry.LocalPort)
	}
	if entry.Port != 12000 {
		t.Errorf("Port = %d, want 12000 (external port)", entry.Port)
	}
}

// TestEndpointFreshnessSummaryReportLines verifies that ReportLines produces
// output that distinguishes stale/failed (needs-revalidation) from usable
// endpoints and includes the correct totals.
func TestEndpointFreshnessSummaryReportLines(t *testing.T) {
	t.Parallel()

	now := time.Now()

	verified := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
	verified.MarkVerified(now)

	stale := transport.NewConfiguredEndpoint("198.51.100.5", 9000, 0)
	stale.MarkStale(now)

	s := status.MakeEndpointFreshnessSummary([]transport.ExternalEndpoint{verified, stale})
	lines := s.ReportLines()

	if len(lines) == 0 {
		t.Fatal("ReportLines() returned no lines")
	}

	// First line must be the summary header with counts.
	header := lines[0]
	if !strings.Contains(header, "endpoint-freshness-summary") {
		t.Errorf("header line does not contain 'endpoint-freshness-summary': %q", header)
	}
	if !strings.Contains(header, "total=2") {
		t.Errorf("header does not contain total=2: %q", header)
	}
	if !strings.Contains(header, "stale=1") {
		t.Errorf("header does not contain stale=1: %q", header)
	}

	// The stale endpoint line must contain "needs-revalidation".
	combined := strings.Join(lines, "\n")
	if !strings.Contains(combined, "needs-revalidation") {
		t.Error("ReportLines must label stale/failed endpoints as [needs-revalidation]")
	}

	// The verified endpoint line must NOT contain "needs-revalidation".
	for _, line := range lines[1:] {
		if strings.Contains(line, "203.0.113.1") && strings.Contains(line, "needs-revalidation") {
			t.Error("verified endpoint must not be labeled as [needs-revalidation]")
		}
	}
}

// TestEndpointFreshnessSummaryIsUsableDistinction verifies that the summary
// correctly marks usable vs non-usable endpoints in entries.
func TestEndpointFreshnessSummaryIsUsableDistinction(t *testing.T) {
	t.Parallel()

	now := time.Now()

	// Unverified configured: IsUsable = true (operator intent).
	unverified := transport.NewConfiguredEndpoint("203.0.113.1", 51830, 0)
	// Stale: IsUsable = false.
	stale := transport.NewConfiguredEndpoint("198.51.100.5", 9000, 0)
	stale.MarkStale(now)
	// Failed: IsUsable = false.
	failed := transport.NewConfiguredEndpoint("192.0.2.1", 7000, 0)
	failed.MarkFailed(now)

	s := status.MakeEndpointFreshnessSummary([]transport.ExternalEndpoint{unverified, stale, failed})

	for _, e := range s.Entries {
		switch e.Host {
		case "203.0.113.1":
			if !e.IsUsable {
				t.Error("unverified configured endpoint entry must be IsUsable=true")
			}
		case "198.51.100.5":
			if e.IsUsable {
				t.Error("stale endpoint entry must be IsUsable=false")
			}
		case "192.0.2.1":
			if e.IsUsable {
				t.Error("failed endpoint entry must be IsUsable=false")
			}
		}
	}
}
