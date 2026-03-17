package controlplane_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/controlplane"
)

func TestClassifyTransportError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		wantKind controlplane.TransportErrorKind
		retryable bool
	}{
		{
			name:      "context canceled → ContextCanceled, not retryable",
			err:       context.Canceled,
			wantKind:  controlplane.TransportErrorKindContextCanceled,
			retryable: false,
		},
		{
			name:      "context deadline exceeded → ContextCanceled, not retryable",
			err:       context.DeadlineExceeded,
			wantKind:  controlplane.TransportErrorKindContextCanceled,
			retryable: false,
		},
		{
			name:      "net timeout error → Timeout, retryable",
			err:       &fakeNetError{timeout: true},
			wantKind:  controlplane.TransportErrorKindTimeout,
			retryable: true,
		},
		{
			name: "net.OpError with connection refused → ConnectionRefused, not retryable",
			err: &net.OpError{
				Op:  "dial",
				Net: "tcp",
				Err: errors.New("connection refused"),
			},
			wantKind:  controlplane.TransportErrorKindConnectionRefused,
			retryable: false,
		},
		{
			name:      "generic error → Unknown, not retryable",
			err:       errors.New("something went wrong"),
			wantKind:  controlplane.TransportErrorKindUnknown,
			retryable: false,
		},
		{
			name:      "context canceled wrapped → ContextCanceled",
			err:       fmt.Errorf("wrapped: %w", context.Canceled),
			wantKind:  controlplane.TransportErrorKindContextCanceled,
			retryable: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			te := controlplane.ClassifyTransportError(tt.err, "127.0.0.1:9999")

			if te.Kind != tt.wantKind {
				t.Errorf("Kind = %q, want %q", te.Kind, tt.wantKind)
			}
			if te.Retryable() != tt.retryable {
				t.Errorf("Retryable() = %t, want %t", te.Retryable(), tt.retryable)
			}
			if te.Endpoint != "127.0.0.1:9999" {
				t.Errorf("Endpoint = %q, want %q", te.Endpoint, "127.0.0.1:9999")
			}
			if te.Unwrap() != tt.err {
				t.Errorf("Unwrap() = %v, want %v", te.Unwrap(), tt.err)
			}
			if te.Error() == "" {
				t.Error("Error() returned empty string")
			}
		})
	}
}

func TestClassifyTransportErrorConnectionRefusedFromRealDial(t *testing.T) {
	t.Parallel()

	// Bind and close a listener so we have a port that actively refuses
	// connections. This verifies the classifier handles real net/http errors.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("ln.Close: %v", err)
	}

	// Attempt a dial against the closed port.
	d := net.Dialer{Timeout: 500 * time.Millisecond}
	_, dialErr := d.Dial("tcp", addr)
	if dialErr == nil {
		t.Skip("connection unexpectedly succeeded — port was re-used")
	}

	te := controlplane.ClassifyTransportError(dialErr, addr)
	if te.Kind != controlplane.TransportErrorKindConnectionRefused {
		t.Errorf("Kind = %q, want %q", te.Kind, controlplane.TransportErrorKindConnectionRefused)
	}
	if te.Retryable() {
		t.Error("Retryable() = true, want false for connection refused")
	}
}

// fakeNetError implements net.Error so we can test timeout classification
// without making a real network call.
type fakeNetError struct {
	timeout   bool
	temporary bool
}

func (e *fakeNetError) Error() string   { return "fake net error" }
func (e *fakeNetError) Timeout() bool   { return e.timeout }
func (e *fakeNetError) Temporary() bool { return e.temporary }
