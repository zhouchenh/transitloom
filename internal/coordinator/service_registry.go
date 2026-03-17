package coordinator

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/service"
)

// ServiceRegistry stores the minimal coordinator-side placeholder registration
// state for this task. Records remain bootstrap-only placeholders until later
// authenticated control sessions and association work exist.
type ServiceRegistry struct {
	mu      sync.Mutex
	records map[string]service.Record
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		records: make(map[string]service.Record),
	}
}

func (r *ServiceRegistry) Apply(nodeName string, registrations []service.Registration, now time.Time) []controlplane.ServiceRegistrationResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	seen := make(map[string]struct{}, len(registrations))
	results := make([]controlplane.ServiceRegistrationResult, 0, len(registrations))

	for _, registration := range registrations {
		result := controlplane.ServiceRegistrationResult{
			ServiceName: registration.Identity.Name,
			ServiceType: string(registration.Identity.Type),
		}

		key := service.RecordKey(nodeName, registration.Identity)
		if _, exists := seen[key]; exists {
			result.Outcome = controlplane.ServiceRegistrationResultOutcomeRejected
			result.Reason = controlplane.ServiceRegistrationResultReasonDuplicateService
			result.Details = []string{
				fmt.Sprintf("service %q appeared more than once in the same registration batch", registration.Identity.Name),
			}
			results = append(results, result)
			continue
		}
		seen[key] = struct{}{}

		if err := registration.Validate(); err != nil {
			result.Outcome = controlplane.ServiceRegistrationResultOutcomeRejected
			result.Reason = controlplane.ServiceRegistrationResultReasonInvalidServiceDecl
			result.Details = []string{
				fmt.Sprintf("service declaration is invalid: %v", err),
			}
			results = append(results, result)
			continue
		}

		record := registration.ToRecord(nodeName, now)
		_, replaced := r.records[key]
		r.records[key] = record

		result.Outcome = controlplane.ServiceRegistrationResultOutcomeRegistered
		result.Reason = controlplane.ServiceRegistrationResultReasonRegistered
		result.RegistryKey = key
		if replaced {
			result.Details = append(result.Details, "replaced an existing bootstrap-only coordinator service record")
		} else {
			result.Details = append(result.Details, "stored a bootstrap-only coordinator service record")
		}
		if registration.RequestedLocalIngress != nil {
			result.Details = append(result.Details, "requested local ingress intent was captured separately from the service binding; no local ingress binding was allocated")
		} else {
			result.Details = append(result.Details, "no local ingress intent was requested for this service")
		}

		results = append(results, result)
	}

	return results
}

func (r *ServiceRegistry) Snapshot() []service.Record {
	r.mu.Lock()
	defer r.mu.Unlock()

	records := make([]service.Record, 0, len(r.records))
	for _, record := range r.records {
		records = append(records, record.Clone())
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Key() < records[j].Key()
	})

	return records
}

// snapshotMap returns a shallow copy of the internal records map for use by
// the association store during validation. The caller must hold no registry
// lock; this method acquires and releases its own lock.
func (r *ServiceRegistry) snapshotMap() map[string]service.Record {
	r.mu.Lock()
	defer r.mu.Unlock()

	snapshot := make(map[string]service.Record, len(r.records))
	for k, v := range r.records {
		snapshot[k] = v
	}
	return snapshot
}
