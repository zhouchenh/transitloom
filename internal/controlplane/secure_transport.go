package controlplane

import "fmt"

// SecureControlMode describes the transport security mode for a control
// connection. It is explicit so that callers and observability output always
// know whether a control session is using the bootstrap-only HTTP placeholder
// or an authenticated TLS transport.
//
// The intended v1 control transport direction (spec/v1-control-plane.md §6):
//   - Primary:  QUIC + TLS 1.3 mTLS
//   - Fallback: TCP + TLS 1.3 mTLS
//
// Bootstrap-only HTTP exists as a placeholder during early implementation
// sequencing. It must be explicitly labeled to prevent it from being treated
// as a secure or authenticated session by callers or operators.
type SecureControlMode string

const (
	// SecureControlModeBootstrapOnlyHTTP is the current bootstrap placeholder.
	// It uses plain HTTP without TLS or transport-layer identity verification.
	// It exists only to support early implementation sequencing and must not
	// be treated as a secure transport.
	SecureControlModeBootstrapOnlyHTTP SecureControlMode = "bootstrap-only-http"

	// SecureControlModeTLSMTCPFallback is the TCP + TLS 1.3 mTLS fallback
	// control transport. Both coordinator and node present valid certificates
	// chained to the Transitloom root CA, verified at the transport layer.
	// This is the intended fallback transport per spec/v1-control-plane.md §6.2.
	SecureControlModeTLSMTCPFallback SecureControlMode = "tls-1.3-mtls-tcp-fallback"

	// SecureControlModeQUICMTLS is the QUIC + TLS 1.3 mTLS primary control
	// transport. Not yet implemented. Declared here so the intended primary
	// mode has a named constant for future implementation rather than an
	// invented string.
	SecureControlModeQUICMTLS SecureControlMode = "quic-tls-1.3-mtls-primary"
)

// SecureTransportStatus records the transport security mode for a control
// listener or session. It is used in observability output so operators can
// tell at a glance whether control interactions are using the bootstrap-only
// HTTP placeholder or the intended authenticated TLS transport.
type SecureTransportStatus struct {
	// Mode is the active transport security mode.
	Mode SecureControlMode

	// Authenticated indicates whether the transport provides mutual TLS
	// identity verification. False for bootstrap-only HTTP.
	Authenticated bool

	// Description is a human-readable explanation of the active mode,
	// including what is and is not yet implemented.
	Description string
}

// BootstrapOnlyTransportStatus returns the status for the current bootstrap
// HTTP transport. It explicitly declares that this is not the intended final
// transport and that identity verification is not provided.
//
// Every operator-facing log line that involves a bootstrap-only control
// session should use this to report the transport mode, so it is never
// ambiguous whether a session is authenticated or bootstrap-only.
func BootstrapOnlyTransportStatus() SecureTransportStatus {
	return SecureTransportStatus{
		Mode:          SecureControlModeBootstrapOnlyHTTP,
		Authenticated: false,
		Description: "bootstrap-only plain HTTP; no TLS, no transport-layer identity verification; " +
			"intended final transports (QUIC+TLS 1.3 mTLS primary, TCP+TLS 1.3 mTLS fallback) are not yet active",
	}
}

// TLSMTCPFallbackTransportStatus returns the status for an active TCP+TLS 1.3
// mTLS fallback control transport.
func TLSMTCPFallbackTransportStatus() SecureTransportStatus {
	return SecureTransportStatus{
		Mode:          SecureControlModeTLSMTCPFallback,
		Authenticated: true,
		Description: "TCP + TLS 1.3 mTLS fallback control transport; " +
			"coordinator and node identity mutually verified via Transitloom PKI certificate chain",
	}
}

// ReportLines returns human-readable lines describing the transport security
// status for operator logging. The output always makes the authentication
// state and mode explicit so operators can verify transport security at a glance.
func (s SecureTransportStatus) ReportLines() []string {
	authLabel := "not-authenticated"
	if s.Authenticated {
		authLabel = "mutually-authenticated"
	}
	return []string{
		fmt.Sprintf("control transport security mode: %s (%s)", s.Mode, authLabel),
		"control transport note: " + s.Description,
	}
}
