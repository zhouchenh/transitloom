package controlplane

import "time"

// Transport timeout and retry constants for bootstrap control interactions.
//
// These values are explicit named constants rather than scattered magic numbers
// so that the rationale for each choice is visible in one place, and so that
// future changes (e.g. when QUIC+mTLS replaces the current HTTP scaffolding)
// can be made in a single reviewed location.
//
// These constants apply to the current bootstrap-only HTTP transport. They are
// not claims about the final QUIC+TLS 1.3 mTLS control transport timeouts,
// which will be defined separately when that transport is implemented.
const (
	// BootstrapConnectTimeout is the per-attempt deadline for a bootstrap
	// control HTTP request from the first connection byte to the last response
	// byte. 3 seconds is deliberately tight: a slow or unreachable endpoint
	// should not block the node from falling through to the next coordinator
	// endpoint quickly.
	BootstrapConnectTimeout = 3 * time.Second

	// BootstrapRetryInitialBackoff is the initial wait before retrying a
	// transient transport failure against the same endpoint. Starting at 250 ms
	// gives a brief recovery window without adding perceptible startup latency
	// when all endpoints are healthy.
	BootstrapRetryInitialBackoff = 250 * time.Millisecond

	// BootstrapRetryMaxBackoff caps the per-endpoint backoff so node startup
	// does not stall excessively when a single endpoint is flapping. 2 seconds
	// is enough to absorb brief transients without hiding persistent failures.
	BootstrapRetryMaxBackoff = 2 * time.Second

	// BootstrapRetryMaxAttempts limits per-endpoint retry count for retryable
	// (timeout) transport failures. Three attempts is enough to absorb a brief
	// transient without masking persistent endpoint problems. Non-retryable
	// failures (connection refused, context canceled) always stop immediately.
	BootstrapRetryMaxAttempts = 3

	// BootstrapServerReadTimeout bounds the time from TCP connection accept to
	// the request body being fully read. 10 seconds guards against slow-loris
	// style connection exhaustion at the bootstrap control endpoint while
	// remaining generous enough for legitimate slow clients.
	BootstrapServerReadTimeout = 10 * time.Second

	// BootstrapServerWriteTimeout bounds the time from request read completion
	// to response being fully written. Bootstrap responses are small JSON
	// objects, so 10 seconds is a generous upper bound.
	BootstrapServerWriteTimeout = 10 * time.Second

	// BootstrapServerIdleTimeout bounds how long the server keeps an idle
	// keep-alive connection open. Bootstrap interactions are quick one-shot
	// exchanges; long-lived idle connections are not useful here.
	BootstrapServerIdleTimeout = 30 * time.Second

	// BootstrapServerShutdownTimeout is the maximum time allowed for a graceful
	// HTTP server shutdown. In-flight bootstrap requests should complete well
	// within this window; if they have not, the server closes anyway.
	BootstrapServerShutdownTimeout = 5 * time.Second

	// BootstrapMaxRequestBodyBytes limits incoming request body size at
	// bootstrap control endpoints. Bootstrap requests carry only short JSON
	// summaries, so 64 KiB is generous while guarding against runaway
	// allocations from unexpectedly large payloads.
	BootstrapMaxRequestBodyBytes = 64 * 1024

	// BootstrapServerMaxHeaderBytes limits incoming request header size.
	// Bootstrap requests carry only minimal headers, so 16 KiB is more than
	// sufficient and tighter than the Go http.Server default of 1 MiB.
	BootstrapServerMaxHeaderBytes = 16 * 1024
)
