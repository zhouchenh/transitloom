package coordinator

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/pki"
	"github.com/zhouchenh/transitloom/internal/service"
	"github.com/zhouchenh/transitloom/internal/status"
)

// SecureControlListener exposes the control session endpoints over TCP+TLS 1.3
// mTLS. It uses the same application-layer handlers as BootstrapListener but
// wraps each TCP listener with TLS so that coordinator and node identity are
// mutually verified at the transport layer before any application data is
// exchanged.
//
// This is the intended TCP+TLS 1.3 fallback transport described in
// spec/v1-control-plane.md section 6.2. The QUIC+TLS 1.3 mTLS primary
// transport (spec section 6.1) will use the same certificate material with a
// different transport wrapper when QUIC is implemented.
//
// SecureControlListener is intentionally a separate type from BootstrapListener.
// BootstrapListener uses plain HTTP with no transport-layer identity
// verification. When both exist in the codebase, the distinction is explicit
// at the type level, not hidden in a flag or runtime mode switch. Operators
// can see from report lines and transport status which listener is which.
//
// Application-layer admission-token validation and live certificate chain
// revocation checks are not yet implemented. The transport-layer mTLS
// verification here is the first meaningful step beyond the bootstrap-only
// HTTP placeholder.
type SecureControlListener struct {
	coordinatorName string
	bootstrap       pki.CoordinatorBootstrapState
	registry        *ServiceRegistry
	associations    *AssociationStore
	relayCfg        config.CoordinatorRelayConfig
	tlsConfig       *tls.Config
	transportStatus controlplane.SecureTransportStatus
	listeners       []net.Listener
	servers         []*http.Server
}

// NewSecureControlListener creates a TLS-wrapped control listener. It binds
// TCP sockets on the endpoints configured in cfg.Control.TCP and wraps each
// with the provided TLS config, which must enforce TLS 1.3 mTLS.
//
// The same application-layer control handlers as BootstrapListener are used
// (control session, service registration, association, path candidates) but
// all connections are encrypted and mutually authenticated via the TLS layer.
//
// tlsConfig must be built using pki.BuildCoordinatorTLSConfig to ensure the
// TLS 1.3 minimum and mTLS (RequireAndVerifyClientCert) requirements are
// enforced. Passing nil returns an error.
func NewSecureControlListener(cfg config.CoordinatorConfig, bootstrap pki.CoordinatorBootstrapState, tlsConfig *tls.Config) (*SecureControlListener, error) {
	if tlsConfig == nil {
		return nil, fmt.Errorf("secure control listener requires a non-nil TLS config; use pki.BuildCoordinatorTLSConfig to build it")
	}
	if !cfg.Control.TCP.Enabled {
		return nil, fmt.Errorf("secure control listener requires control.tcp.enabled=true")
	}
	if len(cfg.Control.TCP.ListenEndpoints) == 0 {
		return nil, fmt.Errorf("secure control listener requires at least one control.tcp.listen_endpoints entry")
	}

	registry := NewServiceRegistry()
	associations := NewAssociationStore(registry)
	handler := newBootstrapControlHandler(cfg.Identity.Name, bootstrap, registry, associations, cfg.Relay)

	listener := &SecureControlListener{
		coordinatorName: cfg.Identity.Name,
		bootstrap:       bootstrap,
		registry:        registry,
		associations:    associations,
		relayCfg:        cfg.Relay,
		tlsConfig:       tlsConfig,
		transportStatus: controlplane.TLSMTCPFallbackTransportStatus(),
		listeners:       make([]net.Listener, 0, len(cfg.Control.TCP.ListenEndpoints)),
		servers:         make([]*http.Server, 0, len(cfg.Control.TCP.ListenEndpoints)),
	}

	for _, endpoint := range cfg.Control.TCP.ListenEndpoints {
		tcpLn, err := net.Listen("tcp", endpoint)
		if err != nil {
			listener.closeListeners()
			return nil, fmt.Errorf("bind secure control listener on %q: %w", endpoint, err)
		}
		// Wrap the TCP listener with TLS. All connections through this
		// listener undergo the TLS 1.3 handshake with mutual authentication
		// before any HTTP request or application data is exchanged.
		tlsLn := tls.NewListener(tcpLn, tlsConfig)

		server := &http.Server{
			Handler:           handler,
			ReadHeaderTimeout: controlplane.BootstrapConnectTimeout,
			ReadTimeout:       controlplane.BootstrapServerReadTimeout,
			WriteTimeout:      controlplane.BootstrapServerWriteTimeout,
			IdleTimeout:       controlplane.BootstrapServerIdleTimeout,
			MaxHeaderBytes:    controlplane.BootstrapServerMaxHeaderBytes,
		}

		listener.listeners = append(listener.listeners, tlsLn)
		listener.servers = append(listener.servers, server)
	}

	return listener, nil
}

// TransportStatus returns the transport security status for this listener.
// It always reports SecureControlModeTLSMTCPFallback, distinguishing it from
// BootstrapListener which would report SecureControlModeBootstrapOnlyHTTP.
func (l *SecureControlListener) TransportStatus() controlplane.SecureTransportStatus {
	return l.transportStatus
}

// BoundEndpoints returns the host:port strings of the bound TLS listeners.
func (l *SecureControlListener) BoundEndpoints() []string {
	endpoints := make([]string, 0, len(l.listeners))
	for _, ln := range l.listeners {
		endpoints = append(endpoints, ln.Addr().String())
	}
	return endpoints
}

// ReportLines returns human-readable status lines for operator logging.
// All lines explicitly identify the transport as TLS 1.3 mTLS, and include
// the honest note that application-layer admission enforcement is not yet
// implemented.
func (l *SecureControlListener) ReportLines() []string {
	lines := make([]string, 0, len(l.listeners)*4+8)
	for _, endpoint := range l.BoundEndpoints() {
		lines = append(lines,
			fmt.Sprintf("coordinator secure control listener (TLS 1.3 mTLS): %s%s", endpoint, controlplane.BootstrapSessionPath),
			fmt.Sprintf("coordinator secure service registration listener (TLS 1.3 mTLS): %s%s", endpoint, controlplane.ServiceRegistrationPath),
			fmt.Sprintf("coordinator secure association listener (TLS 1.3 mTLS): %s%s", endpoint, controlplane.AssociationPath),
			fmt.Sprintf("coordinator secure path-candidates listener (TLS 1.3 mTLS): %s%s", endpoint, controlplane.PathCandidatePath),
		)
	}
	lines = append(lines, l.transportStatus.ReportLines()...)
	lines = append(lines,
		"secure control note: connections without a valid Transitloom node certificate are rejected at the TLS layer",
		"secure control note: application-layer admission-token enforcement and live certificate chain validation are not yet implemented",
	)
	return lines
}

// RegistrySnapshot returns a snapshot of the current service registry state.
func (l *SecureControlListener) RegistrySnapshot() []service.Record {
	if l.registry == nil {
		return nil
	}
	return l.registry.Snapshot()
}

// AssociationSnapshot returns a snapshot of the current association store state.
func (l *SecureControlListener) AssociationSnapshot() []service.AssociationRecord {
	if l.associations == nil {
		return nil
	}
	return l.associations.Snapshot()
}

// RuntimeSummaryLines returns human-readable summary lines covering the
// coordinator's current runtime state: registered services and associations.
func (l *SecureControlListener) RuntimeSummaryLines() []string {
	registrySummary := status.MakeServiceRegistrySummary(l.registry.Snapshot())
	associationSummary := status.MakeAssociationStoreSummary(l.associations.Snapshot())
	lines := registrySummary.ReportLines()
	lines = append(lines, associationSummary.ReportLines()...)
	return lines
}

// Run starts the secure control servers and blocks until the context is
// canceled or a server error occurs.
func (l *SecureControlListener) Run(ctx context.Context) error {
	if len(l.listeners) == 0 {
		return fmt.Errorf("no secure control listeners are configured")
	}

	errCh := make(chan error, len(l.listeners))
	for i, server := range l.servers {
		server := server
		ln := l.listeners[i]
		go func() {
			if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("serve secure control listener on %q: %w", ln.Addr().String(), err)
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

func (l *SecureControlListener) shutdown() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), controlplane.BootstrapServerShutdownTimeout)
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

func (l *SecureControlListener) closeListeners() {
	for _, ln := range l.listeners {
		_ = ln.Close()
	}
}
