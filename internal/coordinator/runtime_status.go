package coordinator

import (
	"github.com/zhouchenh/transitloom/internal/status"
)

// RuntimeSummaryLines returns human-readable summary lines covering the
// coordinator's current runtime state: registered services and associations.
//
// This provides a snapshot of what the coordinator has accepted during the
// current session. The output preserves the architectural distinction between
// service registration state and association state — they are separate concepts
// that must not be collapsed.
//
// Output is suitable for logging or operator inspection. Each section is
// labeled with its category so the reader can see the state of each layer
// without having to cross-reference multiple log entries.
func (l *BootstrapListener) RuntimeSummaryLines() []string {
	registrySummary := status.MakeServiceRegistrySummary(l.registry.Snapshot())
	associationSummary := status.MakeAssociationStoreSummary(l.associations.Snapshot())

	lines := registrySummary.ReportLines()
	lines = append(lines, associationSummary.ReportLines()...)
	return lines
}
