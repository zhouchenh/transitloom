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
)

// BootstrapListener exposes the first minimal node-facing bootstrap session
// endpoint. It intentionally rides only on the current TCP listener scaffolding
// so the code stays honest about QUIC+mTLS and final auth still being future
// work.
type BootstrapListener struct {
	coordinatorName string
	bootstrap       pki.CoordinatorBootstrapState
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

	handler := newBootstrapSessionHandler(cfg.Identity.Name, bootstrap)

	listener := &BootstrapListener{
		coordinatorName: cfg.Identity.Name,
		bootstrap:       bootstrap,
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
	lines := make([]string, 0, len(l.listeners)+2)
	for _, endpoint := range l.BoundEndpoints() {
		lines = append(lines, fmt.Sprintf("coordinator bootstrap control listener: http://%s%s", endpoint, controlplane.BootstrapSessionPath))
	}
	lines = append(lines,
		"coordinator bootstrap control note: this endpoint exchanges only bootstrap-readiness snapshots and structured placeholder results",
		"coordinator bootstrap control note: final QUIC+mTLS/TCP+TLS control sessions and live certificate/admission validation are not implemented yet",
	)
	return lines
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

func newBootstrapSessionHandler(coordinatorName string, bootstrap pki.CoordinatorBootstrapState) http.Handler {
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

	if bootstrap.Phase != pki.CoordinatorBootstrapPhaseReady {
		return bootstrapSessionResponse(
			coordinatorName,
			controlplane.BootstrapSessionOutcomeRejected,
			controlplane.BootstrapSessionReasonCoordinatorAwaitingMaterial,
			fmt.Sprintf("coordinator bootstrap phase is %q", bootstrap.Phase),
			"coordinator can accept bootstrap contact but cannot claim a stronger control-session state until intermediate material exists",
		), http.StatusOK
	}

	switch request.Readiness.OverallPhase {
	case controlplane.ReadinessPhaseReady:
		return bootstrapSessionResponse(
			coordinatorName,
			controlplane.BootstrapSessionOutcomeAccepted,
			controlplane.BootstrapSessionReasonPrerequisitesSatisfied,
			fmt.Sprintf("node %q reported bootstrap phase %q", request.NodeName, request.Readiness.OverallPhase),
			fmt.Sprintf("node readiness detail: identity_phase=%s admission_phase=%s", request.Readiness.IdentityPhase, request.Readiness.AdmissionPhase),
			"bootstrap prerequisites are satisfied, but live certificate validation and live admission enforcement are still not implemented",
		), http.StatusOK
	case controlplane.ReadinessPhaseIdentityBootstrapRequired:
		return bootstrapSessionResponse(
			coordinatorName,
			controlplane.BootstrapSessionOutcomeRejected,
			controlplane.BootstrapSessionReasonNodeIdentityBootstrap,
			fmt.Sprintf("node %q reported bootstrap phase %q", request.NodeName, request.Readiness.OverallPhase),
			"node still needs local identity bootstrap before a later authenticated control session can make sense",
		), http.StatusOK
	case controlplane.ReadinessPhaseAwaitingCertificate:
		return bootstrapSessionResponse(
			coordinatorName,
			controlplane.BootstrapSessionOutcomeRejected,
			controlplane.BootstrapSessionReasonNodeAwaitingCertificate,
			fmt.Sprintf("node %q reported bootstrap phase %q", request.NodeName, request.Readiness.OverallPhase),
			"node has local key material but still needs certificate issuance before normal participation can be evaluated",
		), http.StatusOK
	case controlplane.ReadinessPhaseAdmissionMissing:
		return bootstrapSessionResponse(
			coordinatorName,
			controlplane.BootstrapSessionOutcomeRejected,
			controlplane.BootstrapSessionReasonNodeAdmissionMissing,
			fmt.Sprintf("node %q reported bootstrap phase %q", request.NodeName, request.Readiness.OverallPhase),
			"cached token presence is only a bootstrap signal here, but missing token readiness still blocks bootstrap-level success in this task",
		), http.StatusOK
	case controlplane.ReadinessPhaseAdmissionExpired:
		return bootstrapSessionResponse(
			coordinatorName,
			controlplane.BootstrapSessionOutcomeRejected,
			controlplane.BootstrapSessionReasonNodeAdmissionExpired,
			fmt.Sprintf("node %q reported bootstrap phase %q", request.NodeName, request.Readiness.OverallPhase),
			"cached token expiry does not prove revoke, but it does mean bootstrap-level prerequisites are not currently satisfied",
		), http.StatusOK
	default:
		return bootstrapSessionResponse(
			coordinatorName,
			controlplane.BootstrapSessionOutcomeInvalidRequest,
			controlplane.BootstrapSessionReasonInvalidReadiness,
			fmt.Sprintf("node %q reported unknown bootstrap phase %q", request.NodeName, request.Readiness.OverallPhase),
			"bootstrap-only control sessions require an explicit known readiness phase",
		), http.StatusBadRequest
	}
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
