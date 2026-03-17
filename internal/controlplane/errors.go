package controlplane

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
)

// TransportErrorKind is a normalized category for transport-level failures.
// Callers use it to make retry/skip decisions without parsing raw error strings.
//
// These kinds apply to the current bootstrap-only HTTP transport. The same
// classification approach will apply when the QUIC+mTLS and TCP+TLS transports
// are implemented, but the specific error types encountered there may differ.
type TransportErrorKind string

const (
	// TransportErrorKindTimeout means the request or connection exceeded its
	// deadline. The endpoint may be overloaded or briefly unreachable. This
	// kind is considered retryable up to BootstrapRetryMaxAttempts.
	TransportErrorKindTimeout TransportErrorKind = "timeout"

	// TransportErrorKindConnectionRefused means the endpoint actively refused
	// the connection — the service is not listening. Immediate retry is not
	// useful; the endpoint should be skipped.
	TransportErrorKindConnectionRefused TransportErrorKind = "connection-refused"

	// TransportErrorKindContextCanceled means the caller's context was canceled
	// or its deadline expired before the operation completed. This is not a
	// transport error in the usual sense; it means the operation was
	// deliberately abandoned or the overall attempt budget was exhausted. Do
	// not retry.
	TransportErrorKindContextCanceled TransportErrorKind = "context-canceled"

	// TransportErrorKindProtocol means the transport layer succeeded but the
	// response was malformed, invalid, or semantically inconsistent. This
	// usually indicates a version mismatch or a bug in the coordinator
	// endpoint. Not retryable.
	TransportErrorKindProtocol TransportErrorKind = "protocol"

	// TransportErrorKindUnknown means the error did not match any recognized
	// transport failure category. Treat as non-retryable by default to avoid
	// masking unexpected failures.
	TransportErrorKindUnknown TransportErrorKind = "unknown"
)

// TransportError wraps a transport-level failure with endpoint context and a
// normalized error kind. Use ClassifyTransportError to produce values of this
// type from raw errors returned by the HTTP client.
//
// Having a structured error type rather than a raw error string makes it
// possible for callers to make informed retry/skip decisions without parsing
// error messages, and makes the reason for each failed attempt visible in logs
// and in BootstrapEndpointAttempt records.
type TransportError struct {
	Kind     TransportErrorKind
	Endpoint string
	Err      error
}

func (e TransportError) Error() string {
	return fmt.Sprintf("transport error (%s) to %q: %v", e.Kind, e.Endpoint, e.Err)
}

func (e TransportError) Unwrap() error { return e.Err }

// Retryable reports whether this transport error is worth retrying against the
// same endpoint. Only timeout errors are considered retryable; connection
// refusals and context cancellations are not because they indicate either a
// persistent endpoint problem or a deliberate abort.
func (e TransportError) Retryable() bool {
	return e.Kind == TransportErrorKindTimeout
}

// ClassifyTransportError wraps err with endpoint context and a normalized
// TransportErrorKind. err must not be nil.
//
// This function is the single place where raw Go network errors from the HTTP
// client are translated into the normalized kinds used by the rest of the
// bootstrap transport layer. Centralizing this classification makes it easy
// to extend when new transports (QUIC, TLS) are added.
func ClassifyTransportError(err error, endpoint string) TransportError {
	return TransportError{
		Kind:     classifyTransportErrorKind(err),
		Endpoint: endpoint,
		Err:      err,
	}
}

func classifyTransportErrorKind(err error) TransportErrorKind {
	// context.Canceled means the caller explicitly canceled the operation.
	// Check this before Timeout because some wrappers set both.
	if errors.Is(err, context.Canceled) {
		return TransportErrorKindContextCanceled
	}

	// context.DeadlineExceeded means the caller's own context expired. We
	// classify it as ContextCanceled rather than Timeout because the deadline
	// came from outside the transport layer — retrying the same endpoint under
	// the same context would also fail immediately.
	if errors.Is(err, context.DeadlineExceeded) {
		return TransportErrorKindContextCanceled
	}

	// net.Error.Timeout() covers both per-client timeouts and connection-level
	// deadlines set by the http.Client.
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return TransportErrorKindTimeout
	}

	// Connection refused: the endpoint is not listening. Use a typed check
	// first, then fall back to a string match for portability across platforms
	// and Go versions where the exact wrapping differs.
	if isConnectionRefused(err) {
		return TransportErrorKindConnectionRefused
	}

	return TransportErrorKindUnknown
}

// isConnectionRefused returns true if err represents a TCP connection refusal.
// The typed *net.OpError path is reliable on Linux and Darwin; the string
// fallback covers cases where the error is further wrapped.
func isConnectionRefused(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		// On Linux/Darwin, opErr.Err.Error() includes "connection refused"
		// for ECONNREFUSED. This is more portable than a syscall.ECONNREFUSED
		// comparison, which varies across platforms and build configurations.
		if strings.Contains(opErr.Err.Error(), "connection refused") {
			return true
		}
	}
	// String fallback for cases where the error is not directly a *net.OpError
	// (e.g., wrapped inside *url.Error by net/http).
	return strings.Contains(err.Error(), "connection refused")
}
