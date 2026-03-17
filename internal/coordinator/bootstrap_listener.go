package coordinator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/pki"
	"github.com/zhouchenh/transitloom/internal/service"
)

// BootstrapListener exposes the first minimal node-facing bootstrap session
// endpoint. It intentionally rides only on the current TCP listener scaffolding
// so the code stays honest about QUIC+mTLS and final auth still being future
// work.
type BootstrapListener struct {
	coordinatorName string
	bootstrap       pki.CoordinatorBootstrapState
	registry        *ServiceRegistry
	associations    *AssociationStore
	listeners       []net.Listener
	servers         []*http.Server
}

func NewBootstrapListener(cfg config.CoordinatorConfig, bootstrap pki.CoordinatorBootstrapState) (*BootstrapListener, error) {
	if !cfg.Control.TCP.Enabled {
		return nil, fmt.Errorf("minimal bootstrap control session requires control.tcp.enabled=true until final QUIC+mTLS and TCP+TLS control transports are implemented")
	}
	if len(cfg.Control.TCP.ListenEndpoints) == 0 {
		return nil, fmt.Errorf("minimal bootstrap control session requires at least one control.tcp.listen_endpoints entry")
	}

	registry := NewServiceRegistry()
	associations := NewAssociationStore(registry)
	handler := newBootstrapControlHandler(cfg.Identity.Name, bootstrap, registry, associations)

	listener := &BootstrapListener{
		coordinatorName: cfg.Identity.Name,
		bootstrap:       bootstrap,
		registry:        registry,
		associations:    associations,
		listeners:       make([]net.Listener, 0, len(cfg.Control.TCP.ListenEndpoints)),
		servers:         make([]*http.Server, 0, len(cfg.Control.TCP.ListenEndpoints)),
	}

	for _, endpoint := range cfg.Control.TCP.ListenEndpoints {
		ln, err := net.Listen("tcp", endpoint)
		if err != nil {
			listener.closeListeners()
			return nil, fmt.Errorf("bind minimal bootstrap control listener on %q: %w", endpoint, err)
		}

		server := &http.Server{
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
		}

		listener.listeners = append(listener.listeners, ln)
		listener.servers = append(listener.servers, server)
	}

	return listener, nil
}

func (l *BootstrapListener) BoundEndpoints() []string {
	endpoints := make([]string, 0, len(l.listeners))
	for _, ln := range l.listeners {
		endpoints = append(endpoints, ln.Addr().String())
	}
	return endpoints
}

func (l *BootstrapListener) ReportLines() []string {
	lines := make([]string, 0, len(l.listeners)+4)
	for _, endpoint := range l.BoundEndpoints() {
		lines = append(lines, fmt.Sprintf("coordinator bootstrap control listener: http://%s%s", endpoint, controlplane.BootstrapSessionPath))
		lines = append(lines, fmt.Sprintf("coordinator bootstrap service registration listener: http://%s%s", endpoint, controlplane.ServiceRegistrationPath))
		lines = append(lines, fmt.Sprintf("coordinator bootstrap association listener: http://%s%s", endpoint, controlplane.AssociationPath))
	}
	lines = append(lines,
		"coordinator bootstrap control note: this endpoint exchanges only bootstrap-readiness snapshots and structured placeholder results",
		"coordinator bootstrap service note: registered services remain bootstrap-only placeholder state and do not imply discovery or association authorization",
		"coordinator bootstrap association note: association records are logical connectivity placeholders only; they do not imply path selection, relay eligibility, or forwarding-state installation",
		"coordinator bootstrap control note: final QUIC+mTLS/TCP+TLS control sessions and live certificate/admission validation are not implemented yet",
	)
	return lines
}

func (l *BootstrapListener) RegistrySnapshot() []service.Record {
	if l.registry == nil {
		return nil
	}
	return l.registry.Snapshot()
}

func (l *BootstrapListener) AssociationSnapshot() []service.AssociationRecord {
	if l.associations == nil {
		return nil
	}
	return l.associations.Snapshot()
}

func (l *BootstrapListener) Run(ctx context.Context) error {
	if len(l.listeners) == 0 {
		return fmt.Errorf("no bootstrap listeners are configured")
	}

	errCh := make(chan error, len(l.listeners))
	for i, server := range l.servers {
		server := server
		ln := l.listeners[i]
		go func() {
			if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("serve bootstrap control listener on %q: %w", ln.Addr().String(), err)
			}
		}()
	}

	select {
	case <-ctx.Done():
		return l.shutdown()
	case err := <-errCh:
		_ = l.shutdown()
		return err
	}
}

func newBootstrapControlHandler(coordinatorName string, bootstrap pki.CoordinatorBootstrapState, registry *ServiceRegistry, associations *AssociationStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(controlplane.BootstrapSessionPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		request, err := controlplane.DecodeBootstrapSessionRequest(r.Body)
		if err != nil {
			response := bootstrapSessionResponse(coordinatorName, controlplane.BootstrapSessionOutcomeInvalidRequest, controlplane.BootstrapSessionReasonInvalidReadiness,
				fmt.Sprintf("invalid bootstrap session request: %v", err),
				"bootstrap-only control sessions currently validate only request shape plus local readiness summaries",
			)
			if errors.Is(err, context.Canceled) {
				return
			}
			if writeErr := controlplane.WriteBootstrapSessionResponse(w, http.StatusBadRequest, response); writeErr != nil {
				http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			}
			return
		}

		response, statusCode := evaluateBootstrapSession(coordinatorName, bootstrap, request)
		if err := controlplane.WriteBootstrapSessionResponse(w, statusCode, response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc(controlplane.ServiceRegistrationPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		request, err := controlplane.DecodeServiceRegistrationRequest(r.Body)
		if err != nil {
			response := serviceRegistrationResponse(
				coordinatorName,
				controlplane.ServiceRegistrationOutcomeInvalidRequest,
				controlplane.ServiceRegistrationReasonInvalidRequest,
				0,
				0,
				nil,
				fmt.Sprintf("invalid bootstrap service registration request: %v", err),
				"bootstrap-only service registration currently validates request shape plus explicit service declarations",
			)
			if errors.Is(err, context.Canceled) {
				return
			}
			if writeErr := controlplane.WriteServiceRegistrationResponse(w, http.StatusBadRequest, response); writeErr != nil {
				http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			}
			return
		}

		response, statusCode := evaluateServiceRegistration(coordinatorName, bootstrap, registry, request)
		if err := controlplane.WriteServiceRegistrationResponse(w, statusCode, response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc(controlplane.AssociationPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		request, err := controlplane.DecodeAssociationRequest(r.Body)
		if err != nil {
			response := associationResponse(
				coordinatorName,
				controlplane.AssociationOutcomeInvalidRequest,
				controlplane.AssociationReasonInvalidRequest,
				0,
				0,
				nil,
				fmt.Sprintf("invalid bootstrap association request: %v", err),
				"bootstrap-only association currently validates request shape plus registered service existence",
			)
			if errors.Is(err, context.Canceled) {
				return
			}
			if writeErr := controlplane.WriteAssociationResponse(w, http.StatusBadRequest, response); writeErr != nil {
				http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			}
			return
		}

		response, statusCode := evaluateAssociation(coordinatorName, bootstrap, associations, request)
		if err := controlplane.WriteAssociationResponse(w, statusCode, response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	return mux
}

func evaluateBootstrapSession(coordinatorName string, bootstrap pki.CoordinatorBootstrapState, request controlplane.BootstrapSessionRequest) (controlplane.BootstrapSessionResponse, int) {
	if request.ProtocolVersion != controlplane.BootstrapProtocolVersion {
		return bootstrapSessionResponse(
			coordinatorName,
			controlplane.BootstrapSessionOutcomeInvalidRequest,
			controlplane.BootstrapSessionReasonUnsupportedProtocol,
			fmt.Sprintf("unsupported bootstrap protocol version %q", request.ProtocolVersion),
			fmt.Sprintf("expected protocol version %q", controlplane.BootstrapProtocolVersion),
		), http.StatusBadRequest
	}

	evaluation := evaluateBootstrapGate(bootstrap, request.NodeName, request.Readiness)
	if evaluation.accepted {
		return bootstrapSessionResponse(
			coordinatorName,
			controlplane.BootstrapSessionOutcomeAccepted,
			controlplane.BootstrapSessionReasonPrerequisitesSatisfied,
			evaluation.details...,
		), evaluation.statusCode
	}

	outcome := controlplane.BootstrapSessionOutcomeRejected
	if evaluation.statusCode == http.StatusBadRequest {
		outcome = controlplane.BootstrapSessionOutcomeInvalidRequest
	}
	return bootstrapSessionResponse(coordinatorName, outcome, evaluation.reason, evaluation.details...), evaluation.statusCode
}

func bootstrapSessionResponse(coordinatorName string, outcome controlplane.BootstrapSessionOutcome, reason controlplane.BootstrapSessionReason, details ...string) controlplane.BootstrapSessionResponse {
	trimmedDetails := make([]string, 0, len(details)+1)
	for _, detail := range details {
		detail = strings.TrimSpace(detail)
		if detail == "" {
			continue
		}
		trimmedDetails = append(trimmedDetails, detail)
	}
	trimmedDetails = append(trimmedDetails, "this is a bootstrap-only control result, not proof of a normal authenticated or currently admitted Transitloom session")

	return controlplane.BootstrapSessionResponse{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		CoordinatorName: coordinatorName,
		Outcome:         outcome,
		Reason:          reason,
		BootstrapOnly:   true,
		Details:         trimmedDetails,
	}
}

func evaluateServiceRegistration(coordinatorName string, bootstrap pki.CoordinatorBootstrapState, registry *ServiceRegistry, request controlplane.ServiceRegistrationRequest) (controlplane.ServiceRegistrationResponse, int) {
	evaluation := evaluateBootstrapGate(bootstrap, request.NodeName, request.Readiness)
	if !evaluation.accepted {
		reason := controlplane.ServiceRegistrationReasonBootstrapPrerequisitesNotMet
		if evaluation.reason == controlplane.BootstrapSessionReasonCoordinatorAwaitingMaterial {
			reason = controlplane.ServiceRegistrationReasonCoordinatorAwaitingMaterial
		}
		details := append([]string(nil), evaluation.details...)
		details = append(details, "bootstrap-only service registration requires the same bootstrap prerequisites as the minimal node-to-coordinator control session")
		return serviceRegistrationResponse(
			coordinatorName,
			controlplane.ServiceRegistrationOutcomeRejected,
			reason,
			0,
			0,
			nil,
			details...,
		), evaluation.statusCode
	}

	results := registry.Apply(request.NodeName, request.Services, time.Now().UTC())
	acceptedCount, rejectedCount := countServiceRegistrationResults(results)

	outcome := controlplane.ServiceRegistrationOutcomeAccepted
	reason := controlplane.ServiceRegistrationReasonRegistered
	switch {
	case acceptedCount == 0:
		outcome = controlplane.ServiceRegistrationOutcomeRejected
		reason = controlplane.ServiceRegistrationReasonNoServicesRegistered
	case rejectedCount > 0:
		outcome = controlplane.ServiceRegistrationOutcomePartial
		reason = controlplane.ServiceRegistrationReasonPartiallyRegistered
	}

	return serviceRegistrationResponse(
		coordinatorName,
		outcome,
		reason,
		acceptedCount,
		rejectedCount,
		results,
		fmt.Sprintf("processed %d service declaration(s) from node %q", len(request.Services), request.NodeName),
		"bootstrap-only service registration stores placeholder coordinator records only; it does not imply discovery, association authorization, or authenticated service ownership",
	), http.StatusOK
}

func serviceRegistrationResponse(coordinatorName string, outcome controlplane.ServiceRegistrationOutcome, reason controlplane.ServiceRegistrationReason, acceptedCount, rejectedCount int, results []controlplane.ServiceRegistrationResult, details ...string) controlplane.ServiceRegistrationResponse {
	trimmedDetails := make([]string, 0, len(details)+1)
	for _, detail := range details {
		detail = strings.TrimSpace(detail)
		if detail == "" {
			continue
		}
		trimmedDetails = append(trimmedDetails, detail)
	}
	trimmedDetails = append(trimmedDetails, "this is a bootstrap-only service registration result, not proof of authenticated service ownership, discovery completeness, or association authorization")

	return controlplane.ServiceRegistrationResponse{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		CoordinatorName: coordinatorName,
		Outcome:         outcome,
		Reason:          reason,
		BootstrapOnly:   true,
		AcceptedCount:   acceptedCount,
		RejectedCount:   rejectedCount,
		Results:         results,
		Details:         trimmedDetails,
	}
}

func evaluateAssociation(coordinatorName string, bootstrap pki.CoordinatorBootstrapState, associations *AssociationStore, request controlplane.AssociationRequest) (controlplane.AssociationResponse, int) {
	evaluation := evaluateBootstrapGate(bootstrap, request.NodeName, request.Readiness)
	if !evaluation.accepted {
		reason := controlplane.AssociationReasonBootstrapPrerequisitesNotMet
		if evaluation.reason == controlplane.BootstrapSessionReasonCoordinatorAwaitingMaterial {
			reason = controlplane.AssociationReasonCoordinatorAwaitingMaterial
		}
		details := append([]string(nil), evaluation.details...)
		details = append(details, "bootstrap-only association requires the same bootstrap prerequisites as the minimal node-to-coordinator control session")
		return associationResponse(
			coordinatorName,
			controlplane.AssociationOutcomeRejected,
			reason,
			0,
			0,
			nil,
			details...,
		), evaluation.statusCode
	}

	results := associations.Apply(request.NodeName, request.Associations, time.Now().UTC())
	acceptedCount, rejectedCount := countAssociationResults(results)

	outcome := controlplane.AssociationOutcomeAccepted
	reason := controlplane.AssociationReasonCreated
	switch {
	case acceptedCount == 0:
		outcome = controlplane.AssociationOutcomeRejected
		reason = controlplane.AssociationReasonNoAssociationsCreated
	case rejectedCount > 0:
		outcome = controlplane.AssociationOutcomePartial
		reason = controlplane.AssociationReasonPartiallyCreated
	}

	return associationResponse(
		coordinatorName,
		outcome,
		reason,
		acceptedCount,
		rejectedCount,
		results,
		fmt.Sprintf("processed %d association intent(s) from node %q", len(request.Associations), request.NodeName),
		"bootstrap-only association stores placeholder coordinator records only; it does not imply path selection, relay eligibility, forwarding-state installation, or that traffic can already flow",
	), http.StatusOK
}

func associationResponse(coordinatorName string, outcome controlplane.AssociationOutcome, reason controlplane.AssociationReason, acceptedCount, rejectedCount int, results []controlplane.AssociationResult, details ...string) controlplane.AssociationResponse {
	trimmedDetails := make([]string, 0, len(details)+1)
	for _, detail := range details {
		detail = strings.TrimSpace(detail)
		if detail == "" {
			continue
		}
		trimmedDetails = append(trimmedDetails, detail)
	}
	trimmedDetails = append(trimmedDetails, "this is a bootstrap-only association result, not proof of authenticated authorization, path selection, or forwarding-state readiness")

	return controlplane.AssociationResponse{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		CoordinatorName: coordinatorName,
		Outcome:         outcome,
		Reason:          reason,
		BootstrapOnly:   true,
		AcceptedCount:   acceptedCount,
		RejectedCount:   rejectedCount,
		Results:         results,
		Details:         trimmedDetails,
	}
}

func countAssociationResults(results []controlplane.AssociationResult) (accepted, rejected int) {
	for _, result := range results {
		switch result.Outcome {
		case controlplane.AssociationResultOutcomeCreated:
			accepted++
		case controlplane.AssociationResultOutcomeRejected:
			rejected++
		}
	}
	return accepted, rejected
}

type bootstrapGateEvaluation struct {
	accepted   bool
	reason     controlplane.BootstrapSessionReason
	details    []string
	statusCode int
}

func evaluateBootstrapGate(bootstrap pki.CoordinatorBootstrapState, nodeName string, readiness controlplane.BootstrapReadinessSummary) bootstrapGateEvaluation {
	if bootstrap.Phase != pki.CoordinatorBootstrapPhaseReady {
		return bootstrapGateEvaluation{
			reason: controlplane.BootstrapSessionReasonCoordinatorAwaitingMaterial,
			details: []string{
				fmt.Sprintf("coordinator bootstrap phase is %q", bootstrap.Phase),
				"coordinator can accept bootstrap contact but cannot claim a stronger control-session state until intermediate material exists",
			},
			statusCode: http.StatusOK,
		}
	}

	switch readiness.OverallPhase {
	case controlplane.ReadinessPhaseReady:
		return bootstrapGateEvaluation{
			accepted: true,
			reason:   controlplane.BootstrapSessionReasonPrerequisitesSatisfied,
			details: []string{
				fmt.Sprintf("node %q reported bootstrap phase %q", nodeName, readiness.OverallPhase),
				fmt.Sprintf("node readiness detail: identity_phase=%s admission_phase=%s", readiness.IdentityPhase, readiness.AdmissionPhase),
				"bootstrap prerequisites are satisfied, but live certificate validation and live admission enforcement are still not implemented",
			},
			statusCode: http.StatusOK,
		}
	case controlplane.ReadinessPhaseIdentityBootstrapRequired:
		return bootstrapGateEvaluation{
			reason: controlplane.BootstrapSessionReasonNodeIdentityBootstrap,
			details: []string{
				fmt.Sprintf("node %q reported bootstrap phase %q", nodeName, readiness.OverallPhase),
				"node still needs local identity bootstrap before a later authenticated control session can make sense",
			},
			statusCode: http.StatusOK,
		}
	case controlplane.ReadinessPhaseAwaitingCertificate:
		return bootstrapGateEvaluation{
			reason: controlplane.BootstrapSessionReasonNodeAwaitingCertificate,
			details: []string{
				fmt.Sprintf("node %q reported bootstrap phase %q", nodeName, readiness.OverallPhase),
				"node has local key material but still needs certificate issuance before normal participation can be evaluated",
			},
			statusCode: http.StatusOK,
		}
	case controlplane.ReadinessPhaseAdmissionMissing:
		return bootstrapGateEvaluation{
			reason: controlplane.BootstrapSessionReasonNodeAdmissionMissing,
			details: []string{
				fmt.Sprintf("node %q reported bootstrap phase %q", nodeName, readiness.OverallPhase),
				"cached token presence is only a bootstrap signal here, but missing token readiness still blocks bootstrap-level success in this task",
			},
			statusCode: http.StatusOK,
		}
	case controlplane.ReadinessPhaseAdmissionExpired:
		return bootstrapGateEvaluation{
			reason: controlplane.BootstrapSessionReasonNodeAdmissionExpired,
			details: []string{
				fmt.Sprintf("node %q reported bootstrap phase %q", nodeName, readiness.OverallPhase),
				"cached token expiry does not prove revoke, but it does mean bootstrap-level prerequisites are not currently satisfied",
			},
			statusCode: http.StatusOK,
		}
	default:
		return bootstrapGateEvaluation{
			reason: controlplane.BootstrapSessionReasonInvalidReadiness,
			details: []string{
				fmt.Sprintf("node %q reported unknown bootstrap phase %q", nodeName, readiness.OverallPhase),
				"bootstrap-only control sessions require an explicit known readiness phase",
			},
			statusCode: http.StatusBadRequest,
		}
	}
}

func countServiceRegistrationResults(results []controlplane.ServiceRegistrationResult) (accepted, rejected int) {
	for _, result := range results {
		switch result.Outcome {
		case controlplane.ServiceRegistrationResultOutcomeRegistered:
			accepted++
		case controlplane.ServiceRegistrationResultOutcomeRejected:
			rejected++
		}
	}
	return accepted, rejected
}

func (l *BootstrapListener) shutdown() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var firstErr error
	for _, server := range l.servers {
		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) && firstErr == nil {
			firstErr = err
		}
	}

	l.closeListeners()
	return firstErr
}

func (l *BootstrapListener) closeListeners() {
	for _, ln := range l.listeners {
		_ = ln.Close()
	}
}
