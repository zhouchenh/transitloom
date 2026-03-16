package node

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/service"
)

type ServiceRegistrationAttemptResult struct {
	CoordinatorLabel string
	Endpoint         string
	Response         controlplane.ServiceRegistrationResponse
}

func BuildServiceRegistrationRequest(cfg config.NodeConfig, bootstrap BootstrapState) (controlplane.ServiceRegistrationRequest, error) {
	registrations, err := service.BuildRegistrations(cfg)
	if err != nil {
		return controlplane.ServiceRegistrationRequest{}, err
	}

	return controlplane.ServiceRegistrationRequest{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		NodeName:        cfg.Identity.Name,
		Readiness:       BuildBootstrapSessionRequest(cfg, bootstrap).Readiness,
		Services:        registrations,
	}, nil
}

func AttemptServiceRegistration(ctx context.Context, cfg config.NodeConfig, bootstrap BootstrapState, session BootstrapSessionAttemptResult) (ServiceRegistrationAttemptResult, error) {
	if !session.Response.Accepted() {
		return ServiceRegistrationAttemptResult{}, fmt.Errorf("cannot attempt service registration because the bootstrap control session was not accepted")
	}
	if strings.TrimSpace(session.Endpoint) == "" {
		return ServiceRegistrationAttemptResult{}, fmt.Errorf("cannot attempt service registration without a coordinator endpoint")
	}

	request, err := BuildServiceRegistrationRequest(cfg, bootstrap)
	if err != nil {
		return ServiceRegistrationAttemptResult{}, fmt.Errorf("build service registration request: %w", err)
	}

	client := controlplane.Client{
		HTTPClient: &http.Client{Timeout: 3 * time.Second},
	}

	response, err := client.RegisterServices(ctx, session.Endpoint, request)
	if err != nil {
		return ServiceRegistrationAttemptResult{}, fmt.Errorf("service registration attempt to %q failed: %w", session.Endpoint, err)
	}

	return ServiceRegistrationAttemptResult{
		CoordinatorLabel: session.CoordinatorLabel,
		Endpoint:         session.Endpoint,
		Response:         response,
	}, nil
}

func (r ServiceRegistrationAttemptResult) ReportLines() []string {
	lines := make([]string, 0, len(r.Response.Results)+len(r.Response.Details)+4)

	if r.Endpoint != "" {
		lines = append(lines, fmt.Sprintf("node bootstrap service registration target: coordinator=%s endpoint=%s", r.CoordinatorLabel, r.Endpoint))
	}
	if r.Response.Outcome != "" {
		lines = append(lines, fmt.Sprintf("node bootstrap service registration outcome: %s (%s)", r.Response.Outcome, r.Response.Reason))
		lines = append(lines, fmt.Sprintf("node bootstrap service registration counts: accepted=%d rejected=%d", r.Response.AcceptedCount, r.Response.RejectedCount))
		for _, detail := range r.Response.Details {
			lines = append(lines, "node bootstrap service registration detail: "+detail)
		}
		for _, result := range r.Response.Results {
			lines = append(lines, fmt.Sprintf(
				"node bootstrap service result: service=%s type=%s outcome=%s reason=%s registry_key=%s",
				describeRegisteredService(result.ServiceName),
				describeRegisteredServiceType(result.ServiceType),
				result.Outcome,
				result.Reason,
				describeRegistryKey(result.RegistryKey),
			))
			for _, detail := range result.Details {
				lines = append(lines, "node bootstrap service detail: "+detail)
			}
		}
	}

	return lines
}

func describeRegisteredService(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "<unnamed-service>"
	}
	return name
}

func describeRegisteredServiceType(serviceType string) string {
	serviceType = strings.TrimSpace(serviceType)
	if serviceType == "" {
		return "<unknown-type>"
	}
	return serviceType
}

func describeRegistryKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "<none>"
	}
	return key
}
