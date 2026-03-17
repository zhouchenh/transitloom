package status

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// StatusServer serves read-only runtime status over a local HTTP endpoint.
//
// The server exposes a single GET /status endpoint returning text/plain.
// It is intentionally narrow: no mutation operations, no authentication,
// no remote management. It is designed for local operator inspection only
// and must be bound to a local-only address (e.g., 127.0.0.1:PORT).
//
// Separation of concerns: the server does not know about runtime types.
// The caller provides a Snapshot function that returns current status as
// text lines. This keeps the server package free of imports from
// internal/node or internal/coordinator.
//
// This is not a general admin API. tlctl queries this endpoint to surface
// runtime state that is otherwise only visible in process logs.
type StatusServer struct {
	// Snapshot is called on each request to produce current status lines.
	// Must be safe for concurrent use.
	Snapshot func() []string
}

// NewStatusServer creates a StatusServer backed by the given snapshot function.
func NewStatusServer(snapshot func() []string) *StatusServer {
	return &StatusServer{Snapshot: snapshot}
}

// ListenAndServe starts the status server on addr and blocks until ctx is
// cancelled or a fatal server error occurs.
//
// Returns nil when ctx is cancelled and the server shuts down gracefully.
// Returns an error if the server fails to start or encounters a fatal error.
func (s *StatusServer) ListenAndServe(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.handleStatus)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("status server: %w", err)
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-serverErr:
		return err
	}
}

// handleStatus handles GET /status requests, returning current runtime status.
func (s *StatusServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lines := s.Snapshot()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintln(w, strings.Join(lines, "\n"))
}
